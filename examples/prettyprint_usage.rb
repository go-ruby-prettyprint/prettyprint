# frozen_string_literal: true

require "prettyprint"

# PrettyPrint.format(output, maxwidth) yields a builder. A group prints on one
# line when it fits within maxwidth and otherwise breaks at its breakables.
flat = PrettyPrint.format(+"", 40) do |q|
  q.group(2, "[", "]") do
    q.text "111"
    q.breakable
    q.text "222"
  end
end
puts flat # => [111 222]

# The same document with a narrow width breaks and indents by the group's nest.
broken = PrettyPrint.format(+"", 10) do |q|
  q.group(2, "[", "]") do
    %w[apple banana cherry].each_with_index do |w, i|
      q.text "," unless i.zero?
      q.breakable
      q.text w
    end
  end
end
puts broken # => [\n  apple,\n  banana,\n  cherry]

# singleline_format never breaks: every breakable becomes its separator text.
puts PrettyPrint.singleline_format(+"") { |q| q.text "a"; q.breakable; q.text "b" } # => a b

# Build incrementally with .new / #nest / #flush, then read #output.
q = PrettyPrint.new(+"", 20)
q.text "def foo"
q.nest(2) { q.breakable; q.text "bar" }
q.flush
puts q.output # => def foo bar (fits within maxwidth 20)
