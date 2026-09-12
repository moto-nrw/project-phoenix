package compose

import (
	"context"
	"testing"
	"time"

	peoplecompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestCorrectionLogPreservesCursorBeforeVisibilityAndAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	people, err := peoplecompose.New(peoplecompose.Dependencies{DB: db, Observe: func(peoplecompose.Observation) {}})
	require.NoError(t, err)
	visible := testpkg.CreateTestStudent(t, db, "Visible", "Correction", "1a")
	hidden := testpkg.CreateTestStudent(t, db, "Former", "Correction", "1a")
	testpkg.SetStudentStatus(t, db, hidden.ID, "alumnus")
	instant := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	first := testpkg.CreateTestOfferingCorrection(t, db, visible.ID, "direct", instant)
	last := testpkg.CreateTestOfferingCorrection(t, db, hidden.ID, "direct", instant)
	allowed := true
	log, err := NewCorrectionLog(db, people, func(context.Context) bool { return allowed }, func(AuditObservation) {})
	require.NoError(t, err)
	rows, cursor, err := log.History(testpkg.Ctx(t), requestreview.QueueFilter{Limit: 1})
	require.NoError(t, err)
	require.Empty(t, rows)
	require.NotNil(t, cursor)
	require.Equal(t, last, cursor.ID)
	rows, next, err := log.History(testpkg.Ctx(t), requestreview.QueueFilter{Limit: 1, Before: cursor})
	require.NoError(t, err)
	require.Nil(t, next)
	require.Len(t, rows, 1)
	require.Equal(t, first, rows[0].ID)
	require.Equal(t, "Visible Correction", rows[0].StudentName)
	response, ok := rows[0].Data.(requestreview.DirectCorrectionResponse)
	require.True(t, ok)
	require.Equal(t, "Unbekannt", response.ChangedByName)
	require.Empty(t, response.Diff)
	allowed = false
	rows, _, err = log.History(testpkg.Ctx(t), requestreview.QueueFilter{Limit: 10})
	require.NoError(t, err)
	require.Empty(t, rows)
	_, err = NewCorrectionLog(db, people, nil, func(AuditObservation) {})
	require.Error(t, err)
}
