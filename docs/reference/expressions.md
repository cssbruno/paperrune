# Expressions

Paper expressions calculate property values, control visibility, and select
components from declared data. Expressions are unquoted; string literals
inside them remain quoted.

```paper
text: customer.name
visible: customer.active
color: customer.active ? "#16803A" : "#B42318"
```

## Names and literals

A bare name or dotted path reads data declared by the schema. Paper does not
use a `$data` prefix and does not treat an unknown name as text.

```paper
text: title       # field named title
text: "title"     # literal word title
visible: active   # boolean field named active
```

The property name on the left does not create a variable. In `text: title`,
`text` is the receiving property and `title` is the data path.

| Value | Examples |
| --- | --- |
| Boolean | `true`, `false`, `account.active` |
| String | `"Paid"`, `patient.name` |
| Number | `12`, `-3.5`, `invoice.total` |
| Unit | `12pt`, `8.5mm`, `100%`, `1fr` |
| Component | `@paid-card` |
| Absence | `null` |

Strings use JSON-style double quotes and escapes. Component references are
unquoted and may only name declared components.

## Types belong to properties

The receiving property determines the required result type. Paper does not
convert between strings, numbers, booleans, units, or component references.

Given a boolean field named `active`:

```paper
visible: active                  # valid: bool
bold: active                     # valid: bool
text: active                     # error: text requires string
text: active ? "Yes" : "No"     # valid: string
```

All branches of a conditional must return compatible types. Pairing a value
with `null` is allowed only when the receiving property supports absence.

## Operators

From highest to lowest precedence:

| Operation | Syntax |
| --- | --- |
| Grouping | `(expression)` |
| Unary | `!value`, `-value` |
| Multiply and divide | `*`, `/` |
| Add and subtract | `+`, `-` |
| Ordering | `<`, `<=`, `>`, `>=` |
| Equality and pattern matching | `==`, `!=`, `matches` |
| Boolean AND | `&&` |
| Boolean OR | `||` |
| Conditional | `condition ? whenTrue : whenFalse` |

`+` concatenates two strings or adds two compatible numeric values. It never
converts another type to a string. `matches` performs bounded wildcard matching
on two strings.

```paper
visible: enabled && name matches "Ada*"
text: firstName + " " + lastName
width: (columnCount * 24pt) + 12pt
```

Use parentheses when the intended grouping is not immediately obvious.

## Visibility

`visible` requires a non-null boolean:

```paper
paragraph @confidential:
  visible: user.canViewConfidential
  text: "Confidential information"
```

When the result is `false`, the node and its descendants do not exist in the
rendered document. Paper still checks their expression syntax and schema paths,
but does not evaluate their runtime expressions or load their assets.

`visible: null` and a missing required visibility value are errors. After
removal, Paper rechecks structural rules such as table spans and required page
content.

## Ternary selection

Use a ternary for two outcomes:

```paper
text: paid ? "Paid" : "Payment pending"
size: compact ? 10pt : 12pt
```

Ternaries associate from the right:

```paper
text: status == "paid" ? "Paid" : status == "overdue" ? "Overdue" : "Open"
```

Both branches are parsed and type-checked. Only the selected branch runs, so a
guard can safely prevent an invalid runtime operation:

```paper
width: count != 0 ? total / count * 1pt : 0pt
```

## Switch selection

Use a value switch for three or more outcomes based on one value:

```paper
paragraph @status-label:
  text: switch status:
    case "active": "Active"
    case "suspended": "Suspended"
    case "closed": "Closed"
    default: "Unknown"
```

Use a predicate switch when each case has its own boolean condition:

```paper
paragraph @risk-label:
  text: switch:
    case riskScore >= 90: "Critical"
    case riskScore >= 60: "Review"
    case riskScore >= 1: "Low"
    default: "None"
```

The first matching case wins. Switches do not fall through and have no
`break`. Every switch requires one final `default`. Cases after `default`,
duplicate literal cases, non-boolean predicate cases, and incompatible result
types are errors.

## Component selection

A `use` may select a declared component:

```paper
use @payment-message:
  component: paid ? @paid-message : @pending-message
```

Return `null` to render no component:

```paper
use @optional-warning:
  component: overdue ? @overdue-warning : null
```

Every referenced component is checked, including unselected alternatives.
Paper rejects unknown components, incompatible contracts, and cycles. Data
cannot construct a component name dynamically.

## Optional data and null guards

An absent optional schema field or optional component argument resolves to
`null`. Operations on a nullable value require a guard:

```paper
text: nickname != null ? nickname : fullName
visible: nickname != null && nickname matches "A*"
```

The compiler narrows a path where the condition proves it is non-null:

- the true branch of `path != null`;
- the false branch of `path == null`;
- the safe right-hand side of `&&` or `||`;
- the corresponding branch of a ternary.

An arbitrary condition does not narrow a nullable path. Arithmetic, ordering,
matching, and boolean operations on `null` are errors. Equality is defined:
`null == null` is `true`, and `null` compared with a non-null value is unequal.

## Numbers and units

Expression numbers are exact decimals with at most nine fractional places.
They use canonical notation: no leading `+`, no unnecessary leading zero, and
no trailing fractional zero.

```paper
width: 12.5pt       # valid
width: 12.50pt      # error: non-canonical trailing zero
width: 01pt         # error: non-canonical leading zero
```

The supported unit suffixes are `pt`, `mm`, `cm`, `in`, `px`, `pc`, `em`,
`rem`, `vh`, `vw`, `%`, and `fr`.

Allowed calculations:

- add or subtract identical units;
- convert between `in`, `cm`, `mm`, `pt`, `pc`, and `px` when the result is
  exactly representable;
- multiply a number by a unit in either order;
- divide a unit by a number.

Multiplying two units, dividing by a unit, mixing unrelated units, overflow,
division by zero, and results requiring rounding are errors. For example,
`1 / 2` produces `0.5`, while `1 / 3` fails because it cannot be represented
exactly within nine decimal places.

Context-dependent units such as `%`, `fr`, `em`, `rem`, `vh`, and `vw` combine
only with the same suffix. Expressions cannot read calculated page or layout
measurements.

## Data scopes

Only paths declared in the current scope are available:

| Context | Paths |
| --- | --- |
| Document data | bare schema paths such as `patient.name` |
| Repeat item | `item.name`, `item.active` |
| Loop | `loop.index`, `loop.first`, `loop.last` |
| Component | declared arguments such as `args.title` |

Unknown, misspelled, or inaccessible paths are compile errors. There are no
ambient variables or implicit locals.

## Failure behavior

Expression errors stop compilation or rendering. Paper does not silently use a
switch default, substitute `null`, retain a conditionally invalid node, or
coerce a value to the receiving property type.

Diagnostics identify the expression location and distinguish syntax, path,
type, nullability, component, arithmetic, unit, structure, limit, and
cancellation failures.
