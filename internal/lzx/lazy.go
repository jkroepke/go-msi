package lzx

const (
	chainHashBits = 18
	hash3Bits     = 15
	// goodMatch cuts the remaining search to a quarter.
	goodMatch = 32
	// niceMatch stops the search and the lazy evaluation.
	niceMatch = 128
)

// lazy is the parser of levels 1 to 6: hash chains and one step of lazy
// evaluation.
type lazy struct {
	data     []byte
	head     []int32 // 4-byte hash chains
	head3    []int32 // last position of each 3-byte hash
	prev     []int32
	cost     costs
	slots    int
	rep      repState
	mask     int
	maxDist  int
	depth    int
	inserted int
}

func newLazy(data []byte, opts Options) *lazy {
	slots := numSlots(opts.Window)
	parser := &lazy{
		data:    data,
		slots:   slots,
		cost:    newCosts(slots),
		rep:     initialRep(),
		head:    make([]int32, 1<<chainHashBits),
		head3:   make([]int32, 1<<hash3Bits),
		prev:    make([]int32, 1<<opts.Window),
		mask:    1<<opts.Window - 1,
		maxDist: 1<<opts.Window - 3,
		depth:   1 << (opts.Level - 1),
	}

	for i := range parser.head {
		parser.head[i] = -1
	}

	for i := range parser.head3 {
		parser.head3[i] = -1
	}

	return parser
}

// insertUpTo adds every position below pos to the hash tables.
func (p *lazy) insertUpTo(pos int) {
	for ; p.inserted < pos; p.inserted++ {
		next := p.inserted
		if next+3 > len(p.data) {
			continue
		}

		p.head3[hash3(p.data, next, hash3Bits)] = int32(next) //nolint:gosec // data is below 2 GiB

		if next+4 > len(p.data) {
			continue
		}

		h := hash4(p.data, next, chainHashBits)
		p.prev[next&p.mask] = p.head[h]
		p.head[h] = int32(next) //nolint:gosec // data is below 2 GiB
	}
}

// candidate is a match choice at one position.
type candidate struct {
	length int
	dist   int
	value  int // estimated bits saved over literals
}

// value estimates the bits a match of length bytes at pos saves over
// literals.
func (p *lazy) value(pos, length int, formatted uint32) int {
	saved := 0
	for _, b := range p.data[pos : pos+length] {
		saved += int(p.cost.main[b])
	}

	slot := slotOf(formatted)
	header := min(length-minMatch, 7)
	cost := int(p.cost.main[numChars+slot<<3+header]) + int(footerBits[slot])

	if header == 7 {
		cost += int(p.cost.len[length-9])
	}

	return saved - cost
}

// formattedOf is the formatted offset that dist would get now.
func (p *lazy) formattedOf(dist int) uint32 {
	for i, off := range p.rep {
		if uint32(dist) == off { //nolint:gosec // below the window size
			return uint32(i)
		}
	}

	return uint32(dist + 2) //nolint:gosec // below the window size
}

// best returns the most valuable match at pos.
func (p *lazy) best(pos int) candidate {
	limit := min(maxMatch, FrameSize-pos%FrameSize, len(p.data)-pos)
	result := candidate{}

	if limit < minMatch {
		return result
	}

	for i, off := range p.rep {
		if int(off) > pos {
			continue
		}

		if length := matchLen(p.data, pos, pos-int(off), limit); length >= minMatch {
			if v := p.value(pos, length, uint32(i)); v > result.value {
				result = candidate{length: length, dist: int(off), value: v}
			}
		}
	}

	if limit < 3 || result.length >= niceMatch {
		return result
	}

	p.insertUpTo(pos)

	return p.search(pos, limit, result)
}

// search walks the hash chains at pos for a match better than result.
func (p *lazy) search(pos, limit int, result candidate) candidate { //nolint:cyclop // one walk over the chain
	bestLen := 2

	// consider reports whether the search can stop.
	consider := func(cand int) bool {
		dist := pos - cand
		if cand < 0 || dist > p.maxDist || p.data[cand+bestLen] != p.data[pos+bestLen] {
			return false
		}

		length := matchLen(p.data, pos, cand, limit)
		if length <= bestLen {
			return false
		}

		if v := p.value(pos, length, p.formattedOf(dist)); v > result.value {
			result = candidate{length: length, dist: dist, value: v}
		}

		bestLen = length

		return length >= niceMatch || length == limit
	}

	if consider(int(p.head3[hash3(p.data, pos, hash3Bits)])) || limit < 4 {
		return result
	}

	cand := int(p.head[hash4(p.data, pos, chainHashBits)])

	for chain := p.depth; cand >= 0 && chain > 0; chain-- {
		if pos-cand > p.maxDist || consider(cand) {
			break
		}

		if bestLen >= goodMatch && chain > p.depth/4 {
			chain = p.depth / 4
		}

		next := int(p.prev[cand&p.mask])
		if next >= cand {
			break
		}

		cand = next
	}

	return result
}

// parse turns data[start:end] into tokens.
func (p *lazy) parse(start, end int) []token {
	tokens := make([]token, 0, (end-start)/2)
	rep := p.rep

	var (
		cached    candidate
		cachedPos = -1
	)

	for pos := start; pos < end; {
		cur := cached
		if cachedPos != pos {
			cur = p.best(pos)
		}

		if cur.value > 0 && cur.length < niceMatch && pos+1 < end && (pos+1)%FrameSize != 0 {
			// A literal leaves the repeated offsets alone, so the result at
			// pos+1 stays valid for the next round.
			cached, cachedPos = p.best(pos+1), pos+1
			if cached.value-int(p.cost.main[p.data[pos]]) > cur.value {
				tokens = append(tokens, literal(p.data[pos]))
				pos++

				continue
			}
		}

		if cur.value <= 0 {
			tokens = append(tokens, literal(p.data[pos]))
			pos++

			continue
		}

		p.rep.formatted(uint32(cur.dist)) //nolint:gosec // below the window size
		tokens = append(tokens, match(cur.length, cur.dist))
		pos += cur.length
	}

	stats := count(tokens, rep, p.slots)
	p.cost.set(buildLengths(stats.main, maxCodeLen), buildLengths(stats.len, maxCodeLen))

	return tokens
}
