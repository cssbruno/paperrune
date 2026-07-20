# PaperRune

[![CI][badge-ci]][ci]
[![Custom license][badge-license]][license]
[![GoDoc][badge-doc]][godoc]

<img src="https://raw.githubusercontent.com/cssbruno/paperrune/main/assets/static/image/gopher_pdf.png" alt="PaperRune gopher" width="160">

PaperRune is a pure-Go document toolkit for applications that generate PDF or
standalone HTML from human-readable Paper source. Paper is the only public
authoring format. The PDF serializer is private, and HTML is an output artifact
rather than an input language.

## Highlights

- Generate PDF and standalone HTML from the same immutable `.paper` plan.
- Use schemas, JSON data, themes, reusable components, and exact pagination.
- Sign and verify PDFs with CMS, inspect PDF structure, and sanitize uploads
  through a PDF Content Disarm and Reconstruction boundary.

The main API is `github.com/cssbruno/paperrune/document`. Ownership rules and
the Paper-only public-surface policy is documented in
[`ARCHITECTURE.md`](ARCHITECTURE.md).

## Contents

- [Installation](#installation)
- [Quick start](#quick-start)
- [Choosing a document path](#choosing-a-document-path)
- [Features](#features)
- [Limitations](#limitations)
- [Examples](#examples)
- [Paper to HTML](#paper-to-html)
- [Advanced workflows](#advanced-workflows)
- [Packages and development](#packages-and-development)
- [License](#license)

## Installation

PaperRune requires Go 1.26.5 or newer within the supported Go toolchain. The
library is pure Go; CI runs on Linux and cross-builds for Windows. Other
Go-supported platforms may compile but are not part of the compatibility
guarantee.

```shell
go get github.com/cssbruno/paperrune@latest
```

## Quick start

```go
package main

import "github.com/cssbruno/paperrune/document"

func main() {
	pdf, err := document.NewDocument()
	if err != nil {
		panic(err)
	}

	source := `document @hello:
  page @sheet:
    body @body:
      heading @title:
        level: 1
        text: "Hello, world"`

	if result, err := pdf.WritePaper("hello.paper", source); err != nil || !result.OK() {
		panic(err)
	}

	if err := pdf.OutputFile("hello.pdf"); err != nil {
		panic(err)
	}
}
```

Construction can be configured with typed options:

```go
pdf := document.MustNew(
	document.WithOrientation(document.OrientationPortrait),
	document.WithUnit(document.UnitMillimeter),
	document.WithPageSize(document.PageSizeA4),
	document.WithBestCompression(),
)
```

`WithBestCompression` selects stronger zlib compression for generated page and
template streams. It is not a general-purpose optimizer for images, fonts,
object streams, or existing PDFs.

## Choosing a document path

| Need | Recommended API |
| --- | --- |
| Human-readable document source, themes, and data | `WritePaper`, `PlanPaper`, or `cmd/paper` |
| Report-like documents with changing values | Paper schemas with `WritePaperJSON` or `--data` |
| Browser-viewable output | `PaperPlan.ExportHTML` or `paper render --format html` |
| Large unsigned output with lower peak memory | `OutputFileStream` or `OutputOptions{StreamFinal: true}` |

## Features

- Paper documents, reusable styles/components, themes, data schemas, scenarios,
  headers, footers, page regions, and explicit page breaks.
- Standard PDF fonts and UTF-8 TrueType fonts, including OpenType files with
  TrueType outlines.
- Paper text, lists, tables, images, page regions, positioned canvas content,
  and deterministic rows and columns.
- Document metadata, output intents, attachments, PDF signing, CMS verification,
  and lightweight inspection.
- PDF Content Disarm and Reconstruction through the `pdfcdr` package.

## Limitations

The following are intentionally not general-purpose features:

- HTML/CSS input or browser-owned layout. HTML export embeds exact Paper page
  SVG and does not ask the browser to reflow or paginate content.
- Direct page/text/cell/drawing placement or mixed imperative/declarative
  authoring.
- PDF JavaScript actions; `javascript:` links are rejected.
- DOCX conversion or interactive AcroForm field creation.
- Filling, flattening, or FDF-merging existing interactive forms.
- Unlocking, decrypting, or removing passwords from existing PDFs.
- Arbitrary PDF editing, OCR, or universal semantic text extraction.

Imported pages support classic xref-table PDFs, unencrypted documents, and
pages whose content streams are unfiltered, FlateDecode-compressed, or use the
common ASCII85Decode-to-FlateDecode chain. Xref streams, object streams, other
filter chains, and ambiguous stream lengths fail closed.

Password protection applies to newly generated output. Permission flags are
advisory because PDF readers decide how strictly to enforce them.

## Paper Engine

The repository includes the human-readable `.paper` compiler and a read-first
Paper Studio workspace. Studio displays pages captured from the immutable
display plan; it is not a browser-layout replacement.

```shell
go run ./cmd/paper check testdata/paper/studio-demo.paper
make paper-studio PAPER_STUDIO_FILE=testdata/paper/studio-demo.paper
```

Open `http://127.0.0.1:7331`. Studio intentionally accepts only an explicit
loopback listen host and refuses wildcard, LAN, and public bind addresses.
Run `make test-paper-studio-js` for the dependency-free page-rail model tests.
See [`PAPER_ENGINE_PLAN.md`](PAPER_ENGINE_PLAN.md) and the
[`.paper` asset guide](docs/paper-assets.md) for the current design.

### Responsive `.paper` rows and columns

Rows and columns use a deterministic, fixed-point auto-layout solver. Main-axis
sizes can be intrinsic (`"auto"`), container-relative percentages, or flex
bases with grow/shrink factors. Percentages are resolved from the containing
layout region at plan time, so rendering at 50%, 100%, HiDPI, or print DPI does
not change document geometry.

```text
row @cards:
  gap: 3mm
  height: 45mm
  line-gap: 3mm
  wrap: "wrap"
  justify-content: "space-between"
  align-items: "stretch"
  align-content: "space-around"
  paragraph @primary:
    width: 50%
    flex-grow: 1
    flex-shrink: 1
    min-width: 30%
    text: "Primary"
  paragraph @secondary:
    width: "auto"
    flex-grow: 1.5
    flex-shrink: 1
    align-self: "center"
    text: "Secondary"
```

Use `width` for children inside a row and `height` for children inside a
column. A whole-number `fr` value divides the remaining space proportionally:

```paper
row @summary:
  paragraph @main:
    width: 2fr
    text: "Main"
  paragraph @aside:
    width: 1fr
    text: "Aside"
```

`wrap` accepts `nowrap`, `wrap`, and `wrap-reverse`. Main alignment accepts
`start`, `center`, `end`, `space-between`, `space-around`, and `space-evenly`;
`align-content` also accepts `stretch`. Physical units such as `pt`, `mm`,
`cm`, and `in` remain appropriate for page and print constraints because they
are device-independent rather than screen pixels.

### Render application JSON and check generated edge cases

A `.paper` document can declare a typed schema and receive ordinary JSON at
render time. Types lead each declaration; required fields are the default and
`optional` is explicit. Reusable custom objects live at document scope and can
be used directly by fields and bounded lists:

```paper
object Patient:
  string name

object Medication:
  string name
  string directions

schema:
  string clinic
  optional string phone
  Patient patient
  optional Patient responsible
  list Medication medications:
    max-items: 10
```

Use inline `object fieldName:` or `list object fieldName:` when the shape is
local to one field. Custom object names must resolve to a document-scoped
`object Name:` declaration; unknown names and recursive object graphs are
compile errors.

The JSON adapter rejects unknown fields, missing required fields, wrong types,
duplicate keys, and lists beyond their declared bounds before any PDF state is
changed. A document with one schema uses root-relative paths such as
`bind: "patient.name"` and `source: "medications"`.

When several schemas are intentionally declared, give them bare names and use
that name only to disambiguate a path:

```paper
schema invoice:
  number total

schema receipt:
  string reference

paragraph:
  bind: "invoice.total"
```

The removed `field @name`, `type:`, `required:` and `@schema.field` forms are
not accepted by the grammar.

```shell
go run ./cmd/paper check --data report.json report.paper
go run ./cmd/paper render --data report.json -o report.pdf report.paper
```

`check --edge-cases` generates fixed boundaries for empty text, whitespace,
multiline text, excessive wrapping, 256-character unbroken strings, dense lists,
Portuguese Unicode, punctuation/escaping, and numeric extremes before seeded
random cases. Every case is schema-validated and runs through planning,
painting, structural PDF parsing, page-count comparison, and text extraction.
A seed makes failures replayable.

`--edge-output` keeps every input and PDF plus `edge-report.json`. The report
records empty/whitespace/multiline string counts, the JSON Pointer and size of
the longest string and largest list, input/PDF hashes, per-page extracted-text
hashes, exact page summaries, and positioned layout issues. Layout issues fail
the check by default; use `--edge-max-page-issues`, `--edge-min-text-runes`, and
`--edge-max-pages` to make the acceptance policy explicit.

Add repeatable `--edge-input FILE` options to exercise real user payloads in
addition to, or instead of, generated cases. `--edge-baseline edge-report.json`
detects changed, added, and missing cases. Add `--edge-visual` to rasterize the
final PDFs with Poppler, retain one PNG per PDF page, and create
`edge-visual-review.pdf`. The visual evidence therefore comes from the written
PDF files, not an HTML layout or an internal planning preview.

```shell
go run ./cmd/paper check --edge-cases 16 --seed 42 report.paper
go run ./cmd/paper check --edge-cases 16 --seed 42 --edge-output ./edge-artifacts report.paper
go run ./cmd/paper check --edge-cases 16 --seed 42 --edge-output ./edge-artifacts --edge-visual report.paper
go run ./cmd/paper check --edge-input ./production-shape.json --edge-output ./edge-artifacts --edge-visual report.paper
go run ./cmd/paper check --edge-input ./production-shape.json --edge-baseline ./known-good/edge-report.json report.paper
```

Use `--schema NAME` when the document declares multiple schemas and
`--locale pt-BR` when presentation does not already inherit an explicit
locale. See the [Brazilian lab example](examples/paper-lab-report/README.md).

## Examples

Paper examples live under [`examples/`][examples].

| Workflow | Command | Output |
| --- | --- | --- |
| Paper lab report | `go run ./cmd/paper render --assets examples/paper-lab-report/assets.json --data examples/paper-lab-report/example.json -o output.pdf examples/paper-lab-report/lab-report.paper` | `output.pdf` |
| Paper invoice | `go run ./cmd/paper render --data examples/invoice/example.json -o invoice.pdf examples/invoice/invoice.paper` | `invoice.pdf` |
| Paper to HTML | `go run ./cmd/paper render --format html --data examples/invoice/example.json -o invoice.html examples/invoice/invoice.paper` | `invoice.html` |

## Paper to HTML

HTML is a deterministic output format, not an authoring frontend. Plan Paper
once and export its exact display-list pages as inline SVG:

```go
plan, result, err := document.PlanPaper("invoice.paper", source)
if err != nil || !result.OK() {
	return err
}
html, err := plan.ExportHTML()
```

Each HTML page contains the same immutable SVG display commands used by Paper
Studio. Browser CSS only arranges page sheets; it does not measure text, wrap,
position, or paginate authored content.

## Advanced workflows

### Large PDF output

Normal `Output` and `OutputFile` calls keep final PDF bytes in the document
buffer, which allows repeated output from one `Document`. For very large
unsigned PDFs where peak memory matters more than repeatability, use:

```go
err := pdf.OutputFileStream("large.pdf")
```

Streaming final output is opt-in, consumes the document for final output, and
is disabled for signed output because signing needs the complete byte range.

### Fonts

Standard PDF fonts require no setup:

```paper
style @body:
  font: "Helvetica"
  size: 12pt
```

Register UTF-8 TrueType resources through an explicit Paper asset manifest or
`PaperAssetCatalog`, then select the declared family from Paper styles.

For non-UTF-8 TrueType, OpenType/CFF, or Type1 fonts, generate a JSON font
definition with `font.Make` or `cmd/fontmaker`:

```shell
cd cmd/fontmaker
go build
./fontmaker --embed --enc=../../assets/static/font/cp1252.map --dst=../../assets/static/font ../../assets/static/font/calligra.ttf
```

### Signing and verification

Documents can be signed while writing:

```go
err := pdf.OutputSignedFile("signed.pdf", sign.Options{
	Signer:      signer,
	Certificate: cert,
})
```

The `sign` package uses CMS terminology and supports detached CMS creation,
PDF signature extraction and embedding, signer metadata inspection, and
trusted verification. Trusted verification requires a non-nil root
certificate pool; use the explicitly named integrity-only APIs only when signer
trust is not part of the decision.

### Inspection and sanitization

Use `inspect` for lightweight PDF checks and text extraction from literal text
operators:

```go
count, err := inspect.PageCount(pdfBytes)
text, err := inspect.Text(pdfBytes)
streams, err := inspect.DecodedStreams(pdfBytes)
```

Use `pdfcdr` when an uploaded PDF must cross a reconstruction boundary before
being opened or stored:

```go
clean, err := pdfcdr.Sanitize(input)
if err != nil {
	return err
}
```

CDR removes document actions, annotations, forms, JavaScript, embedded files,
external references, metadata, and unreachable objects while preserving page
painting commands and reachable rendering resources. It is a structural
reconstruction boundary, not malware detection or a substitute for isolating a
downstream PDF viewer.

### Compliance metadata

`document.SetComplianceMetadata` provides PDF/A-4, PDF/UA-2, Arlington, and XMP
metadata foundations. PDF/A mode enforces known generation blockers such as
encryption and JavaScript restrictions, output intents, and embedded fonts.
PDF/UA-2 mode emits tagged PDF structures and semantic content metadata.

This is not a validator replacement. Use `make compliance-fixtures` and
`make compliance-validate` with the external validators required by your
workflow.

## Packages and development

| Package | Purpose |
| --- | --- |
| `document` | Main PDF generation and rendering API |
| `layout` | Internal shared layout model and measurement primitives |
| `font` | Font parsing and JSON font definition generation |
| `importpdf` | Bounded classic-xref parser and imported-page resolver |
| `inspect` | Lightweight PDF structure, stream, page, and text inspection |
| `pdfcdr` | PDF Content Disarm and Reconstruction |
| `sign` | CMS-first PDF signing and signature verification |

Useful repository directories:

```text
cmd/                    command-line tools and Paper Studio
document/               main PDF API
layout/                 renderer-independent document model
examples/               runnable examples
assets/static/          checked-in fonts, images, and fixtures
assets/generated/pdf/   generated example PDFs
tools/                  tool-only module and validation helpers
```

Common development commands:

```shell
go test ./...
go vet ./...
go list ./...
make check
make test-paper-studio-js
```

Test examples generate PDFs in unique temporary directories, so running the
test suite does not remove or overwrite repository assets. See
[`CHANGELOG.md`](CHANGELOG.md) for historical API changes.

## Errors

Paper entry points return errors for compilation or rendering failure.
Once an error is recorded, later operations do not change the PDF. Check
`Ok()`, `Err()`, or `Error()` before trusting output.

## License

PaperRune is released under the PaperRune Health-Sector Restricted License 1.0.
Use, modification, and distribution are free for Non-Health-Sector Use under
the terms of [`LICENSE`][license]. Health-Sector Organizations and vendors
acting for them must obtain a separate written commercial license before
making Health-Sector Use. This is a source-available license, not an
OSI-approved open-source license. For licensing requests, contact the project
maintainers through the repository's normal project contact channel.


[badge-ci]: https://github.com/cssbruno/paperrune/actions/workflows/ci.yml/badge.svg
[badge-doc]: https://img.shields.io/badge/godoc-PaperRune-blue.svg
[badge-license]: https://img.shields.io/badge/license-custom-orange.svg
[ci]: https://github.com/cssbruno/paperrune/actions/workflows/ci.yml
[examples]: examples
[godoc]: https://pkg.go.dev/github.com/cssbruno/paperrune
[license]: https://raw.githubusercontent.com/cssbruno/paperrune/main/LICENSE
