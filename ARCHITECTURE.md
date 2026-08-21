# Architecture

PaperRune has one high-level facade: `document.Document`. New applications
construct it through `document.NewDocument` or `document.MustNew` and author
content exclusively through Paper. The module root
intentionally contains no facade package.

## Package boundaries

- `document` owns the Paper facade, immutable plan exports, and private PDF construction engine.
- `internal/layout` owns the renderer-independent compiler-to-planner model and measurement.

PaperRune has no package that accepts an existing PDF. Parsing, inspection,
page import, CDR, signing, signature verification, and final-byte verification
live in the independent PDFRune module. Neither module imports the other; a
host application may compose them after PaperRune finishes generation.

## Document ownership

`Document` deliberately does not expose the private serializer's page, text,
cell, image-placement, or drawing methods. Private state has concrete owners:

- `pdfSerializationState` allocates and records PDF object numbers.
- `resourceOwnershipState` initializes the document resource registry.
- `resourceStore` owns fonts, images, and templates.
- `resourceObjectNumbers` owns resource-specific PDF object references.
- `attachmentResourceStore` owns attachment object references and compressed
  temporary data.

New code should add behavior to the owning private type. Do not add another
field directly to `Document` when an existing owner is responsible for its
lifetime.

## Rendering and concurrency

A `Document` is a mutable, single-owner build session and is not safe for
concurrent calls. Create one document per independently generated PDF.
Immutable `PaperPlan` values are reusable across PDF and HTML output.

PDF serialization is a terminal PaperRune operation. Runtime code does not
reopen the resulting bytes for inspection, signing, import, sanitization, or
verification.

## Layout invariants

Paper measurement owns track offsets, spans, column constraints, image fitting,
and pagination comparisons in pure shared geometry.

Internal layout fields are cross-stage behavioral contracts. A field must not
be added until measurement, rendering, pagination, and regression tests
implement it. They are not a supported caller authoring surface.

PaperRune uses one private planner behind Paper. New automatic layout behavior
belongs in that planner rather than a second public frontend. Paper resolves
syntax-specific rules and lowers content; the final painter
must consume positioned plan commands without measuring, wrapping, or
paginating. See
[ADR 0003](docs/adr/0003-paper-only-authoring.md).

Plan font resources are immutable identities, not live `Document` lookups.
Standard 14 resources pin canonical metrics. Embedded UTF-8 resources pin the
font-program digest, byte length, and exact planning metrics in the canonical
plan while keeping the verified bytes in a private content-addressed sidecar.
During planning, each mutable single-owner `Document` memoizes that exact
font-program-plus-definition identity; repeated paragraph measurement reuses
the immutable descriptor and owned source bytes instead of rebuilding them.
Compiler font-name resolution reads catalog metadata without copying the font
program, while public catalog resolution continues to return detached bytes.
Reusable `FontCache` values share only immutable parsed tables; each receiving
`Document` owns the mutable font-program bytes patched during subset assembly.
The painter parses and verifies every embedded program during bounded
preflight, before opening the target, then reuses the established TrueType
subset and ToUnicode serializer. This preserves core-font compatibility and
allows one plan to be replayed concurrently without ambient filesystem or font
catalog access.

Page-template source mappings are projected once per planning operation into
body, header, and footer views. Repeated page-master planning reuses those
immutable per-render views; it must not retain source mappings globally or key
a cache by caller-owned slice addresses. Mixed header/footer compositors retain
their established second-projection fallback identities so this allocation
optimization does not change source provenance or canonical plan hashes.

## Plan preview and Studio boundary

The immutable layout plan is the authority for both PDF painting and visual
inspection. `document.PaperPlan` exposes a bounded, self-verifying web-render
payload plus geometry, hit-test, query, and explain projections without
exposing mutable IR. Paper Studio renders visible pages and thumbnails with the
shared Go direct display-list rasterizer compiled to WebAssembly. JavaScript
only loads revision-bound payloads and presents the returned pixels on canvas;
the retained SVG path is diagnostic geometry. Browser CSS may arrange workspace
chrome but must never substitute for page measurement, wrapping, positioning,
pagination, or display-list painting.

The web-render payload contains canonical plan JSON, an exact plan hash,
renderer profile and limits, revision identities, and content-addressed resource
bindings to deduplicated font/image blobs. WASM rejects unknown fields, schema
or renderer mismatches, stale/tampered plan hashes, invalid blobs, and resource
budget violations before painting. It invokes the same rasterizer used by
headless review evidence and therefore is a deployment of the shared renderer,
not a second browser layout implementation.

Standalone HTML export follows the same boundary: each planned page is emitted
as exact inline SVG. HTML/CSS may arrange those page artifacts but never
re-measures or reflows Paper content.

Every Studio page, overlay, hit, and explanation request is bound to the exact
plan revision. A revision mismatch fails instead of mixing evidence, and the
canvas is visibly stale and non-interactive while a replacement is loading.
The page inspector is another bounded retained-plan projection: border/content
rectangles, fragment region membership, causal breaks, semantic roles, and
reading indexes are plan facts. Studio does not synthesize unavailable margin,
padding, baseline, or serialized-PDF verification evidence. Unsupported authored
fonts remain compile errors; Studio may offer an explicit supported-font
replacement but never substitutes one automatically. Overlap
selection follows the deterministic reverse fragment order returned by the
plan hit-test contract.
Accessibility inspection is based on retained-plan reading order and semantic
roles. Serialized-PDF tag verification belongs to PDFRune and is performed by
the host after generation when required.
The development Studio server accepts only explicit loopback hosts because it
serves local source and plan evidence without a remote authentication boundary.
Scenario snapshots and page artifacts are immutable, bounded, and discarded
when the source digest changes.

On a source transition Studio retains at most one detached previous
`PaperPlan`; it never retains that revision's source or AST. Per-page hashes
cover exact page geometry, display payloads, breaks, positioned diagnostics,
semantics, reading order, and provenance. The rail compares those hashes only
when the previous and current scenario identities match. Otherwise it labels
the baseline mismatch and emits no changed-page evidence. Master labels are
the retained first/even/odd selector state plus actual fragment regions and
repetition state; Studio does not invent an authored master identity absent
from the plan.

## Editing state ownership

PaperRune does not provide durable document version control. A source revision
or plan revision is an exact content digest used for optimistic concurrency,
cache identity, and stale-result rejection; it is not a stored user-visible
version.

Paper Studio owns only session-level editing state, including undo and redo.
That state exists to make local authoring usable and is not a durable revision
repository. Long-term history, branching, collaboration, and recovery belong
to the host application or an external system such as Git.

Structured and automated edits follow the same rule as human edits: they
consume exact source bytes plus an expected source digest and return updated
source, diagnostics, and a bounded diff. PaperRune does not maintain a
separate AI history or AI-specific version domain.

## Performance workflow

The Makefile is the source of truth for performance tooling. Use
`bench-paper-engine-ci` for repeated samples, `bench-paper-engine-budget` for
the calibrated gate, and `profile-paper-engine-check` for bounded profiles.
These workflows measure cold end-to-end Paper compilation and warm retained
Paper-plan rendering. Generated reports belong under `artifacts/`; host-specific
benchmark output is not committed.

## Public API policy

The `document` surface is compatibility-sensitive and intentionally small.
Prefer private helpers over aliases and wrapper combinations. Existing-PDF
operations are outside this module and must not be reintroduced as deprecated
wrappers.
