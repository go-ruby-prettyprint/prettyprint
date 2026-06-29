// Copyright (c) the go-ruby-prettyprint/prettyprint authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package prettyprint is a pure-Go (no cgo) reimplementation of Ruby's
// `prettyprint` standard library — the Wadler/Lindig pretty-printing engine that
// MRI's `pp` object inspector is built on.
//
// It is a faithful, byte-identical port of MRI 4.0.5's prettyprint.rb. The
// engine collects a stream of text, breakable separators and groups, then lays
// it out so that each group is printed on a single line when it fits within a
// maximum width and is otherwise broken at its breakable points, with nesting
// (indentation) preserved. The companion [SingleLine] variant emits the same
// stream with no breaks at all (breakables become their separator text).
//
// This package implements only the layout engine. The `pp` object inspector
// that consumes it lives in the host (go-embedded-ruby / rbgo); this library is
// the standalone, reusable backend it binds to.
//
// # References
//
//   - Christian Lindig, Strictly Pretty, March 2000.
//   - Philip Wadler, A prettier printer, March 1998.
package prettyprint

import "strings"

// VERSION mirrors PrettyPrint::VERSION in MRI 4.0.5.
const VERSION = "0.2.0"

// GenSpace generates the indentation string for a given number of columns. It
// corresponds to the `genspace` block of PrettyPrint.new. The default,
// [DefaultGenSpace], returns n ASCII spaces.
type GenSpace func(n int) string

// DefaultGenSpace returns a string of n spaces — the default genspace block,
// `lambda {|n| ' ' * n}`.
func DefaultGenSpace(n int) string {
	return strings.Repeat(" ", n)
}

// node is the common interface of the two buffer element kinds, [text] and
// [breakable]. output renders the node onto the builder, returning the updated
// output width, exactly like the polymorphic `#output` methods in MRI.
type node interface {
	output(out *strings.Builder, outputWidth int) int
	nodeWidth() int
}

// text accumulates one or more string fragments that must stay together. It is
// the port of PrettyPrint::Text.
type text struct {
	objs  []string
	width int
}

func (t *text) output(out *strings.Builder, outputWidth int) int {
	for _, o := range t.objs {
		out.WriteString(o)
	}
	return outputWidth + t.width
}

func (t *text) nodeWidth() int { return t.width }

// add appends a fragment and grows the recorded width, like Text#add.
func (t *text) add(obj string, width int) {
	t.objs = append(t.objs, obj)
	t.width += width
}

// breakable is a candidate line break carrying the separator emitted when the
// enclosing group is not broken. It is the port of PrettyPrint::Breakable.
type breakable struct {
	obj    string
	width  int
	pp     *PrettyPrint
	indent int
	group  *group
}

func (b *breakable) nodeWidth() int { return b.width }

func (b *breakable) output(out *strings.Builder, outputWidth int) int {
	// shift self off the front of the owning group's breakable list.
	b.group.breakables = b.group.breakables[1:]
	if b.group.broken {
		out.WriteString(b.pp.newline)
		out.WriteString(b.pp.genspace(b.indent))
		return b.indent
	}
	if len(b.group.breakables) == 0 {
		b.pp.groupQueue.delete(b.group)
	}
	out.WriteString(b.obj)
	return outputWidth + b.width
}

// group records a nesting level and the breakables registered under it, plus
// whether it has been forced to break. It is the port of PrettyPrint::Group.
type group struct {
	depth      int
	breakables []*breakable
	broken     bool
}

func newGroup(depth int) *group {
	return &group{depth: depth}
}

// groupQueue holds groups bucketed by depth, the structure that lets
// break_outmost_groups dequeue the shallowest still-breakable group. It is the
// port of PrettyPrint::GroupQueue.
type groupQueue struct {
	queue [][]*group
}

func newGroupQueue(groups ...*group) *groupQueue {
	q := &groupQueue{}
	for _, g := range groups {
		q.enq(g)
	}
	return q
}

// enq inserts group into the bucket for its depth, growing the queue as needed.
func (q *groupQueue) enq(g *group) {
	for g.depth >= len(q.queue) {
		q.queue = append(q.queue, nil)
	}
	q.queue[g.depth] = append(q.queue[g.depth], g)
}

// deq returns the shallowest group that still has breakables, marking it (and,
// when a whole depth bucket has none, every group in that bucket) as broken.
func (q *groupQueue) deq() *group {
	for d := range q.queue {
		gs := q.queue[d]
		for i := len(gs) - 1; i >= 0; i-- {
			if len(gs[i].breakables) != 0 {
				g := gs[i]
				// slice!(i, 1): remove element i from this bucket.
				q.queue[d] = append(gs[:i:i], gs[i+1:]...)
				g.broken = true
				return g
			}
		}
		// gs.each { break } then gs.clear: mark all broken, empty the bucket.
		for _, g := range gs {
			g.broken = true
		}
		q.queue[d] = gs[:0]
	}
	return nil
}

// delete removes group from its depth bucket, like GroupQueue#delete.
func (q *groupQueue) delete(g *group) {
	bucket := q.queue[g.depth]
	for i, x := range bucket {
		if x == g {
			q.queue[g.depth] = append(bucket[:i], bucket[i+1:]...)
			return
		}
	}
}

// PrettyPrint is the layout buffer. It is the port of the PrettyPrint class:
// callers build a document with [PrettyPrint.Text], [PrettyPrint.Breakable],
// [PrettyPrint.Group] and [PrettyPrint.Nest], then read it back with
// [PrettyPrint.String] (after [PrettyPrint.Flush]).
type PrettyPrint struct {
	output   strings.Builder
	maxwidth int
	newline  string
	genspace GenSpace

	outputWidth int
	bufferWidth int
	buffer      []node

	groupStack []*group
	groupQueue *groupQueue
	indent     int
}

// New creates a pretty-printing buffer.
//
// maxwidth is the maximum line length (MRI default 79); outputs may still
// overflow it where a single non-breakable text is wider. newline is the line
// break string (default "\n"). genspace generates indentation; when nil,
// [DefaultGenSpace] is used. The zero values matching MRI's defaults are
// produced by [NewDefault].
func New(maxwidth int, newline string, genspace GenSpace) *PrettyPrint {
	if genspace == nil {
		genspace = DefaultGenSpace
	}
	root := newGroup(0)
	return &PrettyPrint{
		maxwidth:   maxwidth,
		newline:    newline,
		genspace:   genspace,
		groupStack: []*group{root},
		groupQueue: newGroupQueue(root),
	}
}

// NewDefault creates a buffer with MRI's defaults: maxwidth 79, "\n" newline and
// the space-generating block.
func NewDefault() *PrettyPrint {
	return New(79, "\n", DefaultGenSpace)
}

// Maxwidth reports the configured maximum line width.
func (q *PrettyPrint) Maxwidth() int { return q.maxwidth }

// Newline reports the configured line-break string.
func (q *PrettyPrint) Newline() string { return q.newline }

// Indent reports the current indentation in columns.
func (q *PrettyPrint) Indent() int { return q.indent }

// CurrentGroup returns the group most recently pushed on the stack, the port of
// PrettyPrint#current_group.
func (q *PrettyPrint) CurrentGroup() *group {
	return q.groupStack[len(q.groupStack)-1]
}

// BreakOutmostGroups breaks buffered groups until the pending output fits within
// maxwidth, draining the breakables (and the trailing texts) of each broken
// group to the output. It is the port of PrettyPrint#break_outmost_groups.
func (q *PrettyPrint) BreakOutmostGroups() {
	for q.maxwidth < q.outputWidth+q.bufferWidth {
		g := q.groupQueue.deq()
		if g == nil {
			return
		}
		for len(g.breakables) != 0 {
			data := q.buffer[0]
			q.buffer = q.buffer[1:]
			q.outputWidth = data.output(&q.output, q.outputWidth)
			q.bufferWidth -= data.nodeWidth()
		}
		for len(q.buffer) != 0 {
			t, ok := q.buffer[0].(*text)
			if !ok {
				break
			}
			q.buffer = q.buffer[1:]
			q.outputWidth = t.output(&q.output, q.outputWidth)
			q.bufferWidth -= t.width
		}
	}
}

// Text adds obj as a run of width columns. It is the port of PrettyPrint#text.
// Use [PrettyPrint.TextString] to default width to len(obj).
func (q *PrettyPrint) Text(obj string, width int) {
	if len(q.buffer) == 0 {
		q.output.WriteString(obj)
		q.outputWidth += width
		return
	}
	last, ok := q.buffer[len(q.buffer)-1].(*text)
	if !ok {
		last = &text{}
		q.buffer = append(q.buffer, last)
	}
	last.add(obj, width)
	q.bufferWidth += width
	q.BreakOutmostGroups()
}

// TextString adds obj with its byte length as the width, matching MRI's
// `text(obj, width=obj.length)` default.
func (q *PrettyPrint) TextString(obj string) {
	q.Text(obj, len(obj))
}

// FillBreakable groups a single breakable so the break decision is made
// individually at this point. It is the port of PrettyPrint#fill_breakable.
func (q *PrettyPrint) FillBreakable(sep string, width int) {
	q.GroupSub(0, "", 0, "", 0, func() {
		q.Breakable(sep, width)
	})
}

// FillBreakableString is FillBreakable with sep " " and width 1.
func (q *PrettyPrint) FillBreakableString() {
	q.FillBreakable(" ", 1)
}

// Breakable records a candidate break that prints sep (width columns) when the
// enclosing group does not break. It is the port of PrettyPrint#breakable.
func (q *PrettyPrint) Breakable(sep string, width int) {
	g := q.groupStack[len(q.groupStack)-1]
	if g.broken {
		q.Flush()
		q.output.WriteString(q.newline)
		q.output.WriteString(q.genspace(q.indent))
		q.outputWidth = q.indent
		q.bufferWidth = 0
		return
	}
	b := &breakable{obj: sep, width: width, pp: q, indent: q.indent, group: g}
	g.breakables = append(g.breakables, b)
	q.buffer = append(q.buffer, b)
	q.bufferWidth += width
	q.BreakOutmostGroups()
}

// BreakableString is Breakable with sep " " and width 1, the all-defaults form.
func (q *PrettyPrint) BreakableString() {
	q.Breakable(" ", 1)
}

// Group runs fn inside a new group nested by indent, optionally emitting openObj
// (openWidth columns) before and closeObj (closeWidth columns) after. It is the
// port of PrettyPrint#group.
func (q *PrettyPrint) Group(indent int, openObj string, openWidth int, closeObj string, closeWidth int, fn func()) {
	q.Text(openObj, openWidth)
	q.GroupSub(0, "", 0, "", 0, func() {
		q.Nest(indent, fn)
	})
	q.Text(closeObj, closeWidth)
}

// GroupDefault runs fn in a fresh group with no extra indent and no open/close
// text — the common `group { ... }` form.
func (q *PrettyPrint) GroupDefault(fn func()) {
	q.Group(0, "", 0, "", 0, fn)
}

// NestedGroup is an alias of [PrettyPrint.GroupDefault] kept for the rbgo
// binding's NestedGroup entry point.
func (q *PrettyPrint) NestedGroup(fn func()) {
	q.GroupDefault(fn)
}

// GroupSub queues a new group one level deeper than the current one and runs fn
// inside it, removing the group again afterwards if it registered no breakables.
// It is the port of PrettyPrint#group_sub. The leading parameters mirror the
// rbgo binding's signature and are unused; callers normally pass zero/empty.
func (q *PrettyPrint) GroupSub(_ int, _ string, _ int, _ string, _ int, fn func()) {
	g := newGroup(q.groupStack[len(q.groupStack)-1].depth + 1)
	q.groupStack = append(q.groupStack, g)
	q.groupQueue.enq(g)
	defer func() {
		q.groupStack = q.groupStack[:len(q.groupStack)-1]
		if len(g.breakables) == 0 {
			q.groupQueue.delete(g)
		}
	}()
	fn()
}

// Nest increases the left margin by indent for the breaks added inside fn. It is
// the port of PrettyPrint#nest.
func (q *PrettyPrint) Nest(indent int, fn func()) {
	q.indent += indent
	defer func() { q.indent -= indent }()
	fn()
}

// Flush writes every buffered element to the output and empties the buffer. It
// is the port of PrettyPrint#flush.
func (q *PrettyPrint) Flush() {
	for _, data := range q.buffer {
		q.outputWidth = data.output(&q.output, q.outputWidth)
	}
	q.buffer = q.buffer[:0]
	q.bufferWidth = 0
}

// String returns the rendered output accumulated so far. Call [PrettyPrint.Flush]
// first to drain any still-buffered elements.
func (q *PrettyPrint) String() string {
	return q.output.String()
}

// Format is the convenience entry point of PrettyPrint.format: it builds a
// buffer, runs fn against it, flushes and returns the rendered string.
func Format(maxwidth int, newline string, genspace GenSpace, fn func(*PrettyPrint)) string {
	q := New(maxwidth, newline, genspace)
	fn(q)
	q.Flush()
	return q.String()
}

// FormatDefault is Format with MRI's defaults (maxwidth 79, "\n", space genspace).
func FormatDefault(fn func(*PrettyPrint)) string {
	return Format(79, "\n", DefaultGenSpace, fn)
}

// SingleLineFormat is the convenience entry point of
// PrettyPrint.singleline_format: it runs fn against a [SingleLine] buffer (whose
// breakables never break) and returns the rendered string.
func SingleLineFormat(fn func(*SingleLine)) string {
	q := NewSingleLine()
	fn(q)
	return q.String()
}

// SingleLine renders the same builder calls as [PrettyPrint] but never breaks a
// line: breakables emit their separator as plain text and groups/nests are
// transparent. It is the port of PrettyPrint::SingleLine.
type SingleLine struct {
	output strings.Builder
	first  []bool
}

// NewSingleLine creates a SingleLine buffer.
func NewSingleLine() *SingleLine {
	return &SingleLine{first: []bool{true}}
}

// Text appends obj. The width argument is accepted for API parity and ignored.
func (s *SingleLine) Text(obj string, _ int) {
	s.output.WriteString(obj)
}

// TextString appends obj, the width-defaulting form.
func (s *SingleLine) TextString(obj string) {
	s.output.WriteString(obj)
}

// Breakable appends sep (no line break). width is ignored.
func (s *SingleLine) Breakable(sep string, _ int) {
	s.output.WriteString(sep)
}

// BreakableString appends a single space.
func (s *SingleLine) BreakableString() {
	s.output.WriteString(" ")
}

// Nest runs fn; indent is ignored.
func (s *SingleLine) Nest(_ int, fn func()) {
	fn()
}

// Group emits openObj, runs fn, then closeObj. indent and the width arguments
// are ignored.
func (s *SingleLine) Group(_ int, openObj string, _ int, closeObj string, _ int, fn func()) {
	s.first = append(s.first, true)
	s.output.WriteString(openObj)
	fn()
	s.output.WriteString(closeObj)
	s.first = s.first[:len(s.first)-1]
}

// GroupDefault runs fn in a group with no open/close text.
func (s *SingleLine) GroupDefault(fn func()) {
	s.Group(0, "", 0, "", 0, fn)
}

// Flush is a no-op, present for parity with [PrettyPrint.Flush]; SingleLine
// writes directly and buffers nothing.
func (s *SingleLine) Flush() {
	_ = s
}

// First reports whether this is the first query of the innermost group, then
// records that it no longer is — the port of SingleLine#first?.
func (s *SingleLine) First() bool {
	i := len(s.first) - 1
	r := s.first[i]
	s.first[i] = false
	return r
}

// String returns the rendered output.
func (s *SingleLine) String() string {
	return s.output.String()
}
