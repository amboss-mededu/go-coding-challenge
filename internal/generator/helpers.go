package generator

import (
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
