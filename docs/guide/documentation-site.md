# Documentation site

The documentation is a [VitePress](https://vitepress.dev/) site rooted at
`docs/`. It uses ordinary Markdown for reference content and one Vue component
for the browser compiler. Node dependencies are exact-pinned in
`package-lock.json`.

## Local commands

```sh
npm ci
npm run docs:dev
npm run docs:check
```

`docs:dev` starts the authoring server. `docs:check` performs the release gate:

1. compile `cmd/paper-studio-wasm` for `js/wasm`;
2. copy Go's matching `wasm_exec.js` runtime;
3. build every VitePress page and validate internal links;
4. instantiate the built WASM under Node;
5. prove that valid Paper produces exact SVG, invalid Paper preserves its
   source diagnostic, and every bundled playground sample compiles.

`make docs-site` and `make docs-site-check` expose the production build and
release gate through the repository Makefile.

## Generated files

`tools/build-docs-wasm.sh` writes these ignored files:

- `docs/public/paperrune.wasm`;
- `docs/public/wasm_exec.js`;
- `docs/.vitepress/dist/` after the VitePress build.

Do not commit those outputs. The Go source, Markdown, Vue component, VitePress
configuration, package manifest, and lockfile are the reproducible inputs.

## WASM bridge

The browser runtime exposes the existing `PaperStudioWASM` object. The docs
playground uses its asynchronous `compile` method:

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

`data` and `scenario` are mutually exclusive. The resolved result contains
`ok`, `pages`, `page`, `hash`, `diagnostics`, `error`, `svg`, `page_width`,
`page_height`, and `fixed_scale`. Expected authoring failures resolve with
`ok: false` and structured diagnostics; invalid bridge requests and unexpected
runtime failures reject the promise.

The docs build does not expose filesystem or network resolution to compiled
Paper. Inline data and declared scenarios work. Imports and asset catalog
references belong in the CLI or Studio workflow.

## GitHub Pages

`.github/workflows/pages.yml` builds, tests, uploads, and deploys the site on
each push to `main`. Actions are pinned to immutable commits. The VitePress
base path is derived from `GITHUB_REPOSITORY`, so the repository deployment is
served under `/paperrune/` without breaking navigation, runtime, or WASM URLs.

To reproduce that path locally:

```sh
PAPERRUNE_DOCS_BASE=/paperrune/ npm run docs:build
PAPERRUNE_DOCS_BASE=/paperrune/ npm run docs:preview
```

The ordinary CI workflow runs `npm run docs:check` for pull requests. The Pages
workflow repeats that gate before deployment, so a broken link, failed static
render, incompatible Go runtime, or non-working compiler cannot publish.
