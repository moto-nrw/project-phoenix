package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These repositories no longer join users.staff. The staff facts are attached
// by the composition layer through School Membership (#2667); the tests drive
// the factory-composed repository and pin the caller-visible contract.

func offboardStaff(tb testing.TB, ctx context.Context, factory *repositories.Factory, staffID int64) {
	tb.Helper()
	require.NoError(tb, factory.Staff.Delete(ctx, staffID))
}

func TestActiveSupervisionsCarryTheirStaffMember(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	factory := repositories.NewFactory(db)

	present := testpkg.CreateTestStaff(t, db, "Anwesende", "Betreuung")
	offboarded := testpkg.CreateTestStaff(t, db, "Ehemalige", "Betreuung")
	activity := testpkg.CreateTestActivityGroup(t, db, "Aufsicht Angebot")
	room := testpkg.CreateTestRoom(t, db, "Aufsicht Raum")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	testpkg.CreateTestGroupSupervisor(t, db, present.ID, group.ID, "supervisor")
	testpkg.CreateTestGroupSupervisor(t, db, offboarded.ID, group.ID, "supervisor")
	// The replaced LEFT JOIN had no soft-delete filter: an offboarded
	// colleague keeps resolving on a supervision that is still recorded.
	offboardStaff(t, ctx, factory, offboarded.ID)

	rows, err := factory.GroupSupervisor.FindByActiveGroupID(ctx, group.ID, false)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	byStaff := make(map[int64]*activeModels.GroupSupervisor, len(rows))
	for _, row := range rows {
		byStaff[row.StaffID] = row
	}
	require.NotNil(t, byStaff[present.ID].Staff)
	assert.Equal(t, present.ID, byStaff[present.ID].Staff.ID)
	assert.Equal(t, present.PersonID, byStaff[present.ID].Staff.PersonID)
	require.NotNil(t, byStaff[offboarded.ID].Staff, "an offboarded supervisor still resolves")
	assert.Equal(t, offboarded.PersonID, byStaff[offboarded.ID].Staff.PersonID)

	bulk, err := factory.GroupSupervisor.FindByActiveGroupIDs(ctx, []int64{group.ID}, false)
	require.NoError(t, err)
	require.Len(t, bulk, 2)
	for _, row := range bulk {
		require.NotNil(t, row.Staff)
	}
}

func createAbsence(tb testing.TB, ctx context.Context, factory *repositories.Factory, staffID int64, approvedBy *int64) {
	tb.Helper()
	today := timezone.TodayDate()
	absence := &activeModels.StaffAbsence{
		StaffID:     staffID,
		AbsenceType: activeModels.AbsenceTypeVacation,
		DateStart:   today,
		DateEnd:     today,
		Status:      activeModels.AbsenceStatusRequested,
		ApprovedBy:  approvedBy,
		CreatedBy:   staffID,
		RequestedAt: time.Now(),
	}
	require.NoError(tb, factory.StaffAbsence.Create(ctx, absence))
}

func TestAbsenceRequestsCarryTheirSubjectAndDeciderPerson(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	factory := repositories.NewFactory(db)

	subject := testpkg.CreateTestStaff(t, db, "Antrag", "Stellerin")
	decider := testpkg.CreateTestStaff(t, db, "Antrag", "Entscheiderin")
	other := testpkg.CreateTestStaff(t, db, "Andere", "Person")
	deciderID := decider.ID
	createAbsence(t, ctx, factory, subject.ID, &deciderID)
	createAbsence(t, ctx, factory, other.ID, nil)

	filter := activeModels.AbsenceRequestFilter{Statuses: []string{activeModels.AbsenceStatusRequested}}
	rows, err := factory.StaffAbsence.ListRequests(ctx, filter)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.NotNil(t, row.SubjectPersonID)
		switch row.StaffID {
		case subject.ID:
			assert.Equal(t, subject.PersonID, *row.SubjectPersonID)
			require.NotNil(t, row.DeciderPersonID)
			assert.Equal(t, decider.PersonID, *row.DeciderPersonID)
		case other.ID:
			assert.Equal(t, other.PersonID, *row.SubjectPersonID)
			assert.Nil(t, row.DeciderPersonID, "an undecided request names no decider")
		}
	}

	t.Run("a person filter narrows to that person's staff", func(t *testing.T) {
		filtered := filter
		filtered.SubjectPersonIDs = []int64{subject.PersonID}
		rows, err := factory.StaffAbsence.ListRequests(ctx, filtered)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, subject.ID, rows[0].StaffID)
	})

	t.Run("an empty person set matches nobody", func(t *testing.T) {
		filtered := filter
		filtered.SubjectPersonIDs = []int64{}
		rows, err := factory.StaffAbsence.ListRequests(ctx, filtered)
		require.NoError(t, err)
		assert.Empty(t, rows, "an empty search result must not widen to everybody")
	})

	t.Run("the limit caps the list", func(t *testing.T) {
		limited := filter
		limited.Limit = 1
		rows, err := factory.StaffAbsence.ListRequests(ctx, limited)
		require.NoError(t, err)
		require.Len(t, rows, 1)
	})
}
