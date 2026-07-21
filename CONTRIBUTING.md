# Contributing

PaperRune uses the Go version declared in `go.mod`; tool dependencies are pinned
separately in `tools/go.mod`.

Start by checking local prerequisites and installing the pinned Go tools:

```sh
make bootstrap
```

| Work area | Required locally |
| --- | --- |
| Core compiler and CLI | Go |
| Studio frontend and WASM | Go, Node, gzip |
| Visual PDF tests | Poppler (`pdftoppm`) |
| Full compliance checks | Docker and external validators |

Use `make test-fast` while iterating, `make test` for the normal suite, and
`make ci` before opening a pull request. `make ci` is the same aggregate target
used by the required GitHub Actions test job. Run `make help` to list the
available repository targets.

Changes to PDF generation, serialization, compliance metadata, or binary
template decoding need malformed-input regressions. Preserve minimized
fuzz inputs in the relevant `testdata/fuzz` corpus when they expose a distinct
failure mode. Security boundaries must return bounded, classifiable errors;
they must not panic or silently reinterpret unsupported syntax.

Follow the ownership and public-surface rules in `ARCHITECTURE.md`. Prefer a
small private helper over a new exported alias or wrapper. Do not add an API
that accepts an existing PDF; that responsibility belongs to PDFRune.

The PDFs under `assets/generated/pdf` are checked-in visual/reference artifacts,
not disposable build output. `make clean` must leave tracked files untouched.
Compliance fixtures are generated under ignored `artifacts/` paths.
