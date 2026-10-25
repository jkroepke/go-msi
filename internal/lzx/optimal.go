package lzx

import (
	"math"
	"runtime"
	"sync"
)

// The optimal parser picks, per block, the token sequence with the fewest
// estimated bits: a shortest path over the block's positions where every
// literal and every match length is an edge. Costs come from the previous
// block's code lengths in a first pass and from the first pass's own trees in
// a second one. The repeated offsets travel along each node's best path.
//
// The data is cut into segments that are parsed in parallel. Each segment
// first loads the window before it into its match finder, so only its
// repeated offsets and cost estimates start cold. Segments have a fixed size,
// so the output does not depend on the number of workers.

const (
	// optimalNice is the match length the optimal parser takes without
	// weighing shorter ones; it also bounds the binary tree comparisons.
	optimalNice  = 64
	treeHashBits = 18
	segmentSize  = 64 * blockSpan
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

// optimal holds one worker's match finder and parser state.
type optimal struct {
	data    []byte
	head    []int32
	head3   []int32
	tree    []int32
	offsets []int32
	list    []pair
	nodes   []pathNode
	cost    costs
	slots   int
	mask    int
	maxDist int
	depth   int
	rep     repState
}

func newOptimal(data []byte, opts Options) *optimal {
	return &optimal{
		data:    data,
		slots:   numSlots(opts.Window),
		head:    make([]int32, 1<<treeHashBits),
		head3:   make([]int32, 1<<hash3Bits),
		tree:    make([]int32, 2<<opts.Window),
		mask:    1<<opts.Window - 1,
		maxDist: 1<<opts.Window - 3,
		depth:   4 << (opts.Level - optimalLevel),
		offsets: make([]int32, blockSpan+1),
		nodes:   make([]pathNode, blockSpan+1),
	}
}

// parseOptimal parses data in segments and hands every block's tokens to
// emit in order.
func parseOptimal(data []byte, opts Options, emit func([]token)) {
	segments := (len(data) + segmentSize - 1) / segmentSize

	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}

	workers = min(workers, segments)

	results := make([]chan [][]token, segments)
	for i := range results {
		results[i] = make(chan [][]token, 1)
	}

	jobs := make(chan int)
	// ahead bounds the parsed segments that wait for the writer.
	ahead := make(chan struct{}, 2*workers)

	var wg sync.WaitGroup

	for range workers {
		wg.Go(func() {
			parser := newOptimal(data, opts)
			for seg := range jobs {
				results[seg] <- parser.segment(seg)
			}
		})
	}

	go func() {
		for seg := range segments {
			ahead <- struct{}{}

			jobs <- seg
		}

		close(jobs)
	}()

	for seg := range segments {
		for _, tokens := range <-results[seg] {
			emit(tokens)
		}

		<-ahead
	}

	wg.Wait()
}

// segment parses one segment into blocks of tokens.
func (p *optimal) segment(seg int) [][]token {
	start := seg * segmentSize
	end := min(start+segmentSize, len(p.data))

	for i := range p.head {
		p.head[i] = -1
	}

	for i := range p.head3 {
		p.head3[i] = -1
	}

	p.cost = newCosts(p.slots)
	p.rep = initialRep()

	for pos := max(0, start-p.maxDist); pos < start; pos++ {
		p.findAll(pos, 0, false)
	}

	blocks := make([][]token, 0, (end-start+blockSpan-1)/blockSpan)
	for at := start; at < end; at += blockSpan {
		blocks = append(blocks, p.block(at, min(at+blockSpan, end)))
	}

	return blocks
}

// findAll appends the plain matches at pos to p.list in order of increasing
// length, at most limit bytes long, and inserts pos into the match finder.
func (p *optimal) findAll(pos, limit int, report bool) {
	if pos+3 > len(p.data) {
		return
	}

	bestLen := 2

	h3 := hash3(p.data, pos, hash3Bits)
	if cand := int(p.head3[h3]); report && cand >= 0 && pos-cand <= p.maxDist {
		bestLen = p.add(min(matchLen(p.data, pos, cand, limit), limit), pos-cand, bestLen)
	}

	p.head3[h3] = int32(pos) //nolint:gosec // data is below 2 GiB

	if pos+4 <= len(p.data) {
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
// position outside the window.
func (p *optimal) walk(pos, limit, bestLen int, report bool) {
	h := hash4(p.data, pos, treeHashBits)
	cur := int(p.head[h])
	p.head[h] = int32(pos) //nolint:gosec // data is below 2 GiB

	treeLimit := min(optimalNice, len(p.data)-pos)
	left, right := 2*(pos&p.mask), 2*(pos&p.mask)+1
	lenLeft, lenRight := 0, 0

	for depth := p.depth; cur >= 0 && pos-cur <= p.maxDist && depth > 0; depth-- {
		length := min(lenLeft, lenRight)
		length += matchLen(p.data, pos+length, cur+length, treeLimit-length)

		if length >= treeLimit {
			// Equal up to the limit: pos takes cur's place in the tree.
			if report {
				p.add(min(matchLen(p.data, pos, cur, limit), limit), pos-cur, bestLen)
			}

			p.tree[left], p.tree[right] = p.tree[2*(cur&p.mask)], p.tree[2*(cur&p.mask)+1]

			return
		}

		if report {
			bestLen = p.add(min(length, limit), pos-cur, bestLen)
		}

		if p.data[cur+length] < p.data[pos+length] {
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

// collect fills p.list and p.offsets with the matches of data[start:end].
func (p *optimal) collect(start, end int) {
	size := end - start
	p.list = p.list[:0]

	for i := 0; i < size; {
		p.offsets[i] = int32(len(p.list)) //nolint:gosec // block sized
		at := start + i
		p.findAll(at, min(maxMatch, FrameSize-at%FrameSize, end-at), true)

		i++

		// A long match is taken as is: the positions it covers get no list,
		// but they still enter the tree.
		if last := len(p.list); last > int(p.offsets[i-1]) && int(p.list[last-1].length) >= optimalNice {
			for skip := int(p.list[last-1].length) - 1; skip > 0 && i < size; skip-- {
				p.offsets[i] = int32(len(p.list)) //nolint:gosec // block sized
				p.findAll(start+i, 0, false)
				i++
			}
		}
	}

	p.offsets[size] = int32(len(p.list)) //nolint:gosec // block sized
}

// block parses data[start:end] in two passes.
func (p *optimal) block(start, end int) []token {
	p.collect(start, end)

	rep := p.rep
	tokens := p.shortestPath(start, end-start)

	// Second pass with the costs of the trees the first pass produces.
	stats := count(tokens, rep, p.slots)
	p.cost.set(buildLengths(stats.main, maxCodeLen), buildLengths(stats.len, maxCodeLen))
	p.rep = rep
	tokens = p.shortestPath(start, end-start)

	stats = count(tokens, rep, p.slots)
	p.cost.set(buildLengths(stats.main, maxCodeLen), buildLengths(stats.len, maxCodeLen))

	return tokens
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
			target.cost, target.from, target.length, target.dist = cost, int32(i), int32(length), dist //nolint:gosec // block sized
		}
	}
}

// shortestPath runs one pass over size positions and leaves the final
// repeated offsets in p.rep.
func (p *optimal) shortestPath(start, size int) []token { //nolint:cyclop // one pass over the edges
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

		if cost := node.cost + p.cost.main[p.data[pos]]; cost < nodes[i+1].cost {
			nodes[i+1].cost, nodes[i+1].from, nodes[i+1].length = cost, int32(i), 1
		}

		limit := min(maxMatch, FrameSize-pos%FrameSize, size-i)
		if limit < minMatch {
			continue
		}

		rep := node.rep
		for r, off := range rep {
			if int(off) <= pos && (r == 0 || off != rep[0]) && (r < 2 || off != rep[1]) {
				p.relax(nodes, i, minMatch-1, matchLen(p.data, pos, pos-int(off), limit), off, uint32(r))
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

	return p.trace(nodes, start)
}

// trace returns the tokens of the best path to the last node.
func (p *optimal) trace(nodes []pathNode, start int) []token {
	last := len(nodes) - 1

	count := 0
	for i := last; i > 0; i = int(nodes[i].from) {
		count++
	}

	tokens := make([]token, count)

	for i := last; i > 0; i = int(nodes[i].from) {
		count--

		if node := nodes[i]; node.length == 1 {
			tokens[count] = literal(p.data[start+int(node.from)])
		} else {
			tokens[count] = match(int(node.length), int(node.dist))
		}
	}

	end := nodes[last]
	p.rep = nodes[end.from].rep

	if end.length > 1 {
		p.rep.formatted(end.dist)
	}

	return tokens
}
