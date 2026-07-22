# Components

Components declare typed arguments, default content, and named slots.

```paper
document:
  component @callout:
    prop @title:
      type: "string"
      required: true
    prop @important:
      type: "bool"
      default: false
    heading:
      color: args.important ? "#B42318" : "#2459D3"
      text: args.title
    slot @body:
      type: "blocks"

  page:
    body:
      use @notice:
        component: @callout
        arg @title: "Read this"
        arg @important: true
        fill @body:
          paragraph:
            text: "Component body"
```

## Props

Props accept `type`, `required`, and `default`. Types are `string`, `bool`,
`number`, `unit`, `length`, and `any`. Component expressions read arguments as
`args.name`.

## Slots

| Property | Values |
| --- | --- |
| `type` | `blocks`, `text`, `list`, `row-column` |
| `required` | bool |
| `cardinality` | `one`, `many` |
| `layout-affecting` | bool |
| `scenarios` | comma-separated scenario names |

Slot children are default content. A required slot cannot have defaults.
`layout-affecting: true` requires a non-empty `scenarios` list and an explicitly
selected scenario. Each matching fill names its scenario.

## Uses and selection

A use selects a direct `@name`, conditional or switch expression, or `null`:

```paper
use @payment-message:
  component: paid ? @paid-message : @pending-message
```

It accepts `arg`, `fill`, optional `bind`/`bind-required`, and `visible`.
Cycles, unknown arguments or fills, incompatible alternatives, and missing
required contracts are errors. See [Expressions](./expressions#component-selection)
for selection and null behavior.

| Use member | Contract |
| --- | --- |
| `component` | direct reference, component expression, or `null` |
| `arg @name: value` | one scalar argument matching a declared prop |
| `fill @slot` | content matching the slot type and cardinality |
| `bind` | optional quoted schema path used inside the instance |
| `bind-required` | reject absent/null bound data when true |
| `visible` | remove the complete instance when false |

For a layout-affecting slot, write `scenario: "@name"` inside each `fill`.
