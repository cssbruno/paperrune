# Command-line reference

Run `paper help COMMAND` for exact flags and examples.

## Commands

| Command | Purpose | Main options |
| --- | --- | --- |
| `fmt FILE` | Canonically format source | `-w`, `--json`; `FILE` may be `-` |
| `check FILE` | Parse, type-check, select data, and plan | data/assets options; `--scenario`; edge-case options |
| `render FILE` | Write PDF or standalone HTML | data/assets options; `--scenario`, `--format`, `-o`, `--json` |
| `studio FILE` | Open Paper Studio | forwards Studio options |
| `explain FILE` | Inspect plan structure | one or more of `--node`, `--key`, `--instance`, `--fragment`, `--page`; `--max-results` |
| `capture FILE` | Capture SVG plan evidence | `--node`, `--fragment`, `--mode`, `--contact-sheet`, `--columns`, `--max-pages`, `--max-crops`, `-o`, `--json` |
| `scenario FILE` | List or resolve scenarios | `--scenario`, `--json` |
| `version` | Print build version | `--json` |

## Shared selection options

| Option | Meaning |
| --- | --- |
| `--data FILE` | Strict external JSON; mutually exclusive with `--scenario` |
| `--schema NAME` | Select a schema when the document declares more than one |
| `--locale LOCALE` | Formatting locale for external/generated data |
| `--scenario NAME` | Select an in-source scenario |
| `--assets FILE` | Explicit content-addressed asset manifest |
| `--asset-root DIR` | Manifest path root; requires `--assets` |

Data options apply to `check` and `render`. Asset options also apply to
`explain` and `capture`.

## Edge-case checks

`paper check` can generate or load deterministic validation cases:

| Option | Meaning |
| --- | --- |
| `--edge-cases COUNT`, `--seed N` | Generate reproducible schema data |
| `--edge-input FILE` | Add a user JSON case; repeatable |
| `--edge-max-items N` | Bound generated list size |
| `--edge-max-pages N` | Bound pages per case |
| `--edge-max-page-issues N` | Allowed layout issues per case |
| `--edge-output DIR` | Write generated JSON, PDF, and report evidence |
| `--edge-baseline FILE` | Compare with an earlier `edge-report.json` |
| `--edge-allow-baseline-change` | Report baseline changes without failing |

## Normal authoring flow

```sh
paper check --data data.json invoice.paper
paper render --data data.json invoice.paper
paper studio invoice.paper
```

Commands take the source file explicitly. Data, asset manifests, output paths,
formats, and locales are selected with command options.

## Output behavior

`paper render FILE` writes bytes to standard output when output is redirected.
When standard output is an interactive terminal, it writes `FILE.pdf` (or
`FILE.html`) instead of emitting binary bytes. `-o FILE` always selects a file;
`-o -` always selects standard output.

Successful file renders print the path, page count, plan hash, and output
SHA-256. Use `--json` with `-o FILE` for machine-readable render metadata.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Command completed successfully |
| `1` | Parsing, validation, rendering, or I/O failed |
| `2` | Command name, flags, or positional arguments were invalid |

In JSON mode, diagnostics use standard output; process failures use standard
error.
