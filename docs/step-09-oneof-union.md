# Step 09 — `oneOf` Discriminated Union

## Goal

Extend the generator to resolve the **`oneOf` discriminated union** at `Article.content` — a `oneOf`
over `#/$defs/RichText` and `#/$defs/PlainText` — into idiomatic Go. Each union member is a local
`$ref` to a `$defs` type already emitted in step 08 (`ArticleRichText`, `ArticlePlainText`). For the
union property, step 09 emits two things into the parent schema's file:

1. a **union struct** `ArticleContent` with one nullable pointer field per variant
   (`RichText *ArticleRichText`, `PlainText *ArticlePlainText`);
2. a custom **`UnmarshalJSON`** on `*ArticleContent` that inspects the `format` discriminator and
   populates the matching pointer field.

`Article` gains a `Content *ArticleContent` field (pointer, so `omitempty` works and nil means
absent). The `format` field inside each variant stays a plain `string` — the `const` is read at
**generation time** to build the discriminator switch, not to retype the field. All other behaviour
from step 08 is unchanged. Recursive-type round-trip verification stays out of scope (step 10).

## Trade-off vs interface approach

A struct-of-pointers allows (in theory) both fields to be non-nil simultaneously, which violates
`oneOf` semantics. This is acceptable because `UnmarshalJSON` enforces mutual exclusivity at decode
time, and the harness only tests unmarshaling. The benefit is much simpler calling code: nil-check
the field you need rather than type-asserting through an interface.

## Inputs

Same as step 08: `schemas/*.json`. Only `Article` carries a `oneOf` (`content`) and the `const`
discriminators (`RichText.format`, `PlainText.format`). `Product` and `Category` have neither.

### Union members in scope (`Article.content.oneOf`)

```json
"content": {
  "oneOf": [
    { "$ref": "#/$defs/RichText" },
    { "$ref": "#/$defs/PlainText" }
  ]
}
```

| member `$ref`       | variant Go type    | struct field  | discriminator        |
|---------------------|--------------------|---------------|----------------------|
| `#/$defs/RichText`  | `ArticleRichText`  | `RichText`    | `format` = `richtext`  |
| `#/$defs/PlainText` | `ArticlePlainText` | `PlainText`   | `format` = `plaintext` |

Notes:

- `content` is **not** in the schema's top-level `required` (`["id","title"]`), so the field is tagged
  `,omitempty`. Using `*ArticleContent` ensures a missing `content` key leaves the field `nil`.
- `RichText.format` is `{"type":"string","const":"richtext"}` and `PlainText.format` is
  `{"type":"string","const":"plaintext"}`. The `"const"` drives the switch; the field itself remains
  `Format string` in each variant struct (unchanged from step 08).
- `RichTextNode` has no `format` and is not a `oneOf` member — untouched.

## `oneOf` detection and discriminator resolution

A property is a union when `len(property.OneOf) > 0`. For each member:

1. `resolveReferences(root.Title, member.Ref)` returns **both** the variant Go type and the bare def
   key in one call → `("ArticleRichText", "RichText")` / `("ArticlePlainText", "PlainText")`. The bare
   key is the raw segment after the last `/`, used to index `root.Defs` and (PascalCased) as the field
   name — no separate `strings.LastIndex` extraction in `setUnionType`.
2. The struct field name is `toPascalCase(defKey)` → `RichText` / `PlainText`.
3. The discriminator **key** and **value** come from that def: the (sorted-first) property whose
   `Const` is a non-empty JSON string → key `format`, value `richtext` / `plaintext`.

## Why the root `*Schema` is threaded

Step 08 threaded `root string` (the schema **title**) so local refs could be prefixed
(`RichTextNode → ArticleRichTextNode`). Step 09 needs the title **and** the `$defs` map at the
property loop: the discriminator `const` lives in the *target def* (`RichText.format.const`), which is
only reachable through `root.Defs`. The ref value alone does not carry it.

The minimal change is to thread the **root `*Schema`** (instead of `root string`) through the two
**recursive** functions only — `generateFields` and `setObjectType`. Everywhere a name prefix was
needed, callers now pass `root.Title`; the new union path reads `root.Defs`. The leaf helpers
(`setGoType`, `setArrayType`, `setPrimitiveType`, `setEnumType`) keep their `root string` signatures
unchanged, and `objectType` is untouched. `resolveReferences` keeps its `root string` parameter but
now returns `(goType, name string)` (see below). These are the signature changes relative to step 08.

## Implementation

All changes are in `internal/generator/generator.go`. No changes to the `Schema` struct (it already
declares `OneOf []*Schema` and `Const json.RawMessage`), `NewGenerator`, `objectType`,
`setPrimitiveType`, `setEnumType`, `setGoType`, or `setArrayType`. `resolveReferences` is expanded to
return the bare ref name alongside the Go type (see below).

### `generateFields` / `setObjectType` — take `root *Schema`, add the `oneOf` case

```go
func generateFields(references *bytes.Buffer, root *Schema, typeName string, s *Schema) string {
	// ... unchanged setup ...
	switch {
	case len(property.Enum) > 0:
		goType = setEnumType(references, typeName, name, property.Enum)
	case len(property.OneOf) > 0:
		goType = setUnionType(references, root, typeName+toPascalCase(name), property.OneOf)
		if goType != "" {
			goType = "*" + goType
		}
	case property.Ref != "":
		goType, _ = resolveReferences(root.Title, property.Ref)
	default:
		if isObj, nullable := objectType(property); isObj {
			goType = setObjectType(references, root, typeName+toPascalCase(name), property)
			if nullable && goType != "" {
				goType = "*" + goType
			}
		} else {
			goType = setGoType(root.Title, property)
		}
	}
	// ... unchanged tag/emit ...
}

func setObjectType(decls *bytes.Buffer, root *Schema, typeName string, s *Schema) string {
	body := generateFields(decls, root, typeName, s)
	if body == "" {
		return ""
	}
	fmt.Fprintf(decls, "type %s struct {\n%s}\n\n", typeName, body)
	return typeName
}
```

### `resolveReferences` — also return the bare ref name

`resolveReferences` already computed the ref's last path segment internally and discarded it, returning
only the root-prefixed Go type. It now returns that bare name too, so `setUnionType` can index
`root.Defs` and build the field name without re-extracting the segment. The bare name is the **raw**
def key (no `toPascalCase`), so it indexes `root.Defs` directly; PascalCasing is left to callers. The
two existing single-value callers (`generateFields`' `$ref` case, `setGoType`) discard the new return
with `_`.

```go
// resolveReferences maps a $ref to its Go type name and the bare ref name. A local ref
// ("#/$defs/RichTextNode") PascalCases the segment after the last "/" and prefixes the root schema
// title → ("ArticleRichTextNode", "RichTextNode"). A cross-schema ref ("Category.json") strips the
// path and ".json" suffix → ("Category", "Category"). Empty → ("", "").
func resolveReferences(root, ref string) (goType, name string) {
	if ref == "" {
		return "", ""
	}
	name = strings.TrimSuffix(filepath.Base(ref), ".json")
	if strings.HasPrefix(ref, "#") {
		return root + toPascalCase(name), name
	}
	return name, name
}
```

### `discriminator` — find the `const`-bearing property

```go
// discriminator returns the JSON key and string value of the first (sorted) property carrying a
// non-empty string const — the field that selects a oneOf variant. ("", "") if none.
func discriminator(s *Schema) (key, value string) {
	names := make([]string, 0, len(s.Properties))
	for n := range s.Properties {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		p := s.Properties[n]
		if len(p.Const) == 0 {
			continue
		}
		var v string
		if err := json.Unmarshal(p.Const, &v); err == nil {
			return n, v
		}
	}
	return "", ""
}
```

### `setUnionType` — emit union struct + `UnmarshalJSON`

Variants are kept in `oneOf` array order, so output is deterministic across runs.

```go
func setUnionType(decls *bytes.Buffer, root *Schema, typeName string, members []*Schema) string {
	type variant struct{ goType, fieldName, disc string }
	var variants []variant
	discKey := ""
	for _, m := range members {
		if m.Ref == "" {
			continue
		}
		goType, defKey := resolveReferences(root.Title, m.Ref)
		def := root.Defs[defKey]
		if def == nil {
			continue
		}
		k, v := discriminator(def)
		if k == "" {
			continue
		}
		discKey = k
		variants = append(variants, variant{
			goType:    goType,
			fieldName: toPascalCase(defKey),
			disc:      v,
		})
	}
	if len(variants) == 0 {
		return ""
	}

	fmt.Fprintf(decls, "type %s struct {\n", typeName)
	for _, v := range variants {
		fmt.Fprintf(decls, "\t%s *%s\n", v.fieldName, v.goType)
	}
	fmt.Fprintf(decls, "}\n\n")

	field := toPascalCase(discKey)
	fmt.Fprintf(decls, "func (c *%s) UnmarshalJSON(data []byte) error {\n", typeName)
	fmt.Fprintf(decls, "\tif string(data) == \"null\" {\n\t\treturn nil\n\t}\n")
	fmt.Fprintf(decls, "\tvar disc struct {\n\t\t%s string `json:%q`\n\t}\n", field, discKey)
	fmt.Fprintf(decls, "\tif err := json.Unmarshal(data, &disc); err != nil {\n\t\treturn err\n\t}\n")
	fmt.Fprintf(decls, "\tswitch disc.%s {\n", field)
	for _, v := range variants {
		fmt.Fprintf(decls, "\tcase %q:\n\t\tc.%s = &%s{}\n\t\treturn json.Unmarshal(data, c.%s)\n",
			v.disc, v.fieldName, v.goType, v.fieldName)
	}
	fmt.Fprintf(decls, "\tdefault:\n\t\treturn fmt.Errorf(\"unknown %s %%q\", disc.%s)\n\t}\n}\n\n", discKey, field)
	return typeName
}
```

### `Generate` — thread `root *Schema`, emit a conditional import block

The generated file needs `encoding/json` and `fmt` only when a union is emitted.
`format.Source` (gofmt) does **not** add imports, so they must be emitted explicitly; the
`hasOneOf` gate keeps `product.go` / `category.go` import-free.

```go
func hasOneOf(s *Schema) bool {
	for _, p := range s.Properties {
		if len(p.OneOf) > 0 {
			return true
		}
	}
	return false
}

// inside the per-schema loop:
body := generateFields(&references, mainSchema, mainSchema.Title, mainSchema)
if body == "" {
	continue
}

// ...sort and emit $defs...

header := "package model\n\n"
if hasOneOf(mainSchema) {
	header += "import (\n\t\"encoding/json\"\n\t\"fmt\"\n)\n\n"
}
fmt.Fprintf(&buf, "%stype %s struct {\n%s}\n", header, mainSchema.Title, body)
if references.Len() > 0 {
	fmt.Fprintf(&buf, "\n%s", references.String())
}
```

## Output

### model/article.go

```go
package model

import (
	"encoding/json"
	"fmt"
)

type Article struct {
	Authors    []string          `json:"authors,omitempty"`
	Category   Category          `json:"category,omitempty"`
	Content    *ArticleContent   `json:"content,omitempty"`
	Dimensions ArticleDimensions `json:"dimensions,omitempty"`
	Id         string            `json:"id"`
	Metadata   *ArticleMetadata  `json:"metadata,omitempty"`
	Status     ArticleStatus     `json:"status,omitempty"`
	Title      string            `json:"title"`
}

type ArticleContent struct {
	RichText  *ArticleRichText
	PlainText *ArticlePlainText
}

func (c *ArticleContent) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var disc struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(data, &disc); err != nil {
		return err
	}
	switch disc.Format {
	case "richtext":
		c.RichText = &ArticleRichText{}
		return json.Unmarshal(data, c.RichText)
	case "plaintext":
		c.PlainText = &ArticlePlainText{}
		return json.Unmarshal(data, c.PlainText)
	default:
		return fmt.Errorf("unknown format %q", disc.Format)
	}
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

type ArticlePlainText struct {
	Format string `json:"format,omitempty"`
	Text   string `json:"text,omitempty"`
}

type ArticleRichText struct {
	Format string                `json:"format,omitempty"`
	Html   string                `json:"html,omitempty"`
	Nodes  []ArticleRichTextNode `json:"nodes,omitempty"`
}

type ArticleRichTextNode struct {
	Children []ArticleRichTextNode `json:"children,omitempty"`
	Text     *string               `json:"text,omitempty"`
	Type     string                `json:"type,omitempty"`
}
```

The additions versus step 08 are: the `import` block, the `Content` field (now `*ArticleContent`),
and the union block (`ArticleContent` struct + `UnmarshalJSON`). The `$defs` declarations are unchanged.

### model/category.go

Unchanged from step 05 — no `oneOf`, no import block.

### model/product.go

Unchanged — `Product` has no `oneOf`.

## CLI invocation

```sh
go run ./cmd
```

## Done when

- [ ] `go run ./cmd` exits 0 and prints three `wrote …` lines
- [ ] `model/article.go` has `import ("encoding/json"; "fmt")`, a `Content *ArticleContent` field,
      and the union block (`ArticleContent` struct with pointer fields + `UnmarshalJSON`)
- [ ] `7.json` (RichText `content`) unmarshals with `Content.RichText` non-nil, `Content.PlainText` nil
- [ ] `8.json` (PlainText `content`) unmarshals with `Content.PlainText` non-nil, `Content.RichText` nil
- [ ] `model/category.go`, `model/product.go`, and the step-08 `$defs` types are unchanged
- [ ] `go test ./...` passes all 8 harness subtests
- [ ] `go vet ./...` is clean
- [ ] Repeated runs produce byte-identical output

## Error handling contract

Inherits all contracts from step 08. `ArticleContent.UnmarshalJSON` returns the underlying error on
malformed JSON, no-ops on a JSON `null` (the zero `*ArticleContent` is left nil), and returns
`fmt.Errorf("unknown format %q", …)` when the discriminator matches no variant. A `oneOf` member
whose def or discriminator cannot be resolved is skipped; if no variant resolves, the union emits
nothing and the field is dropped.

## Out of scope

- `MarshalJSON` on `ArticleContent` (without it, re-marshaling wraps as `{"RichText":…}` or
  `{"PlainText":…}` with the nil field omitted only if the struct field had `omitempty` — a natural
  "with more time" enhancement).
- Recursive-type round-trip verification for `RichTextNode.children` — step 10.
