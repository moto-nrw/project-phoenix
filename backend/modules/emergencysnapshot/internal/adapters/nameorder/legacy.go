// Package legacy holds the German dictionary order the emergency snapshot
// sorts its rows by. It reads the retained internal/collation package, whose
// home #4086 decides; delete this package once the order has an owner.
package legacy

import "github.com/moto-nrw/project-phoenix/sharedkernel/collation"

// CompareNames orders two names in German dictionary order (DIN 5007-1).
func CompareNames(a, b string) int {
	return collation.CompareGerman(a, b)
}
