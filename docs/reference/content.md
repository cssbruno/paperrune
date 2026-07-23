# Content

## Text properties

`paragraph`, `heading`, and table `cell` accept text and box properties. A
heading also accepts `level: 1` through `level: 6`.

| Property | Type | Values or meaning |
| --- | --- | --- |
| `text` | string expression or string | Content when no child text nodes exist |
| `font` | string | Built-in or registered font |
| `size` | positive physical length | Font size |
| `line-height` | positive physical length | Line advance |
| `color` | string | Color such as `#2459D3` |
| `align` | string | `left`, `center`, `right`, `justify` |
| `bold`, `italic` | bool | Font traits |
| `style` | quoted `@name` | Named style applied before local overrides |
| `font-token` | string | Theme font token |
| `size-token`, `line-height-token` | string | Theme length token |
| `color-token` | string | Theme color token |
| `bind` | string | Schema path formatted into the block |
| `bind-required` | bool | Reject absent or null bound values |
| `format` | string | `string`, `bool`, `integer`, `decimal`, `currency` |
| `format-locale` | string | Formatting locale |
| `format-currency` | string | Supported uppercase ISO currency |
| `format-min-fraction` | integer | Minimum fraction digits |
| `format-max-fraction` | integer | Maximum fraction digits |

## Box properties

| Group | Properties | Type |
| --- | --- | --- |
| Margin | `margin`, `margin-top/right/bottom/left` | non-negative physical length |
| Padding | `padding`, `padding-top/right/bottom/left` | non-negative physical length |
| Border width | `border-width`, `border-top/right/bottom/left-width` | non-negative physical length |
| Border color | `border-color` | string color |
| Corners | `border-radius` | non-negative physical length |
| Fill | `background` | string color |

Local properties override named styles.

## Text construction and binding

Use `text: expression` (or child `text` segments) for authored and computed
copy. Use `bind` by itself for JSON-backed copy. A bind-only node uses its
canonical schema path as the deterministic placeholder when no data is
selected; selected data replaces that internal placeholder. Legacy templates
that combine `bind` with fallback `text` remain supported.

| `format` | Input | Additional properties |
| --- | --- | --- |
| `string` | string | locale optional |
| `bool` | bool | locale optional |
| `integer` | number | locale; fraction bounds must be zero |
| `decimal` | number | locale, min/max fraction |
| `currency` | number | locale, uppercase `format-currency`, min/max fraction |

`bind-required: true` rejects missing or null values. Without selected data,
the authored `text` remains the preview fallback. With multiple schemas, pass
the intended schema through the CLI or WASM request.

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

`marker` is `decimal`, `dash`, or `asterisk` and must agree with `ordered`.
Items contain `text` or `paragraph` children and may use `visible`.

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

Images also accept box, style, and `visible`. Paper does not fetch paths or
URLs. See [Assets and fonts](/paper-assets).

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

Table properties: `caption` (string), `repeat-header` (bool), `split` (`rows`
or `avoid`), and `visible` (bool). Columns accept `width`, `min-width`, and
`max-width`. Rows accept `keep-together`, `keep-with-next`, `orphans`, and
`widows`.

Cells accept text and box properties plus `header-cell` (bool), `colspan` and
`rowspan` (positive integers), and `vertical-align` (`top`, `middle`, `bottom`).
Children may be `text`, `paragraph`, `list`, or `image`.
