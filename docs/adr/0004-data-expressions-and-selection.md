# ADR 0004: Typed data expressions and conditional selection

Status: accepted

## Context

Paper needs data-dependent scalar values, visibility, and component selection.
Treating conditions as quoted strings makes field names ambiguous, limits
conditions to hiding nodes, and prevents the compiler from checking result
types before rendering.

## Decision

Paper uses typed, unquoted expressions in expression-capable scalar
properties. Quoted text remains a string literal; bare names and dotted paths
refer only to declared data.

```paper
text: paid ? "Paid" : "Payment pending"
visible: account.active
size: compact ? 10pt : 12pt
```

The language includes booleans, strings, exact bounded decimal numbers, units,
closed component references, and `null`. It supports typed arithmetic,
comparison, matching, boolean operators, ternaries, and value or predicate
switches.

Expressions are deterministic and closed. They cannot access functions,
files, networks, clocks, randomness, processes, reflection, mutable state, or
layout measurements. There are no implicit conversions or ambient variables.

Boolean operators, ternaries, and switches are lazy. Every possible branch is
statically checked, but only the selected branch is evaluated. Optional data
is represented as nullable and may be narrowed with explicit null guards.

`visible` removes a node and its descendants when its expression is `false`.
A component expression may return a declared `@component` reference or `null`.
The compiler validates the final document structure after conditional removal
or selection.

## Determinism and limits

Numbers use a normalized signed `int64` coefficient with at most nine decimal
places. Arithmetic must be exact within that bound; overflow, precision loss,
and division by zero are errors. Units follow explicit compatibility rules and
never depend on measured layout.

Parsing and evaluation retain bounded source, token, depth, instruction,
stack, string, case, expansion, and work limits. The same source, schema, data,
assets, locale, and limits produce the same result and diagnostics.

## Consequences

- The receiving property determines the required expression result type.
- Unknown paths, nullable misuse, incompatible branches, and invalid units fail
  before output is produced.
- Missing optional values resolve to `null`; missing required values are
  errors.
- Dynamic component names and arbitrary sibling selection are not supported.
- Legacy quoted `when` conditions and migration behavior are outside the
  language.

User-facing syntax and examples live in the
[expression reference](../reference/expressions.md).
