package careplanhttp

import (
	http "net/http"
	strings "strings"

	common "github.com/moto-nrw/project-phoenix/api/common"
)

// parseMultiValueParam splits a filter parameter that may name several values
// at once (#2218). Values travel comma-separated (`school_class=3a,4b`);
// repeated parameters (`school_class=3a&school_class=4b`) are accepted too so a
// hand-built URL behaves the same. Blanks are dropped and duplicates collapsed,
// preserving the caller's order.
//
// A separator that can also occur inside a value needs an escape, or the two
// are indistinguishable: users.students.school_class is free text, so a school
// may well run a class called "A,B" (#2218 review). A comma that belongs to the
// value is therefore written `\,` and a literal backslash `\\`; the frontend
// encodes both (lib/multi-value-param.ts) and the export request carries the
// same shape. Values without either character encode to themselves, so every
// hand-written `?school_class=3a,4b` keeps working unchanged.
func parseMultiValueParam(raw []string) []string {
	values := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, entry := range raw {
		for _, value := range splitEscapedList(entry) {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if _, duplicate := seen[value]; duplicate {
				continue
			}
			seen[value] = struct{}{}
			values = append(values, value)
		}
	}
	return values
}

// splitEscapedList cuts one parameter at its unescaped commas and unescapes the
// pieces. A trailing lone backslash is dropped rather than treated as escaping
// the terminator — it cannot have come from the encoder, and swallowing it is
// the reading that never invents a value.
func splitEscapedList(raw string) []string {
	values := []string{}
	var current strings.Builder
	escaped := false
	for _, r := range raw {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == ',':
			values = append(values, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	return append(values, current.String())
}

// determineStudentAccess resolves the access context for the current request.
// Thin wrapper that injects this Resource's services into the shared common
// helper so per-student access checks stay cheap when iterating a list.
func (rs *Resource) determineStudentAccess(r *http.Request) *common.StudentAccessContext {
	return common.DetermineStudentAccess(r, rs.UserContextService)
}
