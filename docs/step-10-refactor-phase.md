# Step 10 — Refactor Phase

This step is a **refactor phase**: no new generated output, no behaviour change. Each refactor
below tightens the generator's internals while keeping `model/*.go` **byte-identical** to step 09.
Refactors are listed in the order they were applied.

## Goal

Improve the structure of `internal/generator/generator.go` without changing what it emits. Every
refactor in this step must satisfy the same invariant:

- `go run ./cmd` regenerates `model/article.go`, `model/category.go`, `model/product.go` with **no
  diff** versus before the refactor.
- `go test ./...` and `go vet ./...` stay green.

---

## Refactor 1 — Replace `discriminator` with a `constValue` leaf helper

### Motivation

Reading a property's `const` lived entirely inside `discriminator`, the function that resolves a
`oneOf` variant's discriminator. `discriminator` did two unrelated jobs at once:

1. **scan** a whole def's properties (sorted) to find *which* one carries a `const`, and
2. **read** that single property's `const` JSON string value.

Job (2) — "does this one property have a string const?" — is a leaf operation in the same family as
`setGoType` / `setPrimitiveType` (single schema node in, value out). Job (1) — the property scan —
needs the property *name* (`format`) for the switch key, so it belongs at the def level, in the only
caller that needs it (`setUnionType`). Splitting them removes the standalone `discriminator` function
and puts the `const` read where the other leaf type helpers live.

### Scope

All changes are in `internal/generator/generator.go`. No changes to the `Schema` struct,
`NewGenerator`, `Generate`, `generateFields`, `setObjectType`, `resolveReferences`, `setGoType`,
`setArrayType`, `setPrimitiveType`, `setEnumType`, or `objectType`. `const` is now read in exactly one
place (`constValue`).

### `constValue` — new leaf helper (next to `setPrimitiveType`)

A single property in, the string `const` out. Returns `("", false)` when the node has no `const` or
its `const` is not a JSON string — same skip semantics the old code relied on.

```go
// constValue returns the schema node's const as a string and true, or ("", false)
// if the node has no const or its const is not a JSON string.
func constValue(s *Schema) (string, bool) {
	if len(s.Const) == 0 {
		return "", false
	}
	var v string
	if err := json.Unmarshal(s.Const, &v); err != nil {
		return "", false
	}
	return v, true
}
```

### `discriminator` — deleted

The whole function (the sorted property scan + `const` read) is removed; its scan moves into
`setUnionType`, its `const` read becomes `constValue`.

### `setUnionType` — inline the discriminator scan

The single `k, v := discriminator(def)` call is replaced by the sorted scan, now delegating the
per-property `const` read to `constValue`. Behaviour is preserved exactly: properties are sorted, the
first one with a string `const` wins, and a variant whose def exposes no discriminator is skipped.
The rest of `setUnionType` (struct-of-pointers + `UnmarshalJSON` emission) is untouched.

```go
		def := root.Defs[defKey]
		if def == nil {
			continue
		}
		names := make([]string, 0, len(def.Properties))
		for n := range def.Properties {
			names = append(names, n)
		}
		sort.Strings(names)
		k, v := "", ""
		for _, n := range names {
			if cv, ok := constValue(def.Properties[n]); ok {
				k, v = n, cv
				break
			}
		}
		if k == "" {
			continue
		}
		discKey = k
		variants = append(variants, variant{
			goType:    goType,
			fieldName: toPascalCase(defKey),
			disc:      v,
		})
```

`sort` is already imported and used elsewhere, so no import changes.

### Why not fold the `const` check into `setGoType`

`setGoType` maps **one property's** schema to a Go type string; it cannot report the property's
*name*, which the switch key needs. A `const` field also still renders as its primitive type
(`Format string`) — the type mapping needs no change. Making `setGoType` also return the `const`
would churn its signature for every caller (`generateFields`, the recursive `setArrayType`, itself)
to carry a signal only the union path uses. Keeping `constValue` as its own leaf is the smaller,
clearer change.

---

## CLI invocation

```sh
go run ./cmd
```

## Done when

- [ ] `go run ./cmd` exits 0 and prints three `wrote …` lines
- [ ] `git diff model/` is empty — `model/article.go`, `model/category.go`, `model/product.go` are
      byte-identical to step 09
- [ ] `discriminator` no longer exists; `constValue` is the only place `Const` is read
- [ ] `go test ./...` passes all 8 harness subtests
- [ ] `go vet ./...` is clean
- [ ] Repeated runs produce byte-identical output

## Out of scope

- Any change to generated output or the `oneOf` / discriminator semantics (step 09 covers those).
- `MarshalJSON` on `ArticleContent` and recursive-type round-trip verification — still future work.
