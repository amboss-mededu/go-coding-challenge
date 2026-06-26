# Step 00 — Full Project Plan

## Challenge Summary

Build a CLI code generator that reads JSON Schema files from `schemas/` and emits
faithful Go type definitions into `model/`. The generator must handle the full
spectrum of features present in the three provided schemas (Product, Category, Article).

## Approach

Incremental, spec-driven development. Each step has its own spec document in `docs/`,
adds one schema concept, and leaves `go test ./...` passing before moving on.
The CLI (`go run ./cmd`) is the single entry point throughout.

## Schema Complexity Overview

| Feature | Where it appears |
|---------|-----------------|
| Primitive fields (string, number, integer, boolean) | Product, Category, Article |
| Required vs optional fields | All three schemas |
| Nullable fields (`string\|null`, `object\|null`) | Product (description), Category (parent), Article (metadata, text) |
| Array fields | Product (tags), Article (authors, nodes) |
| Enum types | Product (classification, unit), Category (kind), Article (status, format) |
| Inline nested objects | Product (dimensions), Article (dimensions, metadata) |
| Cross-schema `$ref` | Article → Category |
| Local definitions (`$defs`) | Article (RichText, PlainText, RichTextNode) |
| `oneOf` discriminated union | Article (content: RichText or PlainText) |
| Recursive types | Article → RichTextNode (children: []RichTextNode) |
| `const` fields | Article → RichText (format: "richtext"), PlainText (format: "plaintext") |

## Incremental Steps

| Step | What it adds | Schemas exercised |
|------|-------------|-------------------|
| **01** — Empty structs | One empty struct per schema title, one file per schema | All three |
| **02** — Primitive fields | Required + optional primitive fields (string, number, integer, boolean) with JSON struct tags | Product, Category |
| **03** — Nullable fields | Optional/nullable fields encoded as pointer types (`*string`, `*int`) | Product (description), Category (parent) |
| **04** — Array fields | Array fields (`[]string`, `[]T`) | Product (tags), Article (authors) |
| **05** — Enums | Custom string type + `const` block per enum; fields reference the new type | Product (classification, unit), Category (kind), Article (status) |
| **06** — Inline nested objects | Named sub-struct for each inline object definition | Product (dimensions), Article (dimensions, metadata) |
| **07** — Cross-schema `$ref` | Resolve `$ref` to another schema file as a field of the referenced type | Article (category → Category) |
| **08** — Local `$defs` | Resolve intra-schema `$defs` references; emit named types for each definition | Article (RichText, PlainText, RichTextNode) |
| **09** — `oneOf` union | Interface type + `UnmarshalJSON` discriminator for `oneOf` fields | Article (content: RichText \| PlainText) |
| **10** — Recursive types | Self-referencing struct fields (circular slice) | Article (RichTextNode.children) |

## Key Design Decisions (to revisit per step)

- **Optional fields** — pointer types (`*T`) rather than wrapper structs; idiomatic Go and works cleanly with `encoding/json`.
- **Enums** — named `string` type with typed constants; preserves the underlying JSON value without custom marshal logic.
- **`oneOf` discriminated union** — a common interface implemented by each variant, with a holding struct that uses a custom `UnmarshalJSON` to inspect the discriminator field (`format` const) and decode into the correct concrete type.
- **`$ref` resolution** — at code-generation time, not runtime; a `$ref` becomes a direct Go type reference.
- **Recursive types** — Go handles this natively via slice fields; no special treatment needed beyond correct `$defs` resolution.
- **File-per-schema** — one `.go` file per top-level schema; `$defs` sub-types live in the same file as their parent schema.

## Definition of Done (full challenge)

- `go run ./cmd` regenerates `model/` from `schemas/` without errors.
- `go test ./...` passes all 8 harness subtests.
- Generated types faithfully represent every schema feature listed above.
- `go vet ./...` is clean.
- A project README covers how to run the generator and the decisions made.
