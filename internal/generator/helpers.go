package generator

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"unicode"
)

// toPascalCase converts camelCase, snake_case, and kebab-case identifiers to PascalCase.
func toPascalCase(s string) string {
	var b strings.Builder
	upper := true
	for _, r := range s {
		if r == '_' || r == '-' {
			upper = true
			continue
		}
		if upper {
			b.WriteRune(unicode.ToUpper(r))
			upper = false
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// refDefKey strips the path and ".json" suffix from a $ref, yielding the raw $defs key
// callers can index root.Defs with. "#/$defs/RichTextNode" → "RichTextNode";
// "Category.json" → "Category". Empty → "".
func refDefKey(ref string) string {
	if ref == "" {
		return ""
	}
	return strings.TrimSuffix(filepath.Base(ref), ".json")
}

// constValue returns the schema node's const as a string and true, or ("", false)
// if the node has no const or its const is not a JSON string.
func constValue(s *Schema) (string, bool) {
	if len(s.Const) == 0 {
		return "", false
	}
	var v string
	if err := json.Unmarshal(s.Const, &v); err != nil {
		return "", false
	}
	return v, true
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
