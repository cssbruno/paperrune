---
layout: home

hero:
  name: PaperRune
  text: Typed templates for PDF and HTML.
  tagline: Write Paper once. Compile deterministic outputs in Go or the browser.
  actions:
    - theme: brand
      text: Open the playground
      link: /playground
    - theme: alt
      text: Read the language
      link: /reference/language
---

## Example

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

[Get started](/guide/getting-started) or run it in the [playground](/playground).
