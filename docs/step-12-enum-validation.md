# Step 12 — Enum Validation at Unmarshal Time

## Goal

Close the **invalid-enum** fidelity gap documented in
[step-11](step-11-schema-fidelity.md). Step 05 emits each string enum as a named `string`
type with a `const` block, but because the underlying kind is plain `string`, the standard
`encoding/json` decoder silently accepts **any** value — `{"kind":"bogus"}` unmarshals
cleanly into `model.Category`.

This step makes the generator emit an `UnmarshalJSON` method on every enum type that
**rejects values outside the schema's enum set**. The implementation is **generic**: it is
driven entirely by the generated constants, so it applies automatically to every existing
enum and to any enum introduced by a future schema — no per-type, hand-written code.

## Inputs

Same schemas as step 05. Every string enum in scope gains validation:

| Enum field                 | Generated type          | Valid values                                              |
|----------------------------|-------------------------|----------------------------------------------------------|
| `Product.classification`   | `ProductClassification` | `basic_science`, `basic-science`, `applied`, `experimental` |
| `Product.dimensions.unit`  | `ProductDimensionsUnit` | `cm`, `inch`                                              |
| `Category.kind`            | `CategoryKind`          | `basic-science`, `basic_science`, `clinical`, `other`    |
| `Article.status`           | `ArticleStatus`         | `draft`, `published`, `archived`                          |

Nested enums (e.g. `Product.dimensions.unit`) are covered too — they flow through the same
`setEnumType` path, so the method is emitted regardless of nesting depth.

## Behaviour contract

For a generated enum type `T`:

- **Valid value** → assigned to the receiver, no error.
- **Invalid value** → `fmt.Errorf("invalid T %q", s)`.
- **JSON `null`** → treated as absent: the receiver keeps its zero value and no error is
  returned. This mirrors the `oneOf` union `UnmarshalJSON` (step 09) and avoids erroring on
  an explicit `null` for a non-nullable enum.
- **Empty string `""`** → rejected, because it is not a member of any enum set.

## Implementation

### `setEnumUnmarshal` in `internal/generator/generator.go`

`setEnumType` already builds the constant identifiers; it now also collects them into a
slice and, when at least one exists, delegates to a new helper that emits the method:

```go
// setEnumUnmarshal emits an UnmarshalJSON method that rejects any value outside the enum's
// constant set, turning silent acceptance of invalid values into an error. It is fully
// generic: it references the generated constants, so it works for any string enum from any
// schema. A JSON null is treated as absent (leaves the zero value), mirroring setUnionType.
func setEnumUnmarshal(decls *bytes.Buffer, typeName string, constNames []string) {
	fmt.Fprintf(decls, "\nfunc (e *%s) UnmarshalJSON(data []byte) error {\n", typeName)
	fmt.Fprintf(decls, "\tif string(data) == \"null\" {\n\t\treturn nil\n\t}\n")
	fmt.Fprintf(decls, "\tvar s string\n")
	fmt.Fprintf(decls, "\tif err := json.Unmarshal(data, &s); err != nil {\n\t\treturn err\n\t}\n")
	fmt.Fprintf(decls, "\tswitch %s(s) {\n", typeName)
	fmt.Fprintf(decls, "\tcase %s:\n", strings.Join(constNames, ", "))
	fmt.Fprintf(decls, "\t\t*e = %s(s)\n\t\treturn nil\n", typeName)
	fmt.Fprintf(decls, "\tdefault:\n\t\treturn fmt.Errorf(\"invalid %s %%q\", s)\n\t}\n}\n", typeName)
}
```

### Why a pointer receiver works on value fields

The method has a pointer receiver (`*T`), yet enum struct fields are plain values
(`Kind CategoryKind`, not `*CategoryKind`). This is fine: during `json.Unmarshal` the parent
struct is addressable, so the decoder calls `UnmarshalJSON` on `&c.Kind`. No field needs to
become a pointer.

### Content-based import detection in `Generate()`

The emitted method references `encoding/json` and `fmt`. Previously imports were added only
for schemas containing a `oneOf`. That check is replaced with detection based on whether the
generated code actually references each package, so enum-only files (e.g. `category.go`,
`product.go`) get the imports they now need:

```go
generated := body + refBuf.String()
var imports []string
if strings.Contains(generated, "json.") {
	imports = append(imports, "\t\"encoding/json\"")
}
if strings.Contains(generated, "fmt.") {
	imports = append(imports, "\t\"fmt\"")
}
```

Struct tags use `json:"..."` (colon), not `json.`, so they never produce a false positive.
The now-unused `hasOneOf` helper is removed.

## Output

### model/category.go

```go
package model

import (
	"encoding/json"
	"fmt"
)

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

func (e *CategoryKind) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch CategoryKind(s) {
	case CategoryKindBasicScience, CategoryKindBasicScience2, CategoryKindClinical, CategoryKindOther:
		*e = CategoryKind(s)
		return nil
	default:
		return fmt.Errorf("invalid CategoryKind %q", s)
	}
}
```

`model/product.go` gains the same method for both `ProductClassification` and
`ProductDimensionsUnit`; `model/article.go` for `ArticleStatus`.

## CLI invocation

```sh
go run ./cmd
```

## Done when

- [ ] `go run ./cmd` exits 0 and regenerates the model files
- [ ] Every generated enum type declares an `UnmarshalJSON` method, including nested enums
      (`ProductDimensionsUnit`)
- [ ] Enum-bearing files import `encoding/json` and `fmt`; files without enums/unions import
      neither
- [ ] `go vet ./...` is clean (no unused imports or helpers)
- [ ] `go test ./...` passes: the 8 harness subtests still pass (valid payloads unaffected),
      and the enum subtests in `fidelity_test.go` now assert rejection of invalid values
- [ ] Repeated runs produce byte-identical output

## Error handling contract

Inherits step 05. New: unmarshalling an enum value outside its set returns
`fmt.Errorf("invalid <Type> %q", value)`. `null` is a no-op. The contract is the same for
every enum, current or future.

## Out of scope

- Required-field enforcement and the `null` vs absent distinction — still open gaps in
  [step-11](step-11-schema-fidelity.md)
- Non-string enums (integer/number)
- `MarshalJSON` (marshalling is unchanged; a typed value already serialises as its string)
