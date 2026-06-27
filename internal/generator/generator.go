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
		var decls bytes.Buffer

		body := generateFields(&decls, mainSchema.Title, mainSchema)
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
func generateFields(decls *bytes.Buffer, typeName string, s *Schema) string {
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
func setObjectType(decls *bytes.Buffer, typeName string, s *Schema) string {
	body := generateFields(decls, typeName, s)
	if body == "" {
		return ""
	}
	fmt.Fprintf(decls, "type %s struct {\n%s}\n\n", typeName, body)
	return typeName
}

// setGoType returns the Go type string for a schema node, or "" if the type cannot
// be mapped to a primitive (array, object, multi-type beyond nullable, etc.).
// Handles both single-string types ("string") and nullable pairs (["string","null"]).
func setGoType(s *Schema) string {
	if s.Type == nil {
		return ""
	}
	var single string
	if err := json.Unmarshal(s.Type, &single); err == nil && single == ARRAY_TYPE {
		return setArrayType(s)
	}
	return setPrimitiveType(s)
}

// setArrayType returns "[]T" for array schemas whose items resolve to a known type.
// Returns "" for unresolvable items (object, $ref, missing, etc.).
func setArrayType(s *Schema) string {
	if s.Items == nil {
		return ""
	}
	elem := setGoType(s.Items)
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
