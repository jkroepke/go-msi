package lzx

import (
	"math"
	"slices"
)

// The parser picks, per frame, the token sequence with the fewest estimated
// bits: a shortest path over the frame's positions where every literal and
// every match length is an edge. Matches never cross a frame, so a frame is
// parsed on its own. Costs come from the code lengths of the previous frame's
// tokens; from level 7 a second pass reparses the frame with the costs of the
// first pass's own tokens. The repeated offsets travel along each node's best
// path.
//
// The match finder is a binary tree over the last 2^searchBits bytes, which
// bounds the encoder's memory independently of the window.

const (
	// optimalNice is the match length the parser takes without weighing
	// shorter ones; it also bounds the binary tree comparisons.
	optimalNice  = 64
	treeHashBits = 18
	hash3Bits    = 15
	// searchBits bounds the match distance: 2^19 bytes. The binary tree takes
	// 8 bytes per position of that span.
	searchBits = 19
	// twoPassLevel is the lowest level that parses every frame twice.
	twoPassLevel = 7
)

type pair struct {
	length int32
	dist   int32
}

type pathNode struct {
	cost   int32
	from   int32 // node index the best edge starts at
	length int32 // 1 for a literal
	dist   uint32
	rep    repState
}

// history holds the stream bytes [base, base+len(buf)). It keeps at least
// the match distance before the block being parsed.
type history struct {
	buf  []byte
	base int
	keep int // bytes kept before the pending block when the buffer slides
}

func newHistory(keep int) *history {
	return &history{buf: make([]byte, 0, 2*keep+blockSpan), keep: keep}
}

// add appends data, sliding the buffer when it is full; pending is the first
// position that must stay.
func (h *history) add(data []byte, pending int) {
	if len(h.buf)+len(data) > cap(h.buf) {
		drop := pending - h.keep - h.base
		h.buf = h.buf[:copy(h.buf, h.buf[drop:])]
		h.base += drop
	}

	h.buf = append(h.buf, data...)
}

func (h *history) end() int { return h.base + len(h.buf) }

// optimal holds the match finder and parser state. Positions are absolute
// stream offsets.
type optimal struct {
	hist    *history
	head    []int32
	head3   []int32
	tree    []int32
	offsets []int32
	list    []pair
	nodes   []pathNode
	first   []token // the first pass of a frame
	tokens  []token // the block being parsed
	cost    costs
	slots   int
	mask    int
	maxDist int
	depth   int
	passes  int
	rep     repState
}

func newOptimal(hist *history, opts Options) *optimal {
	span := min(opts.Window, searchBits)
	parser := &optimal{
		hist:    hist,
		slots:   numSlots(opts.Window),
		head:    make([]int32, 1<<treeHashBits),
		head3:   make([]int32, 1<<hash3Bits),
		tree:    make([]int32, 2<<span),
		mask:    1<<span - 1,
		maxDist: 1<<span - 3,
		depth:   1 << max(0, opts.Level-5),
		passes:  1,
		offsets: make([]int32, FrameSize+1),
		nodes:   make([]pathNode, FrameSize+1),
		first:   make([]token, 0, FrameSize),
		tokens:  make([]token, 0, blockSpan),
		cost:    newCosts(numSlots(opts.Window)),
		rep:     initialRep(),
	}

	if opts.Level >= twoPassLevel {
		parser.passes = 2
	}

	for i := range parser.head {
		parser.head[i] = -1
	}

	for i := range parser.head3 {
		parser.head3[i] = -1
	}

	return parser
}

// findAll appends the plain matches at pos to p.list in order of increasing
// length, at most limit bytes long, and inserts pos into the match finder.
func (p *optimal) findAll(pos, limit int, report bool) {
	if pos+3 > p.hist.end() {
		return
	}

	bestLen := 2

	h3 := hash3(p.hist.buf, pos-p.hist.base, hash3Bits)
	if cand := int(p.head3[h3]); report && cand >= 0 && pos-cand <= p.maxDist {
		bestLen = p.add(min(matchLen(p.hist.buf, pos-p.hist.base, cand-p.hist.base, limit), limit), pos-cand, bestLen)
	}

	p.head3[h3] = int32(pos) //nolint:gosec // data is below 2 GiB

	if pos+4 <= p.hist.end() {
		p.walk(pos, limit, bestLen, report)
	}
}

// add appends a match longer than bestLen and returns the new best length.
func (p *optimal) add(length, dist, bestLen int) int {
	if length <= bestLen {
		return bestLen
	}

	p.list = append(p.list, pair{length: int32(length), dist: int32(dist)}) //nolint:gosec // bounded by the window

	return length
}

// walk inserts pos into the binary tree of its 4-byte prefix and reports the
// matches it passes: a node's left subtree holds the older positions whose
// strings sort below it, the right subtree those above. Children are always
// older than their parent, so a walk from the root stops at the first
// position outside the search span.
//
// The comparisons skip the prefix both bounds of the current subtree share,
// which is only sound while every inserted string was compared on
// optimalNice bytes. A position with less buffered data after it is
// searched but not inserted.
func (p *optimal) walk(pos, limit, bestLen int, report bool) {
	if p.hist.end()-pos < optimalNice {
		p.search(pos, limit, bestLen)

		return
	}

	data, base := p.hist.buf, p.hist.base
	h := hash4(data, pos-base, treeHashBits)
	cur := int(p.head[h])
	p.head[h] = int32(pos) //nolint:gosec // data is below 2 GiB

	treeLimit := optimalNice
	left, right := 2*(pos&p.mask), 2*(pos&p.mask)+1
	lenLeft, lenRight := 0, 0

	for depth := p.depth; cur >= 0 && pos-cur <= p.maxDist && depth > 0; depth-- {
		length := min(lenLeft, lenRight)
		length += matchLen(data, pos+length-base, cur+length-base, treeLimit-length)

		if length >= treeLimit {
			// Equal up to the limit: pos takes cur's place in the tree.
			if report {
				p.add(min(matchLen(data, pos-base, cur-base, limit), limit), pos-cur, bestLen)
			}

			p.tree[left], p.tree[right] = p.tree[2*(cur&p.mask)], p.tree[2*(cur&p.mask)+1]

			return
		}

		if report {
			bestLen = p.add(min(length, limit), pos-cur, bestLen)
		}

		if data[cur+length-base] < data[pos+length-base] {
			p.tree[left] = int32(cur) //nolint:gosec // data is below 2 GiB
			left = 2*(cur&p.mask) + 1
			cur = int(p.tree[left])
			lenLeft = length
		} else {
			p.tree[right] = int32(cur) //nolint:gosec // data is below 2 GiB
			right = 2 * (cur & p.mask)
			cur = int(p.tree[right])
			lenRight = length
		}
	}

	p.tree[left], p.tree[right] = -1, -1
}

// search reports the matches at pos like walk without changing the tree.
func (p *optimal) search(pos, limit, bestLen int) {
	data, base := p.hist.buf, p.hist.base
	cur := int(p.head[hash4(data, pos-base, treeHashBits)])
	treeLimit := min(optimalNice, p.hist.end()-pos)
	lenLeft, lenRight := 0, 0

	for depth := p.depth; cur >= 0 && pos-cur <= p.maxDist && depth > 0; depth-- {
		length := min(lenLeft, lenRight)
		length += matchLen(data, pos+length-base, cur+length-base, treeLimit-length)
		bestLen = p.add(min(length, limit), pos-cur, bestLen)

		if length >= treeLimit {
			return
		}

		if data[cur+length-base] < data[pos+length-base] {
			cur = int(p.tree[2*(cur&p.mask)+1])
			lenLeft = length
		} else {
			cur = int(p.tree[2*(cur&p.mask)])
			lenRight = length
		}
	}
}

// collect fills p.list and p.offsets with the matches of the frame
// [start, end).
func (p *optimal) collect(start, end int) {
	size := end - start
	p.list = p.list[:0]

	for i := 0; i < size; {
		p.offsets[i] = int32(len(p.list)) //nolint:gosec // frame sized
		at := start + i
		p.findAll(at, min(maxMatch, end-at), true)

		i++

		// A long match is taken as is: the positions it covers get no list,
		// but they still enter the tree.
		if last := len(p.list); last > int(p.offsets[i-1]) && int(p.list[last-1].length) >= optimalNice {
			for skip := int(p.list[last-1].length) - 1; skip > 0 && i < size; skip-- {
				p.offsets[i] = int32(len(p.list)) //nolint:gosec // frame sized
				p.findAll(start+i, 0, false)
				i++
			}
		}
	}

	p.offsets[size] = int32(len(p.list)) //nolint:gosec // frame sized
}

// block parses the block [start, end), which the history holds, frame by
// frame. The tokens stay valid until the next call.
func (p *optimal) block(start, end int) []token {
	p.tokens = p.tokens[:0]

	for frame := start; frame < end; frame += FrameSize {
		size := min(FrameSize, end-frame)
		p.collect(frame, frame+size)

		rep := p.rep
		tokens := p.shortestPath(frame, size, p.first[:0])

		if p.passes == 2 {
			stats := count(tokens, rep, p.slots)
			p.cost.set(buildLengths(stats.main, maxCodeLen), buildLengths(stats.len, maxCodeLen))
			p.rep = rep
			tokens = p.shortestPath(frame, size, tokens[:0])
		}

		stats := count(tokens, rep, p.slots)
		p.cost.set(buildLengths(stats.main, maxCodeLen), buildLengths(stats.len, maxCodeLen))
		p.tokens = append(p.tokens, tokens...)
	}

	return p.tokens
}

// relax offers the match lengths in (from, upTo] at node i, at distance
// dist with the formatted offset formatted, to the nodes they reach.
func (p *optimal) relax(nodes []pathNode, i, from, upTo int, dist, formatted uint32) {
	if upTo >= optimalNice {
		// Only the full length of a long match is worth an edge.
		from = upTo - 1
	}

	slot := slotOf(formatted)
	base := nodes[i].cost + int32(footerBits[slot]) //nolint:gosec // at most 17
	sym := p.cost.main[numChars+slot<<3:]

	for length := from + 1; length <= upTo; length++ {
		cost := base
		if length <= 8 {
			cost += sym[length-minMatch]
		} else {
			cost += sym[7] + p.cost.len[length-9]
		}

		if target := &nodes[i+length]; cost < target.cost {
			target.cost, target.from, target.length, target.dist = cost, int32(i), int32(length), dist //nolint:gosec // frame sized
		}
	}
}

// shortestPath runs one pass over the size positions of the frame at start,
// appends the tokens to dst and leaves the final repeated offsets in p.rep.
func (p *optimal) shortestPath(start, size int, dst []token) []token { //nolint:cyclop // one pass over the edges
	data, base := p.hist.buf, p.hist.base
	nodes := p.nodes[:size+1]

	for i := range nodes {
		nodes[i].cost = math.MaxInt32
	}

	nodes[0] = pathNode{rep: p.rep}

	for i := range size {
		node := &nodes[i]
		pos := start + i

		// The repeated offsets follow from the best edge into this node.
		if i > 0 {
			node.rep = nodes[node.from].rep
			if node.length > 1 {
				node.rep.formatted(node.dist)
			}
		}

		if cost := node.cost + p.cost.main[data[pos-base]]; cost < nodes[i+1].cost {
			nodes[i+1].cost, nodes[i+1].from, nodes[i+1].length = cost, int32(i), 1
		}

		limit := min(maxMatch, size-i)
		if limit < minMatch {
			continue
		}

		rep := node.rep
		for r, off := range rep {
			if int(off) <= pos && int(off) <= p.maxDist && (r == 0 || off != rep[0]) && (r < 2 || off != rep[1]) {
				p.relax(nodes, i, minMatch-1, matchLen(data, pos-base, pos-int(off)-base, limit), off, uint32(r))
			}
		}

		prevLen := minMatch

		for _, found := range p.list[p.offsets[i]:p.offsets[i+1]] {
			dist := uint32(found.dist) //nolint:gosec // positive
			if dist != rep[0] && dist != rep[1] && dist != rep[2] {
				p.relax(nodes, i, prevLen, int(found.length), dist, dist+2)
			}

			prevLen = int(found.length)
		}
	}

	return p.trace(nodes, start, dst)
}

// trace appends the tokens of the best path to the last node to dst.
func (p *optimal) trace(nodes []pathNode, start int, dst []token) []token {
	last := len(nodes) - 1

	count := 0
	for i := last; i > 0; i = int(nodes[i].from) {
		count++
	}

	first := len(dst)
	dst = slices.Grow(dst, count)[:first+count]

	for i := last; i > 0; i = int(nodes[i].from) {
		count--

		if node := nodes[i]; node.length == 1 {
			dst[first+count] = literal(p.hist.buf[start+int(node.from)-p.hist.base])
		} else {
			dst[first+count] = match(int(node.length), int(node.dist))
		}
	}

	end := nodes[last]
	p.rep = nodes[end.from].rep

	if end.length > 1 {
		p.rep.formatted(end.dist)
	}

	return dst
}
