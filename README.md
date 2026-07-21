# PaperRune

[![CI][badge-ci]][ci]
[![Custom license][badge-license]][license]
[![GoDoc][badge-doc]][godoc]

<img src="https://raw.githubusercontent.com/cssbruno/paperrune/main/assets/static/image/gopher_pdf.png" alt="PaperRune gopher" width="160">

PaperRune is a pure-Go toolkit for generating PDF and standalone HTML from
human-readable `.paper` documents. One immutable layout plan drives every
output, so pagination does not depend on browser layout.

Paper is the only public authoring format. Direct PDF placement and HTML/CSS
input are intentionally outside the public API.

## Features

- PDF and self-contained HTML from the same `.paper` source.
- Strict JSON schemas for data-driven documents.
- Styles, themes, components, tables, images, headers, footers, rows, columns,
  page regions, and explicit page breaks.
- Content-addressed image and font assets.
- Local authoring and inspection with Paper Studio.
- PDF signing, verification, inspection, and Content Disarm and Reconstruction.

## Install

PaperRune requires the Go version declared in [`go.mod`](go.mod), currently Go
1.26.5. It has no C dependency.

```sh
go get github.com/cssbruno/paperrune@latest
go install github.com/cssbruno/paperrune/cmd/paper@latest
```

When working from a clone, replace `paper` below with `go run ./cmd/paper`.

> [!IMPORTANT]
> PaperRune uses a custom source-available license with a health-sector
> restriction. Review [License](#license) before adopting it.

## Quick start

Render the maintained hello-world example:

```sh
go run ./cmd/paper check examples/hello-world/hello-world.paper
go run ./cmd/paper render -o hello.pdf \
  examples/hello-world/hello-world.paper
```

Render the same source as standalone HTML:

```sh
go run ./cmd/paper render --format html -o hello.html \
  examples/hello-world/hello-world.paper
```

Format Paper source in place:

```sh
go run ./cmd/paper fmt -w document.paper
```

## Go API

The primary package is `github.com/cssbruno/paperrune/document`.

```go
package main

import (
	"fmt"
	"log"

	"github.com/cssbruno/paperrune/document"
)

func main() {
	doc, err := document.NewDocument()
	if err != nil {
		log.Fatal(err)
	}

	source := `document @hello:
  title: "Hello"
  language: "en"

  page @sheet:
    size: "A4"
    margin: 36pt

    body @content:
      heading @title:
        level: 1
        text: "Hello, world"`

	result, err := doc.WritePaper("hello.paper", source)
	if err != nil || !result.OK() {
		for _, diagnostic := range result.Diagnostics {
			fmt.Printf("%s:%d:%d: %s\n", diagnostic.File,
				diagnostic.StartLine, diagnostic.StartColumn, diagnostic.Message)
		}
		if err != nil {
			log.Fatal(err)
		}
		log.Fatal("Paper render failed")
	}

	if err := doc.OutputFile("hello.pdf"); err != nil {
		log.Fatal(err)
	}
}
```

Typed construction options are also available:

```go
doc := document.MustNew(
	document.WithOrientation(document.OrientationPortrait),
	document.WithUnit(document.UnitMillimeter),
	document.WithPageSize(document.PageSizeA4),
	document.WithBestCompression(),
)
```

`Document` is a mutable, single-owner build session and is not safe for
concurrent calls. Immutable `PaperPlan` values can be reused for PDF and HTML.

## Data and assets

Paper schemas reject missing required fields, unknown fields, duplicate keys,
wrong types, and lists beyond their declared bounds before the document
changes.

```paper
object Patient:
  string name

schema:
  string clinic
  Patient patient

paragraph @patient-name:
  bind: "patient.name"
  text: "Patient name"
```

```sh
go run ./cmd/paper check --data report.json report.paper
go run ./cmd/paper render --data report.json -o report.pdf report.paper
```

Use `--schema NAME` when a document declares multiple schemas and `--locale`
when it needs an explicit presentation locale.

Images and embedded fonts use an explicit content-addressed manifest:

```paper
image @hero:
  source: "asset:hero-image"
  width: 240pt
  height: 135pt
  fit: "cover"
```

```sh
go run ./cmd/paper render \
  --assets project.assets.json \
  --asset-root . \
  -o report.pdf report.paper
```

Paper never searches the filesystem, environment, or network while planning.
See the [asset guide](docs/paper-assets.md) for manifest structure, the Go API,
font metadata, fallbacks, and resource limits.

## CLI

| Command | Purpose |
| --- | --- |
| `paper fmt` | Canonically format source; add `-w` to update it |
| `paper check` | Parse, compile, and plan a document |
| `paper render` | Write PDF or standalone HTML |
| `paper capture` | Capture bounded SVG evidence |
| `paper explain` | Inspect selected plan nodes or pages |
| `paper scenario` | List or select declared scenarios |
| `paper workflow` | Run the reviewed headless edit/export workflow |

Common options include `--data`, `--schema`, `--locale`, `--scenario`,
`--assets`, `--asset-root`, and `--json`. Run `paper COMMAND -h` for details.

For deterministic schema edge cases:

```sh
go run ./cmd/paper check --edge-cases 16 --seed 42 report.paper
```

Add `--edge-output DIR` to retain inputs, PDFs, and the report. Add
`--edge-visual` to create a Poppler-rasterized review PDF.

## Paper Studio

Paper Studio is a local authoring and inspection workspace backed by the same
immutable plan as PDF output.

```sh
make paper-studio \
  PAPER_STUDIO_FILE=examples/hello-world/hello-world.paper
```

Open <http://127.0.0.1:7331>. The server accepts loopback hosts only.

The full demo requires its explicit assets:

```sh
make paper-studio-wasm

go run ./cmd/paper-studio \
  -assets testdata/paper/studio-assets.json \
  -asset-root testdata/paper/assets \
  testdata/paper/studio-demo.paper
```

## Output and PDF tools

Plan independently when you need both PDF and HTML:

```go
plan, result, err := document.PlanPaper("report.paper", source)
if err != nil {
	return err
}
if !result.OK() {
	return document.ErrPaperRender
}

html, err := plan.ExportHTML()
```

HTML pages are exact inline SVG artifacts; the browser does not reflow them.

For large unsigned PDFs, `OutputFileStream` lowers peak memory but consumes the
document for final output:

```go
err := doc.OutputFileStream("large.pdf")
```

Related packages:

| Package | Purpose |
| --- | --- |
| `sign` | CMS signing and trusted verification |
| `inspect` | Bounded structure, stream, page, and text inspection |
| `pdfcdr` | Content Disarm and Reconstruction for untrusted PDFs |
| `importpdf` | Bounded classic-xref imported-page support |
| `font` | Font parsing and definition generation |

Compliance metadata is available through `Document.SetComplianceMetadata`.
External validation is still required; see
[`testdata/compliance/README.md`](testdata/compliance/README.md).

## Examples

More examples are documented under [`examples/`](examples/README.md).

```sh
# Invoice
go run ./cmd/paper render \
  --data examples/invoice/example.json \
  -o invoice.pdf examples/invoice/invoice.paper

# Lab report with assets
go run ./cmd/paper render \
  --assets examples/paper-lab-report/assets.json \
  --data examples/paper-lab-report/example.json \
  -o lab-report.pdf examples/paper-lab-report/lab-report.paper
```

See also the [built-in authoring tools](docs/built-in-tools.md).

## Scope and limitations

PaperRune does not provide:

- HTML/CSS input or browser-owned pagination.
- Public direct page, text, cell, or drawing placement.
- Arbitrary PDF editing, OCR, or universal text extraction.
- DOCX conversion or interactive AcroForm creation and filling.
- Unlocking, decrypting, or removing passwords from existing PDFs.
- PDF JavaScript actions; `javascript:` links are rejected.

Imported pages must be unencrypted classic xref-table PDFs using supported
content-stream filters. Unsupported structures fail closed. PDF permission
flags are advisory because readers decide how strictly to enforce them.

For servers, configure explicit `Limits` and `SecurityPolicy` values or start
with `document.WithServerSafeDefaults()`.

## Development

```sh
make check          # tests, vet, and formatting
make modules        # verify both Go modules
make race           # race-enabled tests
make coverage-check
make lint
make nilaway
make gosec
make govulncheck
```

Tool versions are pinned in `tools/go.mod`. See
[`CONTRIBUTING.md`](CONTRIBUTING.md), [`ARCHITECTURE.md`](ARCHITECTURE.md), and
[`CHANGELOG.md`](CHANGELOG.md).

## License

PaperRune is released under the PaperRune Health-Sector Restricted License
1.0. Non-Health-Sector Use is permitted under [`LICENSE`][license].
Health-Sector Organizations and vendors acting for them must obtain a separate
written commercial license before any Health-Sector Use.

This is a source-available license, not an OSI-approved open-source license.

[badge-ci]: https://github.com/cssbruno/paperrune/actions/workflows/ci.yml/badge.svg
[badge-doc]: https://img.shields.io/badge/godoc-PaperRune-blue.svg
[badge-license]: https://img.shields.io/badge/license-custom-orange.svg
[ci]: https://github.com/cssbruno/paperrune/actions/workflows/ci.yml
[godoc]: https://pkg.go.dev/github.com/cssbruno/paperrune
[license]: LICENSE
