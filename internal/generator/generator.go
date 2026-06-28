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

// hasOneOf reports whether any top-level property of s uses oneOf.
func hasOneOf(s *Schema) bool {
	for _, p := range s.Properties {
		if len(p.OneOf) > 0 {
			return true
		}
	}
	return false
}

// Generate returns a map of filename → gofmt-formatted Go source, one entry per schema.
func (g *Generator) Generate() (map[string][]byte, error) {
	schemasMap := make(map[string][]byte, len(g.schemas))

	for _, mainSchema := range g.schemas {
		var buf bytes.Buffer
		var references bytes.Buffer

		body := generateFields(&references, mainSchema, mainSchema.Title, mainSchema)
		if body == "" {
			continue
		}

		defNames := make([]string, 0, len(mainSchema.Defs))
		for name := range mainSchema.Defs {
			defNames = append(defNames, name)
		}
		sort.Strings(defNames)
		for _, name := range defNames {
			setObjectType(&references, mainSchema, mainSchema.Title+toPascalCase(name), mainSchema.Defs[name])
		}

		header := "package model\n\n"
		if hasOneOf(mainSchema) {
			header += "import (\n\t\"encoding/json\"\n\t\"fmt\"\n)\n\n"
		}
		fmt.Fprintf(&buf, "%stype %s struct {\n%s}\n", header, mainSchema.Title, body)
		if references.Len() > 0 {
			fmt.Fprintf(&buf, "\n%s", references.String())
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
		// it looks redundant to return the same value twice, but the first return is for isObject and the second is for nullable
		// but we are evaluating this scenario: ["object", "null"] which means it is an object and it is nullable.
		return hasObject && hasNull, hasObject && hasNull
	}
	return false, false
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

		var goType string
		switch {
		case len(property.Enum) > 0:
			goType = setEnumType(refBuffer, typeName, name, property.Enum)
		case len(property.OneOf) > 0:
			goType = setUnionType(refBuffer, root, typeName+toPascalCase(name), property.OneOf)
			if goType != "" {
				goType = "*" + goType
			}
		case property.Ref != "":
			goType, _ = resolveReferences(root.Title, property.Ref)
		default:
			if isObj, nullable := objectType(property); isObj {
				goType = setObjectType(refBuffer, root, typeName+toPascalCase(name), property)
				if nullable && goType != "" {
					goType = "*" + goType
				}
			} else {
				goType = setGoType(root.Title, property)
			}
		}
		if goType == "" {
			continue
		}

		tag := name
		if !requiredFields[name] {
			tag += ",omitempty"
		}
		fmt.Fprintf(&fields, "\t%s %s `json:\"%s\"`\n", toPascalCase(name), goType, tag)
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

// discriminator returns the JSON key and string value of the first (sorted) property
// carrying a non-empty string const — the field that selects a oneOf variant.
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

// setUnionType emits a struct with one pointer field per variant and an UnmarshalJSON
// for a oneOf union into decls. Returns the struct type name, or "" if no variant resolves.
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

// setGoType returns the Go type string for a schema node, or "" if the type cannot
// be mapped to a primitive (array, object, multi-type beyond nullable, etc.).
// Handles both single-string types ("string") and nullable pairs (["string","null"]).
func setGoType(root string, s *Schema) string {
	if s.Ref != "" {
		gt, _ := resolveReferences(root, s.Ref)
		return gt
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

// setArrayType returns "[]T" for array schemas whose items resolve to a known type.
// Returns "" for unresolvable items (object, $ref, missing, etc.).
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

// setEnumType emits a named string type and a const block for a string enum into decls,
// and returns the generated type name to use as the field's Go type. Non-string enum
// values are skipped; identifier collisions (after PascalCasing) are disambiguated with a
// numeric suffix so every value gets its own constant.
func setEnumType(decls *bytes.Buffer, schemaName, property string, values []json.RawMessage) string {
	typeName := schemaName + toPascalCase(property)
	var consts bytes.Buffer
	seen := make(map[string]bool)
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
		fmt.Fprintf(&consts, "\t%s %s = %q\n", name, typeName, v)
	}

	fmt.Fprintf(decls, "type %s string\n\nconst (\n%s)\n", typeName, consts.String())
	return typeName
}

// resolveReferences maps a $ref to its Go type name and the bare ref name. A local ref
// ("#/$defs/RichTextNode") PascalCases the segment after the last "/" and prefixes the root schema
// title → ("ArticleRichTextNode", "RichTextNode"). A cross-schema ref ("Category.json") strips the
// path and ".json" suffix → ("Category", "Category"). The bare name is the raw def key, so callers
// can index root.Defs with it; PascalCasing is left to callers. Empty → ("", "").
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
