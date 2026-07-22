# Layout

## Rows and columns

```paper
row:
  gap: 12pt
  line-gap: 8pt
  wrap: "wrap"
  justify-content: "space-between"
  align-items: "stretch"
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
| `align-content` | alignment values plus spacing values |
| `reverse` | bool |

Children may be `paragraph`, `heading`, `image`, `table`, or one nested
`row`/`column`. Item properties are `width`, `min-width`, `max-width`, `height`,
`min-height`, `max-height`, `flex-grow`, `flex-shrink`, and `align-self`.
Main-axis size accepts a physical length, percentage, `"auto"`, or positive
whole `fr`. `align-self` is `start`, `center`, `end`, or `stretch`.

| Item property | Values |
| --- | --- |
| main `width`/`height` | physical length, percentage, `"auto"`, positive whole `fr` |
| min/max dimensions | physical length or percentage |
| `flex-grow`, `flex-shrink` | non-negative bounded number |
| `align-self` | `start`, `center`, `end`, `stretch` |

## Canvas and anchors

Canvas provides bounded explicit positioning:

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

Canvas requires positive `width` and `height`. Horizontal defaults are `left`,
`right`, and `center-x`; vertical defaults are `top`, `bottom`, and `center-y`.
Anchors require a unique ID and positive dimensions. Axis constraints reference
`canvas.anchor` or `@sibling.anchor`, optionally followed by `+`/`-` and a
physical offset. Anchors also accept box and `visible` properties.

| Anchor group | Properties |
| --- | --- |
| Size | `width`, `height` |
| Horizontal constraint | `left`, `right`, `center-x`; canvas default applies when omitted |
| Vertical constraint | `top`, `bottom`, `center-y`; canvas default applies when omitted |
| Accessibility | optional `alt` string |
| Presentation | box properties and `visible` |
