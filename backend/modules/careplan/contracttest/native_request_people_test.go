package contracttest_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type requestPlanDirectory struct {
	services.RequestPeople
	student  peopledirectory.StudentRecord
	baseline *peopledirectory.StudentPlan
}

func (d *requestPlanDirectory) FindStudentRecordForMutation(context.Context, int64) (peopledirectory.StudentRecord, error) {
	return d.student, nil
}

func (d *requestPlanDirectory) UpdateStudent(_ context.Context, write peopledirectory.StudentWrite) (peopledirectory.StudentRecord, error) {
	d.baseline = write.Baseline
	// Mutating the loaded legacy projection must not mutate the saved baseline.
	d.student.BusDays["tue"] = false
	updated := write.Record
	updated.AllowedDepartureModes = write.Plan.Effective().AllowedDepartureModes
	return updated, nil
}

type requestPlanAudit struct {
	before, after *usersModels.Student
}

func (*requestPlanAudit) RecordChanges(context.Context, *usersModels.Student, *usersModels.Student, int64, string) error {
	return errors.New("request approval must use authenticated actor auditing")
}

func (a *requestPlanAudit) RecordChangesForActor(_ context.Context, before, after *usersModels.Student, _ int64) error {
	a.before, a.after = before, after
	return nil
}

func TestRequestApprovalPreservesNativePlanAndAuditSnapshots(t *testing.T) {
	t.Parallel()
	for _, canonical := range []bool{false, true} {
		name := "legacy projections"
		if canonical {
			name = "canonical modes"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newCareFixture(t)
			ctx := f.staffCtx(f.staffAccount)
			student, err := f.sf.PeopleDirectory.FindStudentRecord(ctx, f.chain.StudentID)
			require.NoError(t, err)
			note := "Together"
			student.AllowedDepartureModes, student.DepartureDays = nil, nil
			student.BusDays = usersModels.BusDays{"tue": true}
			student.PickupDays = usersModels.PickupDays{"wed": true}
			student.DepartureCompanionNote = &note
			student.EnrolledUntil = "2026-12-31"
			if canonical {
				student.AllowedDepartureModes = usersModels.AllowedDepartureModes{"mon": {usersModels.DepartureBus, usersModels.DeparturePickup}}
			}
			directory := &requestPlanDirectory{RequestPeople: f.sf.PeopleDirectory, student: student}
			audit := &requestPlanAudit{}
			service := services.NewCareScheduleRequestServiceWithPickupChangesAndPolicy(
				f.repos.CarePlan(), directory, f.sf.ArrivalSchedule, f.sf.PickupSchedule,
				nil, nil, nil, f.sf.UserContext, nil, nil,
				testpkg.RequestReviewPolicy{UserContext: f.sf.UserContext}, nil, slog.Default(), audit,
			)
			request := f.createPending(t, careWeekdays(map[string]any{"weekday": 5, "mode": "pickup", "arrival": "08:00", "pickup": "16:00"}))
			_, err = service.Decide(ctx, carerequests.DecideInput{RequestID: request.ID, Approve: true, ReviewedBy: f.staffAccount})
			require.NoError(t, err)
			require.NotNil(t, directory.baseline)
			require.NotNil(t, audit.before)
			require.NotNil(t, audit.after)
			if canonical {
				assert.ElementsMatch(t, student.AllowedDepartureModes["mon"], audit.before.AllowedDepartureModes["mon"])
				assert.NotContains(t, audit.before.AllowedDepartureModes, "tue", "canonical modes take precedence over stale projections")
			} else {
				assert.Equal(t, []usersModels.DepartureMode{usersModels.DepartureBus}, directory.baseline.AllowedDepartureModes["tue"])
				assert.Equal(t, []usersModels.DepartureMode{usersModels.DeparturePickup}, directory.baseline.AllowedDepartureModes["wed"])
				assert.True(t, directory.baseline.BusDays["tue"])
			}
			assert.Equal(t, &note, audit.before.DepartureCompanionNote)
			require.NotNil(t, audit.before.EnrolledUntil)
			assert.Equal(t, "2026-12-31", audit.before.EnrolledUntil.String())
			assert.Equal(t, []usersModels.DepartureMode{usersModels.DeparturePickup}, audit.after.AllowedDepartureModes["fri"])
		})
	}
}
