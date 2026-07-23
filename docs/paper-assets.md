# Assets and fonts

Reference catalog entries by name:

```paper
image @hero:
  source: "asset:hero-image"
  width: 240pt
  height: 135pt
  fit: "cover"
  focus-x: 0.5
  focus-y: 0.35
  alt: "Quarterly revenue by region"
```

Go hosts provide a content-addressed catalog:

```go
sum := sha256.Sum256(pngBytes)
assets, err := document.NewPaperAssetCatalog([]document.PaperAssetResource{{
    Name:      "hero-image",
    MediaType: "image/png",
    Digest:    hex.EncodeToString(sum[:]),
    Data:      pngBytes,
}})
if err != nil {
    return err
}

plan, result, err := document.PlanPaperWithAssets("report.paper", source, assets)
```

`PlanPaperScenarioWithAssets`, `WritePaperWithAssets`, and
`WritePaperScenarioWithAssets` use the same catalog.

Catalog rules:

- names are portable lowercase identifiers;
- only decodable, bounded PNG/JPEG and signed, bounded TTF/OTF resources are
  admitted to the immutable planner catalog;
- encoded bytes and decoded pixel counts have independent hard limits;
- every digest is mandatory and must match the supplied bytes;
- catalogs are immutable and deterministic;
- missing references fail compilation;
- the compiler never searches paths, follows URLs, or reads ambient state;
- verified digests contribute to plan identity.

Inline image data URIs are supported; `asset:name` is preferred for documents.

## CLI and Paper Studio manifest

Pass the manifest explicitly to CLI commands or Studio:

```json
{"assets":[
  {"name":"hero-image","media_type":"image/png","sha256":"<64 lowercase hex characters>","path":"images/hero.png","focus_x":0.5,"focus_y":0.4},
  {"name":"body-regular","media_type":"font/ttf","sha256":"<64 lowercase hex characters>","path":"fonts/body.ttf","family":"Readable Sans","weight":400,"style":"normal","license":"OFL-1.1","fallback":["body-fallback"]},
  {"name":"body-fallback","media_type":"font/ttf","sha256":"<64 lowercase hex characters>","path":"fonts/fallback.ttf","family":"Fallback Sans","license":"OFL-1.1"}
]}
```

```shell
go run ./cmd/paper-studio -assets project.assets.json -asset-root . report.paper

go run ./cmd/paper render -assets project.assets.json -asset-root . -o report.pdf report.paper
```

Without `-asset-root`, paths are relative to the manifest. Absolute paths,
traversal, symlinks, non-regular files, mismatches, unknown fields, and
over-budget catalogs are rejected. Manifests are never discovered implicitly.
Browser inventory returns metadata and source usage, never bytes or paths.

Font media types are `font/ttf`, `font/otf`, and `font/woff2`. Required metadata
includes family and one of these licenses: `OFL-1.1`, `Apache-2.0`,
`Bitstream-Vera`, `MIT`, `CC0-1.0`, or `Proprietary`. Fallback and `replaces`
graphs must be acyclic. TTF/OTF fonts are subsetted for PDF; WOFF2 is currently
metadata-only. Font bytes never reach the browser.

Studio's Resources panel can add and remove catalog entries. It derives
digests, validates the complete catalog, updates the manifest atomically, and
rejects stale revisions or referenced removals.
