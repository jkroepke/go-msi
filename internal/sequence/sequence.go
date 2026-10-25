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
func Number(actions []Action) ([]Numbered, error) { //nolint:cyclop,gocyclo,gocognit // one pass over the format; splitting would scatter it
	nodes := make(map[string]*node)
	ordered := make([]*node, 0, len(actions))

	for _, action := range actions {
		if action.Name == "" {
			return nil, errors.New("sequence: action without a name")
		}

		if _, dup := nodes[action.Name]; dup {
			return nil, fmt.Errorf("sequence: action %s scheduled twice", action.Name)
		}

		set := 0
		if action.Sequence != 0 {
			set++
		}

		if action.Before != "" {
			set++
		}

		if action.After != "" {
			set++
		}

		if set != 1 {
			return nil, fmt.Errorf("sequence: action %s needs exactly one of a number, before or after", action.Name)
		}

		if action.Sequence != 0 && (action.Sequence < MinTermination || action.Sequence > MaxSequence) {
			return nil, fmt.Errorf("sequence: action %s: number %d is outside -4…-1 and 1…%d", action.Name, action.Sequence, MaxSequence)
		}

		n := &node{action: action, fixed: action.Sequence != 0}
		nodes[action.Name] = n
		ordered = append(ordered, n)
	}

	for _, item := range ordered {
		anchorName := item.action.Before + item.action.After
		if anchorName == "" {
			continue
		}

		anchor, ok := nodes[anchorName]
		if !ok {
			return nil, fmt.Errorf("sequence: action %s is scheduled relative to %s, which is not in the table", item.action.Name, anchorName)
		}

		if anchor.fixed && anchor.action.Sequence < 0 {
			return nil, fmt.Errorf("sequence: action %s cannot be scheduled relative to the termination action %s", item.action.Name, anchorName)
		}

		if item.action.Before != "" {
			anchor.before = append(anchor.before, item)
		} else {
			anchor.after = append(anchor.after, item)
		}
	}

	// Every relative node must reach a fixed node through its anchors.
	for _, item := range ordered {
		seen := make(map[*node]bool)
		cur := item

		for !cur.fixed {
			if seen[cur] {
				return nil, fmt.Errorf("sequence: action %s is part of a scheduling cycle", item.action.Name)
			}

			seen[cur] = true
			cur = nodes[cur.action.Before+cur.action.After]
		}
	}

	var expand func(n *node) ([]*node, error)

	expand = func(item *node) ([]*node, error) {
		if item.visiting {
			return nil, fmt.Errorf("sequence: action %s is part of a scheduling cycle", item.action.Name)
		}

		if item.expanded != nil {
			return item.expanded, nil
		}

		item.visiting = true

		var out []*node

		for _, b := range item.before {
			e, err := expand(b)
			if err != nil {
				return nil, err
			}

			out = append(out, e...)
		}

		out = append(out, item)

		for _, a := range item.after {
			e, err := expand(a)
			if err != nil {
				return nil, err
			}

			out = append(out, e...)
		}

		item.visiting = false
		item.expanded = out

		return out, nil
	}

	type block struct {
		name string
		lo   int
		hi   int
	}

	var blocks []block

	var result []Numbered

	for _, item := range ordered {
		if !item.fixed {
			continue
		}

		chain, err := expand(item)
		if err != nil {
			return nil, err
		}

		pos := 0
		for chain[pos] != item {
			pos++
		}

		start := item.action.Sequence

		if start < 0 && len(chain) > 1 {
			return nil, fmt.Errorf("sequence: termination action %s cannot anchor other actions", item.action.Name)
		}

		low := start - pos
		high := start + len(chain) - 1 - pos

		if low < 1 && start > 0 {
			return nil, fmt.Errorf("sequence: the actions before %s would need numbers below 1", item.action.Name)
		}

		if high > MaxSequence {
			return nil, fmt.Errorf("sequence: the actions after %s would need numbers above %d", item.action.Name, MaxSequence)
		}

		blocks = append(blocks, block{name: item.action.Name, lo: low, hi: high})

		for i, c := range chain {
			result = append(result, Numbered{Name: c.action.Name, Condition: c.action.Condition, Sequence: low + i})
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
