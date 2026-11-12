package compose

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

// The unread markers of a changed pickup time (#6855) against the real
// tables: who gets one, that it follows its request's state, that opening
// the child clears it, and that the list stays one statement.

type fixedPickupReach struct {
	responsible []int64
	pushes      []careplan.PickupChangePush
}

func (r *fixedPickupReach) ResponsibleAccountIDs(context.Context, int64, int64) ([]int64, error) {
	return r.responsible, nil
}

func (r *fixedPickupReach) NotifyPickupChanged(_ context.Context, push careplan.PickupChangePush) error {
	r.pushes = append(r.pushes, push)
	return nil
}

// pickupTeamToday is a fixed Thursday; the requests lie after it.
var pickupTeamToday = calendar.NewDate(2026, 11, 12)

func pickupTeam(reach careplan.PickupChangeTeamReach) careplan.PickupChangeTeam {
	return NewPickupChangeTeam(reach, testpkg.StudentDirectory{}, nil, func() calendar.Date { return pickupTeamToday })
}

func pickupRequest(t *testing.T, db *testpkg.DB, ctx context.Context, studentID, submitter int64, date calendar.Date, status string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO schedule.care_schedule_change_requests
		(tenant_id, student_id, submitted_by, request_kind, payload, status)
		VALUES (?, ?, ?, 'pickup_change', ?::jsonb, ?) RETURNING id`,
		tenant.FromContext(ctx), studentID, submitter, fmt.Sprintf(`{"date":%q,"pickup_time":"15:00"}`, date.String()), status,
	).Scan(ctx, &id))
	return id
}

func inPickupSchool[T any](t *testing.T, db *testpkg.DB, ctx context.Context, call func(context.Context) (T, error)) (value T, err error) {
	t.Helper()
	txErr := testpkg.WithinTenantContext(t, ctx, db, tenant.FromContext(ctx), func(txCtx context.Context) error {
		value, err = call(txCtx)
		return err
	})
	if err == nil {
		err = txErr
	}
	return value, err
}

func approvePickup(t *testing.T, db *testpkg.DB, ctx context.Context, team careplan.PickupChangeTeam, notice careplan.PickupChangeNotice) {
	t.Helper()
	_, err := inPickupSchool(t, db, ctx, func(txCtx context.Context) (struct{}, error) {
		return struct{}{}, team.NotifyPickupChangeApproved(txCtx, notice)
	})
	require.NoError(t, err)
}

func unreadPickups(t *testing.T, db *testpkg.DB, ctx context.Context, team careplan.PickupChangeTeam, reader int64) []careplan.PickupChangeTeamNotice {
	t.Helper()
	notices, err := inPickupSchool(t, db, ctx, func(txCtx context.Context) ([]careplan.PickupChangeTeamNotice, error) {
		return team.ListUnreadPickupChangeNotices(txCtx, reader)
	})
	require.NoError(t, err)
	return notices
}

func TestPickupChangeTeamMarksTheTeamAndClearsOnOpen(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	child := testpkg.CreateTestStudent(t, db, "Mia", "Abholung", "2a")
	other := testpkg.CreateTestStudent(t, db, "Ben", "Abholung", "2b")
	parent := testpkg.CreateTestAccount(t, db, "pickup-team-parent")
	reviewer := testpkg.CreateTestAccount(t, db, "pickup-team-reviewer")
	colleague := testpkg.CreateTestAccount(t, db, "pickup-team-colleague")
	office := testpkg.CreateTestAccount(t, db, "pickup-team-office")
	date := pickupTeamToday.AddDays(1)
	request := pickupRequest(t, db, ctx, child.ID, parent.ID, date, "approved")
	otherRequest := pickupRequest(t, db, ctx, other.ID, parent.ID, date, "approved")

	reach := &fixedPickupReach{responsible: []int64{reviewer.ID, colleague.ID, office.ID}}
	team := pickupTeam(reach)
	approvePickup(t, db, ctx, team, careplan.PickupChangeNotice{
		StudentID: child.ID, Date: date, PickupTime: "15:00", ReviewerAccountID: reviewer.ID, RequestID: request,
	})
	require.Len(t, reach.pushes, 1)
	assert.Equal(t, []int64{colleague.ID, office.ID}, reach.pushes[0].RecipientAccountIDs)

	unread := unreadPickups(t, db, ctx, team, colleague.ID)
	require.Len(t, unread, 1)
	assert.Equal(t, careplan.PickupChangeTeamNotice{
		RequestID: request, StudentID: child.ID, StudentFirstName: "Mia", Date: date, PickupTime: "15:00",
		InformedAt: unread[0].InformedAt,
	}, unread[0])
	assert.Empty(t, unreadPickups(t, db, ctx, team, reviewer.ID), "the decider has no marker")
	assert.Empty(t, unreadPickups(t, db, ctx, team, parent.ID), "the family has no marker")

	// Another child's change reaches only who is responsible for that child.
	approvePickup(t, db, ctx, pickupTeam(&fixedPickupReach{responsible: []int64{office.ID}}), careplan.PickupChangeNotice{
		StudentID: other.ID, Date: date, PickupTime: "15:00", ReviewerAccountID: reviewer.ID, RequestID: otherRequest,
	})
	assert.Len(t, unreadPickups(t, db, ctx, team, colleague.ID), 1)
	assert.Len(t, unreadPickups(t, db, ctx, team, office.ID), 2)

	remaining, err := inPickupSchool(t, db, ctx, func(txCtx context.Context) ([]careplan.PickupChangeTeamNotice, error) {
		return team.MarkPickupChangeNoticesRead(txCtx, office.ID, child.ID, revisions(unreadPickups(t, db, ctx, team, office.ID)))
	})
	require.NoError(t, err)
	require.Len(t, remaining, 1, "opening one child clears only its markers")
	assert.Equal(t, other.ID, remaining[0].StudentID)
	assert.Len(t, unreadPickups(t, db, ctx, team, colleague.ID), 1, "the colleague's marker stays until they open the child")

	// A correction that takes the time back marks it unread again, as taken
	// back; a read of the revision the colleague loaded before leaves it.
	loaded := revisions(unreadPickups(t, db, ctx, team, colleague.ID))
	_, err = db.NewRaw(`UPDATE schedule.care_schedule_change_requests SET status = 'rejected' WHERE id = ?`, request).Exec(ctx)
	require.NoError(t, err)
	approvePickup(t, db, ctx, team, careplan.PickupChangeNotice{
		StudentID: child.ID, Date: date, Withdrawn: true, ReviewerAccountID: reviewer.ID, RequestID: request,
	})
	office2 := unreadPickups(t, db, ctx, team, office.ID)
	require.Len(t, office2, 2)
	var withdrawn careplan.PickupChangeTeamNotice
	for _, notice := range office2 {
		if notice.RequestID == request {
			withdrawn = notice
		}
	}
	assert.True(t, withdrawn.Withdrawn)
	assert.Empty(t, withdrawn.PickupTime)
	stale, err := inPickupSchool(t, db, ctx, func(txCtx context.Context) ([]careplan.PickupChangeTeamNotice, error) {
		return team.MarkPickupChangeNoticesRead(txCtx, colleague.ID, child.ID, loaded)
	})
	require.NoError(t, err)
	require.Len(t, stale, 1, "the taken-back time arrived after the colleague loaded the page")
	assert.True(t, stale[0].Withdrawn)

	// A deleted request takes its markers along.
	_, err = db.NewRaw(`DELETE FROM schedule.care_schedule_change_requests WHERE id = ?`, otherRequest).Exec(ctx)
	require.NoError(t, err)
	assert.Len(t, unreadPickups(t, db, ctx, team, office.ID), 1)
}

func revisions(notices []careplan.PickupChangeTeamNotice) []careplan.PickupChangeNoticeRevision {
	out := make([]careplan.PickupChangeNoticeRevision, 0, len(notices))
	for _, notice := range notices {
		out = append(out, careplan.PickupChangeNoticeRevision{RequestID: notice.RequestID, InformedAt: notice.InformedAt})
	}
	return out
}

func TestPickupChangeTeamSkipsPastDays(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	child := testpkg.CreateTestStudent(t, db, "Ida", "Gestern", "3a")
	parent := testpkg.CreateTestAccount(t, db, "pickup-past-parent")
	colleague := testpkg.CreateTestAccount(t, db, "pickup-past-colleague")
	date := pickupTeamToday.AddDays(2)
	request := pickupRequest(t, db, ctx, child.ID, parent.ID, date, "approved")
	reach := &fixedPickupReach{responsible: []int64{colleague.ID}}
	approvePickup(t, db, ctx, pickupTeam(reach), careplan.PickupChangeNotice{
		StudentID: child.ID, Date: date, PickupTime: "15:00", RequestID: request,
	})

	later := NewPickupChangeTeam(reach, testpkg.StudentDirectory{}, nil, func() calendar.Date { return date.AddDays(1) })
	assert.Empty(t, unreadPickups(t, db, ctx, later, colleague.ID), "a day that has passed is no longer news")
}

// The header asks for the reader's markers on every page: one statement
// however many children and requests.
func TestPickupChangeTeamListQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	parent := testpkg.CreateTestAccount(t, db, "pickup-budget-parent")
	reader := testpkg.CreateTestAccount(t, db, "pickup-budget-reader")
	team := pickupTeam(&fixedPickupReach{responsible: []int64{reader.ID}})
	counter := testpkg.CaptureQueriesForContext(t, db)

	measure := func(want int) int {
		t.Helper()
		counter.Reset()
		notices, err := inPickupSchool(t, db, counter.Context(ctx), func(txCtx context.Context) ([]careplan.PickupChangeTeamNotice, error) {
			return team.ListUnreadPickupChangeNotices(txCtx, reader.ID)
		})
		require.NoError(t, err)
		require.Len(t, notices, want)
		for _, notice := range notices {
			require.NotEmpty(t, notice.StudentFirstName, "the budget only counts if the names are resolved")
		}
		querybudgets.AssertQueryBudget(t, "modules.careplan.pickup_change_team_notices.list", counter.Measurement(counter.Matching(func(sqlLower string) bool {
			return strings.Contains(sqlLower, "pickup_change_team_notices") || strings.Contains(sqlLower, "student")
		})))
		return counter.Total()
	}
	add := func(n int) {
		t.Helper()
		for i := range n {
			child := testpkg.CreateTestStudent(t, db, fmt.Sprintf("Kind%d", i), "Budget", "1a")
			date := pickupTeamToday.AddDays(1 + i)
			request := pickupRequest(t, db, ctx, child.ID, parent.ID, date, "approved")
			approvePickup(t, db, ctx, team, careplan.PickupChangeNotice{StudentID: child.ID, Date: date, PickupTime: "15:00", RequestID: request})
		}
	}
	add(1)
	small := measure(1)
	add(3)
	assert.Equal(t, small, measure(4), "the statement count must not grow with the markers")
}
