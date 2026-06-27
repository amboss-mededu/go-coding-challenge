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

	for _, s := range g.schemas {
		var buf bytes.Buffer

		requiredFields := make(map[string]bool, len(s.Required))
		for _, r := range s.Required {
			requiredFields[r] = true
		}

		// TODO, i think i can avoid this loop and sorting by adding maybe a flag to the schema
		// that tells me if the property was already processed that way i can have idempotency
		propertyNames := make([]string, 0, len(s.Properties))
		for propertyName := range s.Properties {
			propertyNames = append(propertyNames, propertyName)
		}
		sort.Strings(propertyNames)

		var fields bytes.Buffer
		for _, property := range propertyNames {
			goType := setGoType(s.Properties[property])
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
			src, err := format.Source(buf.Bytes())
			if err != nil {
				return nil, fmt.Errorf("formatting source for %q: %w", s.Title, err)
			}

			filename := strings.ToLower(s.Title) + ".go"
			schemasMap[filename] = src
		}
	}

	return schemasMap, nil
}

// setGoType returns the Go type string for a schema node, or "" if the type cannot
// be mapped to a primitive (array, object, multi-type beyond nullable, etc.).
// Handles both single-string types ("string") and nullable pairs (["string","null"]).
func setGoType(s *Schema) string {
	if s.Type == nil {
		return ""
	}

	return setPrimitiveType(s)
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
