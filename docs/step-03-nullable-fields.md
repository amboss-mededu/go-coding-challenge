# Step 03 — Nullable Fields

## Goal

Extend the generator to recognise **nullable primitive fields** — properties whose JSON Schema
`"type"` is an array of two strings, one of which is `"null"` (e.g. `["string", "null"]`).
These fields are emitted as **pointer types** (`*string`, `*float64`, etc.) so that `null` in
JSON round-trips correctly as `nil` in Go. All other behaviour from step 02 is unchanged.

## Inputs

Same as step 02: `schemas/*.json`. Two schemas introduce nullable primitives in this step.

### Fields in scope (nullable primitive)

#### Product.json

| Property      | JSON type            | Required | Go type   |
|---------------|----------------------|----------|-----------|
| `description` | `["string", "null"]` | no       | `*string` |

#### Category.json

| Property | JSON type            | Required | Go type   |
|----------|----------------------|----------|-----------|
| `parent` | `["string", "null"]` | no       | `*string` |

**Article.json** — no nullable primitive fields; output is unchanged from step 02.

## Nullable type detection

A schema node is nullable when its `"type"` is a JSON array of exactly two elements where one
is `"null"` and the other is a primitive type name. Order does not matter.

| JSON Schema type       | Go type    |
|------------------------|------------|
| `["string", "null"]`   | `*string`  |
| `["null", "string"]`   | `*string`  |
| `["integer", "null"]`  | `*int`     |
| `["number", "null"]`   | `*float64` |
| `["boolean", "null"]`  | `*bool`    |

Fields whose `"type"` is an array of more than two elements, or whose non-null element is not a
known primitive, remain skipped (as in step 02).

## Why two unmarshal attempts

`Schema.Type` is stored as `json.RawMessage` because the JSON Schema spec allows `"type"` to be
either a JSON string or a JSON array. Go cannot decode both shapes into the same typed field, so
`goType` tries each in sequence:

1. `json.Unmarshal(s.Type, &single string)` — succeeds for `"string"`, `"integer"`, etc.
2. `json.Unmarshal(s.Type, &multi []string)` — succeeds for `["string", "null"]`, etc.
   If the array has exactly two elements and one is `"null"`, the other names the primitive type
   and we prepend `"*"` to the mapped Go type.

## Required vs nullable

A nullable field may or may not be required. The existing `Generate()` tag logic already handles
both cases — no changes needed there:

- **Not required** (all nullable fields in current schemas):

  ```go
  Description *string `json:"description,omitempty"`
  Parent      *string `json:"parent,omitempty"`
  ```

- **Required nullable** (hypothetical — not in current schemas):

  ```go
  SomeField *string `json:"someField"`
  ```

## Implementation

### `setGoType` / `setPrimitiveType` in `internal/generator/generator.go`

The type resolution is split into two functions. `setGoType` guards on a nil `Type`, then delegates to `setPrimitiveType` which handles both JSON shapes:

```go
func setGoType(s *Schema) string {
    if s.Type == nil {
        return ""
    }
    return setPrimitiveType(s)
}

func setPrimitiveType(s *Schema) string {
    // Case 1: "type": "string"  →  single JSON string
    var single string
    if err := json.Unmarshal(s.Type, &single); err == nil {
        return primitiveTypes[single]
    }
    // Case 2: "type": ["string", "null"]  →  nullable primitive (order-independent)
    var multi []string
    if err := json.Unmarshal(s.Type, &multi); err == nil && len(multi) == 2 {
        for _, t := range multi {
            if t != "null" {
                if gt := primitiveTypes[t]; gt != "" {
                    return "*" + gt
                }
            }
        }
    }
    return ""
}
```

### `Generate()` refactor

`Generate()` was also refactored to remove the `wrote` flag and `buf.Reset()` dance. Fields are now accumulated into a separate `fields` buffer first; the struct header is only written once the outcome is known:

```go
var fields bytes.Buffer
for _, property := range propertyNames {
    goType := setGoType(s.Properties[property])
    if goType == "" {
        continue
    }
    tag := property
    if !requiredFields[property] {
        tag += ",omitempty"
    }
    fmt.Fprintf(&fields, "\t%s %s `json:\"%s\"`\n", toPascalCase(property), goType, tag)
}

if fields.Len() > 0 {
    fmt.Fprintf(&buf, "package model\n\ntype %s struct {\n%s}\n", s.Title, fields.String())
    // format.Source + write to schemasMap ...
}
```

Note: schemas with no mappable fields produce no output file (rather than an empty struct). All three current schemas have at least one primitive field so this does not affect the current output.

No changes required in `NewGenerator`.

## Output

### model/product.go

```go
package model

type Product struct {
    Active         bool    `json:"active,omitempty"`
    Classification string  `json:"classification,omitempty"`
    Description    *string `json:"description,omitempty"`
    Id             string  `json:"id"`
    Name           string  `json:"name"`
    Price          float64 `json:"price"`
    Quantity       int     `json:"quantity,omitempty"`
}
```

### model/category.go

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
    Id     string `json:"id"`
    Status string `json:"status,omitempty"`
    Title  string `json:"title"`
}
```

Fields are sorted alphabetically for deterministic output across runs.

## CLI invocation

```sh
go run ./cmd
```

## Done when

- [ ] `go run ./cmd` exits 0 and prints three `wrote …` lines
- [ ] `model/product.go` contains `Description *string \`json:"description,omitempty"\``
- [ ] `model/category.go` contains `Parent *string \`json:"parent,omitempty"\``
- [ ] `go test ./...` passes all 8 harness subtests (including `4.json` with `"parent": null` and `5.json` with `"parent": "cat-1"`)
- [ ] `go vet ./...` is clean
- [ ] Repeated runs produce byte-identical output

## Error handling contract

Inherits all contracts from step 02. No new error conditions introduced.

## Out of scope

- Nullable object types (`["object", "null"]`) — step 06+
- Array fields — step 04
- Enum-constrained fields as custom types — step 05
- Inline nested objects — step 06
- `$ref`, `$defs`, `oneOf`, recursive types — steps 07–10
