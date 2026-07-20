# Built-in authoring tools

PaperRune's built-in tools are editable starter structures, not opaque widgets.
Every tool lowers to ordinary Paper headings, paragraphs, lists, tables, rows,
columns, images, and page regions.

## Palette rules

A built-in tool must satisfy all of these conditions:

- It solves a distinct document-authoring job.
- Its starter content communicates the intended structure.
- Every generated node remains editable in Paper Studio and source.
- It compiles and renders with the default fonts and planner limits.
- It composes safely only in source parents that support its structure.
- It is not merely a cosmetic variation of another tool.

## Visual system

Built-ins share one restrained document palette rather than inventing a new
look for every tool:

- Deep ink establishes titles and primary table headers.
- Teal identifies structure, navigation, and neutral information.
- Green, amber, and red are reserved for positive, caution, and risk meaning.
- Soft surfaces and fine borders separate editable fields without heavy boxes.
- Narrow utility columns are explicitly sized for checks, step numbers, dates,
  page numbers, statuses, and monetary values.
- Checklist rows use empty outlined controls and quiet rules instead of bracket
  characters or a visually heavy shaded column.
- Catalog entries use consistent vertical rhythm so labels, descriptions, and
  previews read as separate tools without wrapping every item in a card.
- Starter copy demonstrates hierarchy with realistic labels instead of generic
  filler wherever the structure has a clear business meaning.
- Image tools use a calm embedded placeholder graphic with descriptive alt text
  so unfinished documents remain presentable and accessible.

## Curated groups

| Group | Tools |
| --- | --- |
| Structure | `title-block`, `cover-block`, `two-column`, `divider` |
| Communication | `note-box`, `quote`, `status-banner`, `disclaimer`, `faq-block` |
| Actions and decisions | `checklist`, `numbered-steps`, `decision-record`, `approval-block`, `signature-row` |
| Reports and evidence | `kpi-strip`, `timeline`, `comparison-table`, `risk-register`, `change-log`, `pros-cons`, `table-of-contents` |
| Media and verification | `image-caption`, `image-grid`, `qr-verification` |
| Business content | `metadata-grid`, `recipient-block`, `invoice-totals`, `clause`, `code-block` |

The ordinary primitives (`paragraph`, `heading`, `list`, `image`, `table`,
`row`, `column`, `canvas`, and `page-break`) remain available separately.

## Removed overlaps

| Removed tool | Replacement | Reason |
| --- | --- | --- |
| `styled-container` | Style an existing paragraph or section | It had no document-specific meaning. |
| `contact-block` | `recipient-block` or `metadata-grid` | The address/contact structures overlapped. |
| `executive-summary` | `section` plus `title-block` | It was only a heading followed by a paragraph. |
| `references-list` | Ordered `list` | It added no structure beyond the list primitive. |

New cosmetic variants should normally become styles or themes. New built-in
tools should be reserved for structures with different semantics, composition,
or editing behavior.
