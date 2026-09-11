package legacy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	enrollmentModule "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const today timezone.Date = "2026-08-24"

var at = today.BerlinMidnight().Add(9 * time.Hour)

func fixedNow() time.Time { return at }

// The fakes embed their retained service interface so only the queue methods
// the adapter touches need implementations; anything else panics on a nil
// interface, which is exactly the failure we want in a test.

type masterFake struct {
	userService.MasterDataReviewService
	pending []*userService.MasterDataReviewItem
	history []*userService.MasterDataHistoryItem
	err     error
	filters []modelBase.RequestQueueFilters
}

func (f *masterFake) ListPending(_ context.Context, filters modelBase.RequestQueueFilters) ([]*userService.MasterDataReviewItem, *userService.HistoryCursor, error) {
	f.filters = append(f.filters, filters)
	return f.pending, nil, f.err
}

func (f *masterFake) ListHistory(_ context.Context, filters modelBase.RequestQueueFilters) ([]*userService.MasterDataHistoryItem, *userService.HistoryCursor, error) {
	f.filters = append(f.filters, filters)
	return f.history, &userService.HistoryCursor{UpdatedAt: at, ID: 1}, f.err
}

type careFake struct {
	scheduleService.CareScheduleRequestService
	pending []*scheduleService.CareRequestReviewItem
	history []*scheduleService.CareRequestHistoryItem
}

func (f *careFake) ListPending(context.Context, modelBase.RequestQueueFilters) ([]*scheduleService.CareRequestReviewItem, *userService.HistoryCursor, error) {
	return f.pending, nil, nil
}

func (f *careFake) ListHistory(context.Context, modelBase.RequestQueueFilters) ([]*scheduleService.CareRequestHistoryItem, *userService.HistoryCursor, error) {
	return f.history, nil, nil
}

type offeringFake struct {
	enrollmentService.OfferingChangeRequestService
	pending     []*enrollmentService.OfferingChangeView
	history     []*enrollmentService.OfferingChangeHistoryItem
	corrections []*enrollmentService.DirectCorrectionItem
	count       int
	countCalls  int
}

func (f *offeringFake) ListPending(context.Context, modelBase.RequestQueueFilters) ([]*enrollmentService.OfferingChangeView, *userService.HistoryCursor, error) {
	return f.pending, nil, nil
}

func (f *offeringFake) ListHistory(context.Context, modelBase.RequestQueueFilters) ([]*enrollmentService.OfferingChangeHistoryItem, *userService.HistoryCursor, error) {
	return f.history, nil, nil
}

func (f *offeringFake) ListDirectCorrections(context.Context, modelBase.RequestQueueFilters) ([]*enrollmentService.DirectCorrectionItem, *userService.HistoryCursor, error) {
	return f.corrections, nil, nil
}

func (f *offeringFake) PendingCount(context.Context) (int, error) {
	f.countCalls++
	return f.count, nil
}

type excusedFake struct {
	excusedrequests.Service
	pending []*excusedrequests.ReviewItem
	history []*excusedrequests.HistoryItem
	filters []excusedrequests.QueueFilter
}

func (f *excusedFake) ListPending(_ context.Context, filter excusedrequests.QueueFilter) ([]*excusedrequests.ReviewItem, *excusedrequests.Cursor, error) {
	f.filters = append(f.filters, filter)
	return f.pending, &excusedrequests.Cursor{UpdatedAt: at, ID: 7}, nil
}

func (f *excusedFake) ListHistory(context.Context, excusedrequests.QueueFilter) ([]*excusedrequests.HistoryItem, *excusedrequests.Cursor, error) {
	return f.history, nil, nil
}

type fakes struct {
	master   *masterFake
	care     *careFake
	offering *offeringFake
	excused  *excusedFake
}

func newFakes() fakes {
	return fakes{master: &masterFake{}, care: &careFake{}, offering: &offeringFake{}, excused: &excusedFake{}}
}

func (f fakes) sources() Sources {
	return Sources{MasterData: f.master, CareSchedule: f.care, Offering: f.offering, Excused: f.excused, Now: fixedNow}
}

func writeQueueCtx() context.Context {
	return context.WithValue(context.Background(), jwt.CtxPermissions, []string{"users:read", "users:update"})
}

func TestNewRejectsMissingQueues(t *testing.T) {
	t.Parallel()
	_, err := New(Sources{})
	require.ErrorIs(t, err, ErrIncompleteSources)

	sources := newFakes().sources()
	sources.Excused = nil
	_, err = New(sources)
	require.ErrorIs(t, err, ErrIncompleteSources)

	query, err := New(newFakes().sources())
	require.NoError(t, err)
	require.NotNil(t, query)
}

func TestQueuesTranslateTheSharedFilterAndCursor(t *testing.T) {
	t.Parallel()
	f := newFakes()
	urgent := true
	filter := requestreview.QueueFilter{
		UrgentOnly: &urgent, UrgentDate: today.String(), StudentIDs: []int64{3, 4}, StudentID: 3, Search: "Emma",
		Before: &requestreview.Cursor{Instant: at, ID: 9}, Limit: 50,
	}
	queues := adapterQueues(f)
	rows, next, err := queues.MasterData.History(context.Background(), filter)
	require.NoError(t, err)
	assert.Empty(t, rows)
	assert.Equal(t, &requestreview.Cursor{Instant: at, ID: 1}, next)
	assert.Equal(t, modelBase.RequestQueueFilters{
		UrgentOnly: &urgent, UrgentDate: today.String(), StudentIDs: []int64{3, 4}, StudentID: 3, Search: "Emma",
		BeforeInstant: at, BeforeID: 9, Limit: 50,
	}, f.master.filters[0], "field for field the same contract")

	_, next, err = queues.Excused.Open(context.Background(), filter)
	require.NoError(t, err)
	assert.Equal(t, &requestreview.Cursor{Instant: at, ID: 7}, next)
	assert.Equal(t, excusedrequests.QueueFilter{
		UrgentOnly: &urgent, UrgentDate: today.String(), StudentIDs: []int64{3, 4}, StudentID: 3, Search: "Emma",
		BeforeInstant: at, BeforeID: 9, Limit: 50,
	}, f.excused.filters[0])
}

func adapterQueues(f fakes) requestreview.Queues {
	todayFn := func() timezone.Date { return today }
	return requestreview.Queues{
		MasterData:        masterDataQueue{service: f.master},
		CareSchedule:      careScheduleQueue{service: f.care, today: todayFn},
		Offering:          offeringQueue{service: f.offering, today: todayFn},
		Excused:           excusedQueue{service: f.excused, today: todayFn},
		DirectCorrections: correctionLog{service: f.offering},
	}
}

func TestOpenRowsCarryTheOwnerRules(t *testing.T) {
	t.Parallel()
	f := newFakes()
	created := at.Add(-2 * time.Hour)
	changed := true
	f.master.pending = []*userService.MasterDataReviewItem{{
		Request: &userModels.StudentDataChangeRequest{
			Model:     modelBase.Model{ID: 11, CreatedAt: created, UpdatedAt: created.Add(time.Minute)},
			StudentID: 1, Target: "person", FieldKey: "first_name", NewValue: json.RawMessage(`"Neu"`), Status: "pending",
		},
		FirstName: "Anna", LastName: "Adler", BulkEligible: true, CurrentValueChanged: &changed,
	}}
	f.care.pending = []*scheduleService.CareRequestReviewItem{
		{
			Request: &scheduleModels.CareScheduleChangeRequest{
				Model:     scheduleModels.Model{ID: 21, CreatedAt: created, UpdatedAt: created},
				StudentID: 2, RequestKind: "weekly_schedule", Status: "pending",
			},
			FirstName: "Ben", LastName: "Berg",
			Diff: []scheduleService.RequestDiffEntry{{Label: "Montag", Weekday: 1, CareKind: "booking"}},
		},
		{
			Request: &scheduleModels.CareScheduleChangeRequest{
				Model:     scheduleModels.Model{ID: 22, CreatedAt: created, UpdatedAt: created},
				StudentID: 2, RequestKind: "pickup_change", Status: "pending",
				Payload: map[string]any{"date": today.AddDays(-1).String()},
			},
			FirstName: "Ben", LastName: "Berg",
		},
	}
	f.offering.pending = []*enrollmentService.OfferingChangeView{{
		Request: &enrollmentModels.OfferingChangeRequest{
			ID: 31, CreatedAt: created, UpdatedAt: created, StudentID: 3, Status: "pending",
			EffectiveFrom: enrollmentModels.OfferingChangeDate(today.AddDays(-3)),
		},
		StudentName: "Cara Cruz",
		Diff:        []enrollmentService.OfferingChangeDiffEntry{{OfferingID: 5, Label: "Ganztag", OldState: "not_booked", NewState: "booked", NewDays: []string{"mon", "tue"}}},
	}}
	f.excused.pending = []*excusedrequests.ReviewItem{{
		Request: &excusedrequests.Request{
			ID: 41, CreatedAt: created, UpdatedAt: created, StudentID: 4, Status: "pending",
			Dates: []excusedrequests.Date{excusedrequests.Date(today.AddDays(-2)), excusedrequests.Date(today)},
		},
		FirstName: "Dora", LastName: "Dahl", BulkEligible: true,
		CurrentStatusByDate: map[string]string{today.String(): "present"},
	}}
	queues := adapterQueues(f)

	master, _, err := queues.MasterData.Open(context.Background(), requestreview.QueueFilter{})
	require.NoError(t, err)
	require.Len(t, master, 1)
	assert.Equal(t, requestreview.TypeMasterData, master[0].Type)
	assert.Equal(t, userService.ParentRequestVersion(created.Add(time.Minute)), master[0].Version)
	assert.Equal(t, created, master[0].SortTime)
	assert.Equal(t, "Anna Adler", master[0].StudentName)
	assert.True(t, master[0].BulkEligible)
	assert.False(t, master[0].Past, "Stammdaten have no effective scope")
	assert.Equal(t, &changed, master[0].CurrentValueChanged)
	assert.Equal(t, userService.ParentRequestConflictKeys(userService.ParentRequestConflictInput{
		RequestType: userModels.ParentRequestTypeMasterData, Target: "person", Field: "first_name",
	}), master[0].ConflictKeys)
	assert.Equal(t, ToMasterDataChangeRequestResponse(f.master.pending[0]), master[0].Data)

	care, _, err := queues.CareSchedule.Open(context.Background(), requestreview.QueueFilter{})
	require.NoError(t, err)
	require.Len(t, care, 2)
	weekly, pickup := care[0], care[1]
	assert.True(t, weekly.UrgentToday, "2026-08-24 is a Monday and the plan changes Monday")
	assert.False(t, weekly.Past, "a weekly plan applies from the decision onwards")
	assert.Equal(t, userService.BulkIneligibleSingleOnly, weekly.BulkIneligibleReason)
	assert.Equal(t, "Betreuungszeiten müssen einzeln geprüft werden.", weekly.BulkIneligibleText)
	assert.Equal(t, userService.ParentRequestConflictKeys(userService.ParentRequestConflictInput{
		RequestType: userModels.ParentRequestTypeCareSchedule, Weekdays: []int{1}, CareKind: "booking",
	}), weekly.ConflictKeys)
	assert.False(t, pickup.UrgentToday)
	assert.True(t, pickup.Past, "yesterday's pickup change is past")
	assert.Equal(t, userService.ParentRequestConflictKeys(userService.ParentRequestConflictInput{
		RequestType: userModels.ParentRequestTypePickupChange, Dates: []string{today.AddDays(-1).String()},
	}), pickup.ConflictKeys)
	assert.Equal(t, ToCareRequestResponse(f.care.pending[1]), pickup.Data)

	offering, _, err := queues.Offering.Open(context.Background(), requestreview.QueueFilter{})
	require.NoError(t, err)
	require.Len(t, offering, 1)
	assert.True(t, offering[0].UrgentToday, "an effective date that passed is urgent")
	assert.True(t, offering[0].Past)
	assert.Equal(t, "Cara Cruz", offering[0].StudentName)
	assert.Equal(t, "Angebote müssen einzeln geprüft werden.", offering[0].BulkIneligibleText)
	assert.Equal(t, userService.ParentRequestConflictKeys(userService.ParentRequestConflictInput{
		RequestType: userModels.ParentRequestTypeOffering, OfferingID: 5,
	}), offering[0].ConflictKeys)
	data, ok := offering[0].Data.(requestreview.OfferingRequestResponse)
	require.True(t, ok)
	assert.Equal(t, "Mo, Di", data.Diff[0].New)
	assert.Equal(t, "nicht gebucht", data.Diff[0].Old)

	excused, _, err := queues.Excused.Open(context.Background(), requestreview.QueueFilter{})
	require.NoError(t, err)
	require.Len(t, excused, 1)
	assert.True(t, excused[0].UrgentToday)
	assert.False(t, excused[0].Past, "the request still covers today")
	assert.True(t, excused[0].BulkEligible)
	assert.Equal(t, map[string]string{today.String(): "present"}, excused[0].CurrentStatusByDate)
	assert.Equal(t, userService.ParentRequestConflictKeys(userService.ParentRequestConflictInput{
		RequestType: userModels.ParentRequestTypeExcusedAbsence, Dates: []string{today.AddDays(-2).String(), today.String()},
	}), excused[0].ConflictKeys)
	assert.Equal(t, ToStaffExcusedRequestResponse(f.excused.pending[0]), excused[0].Data)
}

func TestHistoryRowsCarryDecisionFactsAndCorrectability(t *testing.T) {
	t.Parallel()
	f := newFakes()
	decided := at.Add(-time.Hour)
	updated := at.Add(-30 * time.Minute)
	f.master.history = []*userService.MasterDataHistoryItem{
		{Request: &userModels.StudentDataChangeRequest{Model: modelBase.Model{ID: 1, UpdatedAt: updated}, StudentID: 1, Status: "approved", ReviewedAt: &decided}, FirstName: "Anna", LastName: "Adler", ReviewerName: "Revi Ewer"},
		{Request: &userModels.StudentDataChangeRequest{Model: modelBase.Model{ID: 2, UpdatedAt: updated}, StudentID: 1, Status: "auto_applied"}, FirstName: "Anna", LastName: "Adler"},
	}
	f.care.history = []*scheduleService.CareRequestHistoryItem{
		{Request: &scheduleModels.CareScheduleChangeRequest{Model: scheduleModels.Model{ID: 3, UpdatedAt: updated}, StudentID: 2, RequestKind: "weekly_schedule", Status: "approved", ReviewedAt: &decided}, FirstName: "Ben", LastName: "Berg"},
		{Request: &scheduleModels.CareScheduleChangeRequest{Model: scheduleModels.Model{ID: 4, UpdatedAt: updated}, StudentID: 2, RequestKind: "pickup_change", Status: "rejected", ReviewedAt: &decided}, FirstName: "Ben", LastName: "Berg"},
	}
	f.offering.history = []*enrollmentService.OfferingChangeHistoryItem{
		{Request: &enrollmentModels.OfferingChangeRequest{ID: 5, UpdatedAt: updated, StudentID: 3, Status: "withdrawn"}, StudentName: "Cara Cruz"},
	}
	f.offering.corrections = []*enrollmentService.DirectCorrectionItem{
		{Adjustment: &auditModels.EnrollmentOfferingAdjustment{ID: 6, StudentID: 3, ChangedAt: decided, Reason: "Tippfehler"}, StudentName: "Cara Cruz", ActorName: "Olga Office"},
	}
	f.excused.history = []*excusedrequests.HistoryItem{
		{Request: &excusedrequests.Request{ID: 7, UpdatedAt: updated, StudentID: 4, Status: "done"}, FirstName: "Dora", LastName: "Dahl"},
	}
	queues := adapterQueues(f)

	master, _, err := queues.MasterData.History(context.Background(), requestreview.QueueFilter{})
	require.NoError(t, err)
	assert.Equal(t, decided, master[0].DecidedAt, "reviewed_at wins")
	assert.Equal(t, updated, master[0].SortTime, "the history keys on updated_at")
	assert.True(t, master[0].CanCorrect)
	assert.Equal(t, updated, master[1].DecidedAt, "auto-applied rows fall back to updated_at")
	assert.False(t, master[1].CanCorrect)
	assert.Equal(t, ToMasterDataHistoryResponse(f.master.history[0]), master[0].Data)

	care, _, err := queues.CareSchedule.History(context.Background(), requestreview.QueueFilter{})
	require.NoError(t, err)
	assert.False(t, care[0].CanCorrect, "a weekly plan keeps no pre-decision copy")
	assert.True(t, care[1].CanCorrect, "a pickup change does")

	offering, _, err := queues.Offering.History(context.Background(), requestreview.QueueFilter{})
	require.NoError(t, err)
	assert.False(t, offering[0].CanCorrect)
	assert.Equal(t, updated, offering[0].DecidedAt)

	corrections, _, err := queues.DirectCorrections.History(context.Background(), requestreview.QueueFilter{})
	require.NoError(t, err)
	assert.Equal(t, requestreview.TypeDirectCorrection, corrections[0].Type)
	assert.Equal(t, decided, corrections[0].SortTime)
	assert.Equal(t, decided, corrections[0].DecidedAt)
	assert.Empty(t, corrections[0].Status)
	assert.Equal(t, ToDirectCorrectionResponse(f.offering.corrections[0]), corrections[0].Data)

	excused, _, err := queues.Excused.History(context.Background(), requestreview.QueueFilter{})
	require.NoError(t, err)
	assert.False(t, excused[0].CanCorrect, "a request marked done was never decided")
}

func TestPendingCountsFollowTheRetainedBadge(t *testing.T) {
	t.Parallel()
	f := newFakes()
	f.master.pending = []*userService.MasterDataReviewItem{{Request: &userModels.StudentDataChangeRequest{}}, {Request: &userModels.StudentDataChangeRequest{}}}
	f.offering.count = 5
	f.excused.pending = []*excusedrequests.ReviewItem{{Request: &excusedrequests.Request{}}}
	query, err := New(f.sources())
	require.NoError(t, err)

	count, err := query.PendingCount(writeQueueCtx())
	require.NoError(t, err)
	assert.Equal(t, 8, count)
	assert.Equal(t, 1, f.offering.countCalls, "offerings use the dedicated count instead of building every diff")
	assert.Equal(t, modelBase.RequestQueueFilters{}, f.master.filters[0], "the badge counts the whole queue")
	assert.Equal(t, excusedrequests.QueueFilter{}, f.excused.filters[0])

	count, err = query.PendingCount(context.WithValue(context.Background(), jwt.CtxPermissions, []string{"users:read", "users:absence"}))
	require.NoError(t, err)
	assert.Equal(t, 1, count, "an absence-only reviewer counts the excused queue only")
}

func TestAccessReadsPermissionsFromTheRequestContext(t *testing.T) {
	t.Parallel()
	caller, err := access{}.Caller(writeQueueCtx())
	require.NoError(t, err)
	assert.True(t, caller.ReviewsWriteQueues)
	caller, err = access{}.Caller(context.Background())
	require.NoError(t, err)
	assert.False(t, caller.ReviewsWriteQueues)

	level, err := access{}.ReviewAccess(context.Background())
	require.NoError(t, err)
	assert.Empty(t, level, "an unwired policy reports nothing rather than guessing")

	level, err = access{policy: policyFake{level: "group_leader"}}.ReviewAccess(writeQueueCtx())
	require.NoError(t, err)
	assert.Equal(t, "group_leader", level)
}

type policyFake struct{ level string }

func (p policyFake) AccessLevel(_ context.Context, permissions []string) (string, error) {
	if len(permissions) == 0 {
		return "", errors.New("no permissions in context")
	}
	return p.level, nil
}

func TestOwnerQueueFailureSurfacesUnchanged(t *testing.T) {
	t.Parallel()
	f := newFakes()
	f.master.err = userService.ErrReviewForbidden
	query, err := New(f.sources())
	require.NoError(t, err)

	_, err = query.ListRequests(writeQueueCtx(), requestreview.ListQuery{})
	require.ErrorIs(t, err, userService.ErrReviewForbidden, "the shared error rules keep matching the owner sentinel")
	assert.Equal(t, userService.ErrReviewForbidden.Error(), err.Error(), "the 500 text does not change either")

	_, err = query.PendingCount(writeQueueCtx())
	require.ErrorIs(t, err, userService.ErrReviewForbidden)
}

// The two-tenant proof runs the projection over the real retained queues:
// each school's reviewer lists and counts exactly the requests of that
// school, across all four request tables, under the least-privilege role.
func TestRequestReviewEnforcesRLS(t *testing.T) {
	t.Parallel()
	db, svc := testutil.SetupStudentModule(t)
	query, err := New(Sources{
		MasterData: svc.MasterDataReview, CareSchedule: svc.CareRequests, Offering: svc.OfferingChanges,
		Excused: svc.ExcusedRequests, People: svc.Users, Education: svc.Education, FamilyProtection: svc.FamilyProtection,
	})
	require.NoError(t, err)

	type school struct {
		ctx       context.Context
		tenantID  int64
		studentID int64
		groupName string
		rows      map[string]int64
	}
	var schools []school
	for _, side := range []string{"own", "foreign"} {
		t.Run(side, func(t *testing.T) {
			testpkg.OwnTenant(t)
			ctx := testpkg.Ctx(t)
			tenantID := testpkg.Tenant(t)
			teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "ReviewRLS", side)
			group := testpkg.CreateTestEducationGroup(t, db, "ReviewRLS-"+side)
			testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)
			student := testpkg.CreateTestStudent(t, db, "ReviewRLS", side, "RR1")
			testpkg.AssignStudentToGroup(t, db, student.ID, group.ID)

			masterData := &userModels.StudentDataChangeRequest{
				StudentID: student.ID, SubmittedBy: account.ID, Target: userModels.DataChangeTargetPerson,
				FieldKey: "first_name", NewValue: json.RawMessage(`"Neu"`), Status: userModels.DataChangeStatusPending,
			}
			masterData.TenantID = tenantID
			_, err := db.NewInsert().Model(masterData).Exec(ctx)
			require.NoError(t, err)

			care := &scheduleModels.CareScheduleChangeRequest{
				StudentID: student.ID, SubmittedBy: account.ID, RequestKind: "weekly_schedule", Status: "pending",
				Payload: map[string]any{"weekdays": []any{map[string]any{"weekday": 1, "pickup": "15:00"}}},
			}
			care.TenantID = tenantID
			_, err = db.NewInsert().Model(care).Exec(ctx)
			require.NoError(t, err)

			excused := &activeModels.ExcusedAbsenceRequest{
				StudentID: student.ID, SubmittedBy: account.ID, Dates: []timezone.Date{timezone.TodayDate().AddDays(3)},
				Note: "Arzttermin", AbsenceStatus: "excused", Status: "pending",
			}
			excused.TenantID = tenantID
			_, err = db.NewInsert().Model(excused).Exec(ctx)
			require.NoError(t, err)

			offering := insertPendingOfferingRequest(t, db, ctx, tenantID, student.ID, account.ID, side)

			claims := jwt.AppClaims{ID: int(account.ID)}
			reviewer := context.WithValue(ctx, jwt.CtxClaims, claims)
			reviewer = context.WithValue(reviewer, jwt.CtxPermissions, []string{"admin:*", "users:read", "users:update", "users:absence"})
			schools = append(schools, school{
				ctx: reviewer, tenantID: tenantID, studentID: student.ID, groupName: group.Name,
				rows: map[string]int64{
					"users.student_data_change_requests":     masterData.ID,
					"schedule.care_schedule_change_requests": care.ID,
					"active.excused_absence_requests":        excused.ID,
					"enrollment.offering_change_requests":    offering,
				},
			})
		})
	}
	require.Len(t, schools, 2)

	for table, ownID := range schools[0].rows {
		t.Run(table, func(t *testing.T) {
			for _, side := range schools {
				require.NoError(t, testpkg.WithTenantTx(t, side.ctx, db, side.tenantID, func(txCtx context.Context, tx bun.Tx) error {
					var bypass bool
					require.NoError(t, tx.NewRaw("SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(txCtx, &bypass))
					require.False(t, bypass, "the tenant transaction must run under the least-privilege role")
					var ids []int64
					require.NoError(t, tx.NewSelect().Table(table).Column("id").Where("id IN (?, ?)", ownID, schools[1].rows[table]).Scan(txCtx, &ids))
					require.Equal(t, []int64{side.rows[table]}, ids, "%s leaks across the tenant boundary", table)
					return nil
				}))
			}
		})
	}

	for _, side := range schools {
		counter := testpkg.CaptureQueriesForContext(t, db)
		require.NoError(t, testpkg.WithTenantTx(t, counter.Context(side.ctx), db, side.tenantID, func(txCtx context.Context, _ bun.Tx) error {
			page, err := query.ListRequests(txCtx, requestreview.ListQuery{Search: "ReviewRLS"})
			require.NoError(t, err)
			testpkg.AssertQueryBudget(t, "modules.requestreview.open_page", counter.Queries())
			require.Len(t, page.Items, 4, "each school lists exactly its own four requests")
			types := make([]string, 0, 4)
			for _, item := range page.Items {
				types = append(types, item.RequestType)
				assert.Equal(t, fmt.Sprint(side.studentID), item.StudentID)
				assert.Equal(t, side.groupName, item.GroupName)
			}
			assert.ElementsMatch(t, []string{requestreview.TypeMasterData, requestreview.TypeCareSchedule, requestreview.TypeOffering, requestreview.TypeExcused}, types)
			assert.Empty(t, page.ReviewAccess, "no review policy is wired in this fixture")

			count, err := query.PendingCount(txCtx)
			require.NoError(t, err)
			assert.Equal(t, 4, count)

			history, err := query.ListRequests(txCtx, requestreview.ListQuery{History: true, Search: "ReviewRLS"})
			require.NoError(t, err)
			assert.Empty(t, history.Items)
			return nil
		}))
	}
}

// insertPendingOfferingRequest stores a parent's offering switch the way the
// parents portal does: an approved enrollment child with two offerings and
// one pending change request for the child.
func insertPendingOfferingRequest(t *testing.T, db *bun.DB, ctx context.Context, tenantID, studentID, submittedBy int64, side string) int64 {
	t.Helper()
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	ganztag := testpkg.CreateTestCareOffering(t, db, phase.ID, "Ganztag "+side)
	mittag := testpkg.CreateTestCareOffering(t, db, phase.ID, "Mittagessen "+side)

	request := &enrollmentModule.Request{
		PhaseID: phase.ID, GuardianFirstName: "Erzieh", GuardianLastName: side,
		GuardianEmail: fmt.Sprintf("review-rls-%s-%d@example.test", side, testpkg.UniqueSuffix()),
		StatusToken:   fmt.Sprintf("tok-%d", testpkg.UniqueSuffix()),
	}
	request.TenantID = tenantID
	owner := testutil.NewEnrollmentOwner()
	ownerCtx := testpkg.WithTenantRuntime(t, ctx, db)
	require.NoError(t, owner.InsertRequest(ownerCtx, request))
	child := &enrollmentModule.RequestChild{
		RequestID: request.ID, FirstName: "ReviewRLS", LastName: side,
		DateOfBirth: enrollmentModule.Date(today.AddDays(-2500)), Status: enrollmentModels.ChildStatusApproved,
		CreatedStudentID: &studentID,
	}
	child.TenantID = tenantID
	require.NoError(t, owner.InsertChild(ownerCtx, child))
	for _, offering := range []*enrollmentModels.CareOffering{ganztag, mittag} {
		link := &enrollmentModule.RequestChildOffering{RequestChildID: child.ID, CareOfferingID: offering.ID}
		link.TenantID = tenantID
		require.NoError(t, owner.InsertRequestChildOffering(ownerCtx, link))
	}

	payload, err := json.Marshal(map[string]any{"offerings": []any{map[string]any{"offering_id": ganztag.ID}}})
	require.NoError(t, err)
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.offering_change_requests
		(tenant_id, student_id, request_child_id, submitted_by, payload, effective_from, status)
		VALUES (?, ?, ?, ?, ?::jsonb, ?, ?) RETURNING id`,
		tenantID, studentID, child.ID, submittedBy, string(payload),
		enrollmentModels.OfferingChangeDate(timezone.TodayDate().AddDays(30)), enrollmentModels.OfferingChangeStatusPending,
	).Scan(ctx, &id))
	return id
}
