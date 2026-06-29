<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-prettyprint/brand/main/social/go-ruby-prettyprint-prettyprint.png" alt="go-ruby-prettyprint/prettyprint" width="720"></p>

# prettyprint — go-ruby-prettyprint

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-prettyprint.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of Ruby's
[`prettyprint`](https://docs.ruby-lang.org/en/master/PrettyPrint.html) standard
library** — the Wadler/Lindig pretty-printing *engine* that lays out a stream of
text, breakable separators and groups into a width-constrained, nicely indented
document. It is a faithful, **byte-identical** port of MRI 4.0.5's
`prettyprint.rb`: each group prints on one line when it fits within the maximum
width and is otherwise broken at its breakable points, with nesting preserved —
**without any Ruby runtime**.

It is the layout backend for [go-embedded-ruby](https://github.com/go-embedded-ruby/ruby)
(rbgo), a **standalone, reusable** module with no dependency on the Ruby runtime —
a sibling of [go-ruby-yaml](https://github.com/go-ruby-yaml/yaml),
[go-ruby-regexp](https://github.com/go-ruby-regexp/regexp) and
[go-ruby-marshal](https://github.com/go-ruby-marshal/marshal).

> **What it is — and isn't.** This is the *layout engine* only — the deterministic
> group/breakable/indent/width-fitting algorithm (the buffer, the group stack and
> the depth-bucketed group queue). The `pp` object inspector that *uses* it —
> walking an object graph and emitting `text`/`group`/`breakable` calls — is the
> host's job and stays in rbgo. This library is the standalone Go backend `pp`
> binds to.

## Features

Faithful port of `PrettyPrint`, validated against the `ruby` binary on every
supported platform:

- **Groups** that print flat when they fit and break at their breakables when
  they overflow `maxwidth` — the exact `break_outmost_groups` / depth-bucketed
  `GroupQueue` algorithm MRI uses, so nested groups break outermost-first.
- **Breakables** — `breakable(sep, width)` line-break hints that emit their
  separator when the line is not broken, with the `width` argument for multibyte
  or proportional separators.
- **Nesting / indentation** — `nest(indent)` and the `group(indent, …)` indent
  argument, with a pluggable `genspace` block for the indentation string.
- **Open/close text** — `group(indent, open, close)` wrapping the block in
  bracketing text counted toward the fit decision.
- **Fill mode** — `fill_breakable`, where each break is decided individually.
- **The single-line formatter** — `singleline_format`, where breakables become
  their separator text and nothing ever breaks.
- **Custom `newline`** and **custom `maxwidth`**, matching MRI's `PrettyPrint.new`
  / `PrettyPrint.format` signatures.

CGO-free, dependency-free, **100% test coverage**, `gofmt` + `go vet` clean, and
green across the six 64-bit Go targets (amd64, arm64, riscv64, loong64, ppc64le,
s390x) and three operating systems (Linux, macOS, Windows).

## Install

```sh
go get github.com/go-ruby-prettyprint/prettyprint
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/go-ruby-prettyprint/prettyprint"
)

func main() {
	// PrettyPrint.format(''.dup, 10) { |q| ... }
	out := prettyprint.Format(10, "\n", nil, func(q *prettyprint.PrettyPrint) {
		q.Group(2, "[", 1, "]", 1, func() {
			q.TextString("111")
			q.BreakableString()
			q.TextString("222")
			q.BreakableString()
			q.TextString("333")
		})
	})
	fmt.Printf("%q\n", out) // "[111\n  222\n  333]"

	// The single-line formatter never breaks: breakables become their separator.
	flat := prettyprint.SingleLineFormat(func(q *prettyprint.SingleLine) {
		q.GroupDefault(func() {
			q.TextString("[")
			q.Breakable("", 0)
			q.TextString("1")
			q.BreakableString()
			q.TextString("2")
			q.TextString("]")
		})
	})
	fmt.Printf("%q\n", flat) // "[1 2]"
}
```

### API map (MRI → Go)

| Ruby (`PrettyPrint`)                       | Go                                                              |
| ------------------------------------------ | -------------------------------------------------------------- |
| `PrettyPrint.new(out, maxwidth, nl, &gs)`  | `New(maxwidth, newline, genspace)` / `NewDefault()`            |
| `PrettyPrint.format(...)`                  | `Format(maxwidth, newline, genspace, fn)` / `FormatDefault`    |
| `PrettyPrint.singleline_format(...)`       | `SingleLineFormat(fn)`                                          |
| `#text(obj, width)`                        | `Text(obj, width)` / `TextString(obj)`                         |
| `#breakable(sep, width)`                   | `Breakable(sep, width)` / `BreakableString()`                  |
| `#group(indent, open, close, ow, cw)`      | `Group(indent, open, ow, close, cw, fn)` / `GroupDefault(fn)`  |
| `#group_sub`                               | `GroupSub(...)` / `NestedGroup(fn)`                            |
| `#nest(indent)`                            | `Nest(indent, fn)`                                             |
| `#fill_breakable(sep, width)`              | `FillBreakable(sep, width)` / `FillBreakableString()`          |
| `#current_group`                           | `CurrentGroup()`                                               |
| `#break_outmost_groups`                    | `BreakOutmostGroups()`                                         |
| `#flush`                                   | `Flush()`                                                      |
| `PrettyPrint::VERSION`                     | `VERSION`                                                       |

The default `genspace` is `DefaultGenSpace` (`n` ASCII spaces); pass `nil` to
`New` / `Format` to use it.

## Tests & coverage

The suite is **100% statement-covered** and combines two layers:

- **Deterministic, ruby-free tests** — golden programs with byte-exact expected
  output (verified against `ruby -rprettyprint`), plus white-box tests for the
  group-queue terminal arms. These alone hold the 100% gate on every platform,
  including Windows and the qemu cross-arch lanes where no `ruby` is present.
- **MRI differential oracle** — `oracle_test.go` runs a shared corpus through
  both this package and a generated `PrettyPrint.format` / `singleline_format`
  script and asserts the output is identical. It binmodes stdout/stdin (so
  Windows text-mode never rewrites the bytes), skips when `ruby` is absent, and
  gates on `RUBY_VERSION >= "4.0"`.

```sh
go test -race -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # total: (statements) 100.0%
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-prettyprint/prettyprint authors.
