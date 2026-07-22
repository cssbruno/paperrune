---
aside: false
outline: false
title: WASM playground
---

# WASM playground

Compile Paper source and strict JSON entirely in your browser. Source and data
stay on this page; the WebAssembly module has no filesystem, network, clock, or
process access.

<Playground />

The playground uses the same parser, compiler, planner, diagnostics, and SVG
display-list renderer as PaperRune. It intentionally does not load external
assets or imports; use the CLI or Studio for projects that need them.
