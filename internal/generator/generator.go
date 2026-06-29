package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	ARRAY_TYPE = "array"
)

// Schema represents a JSON Schema node.
type Schema struct {
	Title      string             `json:"title"`      // Title is required for named types.
	Type       json.RawMessage    `json:"type"`       // Type can be a string or an array of strings.
	Required   []string           `json:"required"`   // Required property names (only for object types).
	Properties map[string]*Schema `json:"properties"` // Property name → schema (only for object types).
	Items      *Schema            `json:"items"`      // Items schema (only for array types).
	Ref        string             `json:"$ref"`       // Reference to another schema (only for object types).
	Defs       map[string]*Schema `json:"$defs"`      // Definitions for local references (only for object types).
	Enum       []json.RawMessage  `json:"enum"`       // Enum values (only for string, number, or integer types).
	Const      json.RawMessage    `json:"const"`      // Const value (only for string, number, or integer types).
	OneOf      []*Schema          `json:"oneOf"`      // OneOf schemas (only for union types).
}

// Generator walks JSON Schemas and emits Go source.
type Generator struct {
	dir     string
	schemas []*Schema // schemas list of schemas read from the directory
}

var primitiveTypes = map[string]string{
	"string":  "string",
	"number":  "float64",
	"integer": "int",
	"boolean": "bool",
}

// NewGenerator reads all JSON Schema files in dir and returns a Generator.
func NewGenerator(dir string) (*Generator, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading schemas dir %q: %w", dir, err)
	}

	g := &Generator{dir: dir}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %q: %w", path, err)
		}
		var s Schema
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, fmt.Errorf("parsing %q: %w", path, err)
		}
		if s.Title == "" {
			fmt.Fprintf(os.Stderr, "warning: %q has no title, skipping\n", path)
			continue
		}
		g.schemas = append(g.schemas, &s)
	}

	return g, nil
}

// Generate returns a map of filename → gofmt-formatted Go source, one entry per schema.
func (g *Generator) Generate() (map[string][]byte, error) {
	schemasMap := make(map[string][]byte, len(g.schemas))

	for _, mainSchema := range g.schemas {
		var buf bytes.Buffer
		var refBuf bytes.Buffer

		body := generateFields(&refBuf, mainSchema, mainSchema.Title, mainSchema)
		if body == "" {
			continue
		}

		defNames := make([]string, 0, len(mainSchema.Defs))
		for name := range mainSchema.Defs {
			defNames = append(defNames, name)
		}
		sort.Strings(defNames)
		for _, name := range defNames {
			setObjectType(&refBuf, mainSchema, mainSchema.Title+toPascalCase(name), mainSchema.Defs[name])
		}

		// Add imports only for packages the generated code actually references. Struct
		// tags use json:"..." (colon), not "json.", so they don't trigger a false match.
		fileHeader := "package model\n\n"
		generated := body + refBuf.String()
		var imports []string
		if strings.Contains(generated, "json.") {
			imports = append(imports, "\t\"encoding/json\"")
		}
		if strings.Contains(generated, "fmt.") {
			imports = append(imports, "\t\"fmt\"")
		}
		if len(imports) > 0 {
			fileHeader += "import (\n" + strings.Join(imports, "\n") + "\n)\n\n"
		}

		fmt.Fprintf(&buf, "%stype %s struct {\n%s}\n", fileHeader, mainSchema.Title, body)
		if refBuf.Len() > 0 {
			fmt.Fprintf(&buf, "\n%s", refBuf.String())
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

// generateFields builds the struct body for a struct named typeName from s.Properties,
// emitting any nested enum/object type declarations into decls. Properties are sorted
// for deterministic output.
func generateFields(refBuffer *bytes.Buffer, root *Schema, typeName string, s *Schema) string {
	requiredFields := make(map[string]bool, len(s.Required))
	for _, r := range s.Required {
		requiredFields[r] = true
	}

	propertiesNames := make([]string, 0, len(s.Properties))
	for name := range s.Properties {
		propertiesNames = append(propertiesNames, name)
	}
	sort.Strings(propertiesNames)

	var fields bytes.Buffer
	for _, name := range propertiesNames {
		property := s.Properties[name]

		var goTypeName string
		switch {
		case len(property.Enum) > 0:
			goTypeName = setEnumType(refBuffer, typeName, name, property.Enum)
		case len(property.OneOf) > 0:
			goTypeName = setUnionType(refBuffer, root, typeName+toPascalCase(name), property.OneOf)
			if goTypeName != "" {
				goTypeName = "*" + goTypeName
			}
		case property.Ref != "":
			goTypeName = resolveReferences(root.Title, property.Ref)
		default:
			if isObj, nullable := objectType(property); isObj {
				goTypeName = setObjectType(refBuffer, root, typeName+toPascalCase(name), property)
				if nullable && goTypeName != "" {
					goTypeName = "*" + goTypeName
				}
			} else {
				goTypeName = setGoType(root.Title, property)
			}
		}
		if goTypeName == "" {
			continue
		}

		tag := name
		if !requiredFields[name] {
			tag += ",omitempty"
		}
		fmt.Fprintf(&fields, "\t%s %s `json:\"%s\"`\n", toPascalCase(name), goTypeName, tag)
	}
	return fields.String()
}

// setObjectType emits a named struct declaration for an inline object into decls and
// returns the type name. Nested enums/objects are emitted (recursively) before the
// struct itself. An object with no resolvable fields emits nothing and returns "" so the
// caller skips the field.
func setObjectType(decls *bytes.Buffer, root *Schema, typeName string, s *Schema) string {
	body := generateFields(decls, root, typeName, s)
	if body == "" {
		return ""
	}
	fmt.Fprintf(decls, "type %s struct {\n%s}\n\n", typeName, body)
	return typeName
}

// setUnionType emits a struct with one pointer field per variant and an UnmarshalJSON
// for a oneOf union into decls. Returns the struct type name, or "" if no variant resolves.
func setUnionType(decls *bytes.Buffer, root *Schema, typeName string, members []*Schema) string {
	type variant struct{ goTypeName, fieldName, disc string }
	var variants []variant
	discKey := ""
	for _, m := range members {
		if m.Ref != "" {
			goTypeName := resolveReferences(root.Title, m.Ref)
			defKey := refDefKey(m.Ref)
			def := root.Defs[defKey]
			if def == nil {
				continue
			}
			names := make([]string, 0, len(def.Properties))
			for n := range def.Properties {
				names = append(names, n)
			}
			sort.Strings(names)
			k, v := "", ""
			for _, n := range names {
				if cv, ok := constValue(def.Properties[n]); ok {
					k, v = n, cv
					break
				}
			}
			if k == "" {
				continue
			}
			discKey = k
			variants = append(variants, variant{
				goTypeName: goTypeName,
				fieldName:  toPascalCase(defKey),
				disc:       v,
			})
		}
	}
	if len(variants) == 0 {
		return ""
	}

	fmt.Fprintf(decls, "type %s struct {\n", typeName)
	for _, v := range variants {
		fmt.Fprintf(decls, "\t%s *%s\n", v.fieldName, v.goTypeName)
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
			v.disc, v.fieldName, v.goTypeName, v.fieldName)
	}
	fmt.Fprintf(decls, "\tdefault:\n\t\treturn fmt.Errorf(\"unknown %s %%q\", disc.%s)\n\t}\n}\n\n", discKey, field)
	return typeName
}

// setGoType returns the Go type string for a schema node, or "" if the type cannot
// be mapped to a primitive (array, object, multi-type beyond nullable, etc.).
// Handles both single-string types ("string") and nullable pairs (["string","null"]).
func setGoType(root string, s *Schema) string {
	if s.Ref != "" {
		return resolveReferences(root, s.Ref)
	}
	if s.Type == nil {
		return ""
	}
	var typeName string
	if err := json.Unmarshal(s.Type, &typeName); err == nil && typeName == ARRAY_TYPE {
		return setArrayType(root, s)
	}
	return setPrimitiveType(s)
}

// setArrayType returns "[]T" for array schemas whose items resolve to a known type.
// Returns "" for unresolvable items (object, $ref, missing, etc.).
func setArrayType(root string, s *Schema) string {
	if s.Items == nil {
		return ""
	}
	goTypeName := setGoType(root, s.Items)
	if goTypeName == "" {
		return ""
	}
	return "[]" + goTypeName
}

// setEnumType emits a named string type and a const block for a string enum into decls,
// and returns the generated type name to use as the field's Go type. Non-string enum
// values are skipped; identifier collisions (after PascalCasing) are disambiguated with a
// numeric suffix so every value gets its own constant.
func setEnumType(decls *bytes.Buffer, schemaName, property string, values []json.RawMessage) string {
	typeName := schemaName + toPascalCase(property)
	var consts bytes.Buffer
	seen := make(map[string]bool)
	var constNames []string
	for _, raw := range values {
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			continue // non-string enum value — out of scope for step 05
		}
		name := typeName + toPascalCase(v)
		base := name
		for i := 2; seen[name]; i++ {
			name = fmt.Sprintf("%s%d", base, i)
		}
		seen[name] = true
		constNames = append(constNames, name)
		fmt.Fprintf(&consts, "\t%s %s = %q\n", name, typeName, v)
	}

	fmt.Fprintf(decls, "type %s string\n\nconst (\n%s)\n", typeName, consts.String())

	if len(constNames) > 0 {
		setEnumUnmarshal(decls, typeName, constNames)
	}
	return typeName
}

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

// resolveReferences maps a $ref to its Go type name. A local ref ("#/$defs/RichTextNode")
// PascalCases the segment after the last "/" and prefixes the root schema title →
// "ArticleRichTextNode". A cross-schema ref ("Category.json") strips the path and ".json"
// suffix → "Category". Empty → "".
func resolveReferences(root, ref string) string {
	name := refDefKey(ref)
	if name == "" {
		return ""
	}
	if strings.HasPrefix(ref, "#") {
		return root + toPascalCase(name)
	}
	return name
}

// setPrimitiveType returns the Go type string for a schema node, or "" if the type cannot
// be mapped to a primitive (array, object, multi-type beyond nullable, etc.).
// Handles both single-string types ("string") and nullable pairs (["string","null"]).
func setPrimitiveType(s *Schema) string {
	// Case 1: "type": "string" — single primitive
	var single string
	if err := json.Unmarshal(s.Type, &single); err == nil {
		return primitiveTypes[single]
	}
	// Case 2: "type": ["string", "null"] — nullable primitive (order-independent)
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
