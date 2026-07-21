# PaperRune

[![CI][badge-ci]][ci]
[![Custom license][badge-license]][license]
[![GoDoc][badge-doc]][godoc]

<img src="https://raw.githubusercontent.com/cssbruno/paperrune/main/assets/static/image/gopher_pdf.png" alt="PaperRune gopher" width="160">

Pure-Go `.paper` compiler for PDF and standalone HTML.

- Strict JSON schemas, reusable styles, themes, tables, images, and pagination.
- Immutable plans, deterministic PDF generation, compliance metadata, and HTML export.

PaperRune is intentionally a **generator**, not a PDF editor. It never accepts
an existing PDF as input. Operations on existing PDFs live in the independent
PDFRune project.

## Install

### Prebuilt binary

Each GitHub release contains `paper` and `paper-studio` for Linux, macOS, and
Windows on AMD64 and ARM64. Download the archive for your platform from
[GitHub Releases][releases], verify it against `checksums.txt`, then place both
executables on your `PATH`.

Release archives also contain CycloneDX SBOMs. Published files have GitHub
build-provenance attestations that can be checked with:

```sh
gh attestation verify paperrune_0.16.0-rc.1_linux_amd64.tar.gz --repo cssbruno/paperrune
```

### Go install

Requires the Go version declared in `go.mod`.

```sh
go install github.com/cssbruno/paperrune/cmd/paper@latest
go install github.com/cssbruno/paperrune/cmd/paper-studio@latest
```

### Build from source

```sh
git clone https://github.com/cssbruno/paperrune.git
cd paperrune
make build
go build -o ./bin/paper ./cmd/paper
go build -o ./bin/paper-studio ./cmd/paper-studio
```

Confirm either installation with `paper version` and `paper-studio version`.

## Quick start

Create and render a complete starter project:

```sh
paper init invoice my-invoice
cd my-invoice
paper check
paper render
paper studio
```

The project manifest supplies the source, data, format, and output path. CLI
flags override environment variables, which override the manifest. See the
[project-file reference](docs/reference/project-file.md).

For a single-file workflow, create `hello.paper`:

```paper
document @hello:
  language: "en"
  title: "Hello"
  schema:
    string name
    optional string subtitle
  page @sheet:
    margin: 36pt
    size: "A4"
    body @content:
      heading @title:
        bind: "name"
        level: 1
        text: "Name"
      paragraph @subtitle:
        bind: "subtitle"
        bind-required: false
        text: ""
```

`data.json`:

```json
{"name":"Ada","subtitle":"Generated from JSON"}
```

```sh
paper check --data data.json hello.paper
paper render --data data.json -o hello.pdf hello.paper
paper render --format html --data data.json -o hello.html hello.paper
```

`subtitle` may be omitted or `null`: schema fields use `optional`; their nodes
use `bind-required: false`.

Paper is indentation-sensitive. Format with `paper fmt -w hello.paper`.

## Go API

Embed `.paper` files instead of using indented Go strings:

```go
package main

import (
	_ "embed"
	"encoding/json"
	"log"

	"github.com/cssbruno/paperrune/document"
)

//go:embed hello.paper
var source string

type Input struct {
	Name     string `json:"name"`
	Subtitle string `json:"subtitle,omitempty"`
}

func main() {
	doc := document.MustNew()

	data, err := json.Marshal(Input{
		Name:     "Ada",
		Subtitle: "Generated from Go",
	})
	if err != nil {
		log.Fatal(err)
	}

	if _, err := doc.WritePaperJSON("hello.paper", source, data); err != nil {
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
| `paper init` | Create a runnable Paper project |
| `paper fmt` | Format Paper source |
| `paper check` | Parse, compile, and plan |
| `paper render` | Write PDF or export HTML |
| `paper studio` | Open the local visual authoring environment |
| `paper capture` | Capture SVG evidence |
| `paper explain` | Inspect plan nodes and pages |
| `paper scenario` | List or select scenarios |
| `paper workflow` | Execute an approved delivery workflow |
| `paper version` | Print version information |

Run `paper help COMMAND` for options and examples. See the full
[CLI behavior and exit-code reference](docs/reference/cli.md).

## Assets

```sh
paper render \
  --assets project.assets.json \
  --asset-root . \
  -o report.pdf report.paper
```

See [`docs/paper-assets.md`](docs/paper-assets.md) and the runnable
[`examples/`](examples/README.md).

## Paper Studio

```sh
paper studio examples/hello-world/hello-world.paper
```

Studio binds only to loopback, opens the protected session URL by default, and
prints the URL for copying. Pass `--no-open` in headless environments.

## Generator boundary

The boundary is strict and has no deprecated compatibility wrappers:

| PaperRune does | PaperRune does not do |
| --- | --- |
| Parse `.paper` source and data | Accept PDF bytes, readers, or file paths as input |
| Build and inspect an immutable layout plan | Parse or inspect a serialized PDF |
| Generate a new PDF or standalone HTML | Import pages from an existing PDF |
| Emit PDF/A and PDF/UA generation metadata | Sign, verify, sanitize, decrypt, or modify an existing PDF |

Use the independent `github.com/cssbruno/pdfrune` module for post-generation
operations such as CMS/PAdES signing, signature verification, inspection,
page import, CDR, and final-byte evidence. Neither project imports the other.
The application server owns their composition:

```go
var generated bytes.Buffer
if err := doc.Output(&generated); err != nil {
	return err
}

signed, err := sign.AppendBytesContext(ctx, generated.Bytes(), options)
```

Here `doc` is a PaperRune document and `sign` is PDFRune's signing package.
Keeping the handoff in the server makes the trust boundary visible and lets a
deployment provide its own HSM, KMS, PKCS#11, managed `crypto.Signer`, audit,
authorization, and storage policy. PaperRune itself never handles signing keys.

## Packages

| Package | Purpose |
| --- | --- |
| `document` | Paper planning, PDF generation, and HTML export |
| `font` | Font tooling |

PaperRune does not accept an existing PDF, HTML/CSS input, or provide PDF
editing, OCR, DOCX conversion, AcroForm editing, signing, or decryption.

## Development

```sh
make bootstrap
make test-fast
make test
make ci
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
[releases]: https://github.com/cssbruno/paperrune/releases
