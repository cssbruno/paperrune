# Projects and data

Projects combine a template, schema, data, assets, and output settings.

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

Paths are relative to the project file. Precedence:

1. command flags and an explicit source argument;
2. `PAPER_SOURCE`, `PAPER_DATA`, `PAPER_OUTPUT`, `PAPER_FORMAT`,
   `PAPER_LOCALE`, and `PAPER_ASSETS`;
3. `paper.project.json`;
4. command defaults.

See [Project file](/reference/project-file) for output behavior.

## Declare a schema

Schemas are closed and typed:

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

Types are `string`, `number`, `bool`, `object`, `list`, and declared objects.
`optional` permits a missing field or JSON `null`. Lists require a positive
`max-items`.

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

Rejected: duplicate or unknown keys, trailing JSON, wrong types, missing
required fields, and arrays beyond `max-items`.

Expressions use declared paths directly:

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

`text` is the fallback when no data is selected. Use `text: expression` for
calculated strings.

## Declare scenarios

Scenarios provide in-source preview and test data:

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

`parent: "@base"` inherits a scenario; local values override it. Scenarios
must satisfy the schema. Use `--data` for production JSON.

## Repeat collections

A repeat expands one template child for selected data:

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

Use `item.field` inside the template. `visible` filters items. Nested repeats
may read child lists. Repeat bounds cannot exceed schema bounds.

## Fixed loops

Use a loop for a bounded numeric sequence:

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

Available paths: `loop.index`, `loop.first`, and `loop.last`. `step` must reach
`through`; `max-iterations` must cover the range.
