# Step 04 — Array Fields

## Goal

Extend the generator to recognise **array-typed fields** — properties whose JSON Schema
`"type"` is `"array"` and whose `"items"` resolves to a known primitive (or nullable
primitive). These fields are emitted as **slice types** (`[]string`, `[]float64`, etc.).
All other behaviour from step 03 is unchanged.

## Inputs

Same as step 03: `schemas/*.json`. Two schemas introduce arrays in this step.

### Fields in scope (array of primitive)

#### Product.json

| Property | JSON type | Items type | Required | Go type    |
|----------|-----------|------------|----------|------------|
| `tags`   | `"array"` | `"string"` | no       | `[]string` |

#### Article.json

| Property  | JSON type | Items type | Required | Go type    |
|-----------|-----------|------------|----------|------------|
| `authors` | `"array"` | `"string"` | no       | `[]string` |

**Category.json** — no array fields; output is unchanged from step 03.

## Array type detection

A schema node is an array when its `"type"` is the single JSON string `"array"` and
`"items"` is present. The element type is resolved by calling `setGoType` recursively
on `s.Items`, so nullable item types are handled for free without extra logic.

| items schema                 | Go type     |
|------------------------------|-------------|
| `{"type":"string"}`          | `[]string`  |
| `{"type":"integer"}`         | `[]int`     |
| `{"type":"number"}`          | `[]float64` |
| `{"type":"boolean"}`         | `[]bool`    |
| `{"type":["string","null"]}` | `[]*string` |
| missing / object / `$ref`    | skipped     |

Fields whose items schema cannot be resolved remain silently skipped (same contract
as unknown primitive types in step 02).

## Why dispatch in `setGoType` rather than trying each resolver in sequence

`setGoType` unmarshals `s.Type` once to peek at the shape, then dispatches to the
right resolver — no wasted work. Calling `setPrimitiveType` unconditionally for an
array-typed field would force it to unmarshal `"array"`, fail the `primitiveTypes`
lookup, and return `""` before we could try `setArrayType`. Explicit dispatch avoids
that wasted unmarshal and makes the branching intent clear at a glance.

## Why `setArrayType` does not re-check for `"array"`

`setGoType` already gates the call: `setArrayType` is only invoked after confirming
`s.Type` is the single string `"array"`. `setArrayType` therefore focuses purely on
resolving the items type, keeping it simple.

## Why recursive call into `setGoType` for items

Calling `setGoType(s.Items)` rather than `setPrimitiveType(s.Items)` means that if a
future items schema is itself nullable (`["string","null"]`), it produces `[]*string`
automatically — zero extra code. It also means future steps (e.g. arrays of arrays)
chain naturally.

## Implementation

### Updated `setGoType` in `internal/generator/generator.go`

```go
func setGoType(s *Schema) string {
    if s.Type == nil {
        return ""
    }
    var single string
    if err := json.Unmarshal(s.Type, &single); err == nil && single == "array" {
        return setArrayType(s)
    }
    return setPrimitiveType(s)
}
```

### `setArrayType` in `internal/generator/generator.go`

Add after `setPrimitiveType` (currently the last function in the file):

```go
// setArrayType returns "[]T" for array schemas whose items resolve to a known type.
// Returns "" for unresolvable items (object, $ref, missing, etc.).
func setArrayType(s *Schema) string {
    if s.Items == nil {
        return ""
    }
    elem := setGoType(s.Items)
    if elem == "" {
        return ""
    }
    return "[]" + elem
}
```

No changes to `Generate()`, `NewGenerator()`, the field-building loop, tag logic,
alphabetical sorting, or `format.Source` — all remain exactly as refactored in step 03.

## Output

### model/product.go

```go
package model

type Product struct {
	Active         bool     `json:"active,omitempty"`
	Classification string   `json:"classification,omitempty"`
	Description    *string  `json:"description,omitempty"`
	Id             string   `json:"id"`
	Name           string   `json:"name"`
	Price          float64  `json:"price"`
	Quantity       int      `json:"quantity,omitempty"`
	Tags           []string `json:"tags,omitempty"`
}
```

### model/category.go — unchanged from step 03

```go
package model

type Category struct {
	Id     string  `json:"id,omitempty"`
	Kind   string  `json:"kind,omitempty"`
	Name   string  `json:"name,omitempty"`
	Parent *string `json:"parent,omitempty"`
}
```

### model/article.go

```go
package model

type Article struct {
	Authors []string `json:"authors,omitempty"`
	Id      string   `json:"id"`
	Status  string   `json:"status,omitempty"`
	Title   string   `json:"title"`
}
```

Fields are sorted alphabetically for deterministic output across runs.
`Authors` sorts before `Id`, which is why it appears first.

## CLI invocation

```sh
go run ./cmd
```

## Done when

- [ ] `go run ./cmd` exits 0 and prints three `wrote …` lines
- [ ] `model/product.go` contains `Tags []string \`json:"tags,omitempty"\``
- [ ] `model/article.go` contains `Authors []string \`json:"authors,omitempty"\``
- [ ] `go test ./...` passes all 8 harness subtests, including:
  - `1.json` — Product with `"tags": ["sale", "new"]` (non-empty array)
  - `2.json` — Product with `"tags": []` (empty array)
  - `3.json` — Product with `"tags": ["precision"]` (single-element array)
  - `7.json` — Article with `"authors": ["author-1", "author-2"]` (non-empty array)
  - `8.json` — Article with `"authors": []` (empty array)
- [ ] `go vet ./...` is clean
- [ ] Repeated runs produce byte-identical output

## Error handling contract

Inherits all contracts from step 03. No new error conditions introduced.
Arrays whose `"items"` schema cannot be resolved are silently skipped — the property
is omitted from the struct, consistent with the treatment of unknown types in step 02.

## Out of scope

- Arrays of objects or `$ref` items — step 07+
- Arrays of inline nested objects — step 06+
- Nullable arrays (`["array", "null"]`) — step 06+
- Enum fields — step 05
- Inline nested objects — step 06
- `$ref`, `$defs`, `oneOf`, recursive types — steps 07–10
