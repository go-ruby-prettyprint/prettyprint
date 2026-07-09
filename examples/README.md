# prettyprint examples

Runnable pure-Ruby usage of the `prettyprint` layout engine, verified under the [rbgo](https://github.com/go-embedded-ruby) interpreter.

```sh
rbgo examples/prettyprint_usage.rb
```

| File | Shows |
| --- | --- |
| `prettyprint_usage.rb` | Lay out a document with `PrettyPrint.format` (flat when it fits, broken and indented when it overflows `maxwidth`), a comma-separated `#group` with `#breakable` points, `PrettyPrint.singleline_format`, and incremental building with `.new` / `#nest` / `#flush` / `#output`. |
