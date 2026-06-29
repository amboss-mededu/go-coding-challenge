# Schema-Driven Code Generator — Solution

## How to Run

**Prerequisites:** Go 1.23+

```sh
# Install dependencies
make deps

# Generate model files (writes model/product.go, model/category.go, model/article.go)
make run

# Build the binary, then generate the go structs via the compiled binary
make build
make generate

# Run all tests (TestMain re-runs the generator automatically before each test run)
make test
```

To override the default input/output directories, run the generator directly:

```sh
go run ./cmd --schemas ./schemas --output ./model
```

All tests pass. `go test ./...` covers 8 smoke-test subtests (valid payloads round-trip cleanly) and 5 fidelity subtests (invalid values are correctly rejected or accepted per documented behavior).

---

## Decisions and Why

### One file per schema, local definitions co-located

Each top-level schema produces one `.go` file. Types defined in `$defs` (e.g. `ArticleRichText`, `ArticleRichTextNode`) are emitted into the same file as their parent schema. This keeps related types together and avoids any cross-file dependency issues.

### Optional fields as pointer types

Required fields get a plain value type. Optional and nullable fields get a pointer (`*T`). This gives callers a clear nil/non-nil signal and is idiomatic Go — no wrapper structs or sentinel values needed.

### Enums as named string types with a const block

Each enum field generates a named `string` type (e.g. `CategoryKind`) and a `const` block of typed values. This is type-safe and serialises to the raw JSON string without any custom `MarshalJSON`. An `UnmarshalJSON` method is also emitted on every enum type so that invalid values are rejected at decode time rather than silently accepted.

### `oneOf` as a struct-of-pointers with a generated `UnmarshalJSON`

Each `oneOf` union produces a holding struct where each variant is a pointer field. A generated `UnmarshalJSON` peeks at the discriminator field (found by scanning the variant's `const` property at codegen time) and populates exactly one pointer. Callers check which pointer is non-nil to know which variant arrived.

### `$ref` resolution at codegen time

References are resolved when generating the source, not at runtime. A local `$defs` ref becomes the corresponding Go type name; a cross-schema ref becomes the target schema's type. No runtime lookup or indirection.

### Deterministic output

All property maps and `$defs` maps are sorted before iteration. Repeated runs of the generator produce byte-identical output, which keeps version-control diffs clean.

---

## Walkthrough — Approach and Trade-offs

The implementation grew in 12 documented steps (one spec file per step in `docs/`). Each step added one schema concept and left `go test ./...` passing before moving on:

| Steps | Concepts covered |
|-------|-----------------|
| 01–03 | Empty structs → primitive fields → nullable pointer fields |
| 04–05 | Array fields → string enums |
| 06–07 | Inline nested objects → cross-schema `$ref` |
| 08–09 | Local `$defs` resolution → `oneOf` discriminated union |
| 10 | Internal refactor (no output change) |
| 11–12 | Fidelity gap documentation → enum validation via `UnmarshalJSON` |

### Trade-offs accepted

**Required fields are not enforced at unmarshal time.** Go's `encoding/json` silently assigns zero values to absent fields. Generating correct required-field checks would need a custom `UnmarshalJSON` on every struct that has required fields. The complexity vs. benefit trade-off didn't justify it within the time budget. The gap is documented and tested in `fidelity_test.go`.

**`null` vs absent is not distinguished.** JSON Schema treats an explicit `null` and a missing key differently; Go's decoder collapses both to nil/zero. Closing this gap would add noise to every generated type. Left open as a documented gap.

**`MarshalJSON` for `oneOf` unions is not emitted.** The holding struct currently serialises as a JSON object with all variant fields present (most nil). A correct `MarshalJSON` would delegate to the active variant. This is a round-trip fidelity gap.

**Non-string enums are not covered.** The enum emitter assumes string values; none of the provided schemas use integer or number enums.

### What I'd add with more time

- **Required field enforcement** — generate `UnmarshalJSON` on structs with required fields; the `Schema.Required` slice is already parsed, so this is purely an emitter addition.
- **`MarshalJSON` for `oneOf`** — emit a method that checks which variant pointer is non-nil and delegates serialisation to it.
- **Generator unit tests** — the current test suite validates generated output but not the generator's internal functions. Table-driven tests for helpers like `setPrimitiveType`, `setEnumType`, and `resolveReferences` would make the emitter safer to change.
- **Non-string enum support** — extend the enum emitter to handle `integer` and `number` enum schemas.
