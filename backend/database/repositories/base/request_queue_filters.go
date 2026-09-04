package base

import "strings"

// EscapeILike neutralizes LIKE wildcards in free-text search input. Pair it
// with an explicit ESCAPE '\\' clause.
func EscapeILike(value string) string {
	return strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(value)
}
