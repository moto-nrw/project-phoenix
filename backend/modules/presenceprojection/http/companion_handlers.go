package presenceprojectionhttp

import (
	context "context"

	common "github.com/moto-nrw/project-phoenix/api/common"
	departure "github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

// redactCompanionNames is the shared redaction core behind
// redactUnreadableCompanionNames and the export enrichment: it strips the name
// of every linked child the caller may not read from ALL given link sets, in
// one bulk student read. The link slices are mutated in place.
func (rs *Resource) redactCompanionNames(ctx context.Context, accessCtx *common.StudentAccessContext, linkSets ...[]departure.CompanionLink) error {
	seen := make(map[int64]bool)
	var ids []int64
	for _, links := range linkSets {
		for _, link := range links {
			if !seen[link.CompanionStudentID] {
				seen[link.CompanionStudentID] = true
				ids = append(ids, link.CompanionStudentID)
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	companions, err := rs.studentsByIDs(ctx, ids)
	if err != nil {
		return err
	}

	for _, links := range linkSets {
		for i := range links {
			// A companion that could not be loaded stays redacted: without its
			// group there is nothing to authorize against, and guessing in the
			// permissive direction would leak the very name this guards.
			if hasFullAccessToStudent(accessCtx, companions[links[i].CompanionStudentID]) {
				continue
			}
			links[i].FirstName = ""
			links[i].LastName = ""
		}
	}
	return nil
}
