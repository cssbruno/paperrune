# ADR 0004: Typed data expressions and conditional selection

Status: accepted

The release-focused core described here is implemented by grammar `paper/0.4`:
unquoted typed expressions, lazy boolean/ternary evaluation, preserved value
and predicate switches, computed scalar values, `visible`, `item`/`loop`/`args`
scopes, scenario-neutral checking, and deferred component selection including
`null`. General decimal/unit calculation and advanced nullable schema-flow
analysis remain outside the release scope; they must not be approximated with
binary floating point or implicit conversions if introduced later.

## Context

The existing `when: "..."` form only decides whether a node is retained. A
ternary such as `active ? true : false` does not add useful behavior because it
returns the boolean value it started with. A useful expression system must be
able to calculate property values, select components, and deliberately render
nothing.

`active` is not a Paper keyword and does not replace `true` or `false`. It is an
example name for a boolean field declared by the document schema and supplied
by the document data:

```paper
schema:
  bool active
```

If `active` contains `true` or `false`, these expressions are equivalent:

```paper
visible: active
visible: active == true
visible: active ? true : false
```

The first form is preferred. Ternaries should select meaningfully different
results instead of repeating a boolean.

## Decision

Paper should use typed, unquoted expressions anywhere a scalar property is
expression-capable. The property declares the expected result type. String
literals inside an expression remain quoted.

```paper
paragraph @payment-status:
  text: invoice.paid ? "Paid" : "Payment pending"
  color: invoice.paid ? "#16803A" : "#B42318"
  size: compact ? 10pt : 12pt
```

An expression is deterministic and has no access to functions, files, the
network, clocks, randomness, processes, reflection, or mutable state.

## Names, literals, and references

Paper does not use a `$data` prefix. A bare name or dotted path in an expression
is a data reference, and the compiler must prove that it is declared in the
current schema or structural scope.

```paper
text: patient.name
visible: patient.active
```

Literal strings remain quoted, so a field name and a literal word are always
distinguishable:

```paper
text: text       # read the schema field named text
text: "text"     # use the literal word "text"
```

The property to the left of `:` does not participate in name resolution. In
`text: text`, the first `text` is the property and the second `text` is the
schema field. The receiving property still enforces its result type. Given:

```paper
schema:
  bool text
```

these uses have different outcomes:

```paper
visible: text                  # valid: visible expects bool
bold: text                     # valid: bold expects bool
text: text                     # error: text expects string
text: text ? "true" : "false" # valid: the expression returns string
```

Other literal and reference forms remain syntactically distinct:

```paper
visible: true       # boolean literal
width: 12pt         # unit literal
component: @summary # component-reference literal
```

There are no ambient variables and no implicit local variables. Names come
only from a declared scope:

- root schema fields use their bare paths, such as `patient.name`;
- a repeat declares `item`, such as `item.name` and `item.active`;
- a loop declares `loop`, such as `loop.index` and `loop.first`;
- component properties declare `args`, such as `args.title`.

`item`, `loop`, and `args` are reserved structural scope names. Schema fields
may not use those names at the scope where they would conflict. An unknown,
misspelled, or inaccessible bare path is a compile error; Paper never treats it
as an unquoted string.

## Visibility

The `visible` property accepts a boolean expression:

```paper
paragraph @confidential-note:
  visible: user.canViewConfidential
  text: "Confidential information"
```

Paper evaluates `visible` before evaluating the node's other runtime property
expressions. When it is `false`, the node and its descendants are absent as if
they had not been authored. Their expression syntax and schema paths are still
checked statically, but their runtime bindings and assets are not loaded.

`visible` may be used on renderable nodes, component uses, page breaks, table
rows, and table cells. It is not valid on schemas, scenarios, components,
themes, styles, tokens, or the document root. After removal, Paper revalidates
structural invariants such as table spans and required page content. A document
with no visible page is an error rather than a zero-page output.

`visible: null` is an error. Missing visibility data never silently means
`false`.

## Component selection

A `use` may select between component definitions:

```paper
use @payment-message:
  component: invoice.paid ? @paid-message : @pending-message
```

It may explicitly render nothing:

```paper
use @optional-warning:
  component: invoice.overdue ? @overdue-warning : null
```

An `@reference` inside an expression is a closed component-reference literal,
not a general pointer to any authored node. Selecting arbitrary sibling IDs is
not allowed because it would create ambiguous ownership, duplicate rendering,
unstable provenance, and difficult reference cycles.

Every referenced component must exist and be statically valid. Alternatives
used by one `use` must have compatible slot and fill contracts. Reference cycles
are rejected across every possible branch, including branches not selected by
the current data. Only the selected component is expanded and only its assets
and runtime bindings are evaluated. The resulting instance retains the `use`
node's stable identity and records the selected definition in provenance.

## Ternary expressions

The conditional operator is right-associative:

```paper
component: status == "paid" ? @paid : status == "overdue" ? @overdue : null
```

Its condition must be boolean. Its result branches must have the same type,
except that `null` may be paired with a value where the receiving property
allows absence.

Both result branches are parsed, bounded, reference-checked, and type-checked.
Only the selected branch executes. An error in an unselected runtime
calculation therefore does not fail the render:

```paper
text: useRatio ? total / count : "Not calculated"
```

The example above is still a static type error because its branches return a
number and a string. Lazy execution does not weaken static typing. A valid lazy
example is:

```paper
width: count != 0 ? total / count * 1pt : 0pt
```

When `count` is zero, the division branch is not evaluated.

## Switch expressions

Ternaries are intended for two outcomes. A typed `switch` expression is the
readable form for three or more outcomes.

### Value switch

A value switch evaluates its selector exactly once and compares cases using
typed equality:

```paper
use @status-message:
  component: switch account.status:
    case "active": @active-card
    case "suspended": @suspended-card
    case "closed": @closed-card
    default: null
```

It also works for ordinary properties:

```paper
paragraph @status-label:
  text: switch account.status:
    case "active": "Active"
    case "suspended": "Suspended"
    case "closed": "Closed"
    default: "Unknown"
```

### Predicate switch

A switch without a selector evaluates boolean cases in source order:

```paper
paragraph @risk-label:
  text: switch:
    case riskScore >= 90: "Critical"
    case riskScore >= 60: "Review"
    case riskScore >= 1: "Low"
    default: "None"
```

Switches never fall through and do not have `break`. The first matching case is
selected, and only that result is evaluated. Case tests stop after a match.
Every switch requires exactly one final `default`; callers may write
`default: null` when absence is intentional. Duplicate literal cases,
non-boolean predicate cases, cases after `default`, and incompatible result
types are errors.

## Types and operators

The expression type system contains:

- `bool`
- `string`
- bounded deterministic decimal `number`
- unit values such as `pt`, `mm`, `%`, and `fr`
- component references
- `null`

The initial operator set is:

- parentheses and unary `!` and `-`
- multiplication and division: `*`, `/`
- addition and subtraction: `+`, `-`
- comparisons: `<`, `<=`, `>`, `>=`
- equality: `==`, `!=`
- string pattern comparison: `matches`
- boolean operations: `&&`, `||`
- conditional selection: `?:`

There are no implicit conversions. A string is never automatically converted
to a number, boolean, unit, or component reference. String concatenation with
`+` requires two strings.

Numbers use a bounded decimal representation with explicit precision and scale
limits. Arithmetic overflow, precision overflow, and division by zero are
errors. The implementation must not use platform-dependent binary floating
point as the expression language's semantic representation.

Unit selection is allowed when both results are valid for the receiving
property. Arithmetic may add or subtract compatible absolute dimensions and
may multiply or divide a dimension by a number. Context-dependent units such as
`em`, `%`, `fr`, `vh`, and `vw` cannot be mixed with unrelated units during
expression evaluation. Expressions cannot read measured layout sizes, page
counts, or other layout results, preventing evaluation/layout cycles.

## Null and missing data

`null` does not mean "hide the current node" in every context:

- `component: expression ? @card : null` renders no component.
- An optional property that evaluates to `null` behaves as if that property
  were not authored.
- A required property that evaluates to `null` is an error.
- `visible: null` is an error.
- `null == null` is `true`; equality between `null` and another value is
  `false`.
- Arithmetic, ordering, matching, or boolean operations on `null` are errors.

An undeclared path is always a compile error. An optional declared field that
is absent resolves to `null`. Nullable types must remain visible to static type
checking so guarded expressions can be written safely:

```paper
text: patient.nickname != null ? patient.nickname : patient.name
```

## Evaluation order and scopes

Boolean `&&` and `||`, ternaries, and switch results use real short-circuit
execution. The bytecode must use bounded jumps rather than evaluating every
operand before selecting a value.

Expressions resolve against the scope where they are instantiated:

- top-level nodes use bare schema-root paths such as `patient.name`;
- repeated nodes use the declared `item` scope such as `item.name`;
- loops receive `loop.index`, `loop.first`, and `loop.last`;
- component instances use declared `args` paths;
- nested repeats and components inherit only the paths already allowed by
  their explicit binding contracts.

Dynamic component selection must occur after the data scope is known but before
the selected component is expanded. This requires the scenario, repeat, and
component-lowering pipeline to preserve a stable source identity while
deferring data-dependent component expansion.

## Parsing and formatting

Expressions have no outer quotes:

```paper
visible: account.status == "active"
```

The quotes above belong only to the string literal `"active"`. Inline comments
begin with `#` only outside expression string literals. A ternary remains on one
physical line in the initial grammar. A switch owns an indented `case` block.

The AST must represent expressions separately from strings. Editors must never
store an expression in a string scalar or write arbitrary unvalidated source.
The lexer, parser, formatter, CST patcher, semantic editing API, and Studio must
share the same bounded expression validation.

Canonical formatting preserves expression meaning, uses stable indentation for
switch cases, places `default` last, and remains parse-format-parse stable.

## Diagnostics and failure policy

Expression failures are fail-closed. Paper does not silently select `default`,
return `null`, retain a hidden node, or substitute a static property when an
expression is invalid.

Diagnostics must identify the exact expression or case and distinguish:

- invalid syntax;
- unknown or inaccessible data paths;
- missing required runtime values;
- nullable misuse;
- condition or branch type mismatch;
- invalid or incompatible component references;
- component cycles or slot-contract mismatch;
- duplicate, unreachable, or missing switch cases;
- arithmetic overflow, precision loss, or division by zero;
- incompatible units;
- invalid final structure after nodes are removed;
- configured source, token, depth, instruction, stack, string, case, expansion,
  or work limits;
- cancellation.

All possible branches are statically checked. Only selected branches perform
runtime evaluation or resource loading.

## Limits and determinism

Existing expression and expansion limits remain mandatory. Switches add a
bounded maximum case count, and control-flow bytecode adds bounded jump targets
and total executed work. Nested ternaries and nested switches count against the
same source depth and node limits.

Evaluation order, decimal arithmetic, case ordering, component expansion,
diagnostics, source mapping, and generated output must be deterministic for the
same Paper source, schema, data, assets, locale, and compiler limits.

## Non-goals

- Arbitrary code execution or user-defined functions
- Reflection or dynamic property names
- Selecting arbitrary authored sibling IDs
- Fallthrough switches
- Implicit type conversion
- Reading layout measurements from expressions
- Silently recovering from invalid or missing data

## Implementation sequence

Implementation proceeds in independently tested stages:

1. expression types and lazy control flow;
2. expression/switch lexer, AST, parser, formatter, and CST support;
3. typed source-edit values and Studio integration;
4. computed scalar properties and `visible`;
5. deferred component selection and provenance;
6. repeat/loop/component scope integration;
7. release diagnostics, determinism tests, fuzzing, and full regression
   verification.
