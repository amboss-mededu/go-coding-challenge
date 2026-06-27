# Step 02 — Primitive Fields

## Goal

Extend the generator to emit struct fields for every property whose JSON Schema type is
a single primitive string: `"string"`, `"number"`, `"integer"`, or `"boolean"`. Required
fields get a plain value type; optional fields get `,omitempty` in their JSON tag.
No other schema features (nullable arrays, nested objects, enums) are touched in this step.

## Inputs

Same as step 01: `schemas/*.json` files. Two schemas exercise primitive fields in this step.

### Fields in scope (single primitive type)

**Product.json** — required: `id`, `name`, `price`

| Property         | JSON type   | Required | Go type   |
|------------------|-------------|----------|-----------|
| `id`             | `"string"`  | yes      | `string`  |
| `name`           | `"string"`  | yes      | `string`  |
| `price`          | `"number"`  | yes      | `float64` |
| `quantity`       | `"integer"` | no       | `int`     |
| `active`         | `"boolean"` | no       | `bool`    |
| `classification` | `"string"`  | no       | `string`  |

Skipped for now: `description` (nullable), `tags` (array), `dimensions` (object).

**Category.json** — no required fields

| Property | JSON type  | Required | Go type  |
|----------|------------|----------|----------|
| `id`     | `"string"` | no       | `string` |
| `name`   | `"string"` | no       | `string` |
| `kind`   | `"string"` | no       | `string` |

Skipped for now: `parent` (nullable).

**Article.json** — required: `id`, `title`

| Property | JSON type  | Required | Go type  |
|----------|------------|----------|----------|
| `id`     | `"string"` | yes      | `string` |
| `title`  | `"string"` | yes      | `string` |
| `status` | `"string"` | no       | `string` |

Skipped for now: `authors` (array), `metadata` (nullable object), `category` ($ref), `dimensions` (object), `content` (oneOf).

## Type mapping

| JSON Schema type | Go type   |
|------------------|-----------|
| `"string"`       | `string`  |
| `"number"`       | `float64` |
| `"integer"`      | `int`     |
| `"boolean"`      | `bool`    |

## Field name conversion

JSON property names → exported Go identifiers via `toPascalCase`: splits on `_` and `-`
delimiters and capitalises each word boundary; camelCase interior capitals are preserved.
This handles single words, snake_case, kebab-case, and camelCase uniformly.

| JSON name        | Go name          |
|------------------|------------------|
| `id`             | `Id`             |
| `name`           | `Name`           |
| `price`          | `Price`          |
| `quantity`       | `Quantity`       |
| `active`         | `Active`         |
| `classification` | `Classification` |
| `kind`           | `Kind`           |
| `wordCount`      | `WordCount`      |
| `reading-time`   | `ReadingTime`    |

## Required vs optional

- **Required field** — plain value type, JSON tag without `omitempty`:

  ```go
  Price float64 `json:"price"`
  ```

- **Optional field** — plain value type (pointer types come in step 03), JSON tag with `omitempty`:

  ```go
  Quantity int `json:"quantity,omitempty"`
  ```

## Output

### model/product.go

```go
package model

type Product struct {
    Active         bool    `json:"active,omitempty"`
    Classification string  `json:"classification,omitempty"`
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
    Id   string `json:"id,omitempty"`
    Kind string `json:"kind,omitempty"`
    Name string `json:"name,omitempty"`
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
- [ ] `model/product.go` and `model/category.go` contain the exact struct bodies above
- [ ] `go test ./...` passes all 8 harness subtests
- [ ] `go vet ./...` is clean
- [ ] Repeated runs produce byte-identical output

## Error handling contract

Inherits all contracts from step 01. No new error conditions introduced.

## Out of scope

- Nullable types (`["string", "null"]`) — step 03
- Array fields — step 04
- Enum-constrained fields rendered as custom types — step 05
- Inline nested objects — step 06
- `$ref`, `$defs`, `oneOf`, recursive types — steps 07–10
- Pointer types for optional fields — step 03
