package compose

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
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const today careplan.Date = "2026-08-24"

var at = time.Date(2026, 8, 24, 7, 0, 0, 0, time.UTC)

// The fakes embed their retained service interface so only the queue methods
// the adapter touches need implementations; anything else panics on a nil
// interface, which is exactly the failure we want in a test.

type masterFake struct {
	pending []*masterdatarequests.ReviewItem
	history []*masterdatarequests.HistoryItem
	err     error
	filters []excusedrequests.QueueFilter
}

func (f *masterFake) ListPending(_ context.Context, filters excusedrequests.QueueFilter) ([]*masterdatarequests.ReviewItem, *excusedrequests.Cursor, error) {
	f.filters = append(f.filters, filters)
	return f.pending, nil, f.err
}

func (f *masterFake) ListHistory(_ context.Context, filters excusedrequests.QueueFilter) ([]*masterdatarequests.HistoryItem, *excusedrequests.Cursor, error) {
	f.filters = append(f.filters, filters)
	return f.history, &excusedrequests.Cursor{UpdatedAt: at, ID: 1}, f.err
}

type careFake struct {
	pending []*careplan.CareScheduleReviewItem
	history []*careplan.CareScheduleHistoryItem
}

func (f *careFake) ListPending(context.Context, careplan.RequestQueueFilter) ([]*careplan.CareScheduleReviewItem, *careplan.RequestCursor, error) {
	return f.pending, nil, nil
}

func (f *careFake) ListHistory(context.Context, careplan.RequestQueueFilter) ([]*careplan.CareScheduleHistoryItem, *careplan.RequestCursor, error) {
	return f.history, nil, nil
}

type offeringFake struct {
	pending     []*careplan.OfferingReviewItem
	history     []*careplan.OfferingHistoryItem
	corrections []requestreview.Row
	count       int
	countCalls  int
}

func (f *offeringFake) ListPending(context.Context, careplan.RequestQueueFilter) ([]*careplan.OfferingReviewItem, *careplan.RequestCursor, error) {
	return f.pending, nil, nil
}

func (f *offeringFake) ListHistory(context.Context, careplan.RequestQueueFilter) ([]*careplan.OfferingHistoryItem, *careplan.RequestCursor, error) {
	return f.history, nil, nil
}

func (f *offeringFake) History(context.Context, requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	return f.corrections, nil, nil
}

func (f *offeringFake) PendingCount(context.Context, careplan.Date) (int, error) {
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

func (f fakes) sources() requestreview.Dependencies {
	return requestreview.Dependencies{Queues: adapterQueues(f), Access: queueTestAccess{}, Today: func() requestreview.Date { return requestreview.Date(today) }}
}

func writeQueueCtx() context.Context {
	return context.WithValue(context.Background(), queueWriteKey{}, true)
}

func TestNewRejectsMissingQueues(t *testing.T) {
	t.Parallel()
	_, err := requestreview.NewChecked(requestreview.Dependencies{})
	require.ErrorIs(t, err, requestreview.ErrNotConfigured)

	sources := newFakes().sources()
	sources.Queues.Excused = nil
	_, err = requestreview.NewChecked(sources)
	require.ErrorIs(t, err, requestreview.ErrNotConfigured)

	query, err := requestreview.NewChecked(newFakes().sources())
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
	assert.Equal(t, excusedrequests.QueueFilter{
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
	todayFn := func() careplan.Date { return today }
	return requestreview.Queues{
		MasterData:        nativeMasterQueue(f.master),
		CareSchedule:      nativeCareQueue(f.care, func() careplan.Date { return careplan.Date(todayFn()) }),
		Offering:          nativeOfferingQueue(f.offering, func() careplan.Date { return careplan.Date(todayFn()) }),
		Excused:           nativeExcusedQueue(f.excused, todayFn),
		DirectCorrections: f.offering,
	}
}

func TestOpenRowsCarryTheOwnerRules(t *testing.T) {
	t.Parallel()
	f := newFakes()
	created := at.Add(-2 * time.Hour)
	changed := true
	f.master.pending = []*masterdatarequests.ReviewItem{{
		Request: &masterdatarequests.Request{ID: 11, CreatedAt: created, UpdatedAt: created.Add(time.Minute),
			StudentID: 1, Target: "person", FieldKey: "first_name", NewValue: json.RawMessage(`"Neu"`), Status: "pending",
		},
		FirstName: "Anna", LastName: "Adler", BulkEligible: true, CurrentValueChanged: &changed,
	}}
	f.care.pending = []*careplan.CareScheduleReviewItem{
		{
			Request: &careplan.CareScheduleChangeRequest{
				ID: 21, CreatedAt: created, UpdatedAt: created,
				StudentID: 2, RequestKind: "weekly_schedule", Status: "pending",
			},
			FirstName: "Ben", LastName: "Berg",
			Diff: []careplan.CareRequestDiffEntry{{Label: "Montag", Weekday: 1, CareKind: "booking"}},
		},
		{
			Request: &careplan.CareScheduleChangeRequest{
				ID: 22, CreatedAt: created, UpdatedAt: created,
				StudentID: 2, RequestKind: "pickup_change", Status: "pending",
				Payload: json.RawMessage(`{"date":"` + today.AddDays(-1).String() + `"}`),
			},
			FirstName: "Ben", LastName: "Berg",
		},
	}
	f.offering.pending = []*careplan.OfferingReviewItem{{
		Request: &careplan.OfferingChangeRequest{
			ID: 31, CreatedAt: created, UpdatedAt: created, StudentID: 3, Status: "pending",
			EffectiveFrom: today.AddDays(-3).String(),
		},
		StudentName: "Cara Cruz",
		Diff:        []careplan.OfferingReviewDiffEntry{{OfferingID: 5, Label: "Ganztag", OldState: "not_booked", NewState: "booked", NewDays: []string{"mon", "tue"}}},
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
	assert.Equal(t, excusedrequests.ParentRequestVersion(created.Add(time.Minute)), master[0].Version)
	assert.Equal(t, created, master[0].SortTime)
	assert.Equal(t, "Anna Adler", master[0].StudentName)
	assert.True(t, master[0].BulkEligible)
	assert.False(t, master[0].Past, "Stammdaten have no effective scope")
	assert.Equal(t, &changed, master[0].CurrentValueChanged)
	assert.Equal(t, []string{"md:person:first_name"}, master[0].ConflictKeys)
	assert.Equal(t, ToMasterDataChangeRequestResponse(f.master.pending[0]), master[0].Data)

	care, _, err := queues.CareSchedule.Open(context.Background(), requestreview.QueueFilter{})
	require.NoError(t, err)
	require.Len(t, care, 2)
	weekly, pickup := care[0], care[1]
	assert.True(t, weekly.UrgentToday, "2026-08-24 is a Monday and the plan changes Monday")
	assert.False(t, weekly.Past, "a weekly plan applies from the decision onwards")
	assert.Equal(t, "single_only", weekly.BulkIneligibleReason)
	assert.Equal(t, "Betreuungszeiten müssen einzeln geprüft werden.", weekly.BulkIneligibleText)
	assert.Equal(t, []string{"care:1:booking"}, weekly.ConflictKeys)
	assert.False(t, pickup.UrgentToday)
	assert.True(t, pickup.Past, "yesterday's pickup change is past")
	assert.Equal(t, []string{"pickup:" + today.AddDays(-1).String()}, pickup.ConflictKeys)
	assert.Equal(t, ToCareRequestResponse(f.care.pending[1]), pickup.Data)

	offering, _, err := queues.Offering.Open(context.Background(), requestreview.QueueFilter{})
	require.NoError(t, err)
	require.Len(t, offering, 1)
	assert.True(t, offering[0].UrgentToday, "an effective date that passed is urgent")
	assert.True(t, offering[0].Past)
	assert.Equal(t, "Cara Cruz", offering[0].StudentName)
	assert.Equal(t, "Angebote müssen einzeln geprüft werden.", offering[0].BulkIneligibleText)
	assert.Equal(t, []string{"offer:5"}, offering[0].ConflictKeys)
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
	assert.Equal(t, []string{"absence:" + today.AddDays(-2).String(), "absence:" + today.String()}, excused[0].ConflictKeys)
	assert.Equal(t, ToStaffExcusedRequestResponse(f.excused.pending[0]), excused[0].Data)
}

func TestHistoryRowsCarryDecisionFactsAndCorrectability(t *testing.T) {
	t.Parallel()
	f := newFakes()
	decided := at.Add(-time.Hour)
	updated := at.Add(-30 * time.Minute)
	f.master.history = []*masterdatarequests.HistoryItem{
		{Request: &masterdatarequests.Request{ID: 1, UpdatedAt: updated, StudentID: 1, Status: "approved", ReviewedAt: &decided}, FirstName: "Anna", LastName: "Adler", ReviewerName: "Revi Ewer"},
		{Request: &masterdatarequests.Request{ID: 2, UpdatedAt: updated, StudentID: 1, Status: "auto_applied"}, FirstName: "Anna", LastName: "Adler"},
	}
	f.care.history = []*careplan.CareScheduleHistoryItem{
		{Request: &careplan.CareScheduleChangeRequest{ID: 3, UpdatedAt: updated, StudentID: 2, RequestKind: "weekly_schedule", Status: "approved", ReviewedAt: &decided}, FirstName: "Ben", LastName: "Berg"},
		{Request: &careplan.CareScheduleChangeRequest{ID: 4, UpdatedAt: updated, StudentID: 2, RequestKind: "pickup_change", Status: "rejected", ReviewedAt: &decided}, FirstName: "Ben", LastName: "Berg"},
	}
	f.offering.history = []*careplan.OfferingHistoryItem{
		{Request: &careplan.OfferingChangeRequest{ID: 5, UpdatedAt: updated, StudentID: 3, Status: "withdrawn"}, StudentName: "Cara Cruz"},
	}
	f.offering.corrections = []requestreview.Row{{Type: requestreview.TypeDirectCorrection, ID: 6, StudentID: 3, StudentName: "Cara Cruz", SortTime: decided, DecidedAt: decided,
		Data: requestreview.DirectCorrectionResponse{ID: "6", StudentID: "3", StudentName: "Cara Cruz", ChangedAt: decided, Reason: "Tippfehler", ChangedByName: "Olga Office", Diff: []requestreview.OfferingRequestDiffResponse{}},
	}}
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
	assert.Equal(t, f.offering.corrections[0].Data, corrections[0].Data)

	excused, _, err := queues.Excused.History(context.Background(), requestreview.QueueFilter{})
	require.NoError(t, err)
	assert.False(t, excused[0].CanCorrect, "a request marked done was never decided")
}

func TestPendingCountsFollowTheRetainedBadge(t *testing.T) {
	t.Parallel()
	f := newFakes()
	f.master.pending = []*masterdatarequests.ReviewItem{{Request: &masterdatarequests.Request{}}, {Request: &masterdatarequests.Request{}}}
	f.offering.count = 5
	f.excused.pending = []*excusedrequests.ReviewItem{{Request: &excusedrequests.Request{}}}
	query, err := requestreview.NewChecked(f.sources())
	require.NoError(t, err)

	count, err := query.PendingCount(writeQueueCtx())
	require.NoError(t, err)
	assert.Equal(t, 8, count)
	assert.Equal(t, 1, f.offering.countCalls, "offerings use the dedicated count instead of building every diff")
	assert.Equal(t, excusedrequests.QueueFilter{UrgentDate: today.String()}, f.master.filters[0], "the badge counts the whole queue on the shared day")
	assert.Equal(t, excusedrequests.QueueFilter{UrgentDate: today.String()}, f.excused.filters[0])

	count, err = query.PendingCount(context.WithValue(context.Background(), queueWriteKey{}, false))
	require.NoError(t, err)
	assert.Equal(t, 1, count, "an absence-only reviewer counts the excused queue only")
}

// Queue tests supply identity facts; effective permission matching is tested
// at the inbound request identity boundary.
type queueTestAccess struct{}
type queueWriteKey struct{}

func (queueTestAccess) Caller(ctx context.Context) (requestreview.Caller, error) {
	write, _ := ctx.Value(queueWriteKey{}).(bool)
	return requestreview.Caller{ReviewsWriteQueues: write}, nil
}

func (queueTestAccess) ReviewAccess(context.Context) (string, error) { return "", nil }

func TestOwnerQueueFailureSurfacesUnchanged(t *testing.T) {
	t.Parallel()
	f := newFakes()
	ownerFailure := errors.New("users: change request forbidden")
	f.master.err = ownerFailure
	query, err := requestreview.NewChecked(f.sources())
	require.NoError(t, err)

	_, err = query.ListRequests(writeQueueCtx(), requestreview.ListQuery{})
	require.ErrorIs(t, err, ownerFailure, "the shared error rules keep matching the owner sentinel")
	assert.Equal(t, ownerFailure.Error(), err.Error(), "the 500 text does not change either")

	_, err = query.PendingCount(writeQueueCtx())
	require.ErrorIs(t, err, ownerFailure)
}

// The two-tenant proof runs the projection over the real retained queues:
// each school's reviewer lists and counts exactly the requests of that
// school, across all four request tables, under the least-privilege role.
func TestRequestReviewEnforcesRLS(t *testing.T) {
	t.Parallel()
	db, svc := testutil.SetupStudentModule(t)
	reviewStudents, err := NewStudentDirectory(db, svc.PeopleDirectory, func(DirectoryObservation) {})
	require.NoError(t, err)
	masterData, err := NewMasterDataReviews(db, svc.PeopleDirectory,
		func(context.Context) (ReviewScope, error) {
			return ReviewScope{SchoolWide: true}, nil
		},
		func() ReviewDate { return ReviewDate(reviewToday()) },
		func(CareObservation) {})
	require.NoError(t, err)
	careReviews, err := NewScheduleReviews(db, ScheduleReviewDependencies{
		People: svc.PeopleDirectory,
		Scope: func(context.Context) (ReviewScope, error) {
			return ReviewScope{SchoolWide: true}, nil
		},
		BookingsAuthoritative: func(ctx context.Context) (bool, error) {
			return svc.Settings.ResolveBool(ctx, "enrollment.bookings_authoritative")
		},
		Today:       func() ReviewDate { return ReviewDate(reviewToday()) },
		ObserveCare: func(CareObservation) {}, ObserveTimetable: func(TimetableObservation) {},
	})
	require.NoError(t, err)
	offeringReviews, err := NewOfferingReviews(db, OfferingReviewDependencies{
		People: svc.PeopleDirectory,
		Scope: func(context.Context) (ReviewScope, error) {
			return ReviewScope{SchoolWide: true}, nil
		},
		Today:       func() ReviewDate { return ReviewDate(reviewToday()) },
		ObserveCare: func(CareObservation) {}, ObserveTimetable: func(TimetableObservation) {},
	})
	require.NoError(t, err)
	corrections, err := NewCorrectionLog(db, svc.PeopleDirectory, func(context.Context) bool { return true }, func(AuditObservation) {})
	require.NoError(t, err)
	query, err := requestreview.NewChecked(requestreview.Dependencies{
		Queues: requestreview.Queues{DirectCorrections: corrections, MasterData: nativeMasterQueue(masterData),
			CareSchedule: nativeCareQueue(careReviews, reviewToday), Offering: nativeOfferingQueue(offeringReviews, reviewToday), Excused: nativeExcusedQueue(svc.ExcusedRequests, reviewToday)},
		Access: queueTestAccess{}, Students: reviewStudents, FamilyProtection: NewFamilyProtection(svc.PeopleDirectory),
	})
	require.NoError(t, err)

	var schools []testpkg.RequestReviewSchool
	for _, side := range []string{"own", "foreign"} {
		t.Run(side, func(t *testing.T) {
			testpkg.OwnTenant(t)
			school := testpkg.CreateTestRequestReviewSchool(t, db, side)
			school.Context = context.WithValue(school.Context, queueWriteKey{}, true)
			schools = append(schools, school)
		})
	}
	require.Len(t, schools, 2)

	for table, ownID := range schools[0].Rows {
		t.Run(table, func(t *testing.T) {
			for _, side := range schools {
				require.NoError(t, testpkg.WithTenantTx(t, side.Context, db, side.TenantID, func(txCtx context.Context, tx bun.Tx) error {
					var bypass bool
					require.NoError(t, tx.NewRaw("SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(txCtx, &bypass))
					require.False(t, bypass, "the tenant transaction must run under the least-privilege role")
					var ids []int64
					require.NoError(t, tx.NewSelect().Table(table).Column("id").Where("id IN (?, ?)", ownID, schools[1].Rows[table]).Scan(txCtx, &ids))
					require.Equal(t, []int64{side.Rows[table]}, ids, "%s leaks across the tenant boundary", table)
					return nil
				}))
			}
		})
	}

	for _, side := range schools {
		counter := testpkg.CaptureQueriesForContext(t, db)
		require.NoError(t, testpkg.WithTenantTx(t, counter.Context(side.Context), db, side.TenantID, func(txCtx context.Context, _ bun.Tx) error {
			page, err := query.ListRequests(txCtx, requestreview.ListQuery{Search: "ReviewRLS"})
			require.NoError(t, err)
			testpkg.AssertQueryBudget(t, "modules.requestreview.open_page", counter.Queries())
			require.Len(t, page.Items, 4, "each school lists exactly its own four requests")
			types := make([]string, 0, 4)
			for _, item := range page.Items {
				types = append(types, item.RequestType)
				assert.Equal(t, fmt.Sprint(side.StudentID), item.StudentID)
				assert.Equal(t, side.GroupName, item.GroupName)
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

func nativeCareQueue(query careplan.CareScheduleReviewQuery, today func() careplan.Date) requestreview.Queue {
	queue, err := NewCareScheduleQueue(query, today)
	if err != nil {
		panic(err)
	}
	return queue
}

func nativeOfferingQueue(query careplan.OfferingReviewQuery, today func() careplan.Date) requestreview.Queue {
	queue, err := NewOfferingQueue(query, today)
	if err != nil {
		panic(err)
	}
	return queue
}

func nativeMasterQueue(query careplan.MasterDataReviewQuery) requestreview.Queue {
	queue, err := NewMasterDataQueue(query)
	if err != nil {
		panic(err)
	}
	return queue
}
func nativeExcusedQueue(query excusedrequests.ReviewQuery, today func() careplan.Date) requestreview.Queue {
	queue, err := NewExcusedQueue(query, today)
	if err != nil {
		panic(err)
	}
	return queue
}
func reviewToday() careplan.Date {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic(err)
	}
	return careplan.Date(time.Now().In(location).Format(careplan.DateLayout))
}
