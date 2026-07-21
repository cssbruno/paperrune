# Paper project file

`paper.project.json` makes `paper check`, `paper render`, and `paper studio`
work without repeating source, data, asset, and output arguments. `paper init`
creates this file automatically.

```json
{
  "source": "invoice.paper",
  "data": "data.json",
  "output": "dist/invoice.pdf",
  "format": "pdf",
  "locale": "en",
  "assets": "project.assets.json"
}
```

All paths in the project file are resolved relative to the project file. The
CLI searches the current directory and its parents, so commands also work from
a project subdirectory. Unknown fields and trailing JSON are rejected.

## Precedence

Configuration is resolved in this order:

1. CLI flags and an explicit source file
2. `PAPER_SOURCE`, `PAPER_DATA`, `PAPER_OUTPUT`, `PAPER_FORMAT`,
   `PAPER_LOCALE`, and `PAPER_ASSETS`
3. `paper.project.json`
4. Command defaults

Use `-o -` to force rendered bytes to standard output. Without `-o`, redirected
output retains pipeline behavior; an interactive terminal instead derives a
safe output filename from the source.
