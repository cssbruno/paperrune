# Getting started

Compile a `.paper` template and typed data to PDF, standalone HTML, or SVG.

## Install from `main`

PaperRune does not yet have a tagged release:

```sh
go install github.com/cssbruno/paperrune/cmd/paper@main
go install github.com/cssbruno/paperrune/cmd/paper-studio@main
```

## Create a project

```sh
paper init invoice my-invoice
cd my-invoice
paper check
paper render
paper studio
```

This creates a template, sample JSON, and `paper.project.json`. `check`
validates, `render` writes the configured output, and `studio` opens the local
editor.

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

Validation rejects unknown or missing fields, wrong types, invalid
expressions, and layout errors.

## Format and inspect

```sh
paper fmt -w hello.paper
paper explain --page 1 hello.paper
paper capture -o hello.svg hello.paper
```

`explain` prints selected plan structure. `capture` writes SVG evidence.

## Where to go next

- [Projects and data](./projects-and-data)
- [Language reference](/reference/language)
- [Expressions](/reference/expressions)
- [WASM playground](/playground)
