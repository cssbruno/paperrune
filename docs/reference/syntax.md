# Syntax and values

Paper uses spaces for indentation, one declaration per line, and `#` comments.

```paper
node @optional-id:
  property: value
  child:
    property: value # comment
```

## IDs

IDs match `@[A-Za-z][A-Za-z0-9_-]*`. Most render IDs are optional. Components,
scenarios, themes, styles, slots, repeats, and canvas anchors require stable
IDs.

## Scalar values

| Kind | Syntax | Notes |
| --- | --- | --- |
| String | `"Hello"` | JSON-style escapes; always quoted |
| Boolean | `true`, `false` | Never quoted |
| Number | `12`, `-3.5` | Finite decimal |
| Unit | `12pt`, `40%`, `1fr` | Support depends on the property |
| Null | `null` | Only where absence is allowed |
| Expression | `paid ? "Paid" : "Open"` | Unquoted and statically typed |
| Component | `@summary` | Component-selection expressions only |

Physical units are `pt`, `mm`, `cm`, `in`, `px`, and `pc`. Documented
properties may also accept `%`, `fr`, or `"auto"`. Expressions support `em`,
`rem`, `vh`, and `vw`, subject to the receiving property.

## Property values

The property determines the required type:

```paper
visible: active                  # bool
text: active                     # error: text requires string
text: active ? "Yes" : "No"     # string
```

Switch is a property value, not a render node:

```paper
text: switch status:
  case "paid": "Paid"
  case "overdue": "Overdue"
  default: "Open"
```

Predicate switches omit the selector. The first match wins, `default` is
required, and every result must satisfy the property type. See
[Expressions](./expressions) for operators, precedence, null guards, switches,
and exact arithmetic.

## Diagnostics

Parse, schema, expression, component, data, and layout errors include source
locations. Compilation stops instead of substituting, coercing, or ignoring an
invalid value.
