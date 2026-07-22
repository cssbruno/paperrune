# Paper language reference

This page is the authoring contract for Paper grammar `paper/0.4`. Paper source
uses spaces for indentation, one declaration per line, and `#` comments.

```paper
node @optional-readable-id:
  property: value
  child:
    property: value # comment
```

Readable IDs begin with `@` plus a letter and may contain letters, digits,
`-`, and `_`. They are optional on most render nodes but required where the
language needs a stable identity, such as components, scenarios, themes,
styles, slots, repeat instances, and canvas anchors.

## Scalar values

| Kind | Syntax | Notes |
| --- | --- | --- |
| String | `"Hello"` | JSON-style escapes; always quoted |
| Boolean | `true`, `false` | Never quoted |
| Number | `12`, `-3.5` | Finite decimal |
| Unit | `12pt`, `40%`, `1fr` | Unit support depends on the property |
| Null | `null` | Only where absence is allowed |
| Expression | `paid ? "Paid" : "Open"` | Unquoted; statically typed |
| Component | `@summary` | Only in component-selection expressions |

Physical lengths accept `pt`, `mm`, `cm`, `in`, `px`, and `pc`. Layout contexts
also accept `%`, `fr`, or `"auto"` where documented. The expression engine can
represent `em`, `rem`, `vh`, and `vw`, but a receiving layout property may
reject a context-dependent result it cannot resolve deterministically.

See [Expressions](./expressions) for operators, precedence, switches, null
guards, exact decimal arithmetic, and type rules.

## Document structure

One file contains exactly one `document`, one `page` template, and one `body`.
The page may also contain one `header` and one `footer`.

```paper
document @report:
  title: "Quarterly report"
  language: "en"
  theme: "@brand"
  import: "shared.paper"
  page @sheet:
    size: "A4"
    margin: 36pt
    header:
      text: "Quarterly report"
    body:
      paragraph:
        text: "Body"
    footer:
      text: "Confidential"
```

### Document properties

| Property | Type | Meaning |
| --- | --- | --- |
| `title` | string | Output document title |
| `language` | string | Explicit document language/locale fallback |
| `theme` | quoted `@name` | Select a declared theme |
| `import` | string | Source-relative design import resolved by the caller |

Imports may contribute themes and styles. Imported pages, schemas, scenarios,
and components remain outside the importing document's public structure.

### Page properties

| Property | Type | Values |
| --- | --- | --- |
| `size` | string | `A3`, `A4`, `A5`, `Letter`, `Legal` |
| `width`, `height` | positive physical length | Override the named size |
| `margin` | non-negative physical length | Set all page margins |
| `margin-top/right/bottom/left` | non-negative physical length | Override one side |
| `page-numbers` | bool | Enable numbering |
| `page-number-format` | string | One integer directive, such as `"Page %d"` |
| `page-total-alias` | string | Text replaced with the final page count |
| `page-number-align` | string | `left`, `center`, `right`, `inner`, `outer` |
| `page-number-position` | string | `header`, `footer` |
| `page-number-hide-first` | bool | Suppress the first page number |
| `page-number-start` | integer | Initial number, 1 through 1,048,576 |

`header`, `footer`, and `body` accept ordinary flow blocks. Header and footer
also accept the box properties listed below.

## Flow blocks

A body, component, slot, fill, repeat, or loop may produce these blocks:

| Node | Children | Purpose |
| --- | --- | --- |
| `text: "..."` | none | Plain paragraph in flow; text segment inside text containers |
| `paragraph` | `text` | Styled text block |
| `heading` | `text` | Heading level 1–6 |
| `list` | `item` | Ordered or unordered list |
| `page-break` | none | Force a page boundary after the current flow |
| `row`, `column` | supported layout items | One-dimensional calculated layout |
| `image` | none | Content-addressed or embedded image |
| `table` | columns, header, rows, repeats | Paged tabular layout |
| `canvas` | `anchor` | Explicit same-axis constraint layout |
| `use` | `arg`, `fill` | Instantiate a component |
| `repeat` | one template node | Expand typed collection data |
| `loop` | one template node | Expand a bounded numeric range |

`visible` is accepted by renderable blocks, list items, component uses,
repeats, and loops. It requires a boolean expression. When false, the complete
subtree is removed before structural layout validation.

## Text and box properties

`paragraph`, `heading`, and table `cell` accept both families. A `list` accepts
text and box styles; a row/column text child accepts these plus track sizing.

### Text

| Property | Type | Values or meaning |
| --- | --- | --- |
| `text` | string expression or string | Text content when no child `text` nodes are used |
| `font` | string | Built-in or registered font name |
| `size` | positive physical length | Font size |
| `line-height` | positive physical length | Line advance |
| `color` | string | Color such as `#2459D3` |
| `align` | string | `left`, `center`, `right`, `justify` |
| `bold`, `italic` | bool | Font traits |
| `style` | quoted `@name` | Apply a declared style before local overrides |
| `font-token` | string | Theme token reference |
| `size-token`, `line-height-token` | string | Theme length token reference |
| `color-token` | string | Theme color token reference |
| `bind` | string | Schema path to format into this text block |
| `bind-required` | bool | Reject absent/null bound values when true |
| `format` | string | `string`, `bool`, `integer`, `decimal`, `currency` |
| `format-locale` | string | Explicit locale for formatting |
| `format-currency` | string | Explicit supported uppercase ISO currency |
| `format-min-fraction` | integer | Minimum fraction digits |
| `format-max-fraction` | integer | Maximum fraction digits |

A heading additionally accepts `level: 1` through `level: 6`.

### Box

| Group | Properties | Type |
| --- | --- | --- |
| Margin | `margin`, `margin-top/right/bottom/left` | non-negative physical length |
| Padding | `padding`, `padding-top/right/bottom/left` | non-negative physical length |
| Border width | `border-width`, `border-top/right/bottom/left-width` | non-negative physical length |
| Border color | `border-color` | string color |
| Corners | `border-radius` | non-negative physical length |
| Fill | `background` | string color |

Local properties override values inherited from a named style.

## Lists

```paper
list:
  ordered: false
  marker: "dash"
  item:
    text: "First"
  item:
    paragraph:
      bold: true
      text: "Second"
```

`ordered` is a boolean. `marker` accepts `decimal`, `dash`, or `asterisk` and
must agree with `ordered` when both are present. Each `item` contains one or
more `text` or `paragraph` children and may use `visible`.

## Rows and columns

```paper
row:
  gap: 12pt
  line-gap: 8pt
  wrap: "wrap"
  justify-content: "space-between"
  align-items: "stretch"
  align-content: "start"
  paragraph:
    width: 2fr
    min-width: 120pt
    text: "Main"
  paragraph:
    width: 1fr
    align-self: "center"
    text: "Aside"
```

### Container properties

| Property | Values |
| --- | --- |
| `gap`, `line-gap` | non-negative physical length |
| `width`, `height` | physical cross-axis extent where applicable |
| `wrap` | `nowrap`, `wrap`, `wrap-reverse` |
| `justify-content` | `start`, `center`, `end`, `space-between`, `space-around`, `space-evenly` |
| `align-items` | `start`, `center`, `end`, `stretch` |
| `align-content` | previous alignment values plus spacing values |
| `reverse` | bool |

Children may be `paragraph`, `heading`, `image`, `table`, or one nested
`row`/`column` level. Item sizing properties are `width`, `min-width`,
`max-width`, `height`, `min-height`, `max-height`, `flex-grow`, `flex-shrink`,
and `align-self`. The main-axis size accepts a physical length, percentage,
`"auto"`, or a positive whole `fr` value. `align-self` accepts `start`,
`center`, `end`, or `stretch`.

## Images

```paper
image @chart:
  source: "asset:quarterly-chart"
  width: 100%
  height: "auto"
  max-height: 240pt
  fit: "contain"
  align: "center"
  alt: "Quarterly revenue chart"
  caption: "Figure 1"
```

| Property | Type or values |
| --- | --- |
| `source` | `asset:name` or bounded PNG/JPEG data URI |
| `width`, `height` | physical length, percentage, or `"auto"` |
| `max-width`, `max-height` | physical length or percentage |
| `fit` | `auto`, `contain`, `cover` |
| `focus-x`, `focus-y` | number from 0 through 1 |
| `align` | `left`, `center`, `right` |
| `alt` | required non-empty string unless decorative |
| `decorative` | bool; conflicts with non-empty `alt` |
| `caption` | string |

Images also accept box, style, and `visible` properties. Paper never fetches an
image path or URL during compilation. See [Assets and fonts](/paper-assets).

## Tables

```paper
table:
  caption: "Results"
  repeat-header: true
  split: "rows"
  table-column:
    width: 70%
  table-column:
    width: 30%
  table-header:
    table-row:
      cell:
        header-cell: true
        text: "Name"
      cell:
        header-cell: true
        text: "Value"
  table-row:
    keep-together: true
    cell:
      text: "Latency"
    cell:
      text: "12 ms"
```

Table properties are `caption` (string), `repeat-header` (bool), `split`
(`rows` or `avoid`), and `visible` (bool). A `table-column` accepts `width`,
`min-width`, and `max-width`. A `table-row` accepts `keep-together`,
`keep-with-next`, `orphans`, and `widows`.

A `cell` accepts text and box properties plus `header-cell` (bool), `colspan`
and `rowspan` (positive integers), and `vertical-align` (`top`, `middle`,
`bottom`). Its children may be `text`, `paragraph`, `list`, or `image`.

## Canvas and anchors

Canvas is for explicit, bounded positioning—not general flow:

```paper
canvas:
  width: 300pt
  height: 120pt
  default-horizontal: "left"
  default-vertical: "top"
  anchor @label:
    width: 100pt
    height: 24pt
    left: "canvas.left + 12pt"
    top: "canvas.top + 12pt"
    alt: "Label region"
```

Canvas requires positive `width` and `height`. Defaults accept `left`, `right`,
`center-x` and `top`, `bottom`, `center-y` respectively. Every anchor needs a
unique ID and positive `width` and `height`. Axis constraints reference
`canvas.anchor` or `@sibling.anchor`, optionally followed by `+`/`-` and a
physical offset. Anchors also accept box and `visible` properties.

## Schemas and data

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

Primitive types are `string`, `number`, and `bool`; nested types are `object`,
`list`, and named custom objects. Prefix any field with `optional`. Lists must
have an item type and a positive `max-items`. Schema data is closed: undeclared
fields and coercions are errors.

See [Projects and data](/guide/projects-and-data) for JSON, scenarios, repeat,
and loop examples.

## Components

```paper
document:
  component @callout:
    prop @title:
      type: "string"
      required: true
    prop @important:
      type: "bool"
      default: false
    heading:
      color: args.important ? "#B42318" : "#2459D3"
      text: args.title
    slot @body:
      type: "blocks"

  page:
    body:
      use @notice:
        component: @callout
        arg @title: "Read this"
        arg @important: true
        fill @body:
          paragraph:
            text: "Component body"
```

A `prop` accepts `type`, `required`, and `default`. Supported prop types are
`string`, `bool`, `number`, `unit`, `length`, and `any`. Expressions inside the
component read arguments as `args.name`.

A `slot` accepts `type` (`blocks`, `text`, `list`, `row-column`), `required`,
`cardinality` (`one`, `many`), `layout-affecting`, and a comma-separated
`scenarios` string. Children under a slot are its default content. A required
slot cannot also have defaults.

A `use` requires `component`, which may be a direct `@name`, a conditional or
switch component expression, or `null`. It accepts `arg` and `fill` children,
an optional data `bind`/`bind-required`, and `visible`. Component cycles,
unknown arguments or fills, incompatible alternatives, and missing required
contracts are errors.

## Repeats and loops

| Node | Required properties | Optional properties |
| --- | --- | --- |
| `repeat` | `source` string, `instance-prefix` string, positive `max-items` | boolean `visible` |
| `loop` | integer `from`, `through`, `step`, positive `max-iterations`, `instance-prefix` string | boolean `visible` |

Both contain exactly one template node and may be nested within configured
limits. Repeat expressions use `item.*`; loop expressions use `loop.index`,
`loop.first`, and `loop.last`. Expansion happens only with selected scenario or
JSON data, and generated IDs remain stable through `instance-prefix`.

## Styles

```paper
document:
  style @base:
    font: "Helvetica"
    size: 10pt
    color: "#171A1F"

  style @emphasis:
    style: "@base"
    bold: true

  page:
    body:
      paragraph:
        style: "@emphasis"
        text: "Local properties override the style"
```

Styles contain text and box design properties only. `style: "@parent"` inside
a style provides single inheritance. Cycles and unknown names are errors.

## Themes and tokens

```paper
document:
  theme: "@brand"

  theme @foundation:
    token @ink:
      type: "color"
      value: "#171A1F"
    token @body-size:
      type: "length"
      value: 10pt

  theme @brand:
    parent: "@foundation"
    token @accent:
      type: "color"
      value: "#2459D3"
    scope @print:
      token @accent:
        type: "color"
        value: "#173F5F"

  page:
    body:
      paragraph:
        size-token: "body-size"
        color-token: "accent"
        text: "Theme tokens"
```

Theme token types are `color`, `string`, `length`, `number`, and `bool`. A
token defines either `value` or a quoted `reference` to another token. Themes
may have one `parent`; scopes contain tokens or nested scopes. Text consumes
theme values through `font-token`, `size-token`, `line-height-token`, and
`color-token`.

## Switch properties

Switch is a readable property value, not a separate render node:

```paper
text: switch status:
  case "paid": "Paid"
  case "overdue": "Overdue"
  default: "Open"
```

Predicate switches omit the selector. The first match wins, every switch ends
with one `default`, and all results must satisfy the receiving property's type.
See [Expressions](./expressions#switch-selection).

## Diagnostics and determinism

Paper does not silently coerce types, ignore unsupported properties, fall
through switches, fetch network resources, consult a browser layout engine, or
read host locale/time state. Parse, schema, expression, component, data, and
layout failures produce source-located diagnostics. A successful plan is
immutable, content-addressed, and reusable for PDF, HTML, and SVG capture.
