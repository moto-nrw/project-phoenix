package presenceprojectionhttp

import (
	strings "strings"

	"github.com/moto-nrw/project-phoenix/sharedkernel/collation"
)

// containsFold reports whether s contains substr, ignoring case and accents
// (collation.FoldForSearch on both sides, the search and filter semantics of
// the student lists, #4949).
func containsFold(s, substr string) bool {
	return strings.Contains(collation.FoldForSearch(s), collation.FoldForSearch(substr))
}
