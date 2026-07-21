# Command-line reference

Run `paper help` for the current command list and `paper help COMMAND` for a
command's options and examples. The command definitions are the canonical help
source used by tests and documentation checks.

## Normal authoring flow

```sh
paper init invoice my-invoice
cd my-invoice
paper check
paper render
paper studio
```

`paper check` and `paper render` discover `paper.project.json` in the current
directory or a parent. An explicit source file remains available for scripts
and one-off documents.

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
| `1` | Parsing, validation, rendering, I/O, or workflow operation failed |
| `2` | Command name, flags, or positional arguments were invalid |

Machine-oriented commands keep diagnostics on standard output in JSON mode so
standard error remains available for process-level failures.
