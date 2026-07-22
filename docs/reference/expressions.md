# Expressions

Expressions are unquoted; string literals remain quoted.

```paper
text: customer.name
visible: customer.active
color: customer.active ? "#16803A" : "#B42318"
```

## Names and literals

Bare names and dotted paths read schema data. There is no `$data` prefix.

```paper
text: title       # field named title
text: "title"     # literal word title
visible: active   # boolean field named active
```

In `text: title`, `text` is the property and `title` is the data path.

| Value | Examples |
| --- | --- |
| Boolean | `true`, `false`, `account.active` |
| String | `"Paid"`, `patient.name` |
| Number | `12`, `-3.5`, `invoice.total` |
| Unit | `12pt`, `8.5mm`, `100%`, `1fr` |
| Component | `@paid-card` |
| Absence | `null` |

Strings use JSON escapes. Component references are unquoted.

## Types belong to properties

The receiving property sets the result type; values are not coerced.

Given a boolean field named `active`:

```paper
visible: active                  # valid: bool
bold: active                     # valid: bool
text: active                     # error: text requires string
text: active ? "Yes" : "No"     # valid: string
```

Conditional branches must have compatible types. `null` requires a property
that supports absence.

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

`+` joins strings or adds compatible numbers; it never coerces. `matches`
performs bounded wildcard matching on strings.

```paper
visible: enabled && name matches "Ada*"
text: firstName + " " + lastName
width: (columnCount * 24pt) + 12pt
```

## Visibility

`visible` requires a non-null boolean:

```paper
paragraph @confidential:
  visible: user.canViewConfidential
  text: "Confidential information"
```

When false, the subtree is removed. Its syntax and paths are still checked;
runtime expressions and assets are skipped.

`null` or missing visibility is an error. Structural rules are checked again
after removal.

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

Both branches are type-checked; only the selected branch runs:

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

The first match wins. There is no fallthrough or `break`. A final `default` is
required. Duplicate literals, cases after `default`, non-boolean predicates,
and incompatible results are errors.

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

All alternatives are checked. Unknown components, incompatible contracts, and
cycles are errors. Data cannot construct component names.

## Optional data and null guards

Missing optional values resolve to `null` and require guards:

```paper
text: nickname != null ? nickname : fullName
visible: nickname != null && nickname matches "A*"
```

The compiler narrows a path where the condition proves it is non-null:

- the true branch of `path != null`;
- the false branch of `path == null`;
- the safe right-hand side of `&&` or `||`;
- the corresponding branch of a ternary.

Other conditions do not narrow. Operations on `null` are errors except
equality: `null == null` is true; a non-null value is unequal.

## Numbers and units

Numbers are exact decimals with at most nine fractional places. They forbid a
leading `+`, unnecessary leading zero, and trailing fractional zero.

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

Invalid operations include multiplying units, dividing by a unit, mixing
unrelated units, overflow, division by zero, and rounding. `1 / 2` is `0.5`;
`1 / 3` fails the nine-place exactness limit.

`%`, `fr`, `em`, `rem`, `vh`, and `vw` combine only with the same suffix.
Expressions cannot read calculated layout measurements.

## Data scopes

Only paths declared in the current scope are available:

| Context | Paths |
| --- | --- |
| Document data | bare schema paths such as `patient.name` |
| Repeat item | `item.name`, `item.active` |
| Loop | `loop.index`, `loop.first`, `loop.last` |
| Component | declared arguments such as `args.title` |

Unknown or inaccessible paths are errors. There are no ambient variables.

## Failure behavior

Expression errors stop compilation. Diagnostics identify the location and
failure class; Paper does not substitute, coerce, or ignore invalid values.
