package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/classday"
)

func TestSupervisionDepartureFailsWhenCompanionLookupFails(t *testing.T) {
	t.Parallel()

	svc := newDayReports(ClassDayDependencies{Companions: failingCompanions{}})
	student := &SheetStudent{}

	_, err := svc.supervisionDeparture(context.Background(), student, timezone.NewDate(2026, 8, 24), []int64{7})
	require.Error(t, err)
	assert.ErrorContains(t, err, "companion lookup failed")
	assert.EqualError(t, err, "supervision sheet: load departure companions: class roster report: load departure companions: companion lookup failed")
}

func TestRecordSupervisionSheetAuditStoresStudentID(t *testing.T) {
	t.Parallel()

	log := &fakeClassDayAccessLog{}
	svc := newDayReports(ClassDayDependencies{AccessLog: log})
	sheet := &classday.SupervisionStudentSheet{StudentID: 17, Date: timezone.NewDate(2026, 8, 24)}

	require.NoError(t, svc.recordSupervisionSheetAudit(context.Background(), sheet, 42, "lehrkraft"))
	require.Len(t, log.sheets, 1)
	// The row went to the supervision sheet resource, not the class day view.
	assert.Empty(t, log.views)
	entry := log.sheets[0]
	if assert.NotNil(t, entry.StudentID) {
		assert.Equal(t, sheet.StudentID, *entry.StudentID)
	}
}
