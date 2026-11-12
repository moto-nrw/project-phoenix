package enrollmenthttp

import (
	"maps"

	enrollment "github.com/moto-ogs/project-phoenix/modules/enrollment"
)

type preparedStudentExtraFields struct {
	summary    *enrollment.DecisionSummary
	customData map[string]any
}

// Only linked children with populated answers contribute schema IDs.
func prepareStudentExtraFields(summaries []*enrollment.DecisionSummary, studentID int64) ([]preparedStudentExtraFields, []int64) {
	prepared := make([]preparedStudentExtraFields, 0, len(summaries))
	ids := make([]int64, 0, len(summaries))
	seen := make(map[int64]bool, len(summaries))
	for _, summary := range summaries {
		if summary == nil || summary.Request == nil || summary.Request.SchemaID == nil {
			continue
		}
		child := linkedSummaryChild(summary, studentID)
		if child == nil {
			continue
		}
		data := maps.Collect(child.CustomData.All())
		prepared = append(prepared, preparedStudentExtraFields{summary: summary, customData: data})
		id := *summary.Request.SchemaID
		if len(data) > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return prepared, ids
}
