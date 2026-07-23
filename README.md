# PaperRune

[![CI][badge-ci]][ci]
[![Custom license][badge-license]][license]
[![GoDoc][badge-doc]][godoc]

<img src="https://raw.githubusercontent.com/cssbruno/paperrune/main/assets/static/image/gopher_pdf.png" alt="PaperRune gopher" width="160">

PaperRune turns indentation-sensitive `.paper` templates and JSON data into
PDF or standalone HTML. It includes a command-line compiler, a local visual
Studio, and a Go API.

Read the [complete documentation](https://cssbruno.github.io/paperrune/) or
compile an example in the browser with the WebAssembly playground.

```paper
document @hello:
  language: "en"
  title: "Hello"
  schema:
    string name
  page @sheet:
    size: "A4"
    margin: 36pt
    body @content:
      heading @title:
        bind: "name"
        level: 1
        text: "Hello"
```

```sh
paper render --data data.json -o hello.pdf hello.paper
paper render --format html --data data.json -o hello.html hello.paper
```

PaperRune supports reusable styles and themes, data binding, tables, images,
pagination, deterministic output, tagged PDF, and PDF/A and PDF/UA metadata.
Standards metadata is not a substitute for validating the final document with
an external compliance checker.

## Install

PaperRune does not currently have a tagged release. Install the current main
branch with the Go version declared in `go.mod`:

```sh
go install github.com/cssbruno/paperrune/cmd/paper@main
go install github.com/cssbruno/paperrune/cmd/paper-studio@main
```

Or build both commands from a clone:

```sh
git clone https://github.com/cssbruno/paperrune.git
cd paperrune
mkdir -p bin
go build -o ./bin/paper ./cmd/paper
go build -o ./bin/paper-studio ./cmd/paper-studio
```

Confirm the installation with `paper version` and `paper-studio version`.

## Create a document

Save the opening example as `hello.paper` and create `data.json`:

```json
{"name":"Ada Lovelace"}
```

Then validate and render it:

```sh
paper check --data data.json hello.paper
paper render --data data.json -o hello.pdf hello.paper
```

Paper is indentation-sensitive. Format source with:

```sh
paper fmt -w hello.paper
```

See the [expression reference](docs/reference/expressions.md) for calculated
properties, visibility, switches, optional data, and component selection.

## Commands

| Command | Purpose |
| --- | --- |
| `paper fmt` | Format Paper source |
| `paper check` | Parse, validate, and plan a document |
| `paper render` | Generate PDF or standalone HTML |
| `paper studio` | Open the local visual authoring environment |
| `paper explain` | Inspect planned nodes and pages |
| `paper capture` | Capture planned pages as SVG evidence |
| `paper scenario` | List or inspect data scenarios |
| `paper version` | Print build and version information |

Run `paper help COMMAND` for command-specific options and examples. The
[CLI reference](docs/reference/cli.md) documents behavior and exit codes.

## Paper Studio

Studio opens a local editing session for a Paper file:

```sh
paper studio examples/hello-world/hello-world.paper
```

It listens only on loopback and protects each session with a per-process
token. Undo and redo belong to that local Studio session; use Git or another
host system for durable history and collaboration. Use `--no-open` when
running without a desktop browser.

## Go API

Applications can embed Paper source and render it with the `document` package:

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
	data := []byte(`{"name":"Ada Lovelace"}`)

	if _, err := doc.WritePaperJSON("hello.paper", source, data); err != nil {
		log.Fatal(err)
	}
	if err := doc.OutputFile("hello.pdf"); err != nil {
		log.Fatal(err)
	}
}
```

`Document` is a single-owner build session. Compiled `PaperPlan` values are
immutable and reusable.

## Assets and examples

External images and fonts are loaded through a content-addressed asset
manifest:

```sh
paper render \
  --assets project.assets.json \
  --asset-root . \
  -o report.pdf report.paper
```

See the [asset guide](docs/paper-assets.md) and the maintained
[examples](examples/README.md).

## Scope

PaperRune generates new documents from Paper source and data. It does not read,
edit, sign, sanitize, decrypt, or import existing PDF files, and it does not use
HTML/CSS or DOCX as authoring input. Those boundaries keep untrusted PDF parsing
and signing keys out of the generator.

The only public authoring package is `document`; layout and rendering internals
are private. Custom fonts are supplied directly as TTF/OTF assets.

## Development

```sh
make bootstrap   # install pinned development tools
make test-fast   # formatting, vet, command, and internal tests
make test        # complete Go test suite
make docs-check  # build the CLI and exercise the documented command flow
npm ci
make docs-site-check # build VitePress and smoke-test the browser compiler
```

See [CONTRIBUTING.md](CONTRIBUTING.md), [ARCHITECTURE.md](ARCHITECTURE.md), and
[CHANGELOG.md](CHANGELOG.md).

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
