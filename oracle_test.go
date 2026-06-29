// Copyright (c) the go-ruby-prettyprint/prettyprint authors
//
// SPDX-License-Identifier: BSD-3-Clause

package prettyprint

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// rubyBin locates a usable `ruby` once, skipping the oracle when it is absent
// (the qemu cross-arch lanes and the Windows lane). The deterministic suite alone
// drives the 100% gate there.
func rubyBin(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH; skipping MRI prettyprint oracle")
	}
	// Gate on RUBY_VERSION >= "4.0": prettyprint's layout is stable, but we pin
	// the byte-for-byte oracle to the MRI line this port targets (4.0.5).
	out, err := exec.Command(path, "-e", `print(RUBY_VERSION >= "4.0" ? "y" : "n")`).Output()
	if err != nil || strings.TrimSpace(string(out)) != "y" {
		t.Skipf("ruby is not >= 4.0 (%q); skipping MRI oracle", strings.TrimSpace(string(out)))
	}
	return path
}

// rubyRun evaluates a prettyprint script and returns stdout. The script binmodes
// both stdout and stdin so Windows text-mode never rewrites the bytes (the
// go-ruby-erb lesson); the harness still skips Windows, but the preamble keeps
// the contract explicit and portable.
func rubyRun(t *testing.T, bin, script string) string {
	t.Helper()
	preamble := "$stdout.binmode\n$stdin.binmode\nrequire 'prettyprint'\n"
	cmd := exec.Command(bin, "-e", preamble+script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ruby error: %v\nscript:\n%s\noutput:\n%s", err, script, out)
	}
	return string(out)
}

// op is one builder instruction in the oracle DSL. Each case is a program of ops
// that both this package and a generated Ruby script execute, so the outputs are
// compared byte-for-byte.
type op struct {
	kind  string // text|breakable|fill|group|groupcc|nest
	s     string // text / separator / open
	close string // group close
	w     int    // declared width / indent
	cw    int    // close width
	body  []op   // nested ops for group/nest
}

func tx(s string) op            { return op{kind: "text", s: s} }
func tw(s string, w int) op     { return op{kind: "textw", s: s, w: w} }
func br() op                    { return op{kind: "breakable"} }
func brs(s string, w int) op    { return op{kind: "breakablew", s: s, w: w} }
func fill() op                  { return op{kind: "fill"} }
func fillw(s string, w int) op  { return op{kind: "fillw", s: s, w: w} }
func grp(body ...op) op         { return op{kind: "group", body: body} }
func grpI(i int, body ...op) op { return op{kind: "groupi", w: i, body: body} }
func grpCC(i int, open, close string, body ...op) op {
	return op{kind: "groupcc", w: i, s: open, close: close, cw: len(close), body: body}
}
func nest(i int, body ...op) op { return op{kind: "nest", w: i, body: body} }

// applyGo executes a program against a PrettyPrint buffer.
func applyGo(q *PrettyPrint, ops []op) {
	for _, o := range ops {
		switch o.kind {
		case "text":
			q.TextString(o.s)
		case "textw":
			q.Text(o.s, o.w)
		case "breakable":
			q.BreakableString()
		case "breakablew":
			q.Breakable(o.s, o.w)
		case "fill":
			q.FillBreakableString()
		case "fillw":
			q.FillBreakable(o.s, o.w)
		case "group":
			q.GroupDefault(func() { applyGo(q, o.body) })
		case "groupi":
			q.Group(o.w, "", 0, "", 0, func() { applyGo(q, o.body) })
		case "groupcc":
			q.Group(o.w, o.s, len(o.s), o.close, o.cw, func() { applyGo(q, o.body) })
		case "nest":
			q.Nest(o.w, func() { applyGo(q, o.body) })
		}
	}
}

// applySingle executes a program against a SingleLine buffer.
func applySingle(q *SingleLine, ops []op) {
	for _, o := range ops {
		switch o.kind {
		case "text":
			q.TextString(o.s)
		case "textw":
			q.Text(o.s, o.w)
		case "breakable":
			q.BreakableString()
		case "breakablew":
			q.Breakable(o.s, o.w)
		case "fill", "fillw":
			// SingleLine has no fill_breakable; MRI's pp never calls it on a
			// SingleLine. Treat as a plain breakable for parity with text output.
			if o.s != "" {
				q.Breakable(o.s, o.w)
			} else {
				q.BreakableString()
			}
		case "group":
			q.GroupDefault(func() { applySingle(q, o.body) })
		case "groupi":
			q.Group(o.w, "", 0, "", 0, func() { applySingle(q, o.body) })
		case "groupcc":
			q.Group(o.w, o.s, len(o.s), o.close, o.cw, func() { applySingle(q, o.body) })
		case "nest":
			q.Nest(o.w, func() { applySingle(q, o.body) })
		}
	}
}

// rubyScript renders a program as Ruby prettyprint calls, for the format oracle.
func rubyScript(ops []op, varName string) string {
	var b strings.Builder
	for _, o := range ops {
		switch o.kind {
		case "text":
			b.WriteString(varName + ".text(" + rubyStr(o.s) + ")\n")
		case "textw":
			b.WriteString(varName + ".text(" + rubyStr(o.s) + ", " + strconv.Itoa(o.w) + ")\n")
		case "breakable":
			b.WriteString(varName + ".breakable\n")
		case "breakablew":
			b.WriteString(varName + ".breakable(" + rubyStr(o.s) + ", " + strconv.Itoa(o.w) + ")\n")
		case "fill":
			b.WriteString(varName + ".fill_breakable\n")
		case "fillw":
			b.WriteString(varName + ".fill_breakable(" + rubyStr(o.s) + ", " + strconv.Itoa(o.w) + ")\n")
		case "group":
			b.WriteString(varName + ".group {\n" + rubyScript(o.body, varName) + "}\n")
		case "groupi":
			b.WriteString(varName + ".group(" + strconv.Itoa(o.w) + ") {\n" + rubyScript(o.body, varName) + "}\n")
		case "groupcc":
			b.WriteString(varName + ".group(" + strconv.Itoa(o.w) + ", " + rubyStr(o.s) + ", " + rubyStr(o.close) + ") {\n" + rubyScript(o.body, varName) + "}\n")
		case "nest":
			b.WriteString(varName + ".nest(" + strconv.Itoa(o.w) + ") {\n" + rubyScript(o.body, varName) + "}\n")
		}
	}
	return b.String()
}

// rubyStr renders a Go string as a Ruby double-quoted literal for the small,
// printable separators used in the corpus.
func rubyStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString("\\n")
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// corpus is the shared set of programs exercised by the format and singleline
// oracles. It spans groups that fit and overflow, nesting/indentation, custom
// separators and widths, and fill mode.
func corpus() []struct {
	name string
	ops  []op
} {
	return []struct {
		name string
		ops  []op
	}{
		{"flat_text", []op{tx("hello"), tx(" "), tx("world")}},
		{"group_fits", []op{grp(tx("["), brs("", 0), tx("1"), br(), tx("2"), brs("", 0), tx("]"))}},
		{"group_overflow", []op{grpI(2, tx("["), brs("", 0), tx("111"), br(), tx("222"), br(), tx("333"), brs("", 0), tx("]"))}},
		{"group_open_close", []op{grpCC(2, "[", "]", tx("111"), br(), tx("222"), br(), tx("333"))}},
		{"nested_fit", []op{grpCC(2, "{", "}", tx("a"), br(), grpCC(2, "[", "]", tx("1"), br(), tx("2")), br(), tx("b"))}},
		{"nested_outer_break", []op{grpCC(2, "{", "}", tx("a"), br(), grpCC(2, "[", "]", tx("1"), br(), tx("2")), br(), tx("b"))}},
		{"nest", []op{tx("begin"), nest(4, br(), tx("body1"), br(), tx("body2")), br(), tx("end")}},
		{"fill", []op{grp(tx("1"), fill(), tx("22"), fill(), tx("333"), fill(), tx("4444"), fill(), tx("55555"), fill(), tx("6"))}},
		{"fill_sep", []op{grp(tx("aa"), fillw(", ", 2), tx("bb"), fillw(", ", 2), tx("cc"))}},
		{"custom_breakable", []op{grp(tx("ab"), brs(" | ", 3), tx("cd"), brs(" | ", 3), tx("ef"))}},
		{"text_width", []op{grp(tw("W", 3), br(), tw("W", 3), br(), tw("W", 3))}},
		{"deep3", []op{grpCC(1, "(", ")", tx("a"), br(), grpCC(1, "(", ")", tx("b"), br(), grpCC(1, "(", ")", tx("c"), br(), tx("d"))))}},
		{"siblings", []op{grp(tx("aa"), br(), tx("bb")), grp(tx("cc"), br(), tx("dd"))}},
		{"broken_then_text", []op{grp(tx("a"), br(), tx("verylongtext"))}},
	}
}

// widths pairs each corpus case with a maxwidth that produces a representative
// fit-or-overflow layout.
func widths() map[string]int {
	return map[string]int{
		"flat_text": 79, "group_fits": 79, "group_overflow": 10, "group_open_close": 10,
		"nested_fit": 15, "nested_outer_break": 8, "nest": 10, "fill": 10, "fill_sep": 6,
		"custom_breakable": 8, "text_width": 4, "deep3": 6, "siblings": 4, "broken_then_text": 5,
	}
}

// TestOracleFormat compares PrettyPrint.format output against MRI for every
// corpus program at its chosen width.
func TestOracleFormat(t *testing.T) {
	bin := rubyBin(t)
	w := widths()
	for _, c := range corpus() {
		c := c
		t.Run(c.name, func(t *testing.T) {
			mw := w[c.name]
			got := Format(mw, "\n", nil, func(q *PrettyPrint) { applyGo(q, c.ops) })

			script := "out = PrettyPrint.format(''.dup, " + strconv.Itoa(mw) + ") { |q|\n" +
				rubyScript(c.ops, "q") + "}\nprint out\n"
			want := rubyRun(t, bin, script)

			if got != want {
				t.Errorf("format[%s] width=%d\n go = %q\nruby = %q", c.name, mw, got, want)
			}
		})
	}
}

// TestOracleSingleLine compares PrettyPrint.singleline_format output against MRI:
// breakables become their separator and groups never break, so width is moot.
func TestOracleSingleLine(t *testing.T) {
	bin := rubyBin(t)
	for _, c := range corpus() {
		c := c
		// fill_breakable is undefined on SingleLine in MRI; skip the fill cases.
		if strings.HasPrefix(c.name, "fill") {
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			got := SingleLineFormat(func(q *SingleLine) { applySingle(q, c.ops) })

			script := "out = PrettyPrint.singleline_format(''.dup) { |q|\n" +
				rubyScript(c.ops, "q") + "}\nprint out\n"
			want := rubyRun(t, bin, script)

			if got != want {
				t.Errorf("singleline[%s]\n go = %q\nruby = %q", c.name, got, want)
			}
		})
	}
}

// TestOracleGenSpace checks a non-default genspace (dots) matches MRI, covering
// the genspace plumbing end-to-end against the interpreter.
func TestOracleGenSpace(t *testing.T) {
	bin := rubyBin(t)
	ops := []op{grpCC(2, "[", "]", tx("11"), br(), tx("22"), br(), tx("33"))}
	got := Format(8, "\n", dotSpace, func(q *PrettyPrint) { applyGo(q, ops) })

	script := "out = PrettyPrint.format(''.dup, 8, \"\\n\", lambda {|n| \".\" * n}) { |q|\n" +
		rubyScript(ops, "q") + "}\nprint out\n"
	want := rubyRun(t, bin, script)
	if got != want {
		t.Errorf("genspace\n go = %q\nruby = %q", got, want)
	}
}

// TestOracleCustomNewline checks a non-"\n" newline string matches MRI.
func TestOracleCustomNewline(t *testing.T) {
	bin := rubyBin(t)
	ops := []op{grpI(2, tx("aaaa"), br(), tx("bbbb"), br(), tx("cccc"))}
	got := Format(6, "<NL>", nil, func(q *PrettyPrint) { applyGo(q, ops) })

	script := "out = PrettyPrint.format(''.dup, 6, \"<NL>\") { |q|\n" +
		rubyScript(ops, "q") + "}\nprint out\n"
	want := rubyRun(t, bin, script)
	if got != want {
		t.Errorf("custom newline\n go = %q\nruby = %q", got, want)
	}
}
