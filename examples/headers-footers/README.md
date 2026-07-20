# Paper headers and page numbers

This replacement for the former direct-placement example uses a Paper page
header, a repeating table header, and planner-owned page numbers.

From the repository root:

```sh
go run ./cmd/paper check \
  --data examples/headers-footers/example.json \
  examples/headers-footers/monthly-report.paper

go run ./cmd/paper render \
  --data examples/headers-footers/example.json \
  -o assets/generated/pdf/examples/headers-footers.pdf \
  examples/headers-footers/monthly-report.paper
```
