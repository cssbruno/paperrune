# Node index

Every public `paper/0.4` node is listed here. Follow the linked reference for
property types and examples.

## Document and flow

| Node | Parent | Children | Properties |
| --- | --- | --- | --- |
| `document` | root | page, component, schema, object type, scenario, theme, style | `title`, `language`, `theme`, repeatable `import` |
| `page` | document | one body; optional header and footer | size, dimensions, margins, page numbering |
| `header`, `footer` | page | text, heading, paragraph, list, page-break, row/column, image, table, use, loop | box properties |
| `body` | page | all flow nodes | none |
| `text: VALUE` | flow, text container | none | inline string or string expression |
| `paragraph` | flow | text segments | [text and box properties](./content) |
| `heading` | flow | text segments | paragraph properties plus `level` |
| `list` | flow | item | text/box properties, `ordered`, `marker`, `visible` |
| `item` | list | text, paragraph, use | `visible` |
| `page-break` | flow | none | none |

`body` additionally accepts `canvas`, `repeat`, and `loop`. See
[Document and pages](./document).

## Content and layout

| Node | Parent | Children | Properties |
| --- | --- | --- | --- |
| `row`, `column` | flow | paragraph, heading, image, table, use, one nested row/column level | container sizing/alignment and `visible` |
| `image` | flow, row/column, cell | none | source, sizing, crop, accessibility, box/style, `visible` |
| `table` | flow, row/column | table-column, table-header, table-row, repeat | caption, header repetition, split, `visible` |
| `table-column` | table | none | `width`, `min-width`, `max-width` |
| `table-header` | table | table-row | none |
| `table-row` | table, table-header, repeat | cell | pagination properties |
| `cell` | table-row | text, paragraph, list, image | text/box, spans, header flag, vertical alignment |
| `canvas` | body | anchor | dimensions, default axes, `visible` |
| `anchor` | canvas | none | dimensions, axis constraints, `alt`, box, `visible` |

See [Content](./content) and [Layout](./layout).

## Data and expansion

| Node | Parent | Children | Properties or value |
| --- | --- | --- | --- |
| `schema [name]` | document | typed fields | none |
| `object TypeName` | document | typed fields | none |
| typed field | schema, object field/type | nested fields where applicable | list fields require `max-items` |
| `scenario @name` | document | value, object, keyed-list | `parent`, `locale` |
| `value @name: VALUE` | scenario/object/keyed-list | none | null, string, bool, number |
| `object @name` | scenario/object/keyed-list | fixture values | none |
| `keyed-list @name` | scenario/object/keyed-list | keyed fixture values | none |
| `repeat @name` | body, table, repeat | one template | `source`, `instance-prefix`, `max-items`, `visible` |
| `loop @name` | flow, repeat, loop | one template | `from`, `through`, `step`, `max-iterations`, `instance-prefix`, `visible` |

See [Schemas and expansion](./data).

## Reuse and design

| Node | Parent | Children | Properties or value |
| --- | --- | --- | --- |
| `component @name` | document | prop, slot, flow nodes | none |
| `prop @name` | component | none | `type`, `required`, `default` |
| `slot @name` | component | default flow nodes | `type`, `required`, `cardinality`, `layout-affecting`, `scenarios` |
| `use @instance` | flow | arg, fill | `component`, `bind`, `bind-required`, `visible` |
| `arg @name: VALUE` | use | none | typed scalar or expression |
| `fill @slot` | use | flow nodes | `scenario` for layout-affecting slots |
| `style @name` | document | none | text/box properties and parent `style` |
| `theme @name` | document | token, scope | `parent` |
| `token @name` | theme, scope | none | `type` and exactly one of `value` or `reference` |
| `scope @name` | theme, scope | token, scope | none |

See [Components](./components) and [Styles and themes](./design).
