# Documentation site

[VitePress](https://vitepress.dev/) sources live in `docs/`; dependencies are
pinned in `package-lock.json`.

## Local commands

```sh
npm ci
npm run docs:dev
npm run docs:check
```

`docs:check`:

1. compile `cmd/paper-studio-wasm` for `js/wasm`;
2. copy Go's matching `wasm_exec.js` runtime;
3. build the site and validate links;
4. instantiate the built WASM under Node;
5. test valid, invalid, and bundled playground samples.

Make equivalents: `make docs-site` and `make docs-site-check`.

## Generated files

`tools/build-docs-wasm.sh` writes these ignored files:

- `docs/public/paperrune.wasm`;
- `docs/public/wasm_exec.js`;
- `docs/.vitepress/dist/` after the VitePress build.

Do not commit these files.

## WASM bridge

The playground calls `PaperStudioWASM.compile`:

```js
const result = await PaperStudioWASM.compile({
  source: paperSource,
  data: jsonSource,
  dataName: 'playground',
  page: 1
})
```

The request accepts:

| Field | Type | Meaning |
| --- | --- | --- |
| `source` | required string | Complete Paper source, at most 1 MiB |
| `data` | string | Strict JSON, at most 4 MiB |
| `scenario` | string | Declared scenario name, with or without `@` |
| `page` | positive uint32 | One-based SVG page to capture; defaults to 1 |
| `dataName` | string | Name attached to JSON data diagnostics |
| `schema` | string | Explicit schema selection when needed |
| `locale` | string | Explicit JSON formatting locale |

`data` and `scenario` are mutually exclusive. Results contain `ok`, `pages`,
`page`, `hash`, `diagnostics`, `error`, `svg`, `page_width`, `page_height`, and
`fixed_scale`. Authoring errors resolve with `ok: false`; invalid requests and
runtime failures reject.

WASM supports inline data and scenarios, but not files, network access,
imports, or asset catalogs.

## GitHub Pages

`.github/workflows/pages.yml` tests and deploys pushes to `main`. The base path
is derived from `GITHUB_REPOSITORY`.

To reproduce that path locally:

```sh
PAPERRUNE_DOCS_BASE=/paperrune/ npm run docs:build
PAPERRUNE_DOCS_BASE=/paperrune/ npm run docs:preview
```

CI and Pages both run `npm run docs:check`.
