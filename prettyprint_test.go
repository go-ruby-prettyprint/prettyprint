// Copyright (c) the go-ruby-prettyprint/prettyprint authors
//
// SPDX-License-Identifier: BSD-3-Clause

package prettyprint

import (
	"strings"
	"testing"
)

// dotSpace is a non-default genspace used to prove genspace is honoured: it emits
// n dots instead of n spaces.
func dotSpace(n int) string { return strings.Repeat(".", n) }

// TestVersion pins the version constant to MRI 4.0.5's PrettyPrint::VERSION.
func TestVersion(t *testing.T) {
	if VERSION != "0.2.0" {
		t.Errorf("VERSION = %q, want 0.2.0", VERSION)
	}
}

// TestDefaultGenSpace covers the default space generator and its wiring through
// New(nil) / NewDefault.
func TestDefaultGenSpace(t *testing.T) {
	if got := DefaultGenSpace(3); got != "   " {
		t.Errorf("DefaultGenSpace(3) = %q, want 3 spaces", got)
	}
	if got := DefaultGenSpace(0); got != "" {
		t.Errorf("DefaultGenSpace(0) = %q, want empty", got)
	}
	q := NewDefault()
	if q.genspace(2) != "  " {
		t.Error("NewDefault did not install DefaultGenSpace")
	}
	// New(nil) must also fall back to DefaultGenSpace.
	q2 := New(40, "\n", nil)
	if q2.genspace(1) != " " {
		t.Error("New(nil genspace) did not fall back to DefaultGenSpace")
	}
}

// TestAccessors covers the read-only accessors.
func TestAccessors(t *testing.T) {
	q := New(40, "\r\n", dotSpace)
	if q.Maxwidth() != 40 {
		t.Errorf("Maxwidth = %d", q.Maxwidth())
	}
	if q.Newline() != "\r\n" {
		t.Errorf("Newline = %q", q.Newline())
	}
	if q.Indent() != 0 {
		t.Errorf("initial Indent = %d", q.Indent())
	}
	if q.CurrentGroup().depth != 0 {
		t.Errorf("root group depth = %d", q.CurrentGroup().depth)
	}
	q.Nest(3, func() {
		if q.Indent() != 3 {
			t.Errorf("nested Indent = %d, want 3", q.Indent())
		}
	})
	if q.Indent() != 0 {
		t.Errorf("Indent after nest = %d, want 0", q.Indent())
	}
}

// caseTable drives the bulk of coverage with builder programs and their exact
// expected MRI output (each verified against `ruby -rprettyprint`).
func TestFormatCases(t *testing.T) {
	cases := []struct {
		name     string
		maxwidth int
		newline  string
		genspace GenSpace
		build    func(q *PrettyPrint)
		want     string
	}{
		{
			name:     "group_fits",
			maxwidth: 79,
			build: func(q *PrettyPrint) {
				q.GroupDefault(func() {
					q.TextString("[")
					q.Breakable("", 0)
					q.TextString("1")
					q.BreakableString()
					q.TextString("2")
					q.Breakable("", 0)
					q.TextString("]")
				})
			},
			want: "[1 2]",
		},
		{
			name:     "group_overflow_no_indent",
			maxwidth: 10,
			build: func(q *PrettyPrint) {
				q.GroupDefault(func() {
					q.TextString("[")
					q.Breakable("", 0)
					q.TextString("111")
					q.BreakableString()
					q.TextString("222")
					q.BreakableString()
					q.TextString("333")
					q.Breakable("", 0)
					q.TextString("]")
				})
			},
			want: "[\n111\n222\n333\n]",
		},
		{
			name:     "group_overflow_indent",
			maxwidth: 10,
			build: func(q *PrettyPrint) {
				q.Group(2, "", 0, "", 0, func() {
					q.TextString("[")
					q.Breakable("", 0)
					q.TextString("111")
					q.BreakableString()
					q.TextString("222")
					q.BreakableString()
					q.TextString("333")
					q.Breakable("", 0)
					q.TextString("]")
				})
			},
			want: "[\n  111\n  222\n  333\n  ]",
		},
		{
			name:     "group_open_close",
			maxwidth: 10,
			build: func(q *PrettyPrint) {
				q.Group(2, "[", 1, "]", 1, func() {
					q.TextString("111")
					q.BreakableString()
					q.TextString("222")
					q.BreakableString()
					q.TextString("333")
				})
			},
			want: "[111\n  222\n  333]",
		},
		{
			name:     "nested_inner_fits",
			maxwidth: 15,
			build: func(q *PrettyPrint) {
				q.Group(2, "{", 1, "}", 1, func() {
					q.TextString("a")
					q.BreakableString()
					q.Group(2, "[", 1, "]", 1, func() {
						q.TextString("1")
						q.BreakableString()
						q.TextString("2")
					})
					q.BreakableString()
					q.TextString("b")
				})
			},
			want: "{a [1 2] b}",
		},
		{
			name:     "nested_outer_breaks_inner_fits",
			maxwidth: 8,
			build: func(q *PrettyPrint) {
				q.Group(2, "{", 1, "}", 1, func() {
					q.TextString("a")
					q.BreakableString()
					q.Group(2, "[", 1, "]", 1, func() {
						q.TextString("1")
						q.BreakableString()
						q.TextString("2")
					})
					q.BreakableString()
					q.TextString("b")
				})
			},
			want: "{a\n  [1 2]\n  b}",
		},
		{
			name:     "nest",
			maxwidth: 10,
			build: func(q *PrettyPrint) {
				q.TextString("begin")
				q.Nest(4, func() {
					q.BreakableString()
					q.TextString("body1")
					q.BreakableString()
					q.TextString("body2")
				})
				q.BreakableString()
				q.TextString("end")
			},
			want: "begin\n    body1\n    body2\nend",
		},
		{
			name:     "fill",
			maxwidth: 10,
			build: func(q *PrettyPrint) {
				words := []string{"1", "22", "333", "4444", "55555", "6"}
				q.GroupDefault(func() {
					for i, w := range words {
						if i > 0 {
							q.FillBreakableString()
						}
						q.TextString(w)
					}
				})
			},
			want: "1 22 333\n4444 55555\n6",
		},
		{
			name:     "fill_custom_sep",
			maxwidth: 6,
			build: func(q *PrettyPrint) {
				q.GroupDefault(func() {
					q.TextString("aa")
					q.FillBreakable(", ", 2)
					q.TextString("bb")
					q.FillBreakable(", ", 2)
					q.TextString("cc")
				})
			},
			want: "aa, bb\ncc",
		},
		{
			name:     "genspace_dots",
			maxwidth: 8,
			genspace: dotSpace,
			build: func(q *PrettyPrint) {
				q.Group(2, "[", 1, "]", 1, func() {
					q.TextString("11")
					q.BreakableString()
					q.TextString("22")
					q.BreakableString()
					q.TextString("33")
				})
			},
			want: "[11\n..22\n..33]",
		},
		{
			name:     "custom_newline",
			maxwidth: 6,
			newline:  "<NL>",
			build: func(q *PrettyPrint) {
				q.Group(0, "", 0, "", 0, func() {
					q.TextString("aaa")
					q.BreakableString()
					q.TextString("bbb")
				})
			},
			want: "aaa<NL>bbb",
		},
		{
			name:     "breakable_custom_width",
			maxwidth: 5,
			build: func(q *PrettyPrint) {
				// A breakable whose declared width exceeds its separator forces a
				// break the raw text would not, exercising the width argument.
				q.GroupDefault(func() {
					q.TextString("ab")
					q.Breakable("-", 5)
					q.TextString("cd")
				})
			},
			want: "ab\ncd",
		},
		{
			name:     "text_width_arg",
			maxwidth: 4,
			build: func(q *PrettyPrint) {
				// A wide glyph: one rune, declared 3 columns. The group breaks
				// because the declared widths overflow even though the bytes fit.
				q.GroupDefault(func() {
					q.Text("W", 3)
					q.BreakableString()
					q.Text("W", 3)
				})
			},
			want: "W\nW",
		},
		{
			name:     "empty",
			maxwidth: 79,
			build:    func(q *PrettyPrint) {},
			want:     "",
		},
		{
			name:     "text_only_no_buffer",
			maxwidth: 79,
			build: func(q *PrettyPrint) {
				// Pure text with no pending breakable goes straight to output.
				q.TextString("hello")
				q.TextString(" world")
			},
			want: "hello world",
		},
		{
			name:     "nested_group_alias",
			maxwidth: 79,
			build: func(q *PrettyPrint) {
				q.TextString("x")
				q.NestedGroup(func() {
					q.TextString("y")
					q.BreakableString()
					q.TextString("z")
				})
			},
			want: "xy z",
		},
		{
			name:     "deep_three_levels_overflow",
			maxwidth: 6,
			build: func(q *PrettyPrint) {
				q.Group(1, "(", 1, ")", 1, func() {
					q.TextString("a")
					q.BreakableString()
					q.Group(1, "(", 1, ")", 1, func() {
						q.TextString("b")
						q.BreakableString()
						q.Group(1, "(", 1, ")", 1, func() {
							q.TextString("c")
							q.BreakableString()
							q.TextString("d")
						})
					})
				})
			},
			// Verified against ruby -rprettyprint.
			want: "(a\n (b\n  (c\n   d)))",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			nl := c.newline
			if nl == "" {
				nl = "\n"
			}
			got := Format(c.maxwidth, nl, c.genspace, c.build)
			if got != c.want {
				t.Errorf("%s = %q, want %q", c.name, got, c.want)
			}
		})
	}
}

// TestFormatDefault covers the maxwidth-79 convenience wrapper.
func TestFormatDefault(t *testing.T) {
	got := FormatDefault(func(q *PrettyPrint) {
		q.GroupDefault(func() {
			q.TextString("a")
			q.BreakableString()
			q.TextString("b")
		})
	})
	if got != "a b" {
		t.Errorf("FormatDefault = %q, want %q", got, "a b")
	}
}

// TestManualFlushAndString exercises building without the Format wrapper, the
// explicit Flush, and reading String before/after flushing.
func TestManualFlushAndString(t *testing.T) {
	q := New(79, "\n", nil)
	q.GroupDefault(func() {
		q.TextString("a")
		q.BreakableString()
		q.TextString("b")
	})
	// Still buffered: the breakable and trailing text have not been drained.
	if s := q.String(); s != "a" {
		t.Errorf("pre-flush String = %q, want %q", s, "a")
	}
	q.Flush()
	if s := q.String(); s != "a b" {
		t.Errorf("post-flush String = %q, want %q", s, "a b")
	}
}

// TestBreakableInBrokenGroup forces the `group.break?` branch of Breakable,
// where an already-broken outer group flushes and emits a newline directly.
func TestBreakableInBrokenGroup(t *testing.T) {
	// maxwidth 1 forces the outer group to break immediately; the second
	// breakable then runs while its group is already broken.
	got := Format(1, "\n", nil, func(q *PrettyPrint) {
		q.GroupDefault(func() {
			q.TextString("aa")
			q.BreakableString()
			q.TextString("bb")
			q.BreakableString()
			q.TextString("cc")
		})
	})
	if got != "aa\nbb\ncc" {
		t.Errorf("broken-group breakable = %q, want %q", got, "aa\nbb\ncc")
	}
}

// TestWholeBucketBreak builds two sibling groups at the same depth so deq must
// fall through to the "mark the whole bucket broken and clear it" branch.
func TestWholeBucketBreak(t *testing.T) {
	// Verified against ruby -rprettyprint: two sibling groups under a wide root,
	// each with one breakable, overflowing a width of 4.
	got := Format(4, "\n", nil, func(q *PrettyPrint) {
		q.GroupDefault(func() {
			q.TextString("aa")
			q.BreakableString()
			q.TextString("bb")
		})
		q.GroupDefault(func() {
			q.TextString("cc")
			q.BreakableString()
			q.TextString("dd")
		})
	})
	if got != "aa\nbbcc\ndd" {
		t.Errorf("whole-bucket break = %q, want %q", got, "aa\nbbcc\ndd")
	}
}

// TestGroupDeletedWhenNoBreakables drives group_sub's cleanup path where a group
// with no breakables is deleted from the queue, and the Breakable path that
// deletes an emptied group from the queue on output.
func TestGroupDeletedWhenNoBreakables(t *testing.T) {
	got := Format(79, "\n", nil, func(q *PrettyPrint) {
		// Inner group registers no breakable -> deleted from the queue in defer.
		q.TextString("x")
		q.GroupDefault(func() {
			q.TextString("y")
		})
		// A pending breakable in the buffer so the inner text is buffered, not
		// streamed straight to output (keeps the queue interesting).
		q.GroupDefault(func() {
			q.TextString("z")
			q.BreakableString()
			q.TextString("w")
		})
	})
	if got != "xyz w" {
		t.Errorf("group-delete path = %q, want %q", got, "xyz w")
	}
}

// TestSingleLine covers every SingleLine method, including First and nested
// groups, and confirms breakables never break.
func TestSingleLine(t *testing.T) {
	got := SingleLineFormat(func(q *SingleLine) {
		q.GroupDefault(func() {
			q.TextString("[")
			q.Breakable("", 0)
			q.TextString("1")
			q.BreakableString()
			q.TextString("2")
			q.TextString("]")
		})
	})
	if got != "[1 2]" {
		t.Errorf("SingleLine = %q, want %q", got, "[1 2]")
	}

	// Cover the width-bearing Text/Breakable overloads, Nest, Group open/close,
	// Flush (no-op) and First on nested groups.
	s := NewSingleLine()
	if !s.First() {
		t.Error("first First() should be true")
	}
	if s.First() {
		t.Error("second First() at same level should be false")
	}
	s.Group(2, "<", 1, ">", 1, func() {
		if !s.First() {
			t.Error("First() in fresh inner group should be true")
		}
		s.Nest(4, func() {
			s.Text("t", 9)
			s.Breakable("_", 9)
		})
	})
	s.Flush()
	if got := s.String(); got != "<t_>" {
		t.Errorf("SingleLine nested = %q, want %q", got, "<t_>")
	}
}

// TestGroupSubUnusedParams confirms the rbgo-parity leading parameters of
// GroupSub are accepted and ignored.
func TestGroupSubUnusedParams(t *testing.T) {
	got := Format(79, "\n", nil, func(q *PrettyPrint) {
		q.GroupSub(7, "ignored", 7, "ignored", 7, func() {
			q.TextString("ok")
		})
	})
	if got != "ok" {
		t.Errorf("GroupSub = %q, want %q", got, "ok")
	}
}
