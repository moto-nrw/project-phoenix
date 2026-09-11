package testdb

import "slices"

// LeftoverAllowlist must stay empty: every test package must leave shared
// clone state unchanged. It remains a map so the gate fails closed if a pair
// is ever added deliberately while preserving the package/table diagnostic.
var LeftoverAllowlist = map[string][]string{}

// leftoverAllowed reports whether a leftover in table is tolerated for the
// package that produced the clone. An unlabeled clone (no package stamp) is
// never tolerated: the gate must not be switched off by a missing label.
func leftoverAllowed(pkg, table string) bool {
	if pkg == "" {
		return false
	}
	return slices.Contains(LeftoverAllowlist[pkg], table)
}

// UnallowedTables filters a package's leftovers down to the ones the
// allowlist does not cover — the ones that fail the package.
func UnallowedTables(pkg string, deltas []TableDelta) []TableDelta {
	var out []TableDelta
	for _, d := range deltas {
		if !leftoverAllowed(pkg, d.Table) {
			out = append(out, d)
		}
	}
	return out
}
