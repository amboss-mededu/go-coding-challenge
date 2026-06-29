# Step 08 — Local `$defs`

## Goal

Extend the generator to resolve **intra-schema `$defs` references** — local refs of the form
`{"$ref": "#/$defs/RichTextNode"}` — and to **emit a named Go type for every `$defs` entry** in a
schema. Each definition becomes a top-level struct named `<SchemaTitle><DefName>` (e.g.
`ArticleRichText`), living in the same file as its parent schema. A local ref resolves at generation
time to that same name, so `RichText.nodes` (`array` of `#/$defs/RichTextNode`) becomes
`[]ArticleRichTextNode`. Because the resolved name is a plain Go type, the self-recursive
`RichTextNode.children` field resolves to `[]ArticleRichTextNode` with no extra work — Go supports
self-referential slice fields natively. Cross-schema refs (step 07) are unchanged. `oneOf`
(`Article.content`) and the `const` discriminator are still out of scope. All other behaviour from
step 07 is unchanged.

## Inputs

Same as step 07: `schemas/*.json`. Only `Article` carries a `$defs` block; `Product` and `Category`
have none.

### Definitions in scope (`Article.$defs`)

Each `$defs` entry is emitted as `Article<DefName>`:

| `$defs` key    | Emitted type          | Fields (Go)                                                  |
|----------------|-----------------------|-------------------------------------------------------------|
| `RichText`     | `ArticleRichText`     | `Format string`, `Html string`, `Nodes []ArticleRichTextNode` |
| `RichTextNode` | `ArticleRichTextNode` | `Children []ArticleRichTextNode`, `Text *string`, `Type string` |
| `PlainText`    | `ArticlePlainText`    | `Format string`, `Text string`                              |

Notes:

- `format` carries a `const` (`"richtext"` / `"plaintext"`) but also `"type":"string"`, so it is
  emitted as a plain `string` field. The `const` discriminator refinement is **step 09**.
- `text` in `RichTextNode` is `["string","null"]` → `*string` via the existing nullable handling.
- `nodes` and `children` are arrays whose items are local refs; both resolve to
  `[]ArticleRichTextNode`. `children` is self-recursive — handled natively by Go.

The emitted `$defs` types are **not yet referenced** by `Article` (the `content` `oneOf` field stays
skipped until step 09). Unreferenced package-level types are valid Go and `go vet`-clean; step 09
wires `Article.content` into a union over them.

## `$ref` detection and resolution

A local `$ref` is one whose value starts with `#`. Resolution takes the segment after the last `/`,
PascalCases it, and prefixes the **root schema title**:

| `$ref` value             | Branch        | Resolves to            |
|--------------------------|---------------|------------------------|
| `"Category.json"`        | cross-schema  | `Category`             |
| `"#/$defs/RichText"`     | local `$defs` | `ArticleRichText`      |
| `"#/$defs/RichTextNode"` | local `$defs` | `ArticleRichTextNode`  |
| `""`                     | none          | `""` (field skipped)   |

The local branch needs the parent schema's title, so the **root title is threaded** through the
resolution helpers (see below). This is the one signature change relative to step 07, which kept
`resolveRef` a context-free string transform.

## Why the root title is threaded

Step 07's `resolveRef(ref string)` was a pure string transform because a cross-schema name is fully
determined by the ref value (`Category.json → Category`). A local def name, under the parent-prefixed
convention, is `rootTitle + defName` — information the ref value alone does not carry. The same root
must be used regardless of which struct the ref appears in (inside `ArticleRichText`, a ref to
`#/$defs/RichTextNode` must still produce `ArticleRichTextNode`, not `ArticleRichTextRichTextNode`).
So a `root string` parameter is threaded down `generateFields → setObjectType → setGoType →
setArrayType → resolveRef`. `objectType`, `setPrimitiveType`, and `setEnumType` are untouched.

Local refs also appear as **array items** (`nodes`, `children`), which never pass through the
field-loop `$ref` case — they reach `setArrayType → setGoType(items)`. A `$ref` item has no `"type"`,
so step 07 dropped it. Step 08 makes `setGoType` ref-aware so array-item refs resolve too.

## Implementation

All changes are in `internal/generator/generator.go`. No changes to the `Schema` struct,
`NewGenerator`, `objectType`, `setPrimitiveType`, or `setEnumType`.

### `resolveRef` — handle local refs and take the root title

```go
// resolveRef maps a $ref to its Go type name. A cross-schema ref ("Category.json") strips the path
// and ".json" suffix → "Category". A local ref ("#/$defs/RichTextNode") takes the segment after the
// last "/", PascalCases it, and prefixes the root schema title → "ArticleRichTextNode". Empty → "".
// Cross-schema refs have no existence check: an unresolved reference simply won't compile.
func resolveRef(root, ref string) string {
	if ref == "" {
		return ""
	}
	if strings.HasPrefix(ref, "#") {
		name := ref[strings.LastIndex(ref, "/")+1:]
		return root + toPascalCase(name)
	}
	return strings.TrimSuffix(filepath.Base(ref), ".json")
}
```

### `setGoType` — resolve `$ref` array items, take the root title

```go
func setGoType(root string, s *Schema) string {
	if s.Ref != "" {
		return resolveRef(root, s.Ref)
	}
	if s.Type == nil {
		return ""
	}
	var single string
	if err := json.Unmarshal(s.Type, &single); err == nil && single == ARRAY_TYPE {
		return setArrayType(root, s)
	}
	return setPrimitiveType(s)
}
```

### `setArrayType` — thread the root title to item resolution

```go
func setArrayType(root string, s *Schema) string {
	if s.Items == nil {
		return ""
	}
	elem := setGoType(root, s.Items)
	if elem == "" {
		return ""
	}
	return "[]" + elem
}
```

### `generateFields` / `setObjectType` — thread the root title

```go
func generateFields(decls *bytes.Buffer, root, typeName string, s *Schema) string {
	// ... unchanged setup ...
	switch {
	case len(property.Enum) > 0:
		goType = setEnumType(decls, typeName, name, property.Enum)
	case property.Ref != "":
		goType = resolveRef(root, property.Ref)
	default:
		if isObj, nullable := objectType(property); isObj {
			goType = setObjectType(decls, root, typeName+toPascalCase(name), property)
			if nullable && goType != "" {
				goType = "*" + goType
			}
		} else {
			goType = setGoType(root, property)
		}
	}
	// ... unchanged tag/emit ...
}

func setObjectType(decls *bytes.Buffer, root, typeName string, s *Schema) string {
	body := generateFields(decls, root, typeName, s)
	if body == "" {
		return ""
	}
	fmt.Fprintf(decls, "type %s struct {\n%s}\n\n", typeName, body)
	return typeName
}
```

### `Generate` — emit one struct per `$defs` entry

After building the main struct body (which fills `decls` with field-driven enum/object decls), emit
the `$defs` types into the same `decls` buffer in sorted order:

```go
body := generateFields(&decls, mainSchema.Title, mainSchema.Title, mainSchema)
if body == "" {
	continue
}

defNames := make([]string, 0, len(mainSchema.Defs))
for name := range mainSchema.Defs {
	defNames = append(defNames, name)
}
sort.Strings(defNames)
for _, name := range defNames {
	setObjectType(&decls, mainSchema.Title, mainSchema.Title+toPascalCase(name), mainSchema.Defs[name])
}
```

`$defs` structs are appended after the main struct's field decls, so output is deterministic across
runs (sorted def names, sorted fields). `RichText` referencing `RichTextNode` (declared later) is
fine — same package, no forward declaration needed.

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

The `Article` struct and the `ArticleDimensions`/`ArticleMetadata`/`ArticleStatus` declarations are
unchanged from step 07. The three `$defs` types are the only additions; `content` (`oneOf`, no `$ref`
at the property level) still resolves to `""` and is skipped.

### model/category.go

Unchanged from step 05.

### model/product.go

Unchanged — `Product` has no `$defs`.

## CLI invocation

```sh
go run ./cmd
```

## Done when

- [ ] `go run ./cmd` exits 0 and prints three `wrote …` lines
- [ ] `model/article.go` declares `ArticlePlainText`, `ArticleRichText`, and `ArticleRichTextNode`,
      with `Nodes []ArticleRichTextNode` and the self-recursive `Children []ArticleRichTextNode`
- [ ] `Article` struct, `model/category.go`, and `model/product.go` are unchanged
- [ ] `go test ./...` passes all 8 harness subtests — `7.json` (RichText `content`) and `8.json`
      (PlainText `content`) still unmarshal into `Article{}`; the extra `content` key is ignored
      because `Article` has no `Content` field yet (added in step 09)
- [ ] `go vet ./...` is clean (the new `$defs` types are package-level and may be unreferenced)
- [ ] Repeated runs produce byte-identical output

## Error handling contract

Inherits all contracts from step 07. Local refs now resolve to `root + defName` instead of `""`; a
local ref whose `$defs` entry is missing yields a reference to an undefined type and won't compile —
the same accepted trade-off as cross-schema refs, not a generator error.

## Out of scope

- `Article.content` (`oneOf` discriminated union) — step 09
- `const` discriminator fields (`format: "richtext"` / `"plaintext"`) — step 09
- Recursive-type verification / round-trip payloads for `children` — step 10 (the field is already
  emitted here; step 10 confirms it round-trips)
