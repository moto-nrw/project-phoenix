package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func TestSupervisionDepartureFailsWhenCompanionLookupFails(t *testing.T) {
	t.Parallel()

	svc := newDayReports(ClassDayDependencies{Companions: failingCompanions{}})
	student := &SheetStudent{}

	_, err := svc.supervisionDeparture(context.Background(), student, calendar.NewDate(2026, 8, 24), []int64{7})
	require.Error(t, err)
	assert.ErrorContains(t, err, "companion lookup failed")
	assert.EqualError(t, err, "supervision sheet: load departure companions: class roster report: load departure companions: companion lookup failed")
}

func TestRecordSupervisionSheetAuditStoresStudentID(t *testing.T) {
	t.Parallel()

	log := &fakeClassDayAccessLog{}
	svc := newDayReports(ClassDayDependencies{AccessLog: log})
	sheet := &classday.SupervisionStudentSheet{StudentID: 17, Date: calendar.NewDate(2026, 8, 24)}

	require.NoError(t, svc.recordSupervisionSheetAudit(context.Background(), sheet, 42, "lehrkraft"))
	require.Len(t, log.sheets, 1)
	// The row went to the supervision sheet resource, not the class day view.
	assert.Empty(t, log.views)
	entry := log.sheets[0]
	if assert.NotNil(t, entry.StudentID) {
		assert.Equal(t, sheet.StudentID, *entry.StudentID)
	}
}

func TestFoldSupervisionContactsLimitsPickupPermissionToWeekdays(t *testing.T) {
	t.Parallel()

	rows := []EmergencyContactRow{{
		StudentID: 1, GuardianProfileID: 2, FirstName: "Erika", LastName: "Muster",
		CanPickup: true, PickupWeekdays: []int{2, 4},
	}}

	tuesday := foldSupervisionContacts(rows, 1, calendar.NewDate(2026, 9, 8))
	require.Len(t, tuesday, 1)
	assert.True(t, tuesday[0].canPickup)

	wednesday := foldSupervisionContacts(rows, 1, calendar.NewDate(2026, 9, 9))
	require.Len(t, wednesday, 1)
	assert.False(t, wednesday[0].canPickup)
}
