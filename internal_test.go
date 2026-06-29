// Copyright (c) the go-ruby-prettyprint/prettyprint authors
//
// SPDX-License-Identifier: BSD-3-Clause

package prettyprint

import "testing"

// TestDeqExhausted drives GroupQueue#deq to its terminal `return nil`: a queue
// holding only groups with no breakables. MRI's deq walks every depth bucket,
// marks each such group broken, clears the bucket, and falls off the end
// returning nil. The public builder keeps the buffer drained whenever the queue
// empties, so this terminal arm is exercised structurally here, exactly as the
// MRI source defines it.
func TestDeqExhausted(t *testing.T) {
	g0 := newGroup(0) // depth-0 group, no breakables
	g1 := newGroup(1) // depth-1 group, no breakables
	q := newGroupQueue(g0, g1)

	if got := q.deq(); got != nil {
		t.Fatalf("deq with no breakable groups = %p, want nil", got)
	}
	// Every group is now marked broken and its bucket cleared.
	if !g0.broken || !g1.broken {
		t.Errorf("deq should mark exhausted groups broken: g0=%v g1=%v", g0.broken, g1.broken)
	}
	if len(q.queue[0]) != 0 || len(q.queue[1]) != 0 {
		t.Errorf("deq should clear exhausted buckets: %v", q.queue)
	}
}

// TestBreakOutmostGroupsNilGroup drives the `group == nil` early return of
// break_outmost_groups: the pending buffer still overflows maxwidth, yet the
// group queue holds no breakable group, so deq yields nil and the method
// returns without draining further (mirroring MRI's `return unless group`).
func TestBreakOutmostGroupsNilGroup(t *testing.T) {
	q := New(1, "\n", nil)
	// Hand-build the overflow condition the public API keeps invariant-safe:
	// a buffered text wider than maxwidth, with an empty (breakable-less) queue.
	// deq returns nil on the first iteration, so output stays empty.
	q.buffer = []node{&text{objs: []string{"wide"}, width: 4}}
	q.bufferWidth = 4
	// The root group in the queue has no breakables, so deq returns nil.
	q.BreakOutmostGroups()

	if q.output.Len() != 0 {
		t.Errorf("BreakOutmostGroups drained on nil group: %q", q.output.String())
	}
	if len(q.buffer) != 1 || q.bufferWidth != 4 {
		t.Errorf("buffer mutated on nil-group return: buf=%d width=%d", len(q.buffer), q.bufferWidth)
	}
}
