# Step 05 — Enums

## Goal

Extend the generator to recognise **enum-constrained fields** — properties that carry a
non-empty JSON Schema `"enum"` array of string values. Each such field is emitted as a
**named `string` type** with a **`const` block** listing the allowed values, and the
struct field references that named type instead of plain `string`. This preserves the
underlying JSON value (no custom marshal logic) while giving callers typed constants.
All other behaviour from step 04 is unchanged.

## Inputs

Same as step 04: `schemas/*.json`. All three schemas introduce one top-level string enum
in this step.

### Fields in scope (string enum)

#### Product.json

| Property         | JSON type  | Enum values                                                  | Required | Go type                 |
|------------------|------------|-------------------------------------------------------------|----------|-------------------------|
| `classification` | `"string"` | `basic_science`, `basic-science`, `applied`, `experimental` | no       | `ProductClassification` |

#### Category.json

| Property | JSON type  | Enum values                                          | Required | Go type        |
|----------|------------|------------------------------------------------------|----------|----------------|
| `kind`   | `"string"` | `basic-science`, `basic_science`, `clinical`, `other`| no       | `CategoryKind`  |

#### Article.json

| Property | JSON type  | Enum values                       | Required | Go type         |
|----------|------------|-----------------------------------|----------|-----------------|
| `status` | `"string"` | `draft`, `published`, `archived`  | no       | `ArticleStatus` |

`Product.dimensions.unit` (`["cm","inch"]`) lives inside an inline nested object, which
the generator does not yet descend into, so it is **out of scope** until step 06.

## Enum type detection

A schema node is an enum when its `"enum"` array is non-empty. The generated type name is
`<SchemaTitle><PascalCase(property)>` (e.g. `Product` + `classification` →
`ProductClassification`). Each enum value becomes a constant named
`<TypeName><PascalCase(value)>` whose value is the original JSON string.

| Enum field                | Generated type          | Sample constant                          |
|---------------------------|-------------------------|------------------------------------------|
| `Product.classification`  | `ProductClassification` | `ProductClassificationApplied = "applied"` |
| `Category.kind`           | `CategoryKind`          | `CategoryKindClinical = "clinical"`      |
| `Article.status`          | `ArticleStatus`         | `ArticleStatusDraft = "draft"`           |

Enum detection takes precedence over the primitive path: even though these fields also
declare `"type": "string"`, the presence of `"enum"` routes them to the enum resolver.

## Constant name collisions — numeric suffix

`toPascalCase` strips both `_` and `-`, so `basic_science` and `basic-science` both map to
the identifier `BasicScience`. Two enums hit this deliberately:

- `Product.classification` contains both `basic_science` and `basic-science`.
- `Category.kind` contains both `basic-science` and `basic_science`.

Every value must get its own constant, so the generator emits the **first** value with the
clean identifier and appends a **numeric suffix** (`2`, `3`, …) to each later value that
would reproduce an already-used name — e.g. `BasicScience` and `BasicScience2`. No value is
dropped. First-in-array order decides which value keeps the clean name, which keeps output
deterministic.

## Why enum handling lives in `Generate()`, not `setGoType`

`setGoType` returns a single type string for a field and has no access to the parent
schema title, the property name, or an output buffer. An enum needs all three: it must
emit a **top-level `type` declaration and `const` block** alongside the struct, and name
them after `Title + property`. So the enum branch sits in the `Generate()` field loop,
where that context exists, and delegates to `setEnumType`. Non-enum fields keep flowing
through `setGoType` exactly as before.

## Why a named `string` type rather than a custom Marshal/Unmarshal

A `type ProductClassification string` round-trips through `encoding/json` with zero extra
code — the underlying kind is `string`, so marshalling and unmarshalling behave like a
plain string while the type documents intent and the constants provide safe values. This
matches the step-00 design decision and avoids the complexity reserved for `oneOf`
discriminated unions (step 09).

## Implementation

### `setEnumType` in `internal/generator/generator.go`

Add after `setArrayType` (currently the last resolver in the file):

```go
// setEnumType emits a named string type and a const block for a string enum into decls,
// and returns the generated type name to use as the field's Go type. Non-string enum
// values are skipped; identifier collisions (after PascalCasing) are disambiguated with a
// numeric suffix so every value gets its own constant.
func setEnumType(decls *bytes.Buffer, title, property string, values []json.RawMessage) string {
	typeName := title + toPascalCase(property)
	var consts bytes.Buffer
	seen := make(map[string]bool)
	for _, raw := range values {
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			continue // non-string enum value — out of scope for step 05
		}
		name := typeName + toPascalCase(v)
		// Disambiguate identifier collisions (e.g. basic_science vs basic-science both
		// PascalCase to BasicScience) with a numeric suffix so every value gets a constant.
		base := name
		for i := 2; seen[name]; i++ {
			name = fmt.Sprintf("%s%d", base, i)
		}
		seen[name] = true
		fmt.Fprintf(&consts, "\t%s %s = %q\n", name, typeName, v)
	}
	fmt.Fprintf(decls, "type %s string\n\nconst (\n%s)\n", typeName, consts.String())
	return typeName
}
```

### `Generate()` field loop in `internal/generator/generator.go`

Introduce a `decls` buffer, branch on `Enum`, and append the declarations after the
struct body. Everything else (required-field map, alphabetical sorting, tag logic,
`format.Source`) is unchanged from step 04:

```go
var fields bytes.Buffer
var decls bytes.Buffer
for _, property := range propertyNames {
	prop := s.Properties[property]

	var goType string
	if len(prop.Enum) > 0 {
		goType = setEnumType(&decls, s.Title, property, prop.Enum)
	} else {
		goType = setGoType(prop)
	}
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
	if decls.Len() > 0 {
		fmt.Fprintf(&buf, "\n%s", decls.String())
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("formatting source for %q: %w", s.Title, err)
	}

	filename := strings.ToLower(s.Title) + ".go"
	schemasMap[filename] = src
}
```

`format.Source` re-aligns both the struct columns and the const block, so the manual
spacing above does not need to be precise. No changes to `setGoType`, `setArrayType`,
`setPrimitiveType`, `NewGenerator`, or the `Schema` struct (`Enum []json.RawMessage`
already exists).

## Output

### model/product.go

```go
package model

type Product struct {
	Active         bool                  `json:"active,omitempty"`
	Classification ProductClassification `json:"classification,omitempty"`
	Description    *string               `json:"description,omitempty"`
	Id             string                `json:"id"`
	Name           string                `json:"name"`
	Price          float64               `json:"price"`
	Quantity       int                   `json:"quantity,omitempty"`
	Tags           []string              `json:"tags,omitempty"`
}

type ProductClassification string

const (
	ProductClassificationBasicScience  ProductClassification = "basic_science"
	ProductClassificationBasicScience2 ProductClassification = "basic-science"
	ProductClassificationApplied       ProductClassification = "applied"
	ProductClassificationExperimental  ProductClassification = "experimental"
)
```

`basic-science` collides with `basic_science` (which appears first), so it is emitted as
`ProductClassificationBasicScience2`.

### model/category.go

```go
package model

type Category struct {
	Id     string       `json:"id,omitempty"`
	Kind   CategoryKind `json:"kind,omitempty"`
	Name   string       `json:"name,omitempty"`
	Parent *string      `json:"parent,omitempty"`
}

type CategoryKind string

const (
	CategoryKindBasicScience  CategoryKind = "basic-science"
	CategoryKindBasicScience2 CategoryKind = "basic_science"
	CategoryKindClinical      CategoryKind = "clinical"
	CategoryKindOther         CategoryKind = "other"
)
```

`basic_science` collides with `basic-science` (which appears first), so it is emitted as
`CategoryKindBasicScience2`.

### model/article.go

```go
package model

type Article struct {
	Authors []string      `json:"authors,omitempty"`
	Id      string        `json:"id"`
	Status  ArticleStatus `json:"status,omitempty"`
	Title   string        `json:"title"`
}

type ArticleStatus string

const (
	ArticleStatusDraft     ArticleStatus = "draft"
	ArticleStatusPublished ArticleStatus = "published"
	ArticleStatusArchived  ArticleStatus = "archived"
)
```

Fields are sorted alphabetically for deterministic output across runs. Enum constants are
emitted in schema array order.

## CLI invocation

```sh
go run ./cmd
```

## Done when

- [ ] `go run ./cmd` exits 0 and prints three `wrote …` lines
- [ ] `model/product.go` declares `type ProductClassification string` with its const block and `Classification ProductClassification \`json:"classification,omitempty"\``
- [ ] `model/category.go` declares `type CategoryKind string`; `model/article.go` declares `type ArticleStatus string`
- [ ] Every enum value emits a unique constant; collisions are disambiguated with a numeric suffix (e.g. `BasicScience`, `BasicScience2`) — no duplicate identifiers
- [ ] `go test ./...` passes all 8 harness subtests, including enum payloads:
  - `1.json` — Product with `"classification": "basic_science"`
  - `6.json` — Category with `"kind": "other"`
- [ ] `go vet ./...` is clean
- [ ] Repeated runs produce byte-identical output

## Error handling contract

Inherits all contracts from step 04. No new error conditions introduced. Enum values that
are not JSON strings are silently skipped from the const block (consistent with the
silent-skip treatment of unknown types). A field with an empty/absent `"enum"` is handled
by the existing primitive/array paths exactly as before.

## Out of scope

- `Product.dimensions.unit` and any enums nested inside inline objects — step 06
- Non-string enums (integer/number) and nullable enums (`["string","null"]` + `enum`)
- `const` discriminator fields (`format: "richtext"`) — step 09
- Inline nested objects — step 06
- `$ref`, `$defs`, `oneOf`, recursive types — steps 07–10
```
