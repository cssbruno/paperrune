---
layout: home

hero:
  name: PaperRune
  text: Documents with a language, not a layout API.
  tagline: Write typed, indentation-sensitive Paper. Compile deterministic PDF and standalone HTML from the same immutable plan.
  actions:
    - theme: brand
      text: Open the playground
      link: /playground
    - theme: alt
      text: Read the language
      link: /reference/language
---

<div class="home-proof">
  <div><strong>One authoring surface</strong><span>Paper source drives PDF, HTML, Studio, and the browser playground.</span></div>
  <div><strong>Typed before layout</strong><span>Schemas, expressions, components, and structure fail before output is committed.</span></div>
  <div><strong>Deterministic by design</strong><span>No clocks, network access, ambient files, mutable layout state, or browser pagination.</span></div>
</div>

## Start with a readable document

```paper
document @hello:
  language: "en"
  title: "Hello"
  schema input:
    string name
  page @sheet:
    size: "A4"
    margin: 36pt
    body @content:
      heading @title:
        level: 1
        text: name
```

Use the [getting-started guide](/guide/getting-started) to build locally, or
open the [WASM playground](/playground) to compile this language directly in
your browser.
