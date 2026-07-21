# ADR 0003: Paper is the only public authoring format

Status: accepted

## Decision

PaperRune accepts content through human-readable `.paper` source only.
`document.Document` does not expose HTML-to-PDF, page creation, cursor
positioning, text or cell placement, drawing primitives, or typed-Go layout
models.

PDF and HTML are output formats produced from the same immutable `PaperPlan`.
PDF uses the private serializer. Standalone HTML embeds each exact planned page
as inline SVG, so the browser does not measure, wrap, position, or paginate
authored content.

Output, signing, compliance metadata, output intents, attachments, resource
registration, limits, and security policy remain delivery configuration. They
do not create additional authoring surfaces.

## Consequences

- One Paper source and one plan define both PDF and HTML output.
- Callers cannot mix planned pages with mutable direct-placement or HTML state.
- Examples, commands, and public documentation author exclusively with Paper.
- Serializer regression tests may exercise private engines through test-only
  helpers; those helpers are absent from normal builds.
- New authoring features enter through Paper and lower into the immutable plan.
