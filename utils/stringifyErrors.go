package utils

import (
	"sort"
	"strings"
)

// stringifyErrors turns a map[string]string into one "field: message; ..."
// string, sorted by field name so the output is deterministic.
func StringifyErrors(errs map[string]string) string {
	if len(errs) == 0 {
		return ""
	}

	fields := make([]string, 0, len(errs))
	for field := range errs {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, field+": "+errs[field])
	}
	return strings.Join(parts, "; ")
}
