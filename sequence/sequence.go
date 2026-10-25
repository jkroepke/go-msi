// Package sequence numbers the actions of a Windows Installer sequence table.
// Standard actions sit at fixed numbers; custom actions are scheduled before
// or after an anchor and get the numbers next to it, in definition order
// (docs/spec/MSISPEC.md sections 9.2 and 11.7). A chain that runs into
// another fixed number is an error, not a shift.
package sequence

import (
	"errors"
	"fmt"
	"slices"
)

// Limits of the Sequence column.
const (
	MaxSequence = 32767
	// MinTermination is the lowest termination number: termination actions
	// use -1 (success), -2 (user exit), -3 (fatal error) and -4 (suspend).
	MinTermination = -4
)

// Action is an action to schedule. Exactly one of Sequence, Before or After
// is set: Sequence fixes the number, Before and After name an anchor.
type Action struct {
	Name      string
	Condition string
	Before    string
	After     string
	Sequence  int
}

// Numbered is a scheduled action.
type Numbered struct {
	Name      string
	Condition string
	Sequence  int
}

type node struct {
	action   Action
	before   []*node
	after    []*node
	expanded []*node
	fixed    bool
	visiting bool
}

// Number assigns sequence numbers. The result is sorted by number.
func Number(actions []Action) ([]Numbered, error) {
	nodes := make(map[string]*node)
	ordered := make([]*node, 0, len(actions))

	for _, a := range actions {
		if a.Name == "" {
			return nil, errors.New("sequence: action without a name")
		}

		if _, dup := nodes[a.Name]; dup {
			return nil, fmt.Errorf("sequence: action %s scheduled twice", a.Name)
		}

		set := 0
		if a.Sequence != 0 {
			set++
		}

		if a.Before != "" {
			set++
		}

		if a.After != "" {
			set++
		}

		if set != 1 {
			return nil, fmt.Errorf("sequence: action %s needs exactly one of a number, before or after", a.Name)
		}

		if a.Sequence != 0 && (a.Sequence < MinTermination || a.Sequence > MaxSequence) {
			return nil, fmt.Errorf("sequence: action %s: number %d is outside -4…-1 and 1…%d", a.Name, a.Sequence, MaxSequence)
		}

		n := &node{action: a, fixed: a.Sequence != 0}
		nodes[a.Name] = n
		ordered = append(ordered, n)
	}

	for _, n := range ordered {
		anchorName := n.action.Before + n.action.After
		if anchorName == "" {
			continue
		}

		anchor, ok := nodes[anchorName]
		if !ok {
			return nil, fmt.Errorf("sequence: action %s is scheduled relative to %s, which is not in the table", n.action.Name, anchorName)
		}

		if anchor.fixed && anchor.action.Sequence < 0 {
			return nil, fmt.Errorf("sequence: action %s cannot be scheduled relative to the termination action %s", n.action.Name, anchorName)
		}

		if n.action.Before != "" {
			anchor.before = append(anchor.before, n)
		} else {
			anchor.after = append(anchor.after, n)
		}
	}

	// Every relative node must reach a fixed node through its anchors.
	for _, n := range ordered {
		seen := make(map[*node]bool)
		cur := n

		for !cur.fixed {
			if seen[cur] {
				return nil, fmt.Errorf("sequence: action %s is part of a scheduling cycle", n.action.Name)
			}

			seen[cur] = true
			cur = nodes[cur.action.Before+cur.action.After]
		}
	}

	var expand func(n *node) ([]*node, error)

	expand = func(n *node) ([]*node, error) {
		if n.visiting {
			return nil, fmt.Errorf("sequence: action %s is part of a scheduling cycle", n.action.Name)
		}

		if n.expanded != nil {
			return n.expanded, nil
		}

		n.visiting = true

		var out []*node

		for _, b := range n.before {
			e, err := expand(b)
			if err != nil {
				return nil, err
			}

			out = append(out, e...)
		}

		out = append(out, n)

		for _, a := range n.after {
			e, err := expand(a)
			if err != nil {
				return nil, err
			}

			out = append(out, e...)
		}

		n.visiting = false
		n.expanded = out

		return out, nil
	}

	type block struct {
		name string
		lo   int
		hi   int
	}

	var blocks []block

	var result []Numbered

	for _, n := range ordered {
		if !n.fixed {
			continue
		}

		chain, err := expand(n)
		if err != nil {
			return nil, err
		}

		pos := 0
		for chain[pos] != n {
			pos++
		}

		s := n.action.Sequence

		if s < 0 && len(chain) > 1 {
			return nil, fmt.Errorf("sequence: termination action %s cannot anchor other actions", n.action.Name)
		}

		lo := s - pos
		hi := s + len(chain) - 1 - pos

		if lo < 1 && s > 0 {
			return nil, fmt.Errorf("sequence: the actions before %s would need numbers below 1", n.action.Name)
		}

		if hi > MaxSequence {
			return nil, fmt.Errorf("sequence: the actions after %s would need numbers above %d", n.action.Name, MaxSequence)
		}

		blocks = append(blocks, block{name: n.action.Name, lo: lo, hi: hi})

		for i, c := range chain {
			result = append(result, Numbered{Name: c.action.Name, Condition: c.action.Condition, Sequence: lo + i})
		}
	}

	slices.SortFunc(blocks, func(a, b block) int { return a.lo - b.lo })

	for i := 1; i < len(blocks); i++ {
		if blocks[i].lo <= blocks[i-1].hi {
			return nil, fmt.Errorf("sequence: the actions around %s (%d…%d) overlap those around %s (%d…%d)",
				blocks[i-1].name, blocks[i-1].lo, blocks[i-1].hi, blocks[i].name, blocks[i].lo, blocks[i].hi)
		}
	}

	slices.SortStableFunc(result, func(a, b Numbered) int { return a.Sequence - b.Sequence })

	return result, nil
}

// Find returns the number of an action in a numbered sequence, or 0.
func Find(seq []Numbered, name string) int {
	for _, n := range seq {
		if n.Name == name {
			return n.Sequence
		}
	}

	return 0
}
