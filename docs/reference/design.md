# Styles and themes

## Styles

Styles reuse text and box properties with single inheritance:

```paper
document:
  style @base:
    font: "Helvetica"
    size: 10pt
    color: "#171A1F"

  style @emphasis:
    style: "@base"
    bold: true

  page:
    body:
      paragraph:
        style: "@emphasis"
        text: "Local properties override the style"
```

`style: "@parent"` provides inheritance. Cycles and unknown names are errors.
Style properties are limited to the [text and box families](./content).

## Themes and tokens

```paper
document:
  theme: "@brand"

  theme @foundation:
    token @ink:
      type: "color"
      value: "#171A1F"
    token @body-size:
      type: "length"
      value: 10pt

  theme @brand:
    parent: "@foundation"
    token @accent:
      type: "color"
      value: "#2459D3"
    scope @print:
      token @accent:
        type: "color"
        value: "#173F5F"

  page:
    body:
      paragraph:
        size-token: "body-size"
        color-token: "accent"
        text: "Theme tokens"
```

Token types are `color`, `string`, `length`, `number`, and `bool`. A token has
`value` or a quoted `reference`. Themes may have one `parent`; scopes may nest.
Text consumes tokens through `font-token`, `size-token`, `line-height-token`,
and `color-token`.

| Declaration | Properties | Children |
| --- | --- | --- |
| `theme @name` | optional quoted `parent` | token, scope |
| `scope @name` | none | token, nested scope |
| `token @name` | quoted `type`; exactly one `value` or `reference` | none |

References inherit the referenced token type. Unknown tokens, type mismatches,
duplicate names, inheritance cycles, and reference cycles are errors.
