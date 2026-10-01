package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// pickupWeekdayMonday is Monday in the ISO weekday numbering of the pickup
// schedules.
const pickupWeekdayMonday = 1

// --- CareUsage display enrichment (#2215) ------------------------------

func TestCareUsageEnrichesGuardiansAndSchedulePickup(t *testing.T) {
	t.Parallel()

	reportDate := calendar.NewDate(2026, 8, 24).AddDays(30)
	studentID := int64(700)
	excludedStudentID := int64(701)
	guardianEmail := "max@example.org"
	guardianRepo := &fakeClassRosterRequestGuardianRepo{guardians: []*enrollment.RequestGuardian{{
		RequestID: 11,
		FirstName: "Max",
		LastName:  "Muster",
		Email:     &guardianEmail,
	}}}
	pickupSvc := &fakeCareUsagePickupScheduleSvc{rows: []*careplan.PickupSchedule{{
		StudentID:  studentID,
		Weekday:    pickupWeekdayMonday,
		PickupTime: time.Date(1, 1, 1, 14, 30, 0, 0, time.UTC),
	}}}
	svc := NewReports(ReportDependencies{
		// The phase starts after this day, so the report reads the selection
		// at the phase start rather than today.
		Now: func() time.Time { return reportDate.AddDays(-30).BerlinMidnight() },
		Requests: &fakeCareUsageRequestRepo{requests: []*enrollmentModels.Request{{
			ID:                11,
			GuardianFirstName: "Eva",
			GuardianLastName:  "Muster",
			GuardianEmail:     "eva@example.org",
			SubmittedAt:       time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC),
		}, {
			ID: 12, SubmittedAt: time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC),
		}}},
		Children: &fakeClassRosterChildRepo{children: []*reportChild{{
			ID:               21,
			RequestID:        11,
			FirstName:        "Lina",
			LastName:         "Muster",
			Status:           enrollmentModels.ChildStatusApproved,
			CreatedStudentID: &studentID,
		}, {
			ID: 22, RequestID: 12, FirstName: "Nicht", LastName: "Enthalten",
			Status: enrollmentModels.ChildStatusApproved, CreatedStudentID: &excludedStudentID,
		}}},
		Guardians: guardianRepo,
		Offerings: &fakeClassRosterCareOfferingRepo{},
		Phases: &fakeClassRosterPhaseRepo{phase: &enrollment.Phase{
			ServiceStartDate: enrollment.Date(reportDate),
			ServiceEndDate:   enrollment.Date(reportDate.AddDays(365)),
		}},
		PickupSchedules: pickupSvc,
	})

	report, err := svc.careUsage(context.Background(), enrollment.CareUsageFilters{PhaseID: 55, Status: "all", Search: "Lina"}, true)

	require.NoError(t, err)
	require.Len(t, report.Rows, 1)
	row := report.Rows[0]
	// Maintained Kind-Gehzeit of the linked student is attached for
	// display; the snapshot PickupByDay stays untouched.
	assert.Equal(t, "14:30", row.SchedulePickupByDay["mon"])
	assert.Empty(t, row.PickupByDay["mon"])
	assert.Equal(t, []string{"mon", "tue", "wed", "thu", "fri"}, row.CareDays)
	// All request guardians, primary first.
	require.Len(t, row.Guardians, 2)
	assert.Equal(t, "Eva Muster", row.Guardians[0].Name)
	assert.Equal(t, "Max Muster", row.Guardians[1].Name)
	assert.Equal(t, guardianEmail, row.Guardians[1].Email)
	assert.Equal(t, []int64{11}, guardianRepo.requestIDs)
	assert.Equal(t, []int64{studentID}, pickupSvc.studentIDs)
	assert.Equal(t, reportDate, pickupSvc.date)
}

func TestCareUsageDoesNotEnrichSchedulePickupBeforeApproval(t *testing.T) {
	t.Parallel()

	studentID := int64(700)
	svc := NewReports(ReportDependencies{
		Requests: &fakeCareUsageRequestRepo{requests: []*enrollmentModels.Request{{
			ID:          11,
			SubmittedAt: time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC),
		}}},
		Children: &fakeClassRosterChildRepo{children: []*reportChild{{
			ID:               21,
			RequestID:        11,
			FirstName:        "Lina",
			LastName:         "Muster",
			Status:           enrollmentModels.ChildStatusSubmitted,
			MatchedStudentID: &studentID,
		}}},
		Offerings: &fakeClassRosterCareOfferingRepo{},
		Phases:    &fakeClassRosterPhaseRepo{},
		PickupSchedules: &fakeCareUsagePickupScheduleSvc{rows: []*careplan.PickupSchedule{{
			StudentID:  studentID,
			Weekday:    pickupWeekdayMonday,
			PickupTime: time.Date(1, 1, 1, 14, 30, 0, 0, time.UTC),
		}}},
	})

	report, err := svc.careUsage(context.Background(), enrollment.CareUsageFilters{PhaseID: 55, Status: "all"}, true)

	require.NoError(t, err)
	require.Len(t, report.Rows, 1)
	assert.Empty(t, report.Rows[0].SchedulePickupByDay)
}

func TestCareUsageSkipsCompactEnrichment(t *testing.T) {
	t.Parallel()

	enrichmentErr := errors.New("enrichment unavailable")
	svc := NewReports(ReportDependencies{
		Requests: &fakeCareUsageRequestRepo{requests: []*enrollmentModels.Request{{
			ID:          11,
			SubmittedAt: time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC),
		}}},
		Children: &fakeClassRosterChildRepo{children: []*reportChild{{
			ID: 21, RequestID: 11, FirstName: "Lina", LastName: "Muster",
		}}},
		Guardians:       &fakeClassRosterRequestGuardianRepo{err: enrichmentErr},
		Offerings:       &fakeClassRosterCareOfferingRepo{},
		Phases:          &fakeClassRosterPhaseRepo{},
		PickupSchedules: &fakeCareUsagePickupScheduleSvc{err: enrichmentErr},
	})

	report, err := svc.CareUsage(context.Background(), enrollment.CareUsageFilters{PhaseID: 55, Status: "all"})

	require.NoError(t, err)
	require.Len(t, report.Rows, 1)
	assert.Nil(t, report.Rows[0].CareDays)
	assert.Nil(t, report.Rows[0].SchedulePickupByDay)
}
