# Data-driven Brazilian lab report

From the repository root, validate and render ordinary JSON data:

The source declares reusable `Patient` and `LabResult` objects, then uses them
directly as `Patient patient` and `list LabResult results:` schema fields.

```sh
go run ./cmd/paper check \
  --assets examples/paper-lab-report/assets.json \
  --data examples/paper-lab-report/example.json \
  examples/paper-lab-report/lab-report.paper

go run ./cmd/paper render \
  --assets examples/paper-lab-report/assets.json \
  --data examples/paper-lab-report/example.json \
  -o /tmp/lab-report.pdf \
  examples/paper-lab-report/lab-report.paper
```

Generate reproducible structural and layout edge cases. Every case is
schema-validated, planned, painted, and checked for a complete generated PDF:

```sh
go run ./cmd/paper check \
	--assets examples/paper-lab-report/assets.json \
	--edge-cases 12 --seed 42 \
	--edge-output /tmp/lab-report-edge-cases \
	examples/paper-lab-report/lab-report.paper
```

Read `edge-report.json` to inspect empty/whitespace/multiline values,
longest-string and largest-list JSON Pointers, deterministic input and output
hashes, planned page counts, and positioned layout issues. Use PDFRune or an
external validator when serialized-PDF inspection or raster evidence is
required.

Use repeatable `--edge-input FILE` options for real laboratory payloads, and
`--edge-baseline edge-report.json` to reject output regressions. By default any
positioned layout issue fails the command; the page-issue and planned-page
thresholds can be changed explicitly when a paper has a documented
acceptance policy.

Repeat the same seed to reproduce a failure exactly. The command exits nonzero
when any generated case exposes a layout or PDF problem; generated inputs and
successful PDFs remain available under `--edge-output`.
