# Paper headers and page numbers

This example uses a Paper page header, a repeating table header, and
planner-owned page numbers.

Automatic page counters support `header` or `footer` placement; `left`,
`center`, `right`, mirrored `inner`, or mirrored `outer` alignment; optional
first-page suppression; a custom starting number; and custom current/total
formatting.

From the repository root:

```sh
go run ./cmd/paper check \
  --data examples/headers-footers/example.json \
  examples/headers-footers/monthly-report.paper

go run ./cmd/paper render \
  --data examples/headers-footers/example.json \
  -o assets/generated/pdf/examples/headers-footers.pdf \
  examples/headers-footers/monthly-report.paper

go run ./cmd/paper render \
  -o output/pdf/header-page-counter.pdf \
  examples/headers-footers/header-counter.paper
```
