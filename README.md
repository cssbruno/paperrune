# PaperRune

[![CI][badge-ci]][ci]
[![Custom license][badge-license]][license]
[![GoDoc][badge-doc]][godoc]

<img src="https://raw.githubusercontent.com/cssbruno/paperrune/main/assets/static/image/gopher_pdf.png" alt="PaperRune gopher" width="160">

Pure-Go `.paper` compiler for PDF and standalone HTML.

- Strict JSON schemas, reusable styles, themes, tables, images, and pagination.
- PDF signing, inspection, compliance metadata, and CDR.

## Install

Requires Go 1.26.5.

```sh
go get github.com/cssbruno/paperrune@latest
go install github.com/cssbruno/paperrune/cmd/paper@latest
```

From a clone, use `go run ./cmd/paper` instead of `paper`.

## Quick start

`hello.paper`:

```paper
document @hello:
  language: "en"
  title: "Hello"
  page @sheet:
    margin: 36pt
    size: "A4"
    body @content:
      heading @title:
        level: 1
        text: "Hello, world"
```

```sh
paper check hello.paper
paper render -o hello.pdf hello.paper
paper render --format html -o hello.html hello.paper
```

Paper is indentation-sensitive. Format with `paper fmt -w hello.paper`.

## Go API

Embed `.paper` files instead of using indented Go strings:

```go
package main

import (
	_ "embed"
	"log"

	"github.com/cssbruno/paperrune/document"
)

//go:embed hello.paper
var source string

func main() {
	doc := document.MustNew()

	if _, err := doc.WritePaper("hello.paper", source); err != nil {
		log.Fatal(err)
	}
	if err := doc.OutputFile("hello.pdf"); err != nil {
		log.Fatal(err)
	}
}
```

`Document` is single-owner. `PaperPlan` is immutable and reusable.

## CLI

| Command | Purpose |
| --- | --- |
| `paper fmt` | Format Paper source |
| `paper check` | Parse, compile, and plan |
| `paper render` | Write PDF or export HTML |
| `paper capture` | Capture SVG evidence |
| `paper explain` | Inspect plan nodes and pages |
| `paper scenario` | List or select scenarios |

Run `paper COMMAND -h` for all options.

## Data and assets

```sh
# JSON data
paper check --data data.json report.paper
paper render --data data.json -o report.pdf report.paper

# Assets
paper render \
  --assets project.assets.json \
  --asset-root . \
  -o report.pdf report.paper
```

See [`docs/paper-assets.md`](docs/paper-assets.md) and the runnable
[`examples/`](examples/README.md).

## Paper Studio

```sh
make paper-studio \
  PAPER_STUDIO_FILE=examples/hello-world/hello-world.paper
```

Open <http://127.0.0.1:7331>. Studio accepts loopback hosts only.

## Packages

| Package | Purpose |
| --- | --- |
| `document` | Paper planning, PDF generation, and HTML export |
| `sign` | CMS signing and verification |
| `inspect` | PDF inspection and text extraction |
| `pdfcdr` | Content Disarm and Reconstruction |
| `importpdf` | Imported PDF pages |
| `font` | Font tooling |

PaperRune does not accept HTML/CSS input or provide arbitrary PDF editing,
OCR, DOCX conversion, AcroForm editing, or PDF decryption.

## Development

```sh
make check
make modules
make race
make coverage-check
make lint
make nilaway
make gosec
make govulncheck
```

See [`CONTRIBUTING.md`](CONTRIBUTING.md),
[`ARCHITECTURE.md`](ARCHITECTURE.md), and [`CHANGELOG.md`](CHANGELOG.md).

## License

PaperRune uses the [PaperRune Health-Sector Restricted License 1.0][license].
It is source-available, not OSI-approved. Health-Sector Use requires a separate
written commercial license.

[badge-ci]: https://github.com/cssbruno/paperrune/actions/workflows/ci.yml/badge.svg
[badge-doc]: https://img.shields.io/badge/godoc-PaperRune-blue.svg
[badge-license]: https://img.shields.io/badge/license-custom-orange.svg
[ci]: https://github.com/cssbruno/paperrune/actions/workflows/ci.yml
[godoc]: https://pkg.go.dev/github.com/cssbruno/paperrune
[license]: LICENSE
