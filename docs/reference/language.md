# Paper language

Paper `paper/0.4` is an indentation-sensitive language for typed, deterministic
documents.

## Choose by task

| I want to… | Reference |
| --- | --- |
| Look up any language node | [Node index](./nodes) |
| Learn declarations, IDs, values, and units | [Syntax and values](./syntax) |
| Configure the document, page, header, body, or footer | [Document and pages](./document) |
| Add text, lists, images, or tables | [Content](./content) |
| Arrange rows, columns, or constrained canvas regions | [Layout](./layout) |
| Declare schemas or expand data | [Schemas and expansion](./data) |
| Define reusable blocks and select them conditionally | [Components](./components) |
| Reuse visual properties and theme tokens | [Styles and themes](./design) |
| Calculate values, visibility, or switches | [Expressions](./expressions) |

## Document anatomy

```paper
document @report:
  title: "Report"
  language: "en"

  schema input:
    string customer
    bool paid

  page @sheet:
    size: "A4"
    margin: 36pt
    body @content:
      heading @title:
        level: 1
        text: customer
      paragraph @status:
        text: paid ? "Paid" : "Open"
```

| Part | Role |
| --- | --- |
| `document` | Root configuration and declarations |
| `schema` | Closed contract for JSON or scenario data |
| `page` | Size, margins, numbering, and repeating regions |
| `body` | Flow content expanded into pages |
| `heading`, `paragraph`, … | Render nodes with typed properties |

## Compilation model

| Stage | Rejects |
| --- | --- |
| Parse | Invalid indentation, grammar, IDs, or scalar syntax |
| Type | Unknown properties, paths, components, or incompatible values |
| Select | Invalid data, scenarios, visibility, repeats, or loops |
| Plan | Unsatisfied structure, limits, assets, or layout contracts |
| Render | Output-specific failures |

Paper does not coerce values, ignore unsupported properties, fetch resources,
or read ambient locale and time. Successful plans are immutable and
content-addressed across PDF, HTML, and SVG.
