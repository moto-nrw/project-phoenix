package application

import (
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// visitExpiries keeps each student's own cutoff in the set-based deletion
// (#6455) and refuses input that would delete with a guessed window.
func TestVisitExpiriesKeepEachStudentsCutoff(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	boundary := time.Date(2026, 11, 9, 23, 0, 0, 0, time.UTC)

	expiries, err := visitExpiries([]studentpresence.VisitRetentionCount{
		{StudentID: 41, RetentionDays: 30},
		{StudentID: 42, RetentionDays: 7},
		{StudentID: 43, RetentionDays: 0, CompletedBefore: &boundary},
	}, now)
	require.NoError(t, err)
	assert.Equal(t, []ports.VisitExpiry{
		{StudentID: 41, Cutoff: now.AddDate(0, 0, -30)},
		{StudentID: 42, Cutoff: now.AddDate(0, 0, -7)},
		{StudentID: 43, Cutoff: boundary, ByExit: true},
	}, expiries)

	for name, students := range map[string][]studentpresence.VisitRetentionCount{
		"missing student":   {{RetentionDays: 30}},
		"duplicate student": {{StudentID: 41, RetentionDays: 30}, {StudentID: 41, RetentionDays: 7}},
		"no window":         {{StudentID: 41}},
	} {
		_, err := visitExpiries(students, now)
		require.Error(t, err, name)
	}
	_, err = visitExpiries([]studentpresence.VisitRetentionCount{{StudentID: 41, RetentionDays: 30}}, time.Time{})
	require.Error(t, err, "zero clock")

	expiries, err = visitExpiries(nil, now)
	require.NoError(t, err)
	assert.Empty(t, expiries)
}
