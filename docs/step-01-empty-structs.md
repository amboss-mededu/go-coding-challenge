# Step 01 — Empty Named Structs

## Goal

Implement the first working slice of the code generator: read every JSON Schema file
in the `schemas/` directory, extract each schema's `title` field, and emit one Go
source file per schema into the output directory (`model/` by default). Each file
declares a single empty struct named after the schema title.

## Inputs

| Source | Description |
|--------|-------------|
| `schemas/*.json` | One JSON Schema file per domain type. Each file must have a top-level `"title"` string field whose value is a valid Go identifier. |

Current schemas and their titles:

| File | Title |
|------|-------|
| `schemas/Product.json`  | `Product`  |
| `schemas/Category.json` | `Category` |
| `schemas/Article.json`  | `Article`  |

## Output

One `.go` file per schema in the output directory (default `./model/`).

| Schema file | Generated file | Content |
|-------------|----------------|---------|
| `schemas/Article.json`  | `model/article.go`  | `package model` + `type Article struct{}` |
| `schemas/Category.json` | `model/category.go` | `package model` + `type Category struct{}` |
| `schemas/Product.json`  | `model/product.go`  | `package model` + `type Product struct{}` |

Filenames are `strings.ToLower(title) + ".go"`. Each file is `gofmt`-formatted.

## CLI invocation

```sh
go run ./cmd --schemas ./schemas --output ./model
```

Or using defaults:

```sh
go run ./cmd
```

## Done when

- [ ] `go run ./cmd` exits with status 0 and prints one `wrote ...` line per schema
- [ ] `model/article.go`, `model/category.go`, `model/product.go` exist with correct empty structs
- [ ] `go test ./...` passes (harness unmarshals payloads into the types)
- [ ] `go vet ./...` reports no issues
- [ ] Output is identical across repeated runs (deterministic)

## Error handling contract

| Condition | Behaviour |
|-----------|-----------|
| `--schemas` directory does not exist | Return error, exit non-zero |
| A `.json` file fails to parse | Return error naming the file, exit non-zero |
| A parsed schema has an empty `title` | Skip the file with a warning to stderr |
| Output directory does not exist | `os.WriteFile` returns error, exit non-zero |

## Out of scope

- Struct fields (added in later steps)
- Optionality / pointer vs value semantics
- Nested/inline types, enums, `oneOf`, `$ref` resolution
- Schema validation beyond "file parses as JSON with a title field"
- Identifier sanitization for titles containing hyphens or spaces
- Cleaning up stale `.go` files in the output directory
