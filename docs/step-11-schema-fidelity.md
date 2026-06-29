# Step 11 — Schema Fidelity: Documenting Runtime Gaps

## Context

The README notes:

> "It's possible to make all tests pass even though the generated Go code is still missing a lot of necessary information."

Steps 01–10 produce structurally correct Go types and all 8 harness subtests pass. However, the smoke tests only verify that `json.Unmarshal` does not error on **valid** payloads. They give no signal when **invalid** JSON is silently accepted.

This step adds no generator changes. Instead it introduces `fidelity_test.go` — a test file that documents the runtime gaps between "tests pass" and "schema is fully enforced."

---

## Fidelity Gaps

| Gap | Schema requires | Go behaviour today |
|-----|----------------|--------------------|
| Missing required field | Error — `id`/`name`/`price` required in `Product` | Silent zero-value (`Id = ""`) |
| Missing required field | Error — `id`/`title` required in `Article` | Silent zero-value (`Title = ""`) |
| Invalid enum value | Error — `CategoryKind` has exactly 4 valid values | ~~Any string accepted silently~~ — **closed in [step 12](step-12-enum-validation.md)** |
| `null` vs absent | Distinct in JSON Schema | Both resolve to same Go zero/nil after unmarshal |

**Trade-off accepted (at step 11):** Required field enforcement and enum validation are not emitted by the generator. The Go type system represents the shape; runtime constraint checking would require custom `UnmarshalJSON` on every affected type.

> **Update:** The invalid-enum gap is **closed** in [step 12](step-12-enum-validation.md), which makes the generator emit a generic `UnmarshalJSON` on every enum type. The required-field and `null` vs absent gaps remain open.

---

## Test File: `fidelity_test.go`

Lives in the root package alongside `harness_test.go`.

### Part 1 — Generator pre-check

`TestGeneratorRuns` shells out to `go run ./cmd` and asserts exit code 0. If the generator itself is broken, the rest of the suite is meaningless.

### Part 2 — Fidelity gap tests

`TestSchemaFidelityGaps` exercises invalid JSON against the generated types and asserts the **current** (gap) behaviour. Each subtest carries a `// schema says:` comment describing what a fully-enforced implementation would do.

Tests are written so they **pass today** and **break automatically** if the generator is later enhanced to enforce the constraint — the `t.Fatalf` inside each subtest fires when the gap is closed (e.g. an error is returned where none was expected before).

---

## Done When

- `go test ./...` passes including `TestGeneratorRuns` and `TestSchemaFidelityGaps`
- `go vet ./...` is clean
- Each gap subtest name appears in the test output
- No generator or model files are modified
