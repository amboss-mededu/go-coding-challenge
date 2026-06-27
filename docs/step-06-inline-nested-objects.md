# Step 06 — Inline Nested Objects

## Goal

Extend the generator to descend into **inline nested objects** — properties whose schema
is itself an object (`"type": "object"` or `"type": ["object","null"]` with a
`"properties"` map). Each inline object is emitted as a **named sub-struct**, and the
field references that struct by name. Nullable objects (`["object","null"]`) become
**pointer fields** (`*T`). Enums that live *inside* an inline object — deferred from
step 05 — are now resolved too (e.g. `Product.dimensions.unit`). All other behaviour from
step 05 is unchanged.

## Inputs

Same as step 05: `schemas/*.json`. Two schemas introduce inline objects; `Category` has
none.

### Fields in scope (inline object)

#### Product.json

| Property     | JSON type  | Nested object shape                                                   | Required | Go type            |
|--------------|------------|-----------------------------------------------------------------------|----------|--------------------|
| `dimensions` | `"object"` | `width: number`, `height: number`, `unit: string enum ["cm","inch"]` | no       | `ProductDimensions` |

#### Article.json

| Property     | JSON type            | Nested object shape                          | Required | Go type            |
|--------------|----------------------|----------------------------------------------|----------|--------------------|
| `dimensions` | `"object"`           | `width: integer`, `height: integer`          | no       | `ArticleDimensions` |
| `metadata`   | `["object","null"]`  | `wordCount: integer`, `readingTime: number`  | no       | `*ArticleMetadata`  |

None of the inline objects declare their own `required` array, so every sub-field is
optional and carries `,omitempty`.

`Article.category` (`$ref`) and `Article.content` (`oneOf`) are **not** inline objects —
they carry no `"type"` and are left for steps 07 and 09.

## Nested object type detection and naming

A property is an inline object when its `"type"` is the single string `"object"` or a
two-element array containing `"object"` and `"null"`. The generated type name is
`<ContainingTypeName><PascalCase(property)>`:

| Inline object        | Generated type      | Nullable? | Field type          |
|----------------------|---------------------|-----------|---------------------|
| `Product.dimensions` | `ProductDimensions` | no        | `ProductDimensions` |
| `Article.dimensions` | `ArticleDimensions` | no        | `ArticleDimensions` |
| `Article.metadata`   | `ArticleMetadata`   | yes       | `*ArticleMetadata`  |

The containing type name is used as the prefix (not just the schema title) so the scheme
composes through nesting. At the top level the containing type name *is* the schema title,
so existing names are unchanged.

### Nested enums (deferred from step 05)

Because sub-fields flow through the same resolver as top-level fields, an enum inside an
inline object is now named off its containing struct:

| Nested enum field          | Generated type          | Sample constant                       |
|----------------------------|-------------------------|---------------------------------------|
| `Product.dimensions.unit`  | `ProductDimensionsUnit` | `ProductDimensionsUnitCm = "cm"`      |

## Nullable objects → pointer

`["object","null"]` resolves to `*T`, matching the step-03 nullable-primitive rule
(`*string`, `*int`). `encoding/json` decodes a JSON `null` into a nil pointer and an object
into an allocated struct, so `"metadata": null` (8.json) and a populated `metadata`
(7.json) both round-trip with no custom logic.

## Why object handling lives in the field loop, not `setGoType`

`setGoType` returns a single type string and has no access to the parent type name or an
output buffer. An inline object — like an enum — must emit a **top-level `type` struct
declaration** named after its parent and append it alongside the main struct. So object
detection sits in the field loop next to the enum branch and delegates to `setObjectType`.
To avoid duplicating the field loop between the top-level struct and nested ones, the loop
itself is extracted into a shared `genFields` helper that both paths call.

## Implementation

All changes are in `internal/generator/generator.go`. No changes to the `Schema` struct,
`NewGenerator`, `setGoType`, `setArrayType`, `setPrimitiveType`, or `setEnumType`
(`setEnumType` already takes the prefix as its `schemaName` argument).

### `objectType` — detect inline objects

```go
// objectType reports whether s is an inline object and whether it is nullable
// (type ["object","null"]). Single "object" → (true,false); ["object","null"] →
// (true,true); anything else → (false,false). $ref and oneOf nodes carry no "type"
// and so are never treated as inline objects (handled in later steps).
func objectType(s *Schema) (isObject, nullable bool) {
	var single string
	if err := json.Unmarshal(s.Type, &single); err == nil {
		return single == "object", false
	}
	var multi []string
	if err := json.Unmarshal(s.Type, &multi); err == nil && len(multi) == 2 {
		var hasObject, hasNull bool
		for _, t := range multi {
			switch t {
			case "object":
				hasObject = true
			case "null":
				hasNull = true
			}
		}
		return hasObject && hasNull, hasObject && hasNull
	}
	return false, false
}
```

### `genFields` — shared, recursive field loop

Extracted from the current `Generate()` body. Builds the struct body for `typeName`,
emitting any nested enum/object declarations into `decls`. Enums and inline objects are
named `typeName + PascalCase(property)`.

```go
// genFields builds the struct body for a struct named typeName from s.Properties,
// emitting any nested enum/object type declarations into decls. Properties are sorted
// for deterministic output.
func genFields(decls *bytes.Buffer, typeName string, s *Schema) string {
	requiredFields := make(map[string]bool, len(s.Required))
	for _, r := range s.Required {
		requiredFields[r] = true
	}

	propertyNames := make([]string, 0, len(s.Properties))
	for propertyName := range s.Properties {
		propertyNames = append(propertyNames, propertyName)
	}
	sort.Strings(propertyNames)

	var fields bytes.Buffer
	for _, propertyName := range propertyNames {
		property := s.Properties[propertyName]

		var goType string
		switch {
		case len(property.Enum) > 0:
			goType = setEnumType(decls, typeName, propertyName, property.Enum)
		default:
			if isObj, nullable := objectType(property); isObj {
				goType = setObjectType(decls, typeName+toPascalCase(propertyName), property)
				if nullable && goType != "" {
					goType = "*" + goType
				}
			} else {
				goType = setGoType(property)
			}
		}
		if goType == "" {
			continue
		}

		tag := propertyName
		if !requiredFields[propertyName] {
			tag += ",omitempty"
		}
		fmt.Fprintf(&fields, "\t%s %s `json:\"%s\"`\n", toPascalCase(propertyName), goType, tag)
	}
	return fields.String()
}
```

### `setObjectType` — emit a named sub-struct

```go
// setObjectType emits a named struct declaration for an inline object into decls and
// returns the type name. Nested enums/objects are emitted (recursively) before the
// struct itself. An object with no resolvable fields emits nothing and returns "" so the
// caller skips the field.
func setObjectType(decls *bytes.Buffer, typeName string, s *Schema) string {
	body := genFields(decls, typeName, s)
	if body == "" {
		return ""
	}
	fmt.Fprintf(decls, "type %s struct {\n%s}\n\n", typeName, body)
	return typeName
}
```

### `Generate()` — call the shared helper for the top-level struct

The inline loop is replaced by a single `genFields` call; everything else (per-schema
loop, `format.Source`, filename) is unchanged.

```go
func (g *Generator) Generate() (map[string][]byte, error) {
	schemasMap := make(map[string][]byte, len(g.schemas))

	for _, mainSchema := range g.schemas {
		var buf bytes.Buffer
		var decls bytes.Buffer

		body := genFields(&decls, mainSchema.Title, mainSchema)
		if body == "" {
			continue
		}

		fmt.Fprintf(&buf, "package model\n\ntype %s struct {\n%s}\n", mainSchema.Title, body)
		if decls.Len() > 0 {
			fmt.Fprintf(&buf, "\n%s", decls.String())
		}

		src, err := format.Source(buf.Bytes())
		if err != nil {
			return nil, fmt.Errorf("formatting source for %q: %w", mainSchema.Title, err)
		}

		filename := strings.ToLower(mainSchema.Title) + ".go"
		schemasMap[filename] = src
	}

	return schemasMap, nil
}
```

`format.Source` re-aligns every struct's columns and const blocks, so manual spacing need
not be precise. Declarations are appended in the order their properties are visited
(alphabetical), which keeps output byte-identical across runs.

## Output

### model/product.go

```go
package model

type Product struct {
	Active         bool                  `json:"active,omitempty"`
	Classification ProductClassification `json:"classification,omitempty"`
	Description    *string               `json:"description,omitempty"`
	Dimensions     ProductDimensions     `json:"dimensions,omitempty"`
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

type ProductDimensionsUnit string

const (
	ProductDimensionsUnitCm   ProductDimensionsUnit = "cm"
	ProductDimensionsUnitInch ProductDimensionsUnit = "inch"
)

type ProductDimensions struct {
	Height float64               `json:"height,omitempty"`
	Unit   ProductDimensionsUnit `json:"unit,omitempty"`
	Width  float64               `json:"width,omitempty"`
}
```

Declaration order follows alphabetical property visiting: `classification` (2nd) emits
`ProductClassification`; `dimensions` (4th) emits `ProductDimensionsUnit` (from its `unit`
sub-field, emitted before the struct) then `ProductDimensions`.

### model/article.go

```go
package model

type Article struct {
	Authors    []string          `json:"authors,omitempty"`
	Dimensions ArticleDimensions `json:"dimensions,omitempty"`
	Id         string            `json:"id"`
	Metadata   *ArticleMetadata  `json:"metadata,omitempty"`
	Status     ArticleStatus     `json:"status,omitempty"`
	Title      string            `json:"title"`
}

type ArticleDimensions struct {
	Height int `json:"height,omitempty"`
	Width  int `json:"width,omitempty"`
}

type ArticleMetadata struct {
	ReadingTime float64 `json:"readingTime,omitempty"`
	WordCount   int     `json:"wordCount,omitempty"`
}

type ArticleStatus string

const (
	ArticleStatusDraft     ArticleStatus = "draft"
	ArticleStatusPublished ArticleStatus = "published"
	ArticleStatusArchived  ArticleStatus = "archived"
)
```

`category` (`$ref`) and `content` (`oneOf`) still resolve to `""` and are skipped. Decls
appear in property-visit order: `dimensions` → `ArticleDimensions`, `metadata` →
`ArticleMetadata`, `status` → `ArticleStatus`.

### model/category.go

Unchanged from step 05 — `Category` has no inline objects.

## CLI invocation

```sh
go run ./cmd
```

## Done when

- [ ] `go run ./cmd` exits 0 and prints three `wrote …` lines
- [ ] `model/product.go` declares `type ProductDimensions struct { … }` with `Width`,
      `Height`, `Unit` and `Dimensions ProductDimensions \`json:"dimensions,omitempty"\``
- [ ] `Product.dimensions.unit` emits `type ProductDimensionsUnit string` with its const
      block (`Cm`, `Inch`) — the step-05 deferral is resolved
- [ ] `model/article.go` declares `type ArticleDimensions struct { … }` (int fields) and
      `type ArticleMetadata struct { … }`, with `Metadata *ArticleMetadata` (pointer,
      because the schema type is `["object","null"]`)
- [ ] `go test ./...` passes all 8 harness subtests, including the nested-object payloads:
  - `3.json` — Product with `"dimensions": {"width":10.5,"height":3.25,"unit":"cm"}`
  - `7.json` — Article with populated `dimensions` and `metadata`
  - `8.json` — Article with `"metadata": null` (decodes to a nil `*ArticleMetadata`)
- [ ] `go vet ./...` is clean
- [ ] Repeated runs produce byte-identical output

## Error handling contract

Inherits all contracts from step 05. No new error conditions. An inline object with no
resolvable fields emits no declaration and the field is skipped (consistent with the
silent-skip treatment of unresolvable types). Non-string values inside a nested enum are
skipped exactly as at the top level.

## Out of scope

- `Article.category` (`$ref` → another schema) — step 07
- `Article` `$defs` (RichText, PlainText, RichTextNode) — step 08
- `Article.content` (`oneOf` discriminated union) — step 09
- Recursive types (`RichTextNode.children`) — step 10
- `const` discriminator fields (`format: "richtext"`) — step 09
