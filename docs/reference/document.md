# Document and pages

One file has one `document`, one `page`, and one `body`. A page may also have
one `header` and one `footer`.

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

## Document properties

| Property | Type | Meaning |
| --- | --- | --- |
| `title` | string | Output title |
| `language` | string | Document language and locale fallback |
| `theme` | quoted `@name` | Selected theme |
| `import` | string | Source-relative design import resolved by the caller |

Imports contribute themes and styles, not public pages, schemas, scenarios, or
components.

### Imports

`import` may be repeated. The caller resolves each source-relative path through
an explicit bounded resolver; Paper never searches the filesystem or network.
Import cycles, missing files, and conflicting declarations are errors. The WASM
playground does not provide a resolver.

## Page properties

| Property | Type | Values |
| --- | --- | --- |
| `size` | string | `A3`, `A4`, `A5`, `Letter`, `Legal` |
| `width`, `height` | positive physical length | Override named size |
| `margin` | non-negative physical length | All margins |
| `margin-top/right/bottom/left` | non-negative physical length | One margin |
| `page-numbers` | bool | Enable numbering |
| `page-number-format` | string | One integer directive, such as `"Page %d"` |
| `page-total-alias` | string | Text replaced with final page count |
| `page-number-align` | string | `left`, `center`, `right`, `inner`, `outer` |
| `page-number-position` | string | `header`, `footer` |
| `page-number-hide-first` | bool | Suppress first page number |
| `page-number-start` | integer | Initial number, 1 through 1,048,576 |

`header`, `footer`, and `body` accept flow blocks. Header and footer also accept
[box properties](./content#box-properties).

## Flow blocks

| Node | Children | Purpose |
| --- | --- | --- |
| `text: "…"` | none | Flow paragraph or text segment |
| `paragraph` | `text` | Styled text |
| `heading` | `text` | Heading level 1–6 |
| `list` | `item` | Ordered or unordered list |
| `page-break` | none | Force a page boundary |
| `row`, `column` | layout items | One-dimensional layout |
| `image` | none | Catalog or embedded image |
| `table` | columns, header, rows, repeats | Paged table |
| `canvas` | `anchor` | Constrained positioning |
| `use` | `arg`, `fill` | Component instance |
| `repeat` | one template node | Collection expansion |
| `loop` | one template node | Numeric expansion |

Renderable blocks accept boolean `visible`. False removes the subtree before
structural validation.

`page-break` is a leaf node with no value or properties. It ends the current
flow page after preceding content.
