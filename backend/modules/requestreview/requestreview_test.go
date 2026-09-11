package requestreview

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var base = time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

// fakeQueue mimics the owner queues' contract: rows sorted newest-first,
// page strictly before the keyset position, the child filters applied, a
// limit+1 probe for the next cursor. A fake that ignored any of those would
// make the cursor and filter assertions below meaningless.
type fakeQueue struct {
	open      []Row
	history   []Row
	err       error
	openCalls []QueueFilter
	histCalls []QueueFilter
	count     int
	countErr  error
}

func keyset(rows []Row, filter QueueFilter) ([]Row, *Cursor) {
	matched := make([]Row, 0, len(rows))
	for _, row := range rows {
		if filter.Before != nil && (row.SortTime.After(filter.Before.Instant) ||
			(row.SortTime.Equal(filter.Before.Instant) && row.ID >= filter.Before.ID)) {
			continue
		}
		if filter.UrgentOnly != nil && row.UrgentToday != *filter.UrgentOnly {
			continue
		}
		if filter.StudentID != 0 && row.StudentID != filter.StudentID {
			continue
		}
		if len(filter.StudentIDs) > 0 {
			found := false
			for _, id := range filter.StudentIDs {
				found = found || id == row.StudentID
			}
			if !found {
				continue
			}
		}
		if filter.Search != "" && !strings.Contains(strings.ToLower(row.StudentName), strings.ToLower(filter.Search)) {
			continue
		}
		matched = append(matched, row)
	}
	if filter.Limit <= 0 || len(matched) <= filter.Limit {
		return matched, nil
	}
	matched = matched[:filter.Limit]
	last := matched[len(matched)-1]
	return matched, &Cursor{Instant: last.SortTime, ID: last.ID}
}

func (q *fakeQueue) Open(_ context.Context, filter QueueFilter) ([]Row, *Cursor, error) {
	q.openCalls = append(q.openCalls, filter)
	if q.err != nil {
		return nil, nil, q.err
	}
	rows, next := keyset(q.open, filter)
	return rows, next, nil
}

func (q *fakeQueue) History(_ context.Context, filter QueueFilter) ([]Row, *Cursor, error) {
	q.histCalls = append(q.histCalls, filter)
	if q.err != nil {
		return nil, nil, q.err
	}
	rows, next := keyset(q.history, filter)
	return rows, next, nil
}

func (q *fakeQueue) OpenCount(context.Context) (int, error) { return q.count, q.countErr }

type fakeAccess struct {
	caller    Caller
	callerErr error
	access    string
	accessErr error
}

func (a fakeAccess) Caller(context.Context) (Caller, error)       { return a.caller, a.callerErr }
func (a fakeAccess) ReviewAccess(context.Context) (string, error) { return a.access, a.accessErr }

type fakeStudents struct {
	names map[int64]string
	err   error
	got   []int64
}

func (s *fakeStudents) GroupNames(_ context.Context, ids []int64) (map[int64]string, error) {
	s.got = ids
	return s.names, s.err
}

type fakeProtection struct {
	protected map[int64]bool
	err       error
}

func (p fakeProtection) Protected(context.Context, []int64) (map[int64]bool, error) {
	return p.protected, p.err
}

type fixture struct {
	master, care, offering, excused, corrections *fakeQueue
	access                                       *fakeAccess
	students                                     *fakeStudents
	protection                                   *fakeProtection
}

func newFixture() *fixture {
	return &fixture{
		master: &fakeQueue{}, care: &fakeQueue{}, offering: &fakeQueue{}, excused: &fakeQueue{}, corrections: &fakeQueue{},
		access: &fakeAccess{caller: Caller{ReviewsWriteQueues: true}, access: "admin"},
	}
}

func (f *fixture) query() Query {
	deps := Dependencies{
		Queues: Queues{
			MasterData: f.master, CareSchedule: f.care, Offering: f.offering, Excused: f.excused,
			DirectCorrections: f.corrections,
		},
		Access: f.access,
	}
	if f.students != nil {
		deps.Students = f.students
	}
	if f.protection != nil {
		deps.FamilyProtection = f.protection
	}
	return New(deps)
}

func openRow(typ string, id int64, studentID int64, name string, at time.Time) Row {
	return Row{
		Type: typ, ID: id, StudentID: studentID, StudentName: name, SortTime: at, Status: "pending",
		Version: at.UTC().Format(time.RFC3339Nano), Data: map[string]any{"id": id},
	}
}

func historyRow(typ string, id int64, studentID int64, name string, decidedAt time.Time, status string) Row {
	return Row{
		Type: typ, ID: id, StudentID: studentID, StudentName: name, SortTime: decidedAt,
		DecidedAt: decidedAt, Status: status, CanCorrect: status == "approved" || status == "rejected",
		Data: map[string]any{"id": id},
	}
}

func parse(t *testing.T, raw string) ListQuery {
	t.Helper()
	values, err := url.ParseQuery(raw)
	require.NoError(t, err)
	q, err := ParseListQuery(values)
	require.NoError(t, err)
	return q
}

func itemIDs(page Page) []string {
	ids := make([]string, 0, len(page.Items))
	for _, item := range page.Items {
		data := item.Data.(map[string]any)
		ids = append(ids, item.RequestType+":"+jsonNumber(data["id"]))
	}
	return ids
}

func jsonNumber(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func TestNewRequiresQueuesAndAccess(t *testing.T) {
	t.Parallel()
	_, err := New(Dependencies{}).ListRequests(context.Background(), ListQuery{})
	require.ErrorIs(t, err, ErrNotConfigured)
	_, err = New(Dependencies{}).PendingCount(context.Background())
	require.ErrorIs(t, err, ErrNotConfigured)

	f := newFixture()
	_, err = New(Dependencies{
		Queues: Queues{MasterData: f.master, CareSchedule: f.care, Offering: f.offering, Excused: f.excused},
		Access: f.access,
	}).ListRequests(context.Background(), ListQuery{})
	require.ErrorIs(t, err, ErrNotConfigured, "the correction log is part of the wiring")
}

func TestParseListQueryValidatesTheWire(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		"view=archive", "types=unknown", "status=approved", "from=2026-08-01", "to=2026-08-01",
		"view=history&status=pending", "view=history&from=01.08.2026", "view=history&from=2026-08-02&to=2026-08-01",
		"limit=0", "limit=x", "student_id=0", "student_id=abc", "cursor=%21%21%21", "cursor=bm90LWpzb24",
	} {
		values, err := url.ParseQuery(raw)
		require.NoError(t, err, raw)
		_, err = ParseListQuery(values)
		assert.ErrorIs(t, err, ErrInvalidQuery, raw)
	}

	q := parse(t, "types=excused,master_data,excused&limit=500&search=%20Emma%20")
	assert.Equal(t, []string{TypeMasterData, TypeExcused}, q.Types, "canonical order, deduplicated")
	assert.Equal(t, MaxLimit, q.Limit)
	assert.Equal(t, "Emma", q.Search)
	assert.False(t, q.History)

	open := parse(t, "types=direct_correction,offering")
	assert.Equal(t, []string{TypeOffering}, open.Types, "corrections have no open state")

	history := parse(t, "view=history&status=approved,withdrawn&from=2026-08-01&to=2026-08-19&student_id=42")
	assert.True(t, history.History)
	assert.Equal(t, map[string]struct{}{"approved": {}, "withdrawn": {}}, history.Statuses)
	assert.Equal(t, int64(42), history.StudentID)
	assert.Equal(t, "2026-08-01T00:00:00+02:00", history.From.Format(time.RFC3339))
	assert.Equal(t, "2026-08-19T23:59:59+02:00", history.To.Format(time.RFC3339))
	assert.Equal(t, TypeOrder, history.Types)

	defaults := parse(t, "")
	assert.Equal(t, DefaultLimit, defaults.Limit)
	assert.Equal(t, []string{TypeMasterData, TypeCareSchedule, TypeOffering, TypeExcused}, defaults.Types)
}

func TestListMergesQueuesNewestFirstWithDeterministicTieBreak(t *testing.T) {
	t.Parallel()
	f := newFixture()
	f.master.open = []Row{openRow(TypeMasterData, 3, 1, "Anna Adler", base.Add(-time.Minute)), openRow(TypeMasterData, 1, 1, "Anna Adler", base.Add(-3*time.Minute))}
	f.care.open = []Row{openRow(TypeCareSchedule, 7, 2, "Ben Berg", base.Add(-time.Minute))}
	f.offering.open = []Row{openRow(TypeOffering, 9, 3, "Cara Cruz", base)}
	f.excused.open = []Row{openRow(TypeExcused, 5, 4, "Dora Dahl", base.Add(-time.Minute)), openRow(TypeExcused, 4, 4, "Dora Dahl", base.Add(-time.Minute))}

	page, err := f.query().ListRequests(context.Background(), parse(t, ""))
	require.NoError(t, err)
	assert.Equal(t, []string{"offering:9", "master_data:3", "care_schedule:7", "excused:5", "excused:4", "master_data:1"}, itemIDs(page))
	assert.Empty(t, page.NextCursor)
	assert.Equal(t, "admin", page.ReviewAccess)
	assert.Equal(t, "1", page.Items[1].StudentID)
	assert.Equal(t, "Anna Adler", page.Items[1].StudentName)
	assert.Equal(t, base.Add(-time.Minute), page.Items[1].OccurredAt)
	assert.Equal(t, base.Add(-time.Minute).UTC().Format(time.RFC3339Nano), page.Items[1].ExpectedVersion)
	assert.Nil(t, page.Items[0].CurrentValueChanged)
	assert.False(t, page.Items[0].FamilyProtected)
	assert.Empty(t, page.Items[0].GroupName, "no directory port, no group")
}

// Corrections have no open state, so an open view asking only for them is
// empty. The query is validated again inside ListRequests; that second pass
// must not widen the emptied type set back to every queue.
func TestListOpenViewOfOnlyCorrectionsIsEmpty(t *testing.T) {
	t.Parallel()
	f := newFixture()
	f.master.open = []Row{openRow(TypeMasterData, 1, 1, "Anna Adler", base)}
	f.care.open = []Row{openRow(TypeCareSchedule, 2, 2, "Ben Berg", base)}
	f.offering.open = []Row{openRow(TypeOffering, 3, 3, "Cara Cruz", base)}
	f.excused.open = []Row{openRow(TypeExcused, 4, 4, "Dora Dahl", base)}

	page, err := f.query().ListRequests(context.Background(), parse(t, "view=open&types=direct_correction"))
	require.NoError(t, err)
	assert.Empty(t, page.Items)
	assert.Empty(t, page.NextCursor)
}

func TestListPagesAcrossQueuesWithOneCursor(t *testing.T) {
	t.Parallel()
	f := newFixture()
	for i := range 6 {
		f.master.open = append(f.master.open, openRow(TypeMasterData, int64(10+i), 1, "Anna", base.Add(-time.Duration(2*i)*time.Minute)))
		f.excused.open = append(f.excused.open, openRow(TypeExcused, int64(20+i), 2, "Ben", base.Add(-time.Duration(2*i+1)*time.Minute)))
	}
	q := f.query()
	var seen []string
	cursor := ""
	for range 10 {
		page, err := q.ListRequests(context.Background(), parse(t, "limit=5&cursor="+cursor))
		require.NoError(t, err)
		seen = append(seen, itemIDs(page)...)
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	assert.Equal(t, []string{
		"master_data:10", "excused:20", "master_data:11", "excused:21", "master_data:12", "excused:22",
		"master_data:13", "excused:23", "master_data:14", "excused:24", "master_data:15", "excused:25",
	}, seen, "every row exactly once, in order, across pages")
}

func TestListServesUrgentRequestsBeforeTheRest(t *testing.T) {
	t.Parallel()
	f := newFixture()
	urgent := openRow(TypeExcused, 2, 1, "Anna", base.Add(-time.Hour))
	urgent.UrgentToday = true
	f.excused.open = []Row{openRow(TypeExcused, 3, 1, "Anna", base), urgent, openRow(TypeExcused, 1, 1, "Anna", base.Add(-2*time.Hour))}

	page, err := f.query().ListRequests(context.Background(), parse(t, "limit=2"))
	require.NoError(t, err)
	assert.Equal(t, []string{"excused:2", "excused:3"}, itemIDs(page), "the older urgent request leads")
	assert.True(t, page.Items[0].UrgentToday)
	require.NotEmpty(t, page.NextCursor)

	rest, err := f.query().ListRequests(context.Background(), parse(t, "limit=2&cursor="+page.NextCursor))
	require.NoError(t, err)
	assert.Equal(t, []string{"excused:1"}, itemIDs(rest))
	assert.Empty(t, rest.NextCursor)
}

func TestListJudgesUrgencyAgainstTheInjectedDay(t *testing.T) {
	t.Parallel()
	f := newFixture()
	f.excused.open = []Row{openRow(TypeExcused, 1, 1, "Anna", base)}
	deps := Dependencies{
		Queues: Queues{MasterData: f.master, CareSchedule: f.care, Offering: f.offering, Excused: f.excused, DirectCorrections: f.corrections},
		Access: f.access,
		Today:  func() Date { return "2026-08-24" },
	}
	_, err := New(deps).ListRequests(context.Background(), parse(t, ""))
	require.NoError(t, err)
	phases := 0
	for _, call := range f.excused.openCalls {
		if call.UrgentOnly == nil {
			assert.Empty(t, call.UrgentDate, "the conflict scan spans both phases and judges no urgency")
			continue
		}
		phases++
		assert.Equal(t, "2026-08-24", call.UrgentDate, "both phases judge urgency against the same injected day")
	}
	assert.Equal(t, 2, phases, "the urgent phase and the normal phase both ran")

	// Without an injected clock the projection falls back to the wall clock,
	// and both phases still agree on one day.
	f = newFixture()
	f.excused.open = []Row{openRow(TypeExcused, 1, 1, "Anna", base)}
	_, err = f.query().ListRequests(context.Background(), parse(t, ""))
	require.NoError(t, err)
	var fallback string
	for _, call := range f.excused.openCalls {
		if call.UrgentOnly == nil {
			continue
		}
		assert.Regexp(t, `^\d{4}-\d{2}-\d{2}$`, call.UrgentDate)
		if fallback == "" {
			fallback = call.UrgentDate
		}
		assert.Equal(t, fallback, call.UrgentDate, "both phases share one day")
	}
	assert.NotEmpty(t, fallback)
}

func TestListNarrowsCallersWithoutTheWriteQueuesToExcused(t *testing.T) {
	t.Parallel()
	f := newFixture()
	f.access.caller = Caller{ReviewsWriteQueues: false}
	f.master.open = []Row{openRow(TypeMasterData, 1, 1, "Anna", base)}
	f.excused.open = []Row{openRow(TypeExcused, 2, 1, "Anna", base.Add(-time.Minute))}

	page, err := f.query().ListRequests(context.Background(), parse(t, ""))
	require.NoError(t, err)
	assert.Equal(t, []string{"excused:2"}, itemIDs(page))
	assert.Empty(t, f.master.openCalls, "a queue the caller may not open is never queried, not even for the conflict scan")
	assert.Empty(t, f.care.openCalls)
	assert.Empty(t, f.offering.openCalls)

	count, err := f.query().PendingCount(context.Background())
	require.NoError(t, err)
	assert.Zero(t, count)
}

func TestListAppliesTypeStudentAndSearchFilters(t *testing.T) {
	t.Parallel()
	f := newFixture()
	f.master.open = []Row{openRow(TypeMasterData, 1, 1, "Anna Adler", base), openRow(TypeMasterData, 2, 2, "Emma Ende", base.Add(-time.Minute))}
	f.excused.open = []Row{openRow(TypeExcused, 3, 2, "Emma Ende", base.Add(-2*time.Minute))}

	page, err := f.query().ListRequests(context.Background(), parse(t, "types=master_data&search=emma"))
	require.NoError(t, err)
	assert.Equal(t, []string{"master_data:2"}, itemIDs(page))
	assert.Empty(t, f.excused.openCalls, "the type filter skips the queue")
	assert.Equal(t, "emma", f.master.openCalls[0].Search)

	calls := len(f.master.openCalls)
	page, err = f.query().ListRequests(context.Background(), parse(t, "student_id=2"))
	require.NoError(t, err)
	assert.Equal(t, []string{"master_data:2", "excused:3"}, itemIDs(page))
	var emmaID int64 = 2
	assert.Equal(t, emmaID, f.master.openCalls[calls].StudentID, "the child filter runs in the owner queue")
}

func TestHistoryFiltersStatusAndDecisionRangeAndListsCorrections(t *testing.T) {
	t.Parallel()
	f := newFixture()
	f.master.history = []Row{
		historyRow(TypeMasterData, 1, 1, "Anna", base, "auto_applied"),
		historyRow(TypeMasterData, 2, 1, "Anna", base.Add(-24*time.Hour), "rejected"),
		historyRow(TypeMasterData, 3, 1, "Anna", base.Add(-10*24*time.Hour), "approved"),
	}
	f.excused.history = []Row{historyRow(TypeExcused, 4, 1, "Anna", base.Add(-time.Hour), "withdrawn")}
	f.corrections.history = []Row{historyRow(TypeDirectCorrection, 5, 1, "Anna", base.Add(-2*time.Hour), "")}

	page, err := f.query().ListRequests(context.Background(), parse(t, "view=history"))
	require.NoError(t, err)
	assert.Equal(t, []string{"master_data:1", "excused:4", "direct_correction:5", "master_data:2", "master_data:3"}, itemIDs(page))
	assert.Empty(t, page.ReviewAccess, "history omits the review reach")
	assert.False(t, page.Items[0].CanCorrect, "auto-applied rows were never decided")
	assert.True(t, page.Items[3].CanCorrect)
	assert.Empty(t, f.master.openCalls, "history never touches the open queues")

	page, err = f.query().ListRequests(context.Background(), parse(t, "view=history&status=approved"))
	require.NoError(t, err)
	assert.Equal(t, []string{"master_data:1", "master_data:3"}, itemIDs(page), "auto-applied counts as approved")

	page, err = f.query().ListRequests(context.Background(), parse(t, "view=history&from=2026-08-09&to=2026-08-10"))
	require.NoError(t, err)
	assert.Equal(t, []string{"master_data:1", "excused:4", "direct_correction:5", "master_data:2"}, itemIDs(page))

	page, err = f.query().ListRequests(context.Background(), parse(t, "view=history&types=direct_correction"))
	require.NoError(t, err)
	assert.Equal(t, []string{"direct_correction:5"}, itemIDs(page))
}

func TestHistoryCursorAdvancesPastFilteredRows(t *testing.T) {
	t.Parallel()
	f := newFixture()
	// Fifty rejected rows per fetch page never match; the scan budget ends
	// without a visible row, but the cursor must still move so the client
	// does not loop.
	for i := range 250 {
		f.master.history = append(f.master.history, historyRow(TypeMasterData, int64(1000-i), 1, "Anna", base.Add(-time.Duration(i)*time.Minute), "rejected"))
	}
	f.master.history = append(f.master.history, historyRow(TypeMasterData, 1, 1, "Anna", base.Add(-300*time.Minute), "approved"))

	first, err := f.query().ListRequests(context.Background(), parse(t, "view=history&status=approved&types=master_data"))
	require.NoError(t, err)
	assert.Empty(t, first.Items, "the scan budget ends before the approved row")
	require.NotEmpty(t, first.NextCursor, "an underfilled page still hands out a cursor")

	second, err := f.query().ListRequests(context.Background(), parse(t, "view=history&status=approved&types=master_data&cursor="+first.NextCursor))
	require.NoError(t, err)
	assert.Equal(t, []string{"master_data:1"}, itemIDs(second))
	assert.Empty(t, second.NextCursor)
}

func TestPastRequestsLoseBulkEligibility(t *testing.T) {
	t.Parallel()
	f := newFixture()
	past := openRow(TypeExcused, 1, 1, "Anna", base)
	past.Past = true
	past.BulkEligible = true
	current := openRow(TypeExcused, 2, 1, "Anna", base.Add(-time.Minute))
	current.BulkEligible = true
	f.excused.open = []Row{past, current}

	page, err := f.query().ListRequests(context.Background(), parse(t, ""))
	require.NoError(t, err)
	assert.True(t, page.Items[0].Past)
	assert.False(t, page.Items[0].BulkEligible)
	assert.Equal(t, BulkIneligiblePast, page.Items[0].BulkIneligibleReason)
	assert.Equal(t, "Diese Anfrage betrifft nur vergangene Tage.", page.Items[0].BulkIneligibleText)
	assert.True(t, page.Items[1].BulkEligible)
	assert.Empty(t, page.Items[1].BulkIneligibleReason)
}

func TestOpenPageGroupsConflictsAcrossTheWholeQueue(t *testing.T) {
	t.Parallel()
	f := newFixture()
	monday := openRow(TypeCareSchedule, 1, 1, "Anna", base)
	monday.ConflictKeys = []string{"care:1:booking"}
	mondayAgain := openRow(TypeCareSchedule, 2, 1, "Anna", base.Add(-time.Minute))
	mondayAgain.ConflictKeys = []string{"care:1:booking", "care:2:booking"}
	// The third contradiction sits far behind the page window.
	offPage := openRow(TypeCareSchedule, 3, 1, "Anna", base.Add(-time.Hour))
	offPage.ConflictKeys = []string{"care:1:booking"}
	lone := openRow(TypeMasterData, 4, 2, "Ben", base.Add(-time.Second))
	lone.ConflictKeys = []string{"master:person:first_name"}
	f.care.open = []Row{monday, mondayAgain, offPage}
	f.master.open = []Row{lone}

	page, err := f.query().ListRequests(context.Background(), parse(t, "limit=2"))
	require.NoError(t, err)
	require.Equal(t, []string{"care_schedule:1", "master_data:4"}, itemIDs(page))
	assert.Equal(t, "care:1:booking", page.Items[0].ConflictKey)
	assert.Equal(t, 3, page.Items[0].ConflictGroupSize, "counted over every open request of the child, not the page")
	assert.Equal(t, []string{"care:1:booking"}, page.Items[0].ConflictKeys)
	assert.Empty(t, page.Items[1].ConflictKey, "a lone request reports no group")
	assert.Zero(t, page.Items[1].ConflictGroupSize)

	scan := f.care.openCalls[len(f.care.openCalls)-1]
	assert.ElementsMatch(t, []int64{1, 2}, scan.StudentIDs)
	assert.Equal(t, conflictScopeLimit, scan.Limit)
	assert.Nil(t, scan.UrgentOnly, "the scan covers both urgency phases")
	assert.Len(t, f.excused.openCalls, 3, "the excused queue is scanned once per phase and once for conflicts")
}

func TestOpenPageDecoratesGroupsAndFamilyProtection(t *testing.T) {
	t.Parallel()
	f := newFixture()
	f.students = &fakeStudents{names: map[int64]string{1: "Löwen"}}
	f.protection = &fakeProtection{protected: map[int64]bool{2: true}}
	f.master.open = []Row{openRow(TypeMasterData, 1, 1, "Anna", base), openRow(TypeMasterData, 2, 2, "Ben", base.Add(-time.Minute))}

	page, err := f.query().ListRequests(context.Background(), parse(t, ""))
	require.NoError(t, err)
	assert.Equal(t, "Löwen", page.Items[0].GroupName)
	assert.Empty(t, page.Items[1].GroupName)
	assert.False(t, page.Items[0].FamilyProtected)
	assert.True(t, page.Items[1].FamilyProtected)
	assert.Equal(t, []int64{1, 2}, f.students.got)

	history, err := f.query().ListRequests(context.Background(), parse(t, "view=history"))
	require.NoError(t, err)
	assert.Empty(t, history.Items)
}

func TestOwnerFailuresSurfaceThroughTheExistingContract(t *testing.T) {
	t.Parallel()
	queueErr := errors.New("care queue: review policy refused the caller")

	f := newFixture()
	f.care.err = queueErr
	_, err := f.query().ListRequests(context.Background(), parse(t, ""))
	require.ErrorIs(t, err, queueErr)
	assert.Equal(t, queueErr.Error(), err.Error(), "a queue failure keeps its exact text for the shared renderer")

	f = newFixture()
	f.master.open = []Row{openRow(TypeMasterData, 1, 1, "Anna", base)}
	f.excused.err = queueErr
	_, err = f.query().ListRequests(context.Background(), parse(t, "types=master_data,excused"))
	require.ErrorIs(t, err, queueErr)

	f = newFixture()
	f.master.open = []Row{openRow(TypeMasterData, 1, 1, "Anna", base)}
	f.students = &fakeStudents{err: queueErr}
	_, err = f.query().ListRequests(context.Background(), parse(t, ""))
	require.ErrorIs(t, err, queueErr)
	assert.Equal(t, "load students for request queue groups: "+queueErr.Error(), err.Error())

	f = newFixture()
	f.master.open = []Row{openRow(TypeMasterData, 1, 1, "Anna", base)}
	f.protection = &fakeProtection{err: queueErr}
	_, err = f.query().ListRequests(context.Background(), parse(t, ""))
	assert.Equal(t, "load family protection for request queue: "+queueErr.Error(), err.Error())

	f = newFixture()
	f.access.accessErr = queueErr
	_, err = f.query().ListRequests(context.Background(), parse(t, ""))
	require.ErrorIs(t, err, queueErr)

	f = newFixture()
	f.access.callerErr = queueErr
	_, err = f.query().ListRequests(context.Background(), parse(t, ""))
	require.ErrorIs(t, err, queueErr)
	_, err = f.query().PendingCount(context.Background())
	require.ErrorIs(t, err, queueErr)

	f = newFixture()
	f.offering.countErr = queueErr
	_, err = f.query().PendingCount(context.Background())
	require.ErrorIs(t, err, queueErr)
	assert.Equal(t, queueErr.Error(), err.Error())
}

func TestPendingCountSumsTheQueuesTheCallerMayOpen(t *testing.T) {
	t.Parallel()
	f := newFixture()
	f.master.count, f.care.count, f.offering.count, f.excused.count = 1, 2, 3, 4

	count, err := f.query().PendingCount(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 10, count)

	f.access.caller = Caller{}
	count, err = f.query().PendingCount(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 4, count, "an absence-only reviewer counts the excused queue and nothing else")
}

func TestListRejectsAHandBuiltInvalidQuery(t *testing.T) {
	t.Parallel()
	f := newFixture()
	_, err := f.query().ListRequests(context.Background(), ListQuery{Types: []string{"nope"}})
	require.ErrorIs(t, err, ErrInvalidQuery)
	_, err = f.query().ListRequests(context.Background(), ListQuery{Cursor: "not-a-cursor"})
	require.ErrorIs(t, err, ErrInvalidQuery)
	_, err = f.query().ListRequests(context.Background(), ListQuery{Statuses: map[string]struct{}{"approved": {}}})
	require.ErrorIs(t, err, ErrInvalidQuery, "history-only filters on the open view")
}

func TestItemWireShapeIsStable(t *testing.T) {
	t.Parallel()
	f := newFixture()
	row := openRow(TypeExcused, 1, 1, "Anna Adler", base)
	changed := true
	row.CurrentValueChanged = &changed
	row.CurrentStatusByDate = map[string]string{"2026-08-10": "present"}
	row.BulkEligible = true
	row.ConflictKeys = []string{"absence:2026-08-10"}
	f.excused.open = []Row{row}

	page, err := f.query().ListRequests(context.Background(), parse(t, "types=excused"))
	require.NoError(t, err)
	raw, err := json.Marshal(page)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"items": [{
			"request_type": "excused",
			"occurred_at": "2026-08-10T12:00:00Z",
			"student_id": "1",
			"student_name": "Anna Adler",
			"expected_version": "2026-08-10T12:00:00Z",
			"urgent_today": false,
			"past": false,
			"bulk_eligible": true,
			"conflict_keys": ["absence:2026-08-10"],
			"current_value_changed": true,
			"current_status_by_date": {"2026-08-10": "present"},
			"family_protected": false,
			"data": {"id": 1}
		}],
		"review_access": "admin"
	}`, string(raw))
}
