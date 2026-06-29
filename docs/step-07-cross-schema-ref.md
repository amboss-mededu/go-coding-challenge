# Step 07 — Cross-schema `$ref`

## Goal

Extend the generator to resolve **cross-schema `$ref`** nodes — properties whose schema is a
reference to *another schema file* (`{"$ref": "Category.json"}`). The reference is resolved at
generation time into a **direct Go type reference**: `Article.category` becomes a field of type
`Category`, the type already generated from `Category.json`. Resolution is purely string-based
(strip the path and `.json` suffix) and emits the reference unconditionally — no lookup, no
existence check. Local references (`#/$defs/...`) are left untouched (`""`) for later steps.
All other behaviour from step 06 is unchanged.

## Inputs

Same as step 06: `schemas/*.json`. Only `Article` carries a cross-schema `$ref` at the property
level; `Product` and `Category` have none.

### Fields in scope (cross-schema `$ref`)

#### Article.json

| Property   | Schema node                | Resolves to       | Required | Go type    |
|------------|----------------------------|-------------------|----------|------------|
| `category` | `{"$ref":"Category.json"}` | `Category` schema | no       | `Category` |

`category` is not in `Article.required`, so it carries `,omitempty`. The `$ref` node has no
`"type"` and no `"null"`, so it is **not nullable** and the field is a value type (no pointer) —
consistent with step 06's treatment of non-nullable inline objects (`Dimensions ArticleDimensions`).

`Article.content` (`oneOf`) and the `$ref`s inside `$defs` (`#/$defs/RichTextNode`) are **not**
cross-schema file references and are left for steps 08–10.

## `$ref` detection and resolution

A property is a cross-schema `$ref` when its `"$ref"` value is non-empty and does **not** start
with `#`. Resolution strips the path and the `.json` suffix from the ref value:

| `$ref` value         | `filepath.Base` | `TrimSuffix(".json")` | Go type    |
|----------------------|-----------------|-----------------------|------------|
| `"Category.json"`    | `Category.json` | `Category`            | `Category` |
| `"#/$defs/RichText"` | —               | — (local, returns "") | (skipped)  |

This relies on the project convention that a ref's file base name equals the referenced schema's
title — which holds for `Article.json` / `Category.json` / `Product.json`. The referenced type is
emitted from its own schema (`Category` lives in `model/category.go`), so the `$ref` only needs to
name it.

### No existence guard (deliberate)

`format.Source` only *formats* the generated source — it does not type-check it — so the reference
is emitted whether or not a `Category` type actually gets generated. In normal use the referenced
schema is present in `schemas/` and is generated, so the reference resolves. If it was never added,
`model/` won't compile (`undefined: Category`) and the user fixes it manually. This keeps the code
simple: no set of known titles, and no extra parameter threaded through `generateFields` /
`setObjectType`.

Declaration and file order are a non-issue regardless — all generated types live in the same
`model` package, and Go requires no forward declaration, so `article.go` referencing `Category`
(defined in `category.go`) compiles independent of generation order.

## Why ref handling lives in the field loop, not `setGoType`

A `$ref` node carries no `"type"`, so today both `objectType` and `setGoType` return `""` and the
field is silently dropped (this is why the current `model/article.go` has no `Category` field).
Like the enum and inline-object branches added in step 06, `$ref` resolution is a dedicated branch
in the field loop. Unlike those, it needs no output buffer and no parent type name — resolution is
a pure string transform — so it is a standalone helper called from the `switch`.

## Implementation

All changes are in `internal/generator/generator.go`. No changes to the `Schema` struct,
`NewGenerator`, `Generate`, `setGoType`, `setArrayType`, `setPrimitiveType`, `setEnumType`,
`objectType`, or the signatures of `generateFields` / `setObjectType`.

### `resolveRef` — resolve a cross-schema `$ref` to a type name

```go
// resolveRef maps a cross-schema $ref like "Category.json" to its Go type name by stripping
// the path and ".json" suffix. Local refs ("#/...") are out of scope (steps 08+) and return "".
// There is no existence check: if the referenced schema is never generated, the emitted
// reference won't compile and must be fixed manually.
func resolveRef(ref string) string {
	if ref == "" || strings.HasPrefix(ref, "#") {
		return ""
	}
	return strings.TrimSuffix(filepath.Base(ref), ".json")
}
```

### `generateFields` — add the `$ref` branch

A new `case` is added to the field-loop `switch`, between the enum case and the object/primitive
default. Everything else in the loop is unchanged.

```go
switch {
case len(property.Enum) > 0:
	goType = setEnumType(decls, typeName, name, property.Enum)
case property.Ref != "":
	goType = resolveRef(property.Ref)
default:
	if isObj, nullable := objectType(property); isObj {
		goType = setObjectType(decls, typeName+toPascalCase(name), property)
		if nullable && goType != "" {
			goType = "*" + goType
		}
	} else {
		goType = setGoType(property)
	}
}
```

A local ref resolves to `""` and the field is skipped (consistent with the silent-skip treatment
of unresolvable types). A cross-schema ref resolves to the referenced type name and the field is
emitted with an `,omitempty` tag (it is optional).

## Output

### model/article.go

```go
package model

type Article struct {
	Authors    []string          `json:"authors,omitempty"`
	Category   Category          `json:"category,omitempty"`
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

Alphabetical property visiting puts `category` (2nd) between `authors` and `dimensions`. The new
`Category` field is the only change from step 06; `content` (`oneOf`, no `$ref` at the property
level) still resolves to `""` and is skipped, and the `ArticleDimensions` / `ArticleMetadata` /
`ArticleStatus` declarations are unchanged.

### model/category.go

Unchanged from step 05 — `Category` is generated from its own schema; the `$ref` in `Article`
simply points at it.

### model/product.go

Unchanged — `Product` has no cross-schema references.

## CLI invocation

```sh
go run ./cmd
```

## Done when

- [ ] `go run ./cmd` exits 0 and prints three `wrote …` lines
- [ ] `model/article.go` declares `Category Category \`json:"category,omitempty"\`` (value type,
      because the `$ref` is not nullable)
- [ ] `model/category.go` and `model/product.go` are unchanged
- [ ] `go test ./...` passes all 8 harness subtests, including the cross-ref payloads:
  - `7.json` — Article with a populated `"category": {"id":"cat-1","name":"Anatomy",
    "kind":"basic-science","parent":null}` that round-trips into `Category`
  - `8.json` — Article with no `category` (the value-type field stays zero-valued)
- [ ] `go vet ./...` is clean
- [ ] Repeated runs produce byte-identical output

## Error handling contract

Inherits all contracts from step 06. Local refs (`#/...`) resolve to `""` and the field is skipped
(no new error conditions). Cross-schema refs are emitted as-is with no existence check: a ref to a
schema that is never generated yields a reference to an undefined type, so `model/` won't compile
until the missing schema is added or the field is removed by hand. This is an accepted trade-off
for simplicity, not a generator error.

## Out of scope

- `Article` `$defs` (RichText, PlainText, RichTextNode) and local `#/$defs/...` refs — step 08
- `Article.content` (`oneOf` discriminated union) — step 09
- Recursive types (`RichTextNode.children`) — step 10
- `const` discriminator fields (`format: "richtext"`) — step 09
