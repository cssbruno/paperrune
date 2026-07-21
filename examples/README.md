# PaperRune examples

The maintained examples use Paper, the sole public authoring format. The same
source can be rendered to PDF or standalone HTML.

## Paper

- `headers-footers` — repeating page/table headers and planner-owned page numbers.
- `hello-world/hello-world.paper` — smallest native Paper document.
- `invoice/invoice.paper` — data-driven Paper invoice and line-item table.
- `paper-lab-report` — data-driven laboratory report source.
- `paper-receituario-a5` — A5 prescription and controlled-prescription themes.
- `table-report/table-report.paper` — multipage Paper table with JSON rows.

Render Paper through `cmd/paper`; each example directory documents its asset
and data flags.

```sh
go run ./cmd/paper render -o /tmp/hello-world.pdf \
  examples/hello-world/hello-world.paper

go run ./cmd/paper render --data examples/invoice/example.json \
  -o /tmp/invoice.pdf examples/invoice/invoice.paper

go run ./cmd/paper render --data examples/table-report/example.json \
  -o /tmp/table-report.pdf examples/table-report/table-report.paper

go run ./cmd/paper render --format html \
  --data examples/invoice/example.json \
  -o /tmp/invoice.html examples/invoice/invoice.paper
```
