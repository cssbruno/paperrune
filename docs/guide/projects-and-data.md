# Projects and data

PaperRune keeps the template, data contract, data values, and output settings
explicit. It does not read ambient variables from the host during compilation.

## Project discovery

`paper check`, `paper render`, and `paper studio` search the current directory
and its parents for `paper.project.json`:

```json
{
  "source": "invoice.paper",
  "data": "invoice.json",
  "output": "dist/invoice.pdf",
  "format": "pdf",
  "locale": "en",
  "assets": "project.assets.json"
}
```

Every path is relative to the project file. Configuration precedence is:

1. command flags and an explicit source argument;
2. `PAPER_SOURCE`, `PAPER_DATA`, `PAPER_OUTPUT`, `PAPER_FORMAT`,
   `PAPER_LOCALE`, and `PAPER_ASSETS`;
3. `paper.project.json`;
4. command defaults.

See the [project-file reference](/reference/project-file) for output and
standard-stream behavior.

## Declare a schema

A schema is closed and typed. Primitive fields do not end in a colon. Object
and list fields own an indented block:

```paper
document:
  object Address:
    string street
    string city

  schema invoice:
    string number
    bool paid
    optional string note
    Address billingAddress
    list object items:
      max-items: 100
      string description
      number quantity
```

Supported primitives are `string`, `number`, and `bool`. `object` creates a
nested record. `list TYPE name:` creates a bounded collection of a primitive,
inline object, or declared custom object. `optional` permits a missing field or
JSON `null`; every other field is required.

Every list must declare a positive `max-items`. The bound is part of the
template contract and protects compilation from unbounded expansion.

## Supply JSON

JSON keys match schema field names exactly:

```json
{
  "number": "INV-1042",
  "paid": false,
  "note": null,
  "billingAddress": {
    "street": "10 Analytical Engine Way",
    "city": "London"
  },
  "items": [
    {"description": "Planning", "quantity": 2}
  ]
}
```

Paper rejects duplicate JSON keys, trailing JSON, unknown fields, wrong scalar
types, missing required fields, and arrays longer than `max-items`.

Use bare declared paths in expressions. A property name is the receiver, not a
variable declaration:

```paper
text: number
visible: paid == false
text: note != null ? note : "No note"
```

## Bind values to text

`bind` is the formatting-oriented alternative to an expression in `text`:

```paper
paragraph:
  bind: "total"
  bind-required: true
  format: "currency"
  format-locale: "en-US"
  format-currency: "USD"
  format-min-fraction: 2
  format-max-fraction: 2
  text: "$0.00"
```

The `text` value is the template fallback when no concrete data selection is
being rendered. For calculated strings, use `text: expression` directly.

## Declare scenarios

Scenarios are in-source fixtures for Studio, tests, and deterministic previews:

```paper
scenario @overdue:
  locale: "en-US"
  value @number: "INV-1042"
  value @paid: false
  value @note: null
  object @billingAddress:
    value @street: "10 Analytical Engine Way"
    value @city: "London"
  keyed-list @items:
    object @first:
      value @description: "Planning"
      value @quantity: 2
```

`parent: "@base"` inherits another scenario before local values override it.
Scenario values still have to satisfy the selected schema. For production
data, prefer `--data` and JSON.

## Repeat collections

A repeat has exactly one template child. It is expanded only when concrete
JSON or a scenario is selected:

```paper
repeat @item-rows:
  source: "items"
  instance-prefix: "items"
  max-items: 100
  table-row:
    cell:
      paragraph:
        text: item.description
    cell:
      paragraph:
        bind: "quantity"
        format: "integer"
        format-locale: "en-US"
        text: "0"
```

Inside the template, use `item.field`. An optional `visible` expression filters
items, and nested repeats may read a child list of the current object. The
repeat's `max-items` cannot promise more output than the schema permits.

## Fixed loops

Use a loop for a bounded numeric sequence rather than a data collection:

```paper
loop @copies:
  from: 1
  through: 3
  step: 1
  max-iterations: 3
  instance-prefix: "copies"
  paragraph:
    visible: loop.first || loop.last
    text: "Copy"
```

`loop.index`, `loop.first`, and `loop.last` are available inside the template.
The direction of `step` must reach `through`, and `max-iterations` must cover
the authored range.
