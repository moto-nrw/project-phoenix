package application

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/stretchr/testify/assert"
)

// The replace plan decides which live qualification a submitted row continues
// (ADR 0021, #3399); the integration test covers the writes.
func TestPlanQualificationReplace(t *testing.T) {
	t.Parallel()

	live := []domain.StaffQualification{
		{ID: 101, Name: "Fortbildung", AcquiredOn: "2024-01-01"},
		{ID: 102, SortOrder: 1, Name: "Fortbildung", AcquiredOn: "2025-01-01"},
		{ID: 103, SortOrder: 2, Name: "Schwimmschein"},
		{ID: 104, SortOrder: 3, Name: "Erste Hilfe", ExpiresOn: "2026-01-01"},
	}
	submitted := []domain.StaffQualification{
		{Name: "Fortbildung", AcquiredOn: "2025-01-01"},
		{Name: "Erste Hilfe", ExpiresOn: "2028-01-01"},
		{Name: "Fortbildung", AcquiredOn: "2024-01-01"},
		{Name: "Fortbildung", AcquiredOn: "2026-01-01"},
	}

	plan := planQualificationReplace(live, submitted)

	assert.Equal(t, []plannedQualification{
		{action: qualificationUpdate, value: domain.StaffQualification{ID: 102, Name: "Fortbildung", AcquiredOn: "2025-01-01"}},
		{action: qualificationUpdate, value: domain.StaffQualification{ID: 104, SortOrder: 1, Name: "Erste Hilfe", ExpiresOn: "2028-01-01"}},
		{action: qualificationUpdate, value: domain.StaffQualification{ID: 101, SortOrder: 2, Name: "Fortbildung", AcquiredOn: "2024-01-01"}},
		{action: qualificationInsert, value: domain.StaffQualification{SortOrder: 3, Name: "Fortbildung", AcquiredOn: "2026-01-01"}},
	}, plan.rows, "equal rows pair before a same-name row takes the dates")
	assert.Equal(t, []int64{103}, plan.retire)

	unchanged := planQualificationReplace(live, live)
	assert.Empty(t, unchanged.retire, "saving the same list retires nothing")
	for index, row := range unchanged.rows {
		assert.Equal(t, plannedQualification{action: qualificationKeep, value: live[index]}, row)
	}
}
