# Getting started

PaperRune turns an indentation-sensitive `.paper` template plus typed data into
PDF, standalone HTML, or an exact SVG page capture. Paper is the only public
authoring format; HTML and PDF are outputs.

## Install from `main`

PaperRune does not yet have a tagged release. Install the current commands with
the Go version declared by the repository:

```sh
go install github.com/cssbruno/paperrune/cmd/paper@main
go install github.com/cssbruno/paperrune/cmd/paper-studio@main
```

Confirm both commands are available:

```sh
paper version
paper-studio version
```

## Create a project

The quickest maintained workflow is generated for you:

```sh
paper init invoice my-invoice
cd my-invoice
paper check
paper render
paper studio
```

The directory contains a Paper template, sample JSON, and
`paper.project.json`. `paper check` validates the complete template and data
contract without publishing an output. `paper render` writes the configured
PDF or HTML file. `paper studio` opens the same project in the local visual
environment.

## Create one file manually

Save this as `hello.paper`:

```paper
document @hello:
  language: "en"
  title: "Hello"

  schema input:
    string name
    bool premium

  page @sheet:
    size: "A4"
    margin: 36pt
    body @content:
      heading @title:
        level: 1
        color: premium ? "#2459D3" : "#171A1F"
        text: name
```

Create `hello.json`:

```json
{
  "name": "Ada Lovelace",
  "premium": true
}
```

Then check and render it:

```sh
paper check --data hello.json hello.paper
paper render --data hello.json -o hello.pdf hello.paper
paper render --format html --data hello.json -o hello.html hello.paper
```

Paper validates JSON against the selected schema before layout. Unknown
fields, missing required fields, incorrect types, invalid expressions, and
layout contract failures are diagnostics instead of silent coercions.

## Format and inspect

```sh
paper fmt -w hello.paper
paper explain --data hello.json hello.paper
paper capture --data hello.json --output-dir captures hello.paper
```

Formatting is deterministic. `explain` reports the planned structure and page
count; `capture` writes exact SVG evidence for planned pages.

## Where to go next

- [Projects and data](./projects-and-data) explains JSON, schemas, scenarios,
  project discovery, and precedence.
- [Complete language reference](/reference/language) lists every public
  authoring construct and property family.
- [Expressions](/reference/expressions) covers booleans, ternaries, switches,
  null guards, numbers, and units.
- [WASM playground](/playground) runs the real compiler locally in the browser.
