# Schemas and expansion

## Schemas

```paper
object Address:
  string street
  string city

schema customer:
  string name
  optional string note
  bool active
  number balance
  Address address
  object preferences:
    string locale
  list object contacts:
    max-items: 20
    string label
    string value
```

Types are `string`, `number`, `bool`, `object`, `list`, and named objects.
Prefix a field with `optional`. Lists require an item type and positive
`max-items`. Schemas reject undeclared fields and coercion.

See [Projects and data](/guide/projects-and-data) for JSON and scenarios.

## Scenarios

Scenarios are typed in-source fixtures:

```paper
scenario @overdue:
  locale: "en-US"
  parent: "@base"
  value @number: "INV-1042"
  value @paid: false
  value @note: null
  object @billingAddress:
    value @city: "London"
  keyed-list @items:
    object @first:
      value @description: "Planning"
      value @quantity: 2
```

| Declaration | Value |
| --- | --- |
| `value @name` | null, string, bool, or finite unitless number |
| `object @name` | named fixture fields |
| `keyed-list @name` | stable keyed fixture values |
| `parent` | optional quoted scenario name; cycles are invalid |
| `locale` | optional quoted formatting locale |

Keys are unique within an object or list. The resolved fixture must satisfy the
selected schema. Use `paper scenario` to list or inspect fixtures.

## Expansion

| Node | Required properties | Optional properties | Scope |
| --- | --- | --- | --- |
| `repeat` | `source`, `instance-prefix`, positive `max-items` | boolean `visible` | `item.*` |
| `loop` | integer `from`, `through`, `step`, positive `max-iterations`, `instance-prefix` | boolean `visible` | `loop.index`, `loop.first`, `loop.last` |

Both contain one template node and may nest within configured limits. Expansion
requires selected JSON or scenario data. `instance-prefix` keeps generated IDs
stable.

Repeat templates may be flow nodes, table rows, uses, repeats, or loops. Loop
templates may be flow nodes, uses, or nested loops. Repeat scope exposes
`item.*`; loop scope exposes `loop.index`, `loop.first`, and `loop.last`.

```paper
repeat @contacts:
  source: "contacts"
  instance-prefix: "contact"
  max-items: 20
  paragraph:
    text: item.label + ": " + item.value
```

```paper
loop @copies:
  from: 1
  through: 3
  step: 1
  max-iterations: 3
  instance-prefix: "copy"
  paragraph:
    text: "Copy"
```
