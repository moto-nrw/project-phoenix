package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

type pickupTeamStoreDouble struct {
	upserted  []int64
	date      calendar.Date
	requestID int64
	rows      []ports.PickupChangeNoticeRow
	from      calendar.Date
	marked    [2]int64
	err       error
}

func (s *pickupTeamStoreDouble) UpsertPickupChangeNotices(_ context.Context, requestID int64, date calendar.Date, recipients []int64) (domain.OperationStats, error) {
	s.requestID, s.date, s.upserted = requestID, date, recipients
	return domain.OperationStats{Queries: 1}, s.err
}

func (s *pickupTeamStoreDouble) ListUnreadPickupChangeNotices(_ context.Context, _ int64, from calendar.Date, _ int) ([]ports.PickupChangeNoticeRow, domain.OperationStats, error) {
	s.from = from
	return s.rows, domain.OperationStats{Queries: 1}, nil
}

func (s *pickupTeamStoreDouble) MarkPickupChangeNoticesRead(_ context.Context, reader, studentID int64) (domain.OperationStats, error) {
	s.marked = [2]int64{reader, studentID}
	return domain.OperationStats{Queries: 1}, nil
}

type pickupTeamReachDouble struct {
	responsible []int64
	excluded    int64
	pushes      []careplan.PickupChangePush
	pushErr     error
}

func (r *pickupTeamReachDouble) ResponsibleAccountIDs(_ context.Context, _, excluded int64) ([]int64, error) {
	r.excluded = excluded
	return r.responsible, nil
}

func (r *pickupTeamReachDouble) NotifyPickupChanged(_ context.Context, push careplan.PickupChangePush) error {
	r.pushes = append(r.pushes, push)
	return r.pushErr
}

// Thursday, 12 Nov 2026.
var pickupTeamToday = calendar.NewDate(2026, 11, 12)

func newPickupTeam(store *pickupTeamStoreDouble, reach *pickupTeamReachDouble) *PickupChangeTeamService {
	direct := func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
	return NewPickupChangeTeam(store, reach, func(ports.Observation) {}, direct, func() calendar.Date { return pickupTeamToday })
}

func pickupNotice(date calendar.Date) careplan.PickupChangeNotice {
	return careplan.PickupChangeNotice{TenantID: 3, StudentID: 77, Date: date, PickupTime: "15:00", ReviewerAccountID: 9, RequestID: 501}
}

func TestPickupChangeTeamMarksAndTellsTheResponsibleStaff(t *testing.T) {
	t.Parallel()
	store := &pickupTeamStoreDouble{}
	reach := &pickupTeamReachDouble{responsible: []int64{11, 9, 12, 11}}

	require.NoError(t, newPickupTeam(store, reach).NotifyPickupChangeApproved(t.Context(), pickupNotice(pickupTeamToday.AddDays(1))))

	assert.Equal(t, int64(9), reach.excluded, "the person who decided needs no notice")
	assert.Equal(t, []int64{11, 12}, store.upserted, "one marker per colleague, never the decider")
	assert.Equal(t, int64(501), store.requestID)
	assert.Equal(t, pickupTeamToday.AddDays(1), store.date)
	require.Len(t, reach.pushes, 1)
	push := reach.pushes[0]
	assert.Equal(t, []int64{11, 12}, push.RecipientAccountIDs)
	assert.Equal(t, "Abholzeit geändert", push.Title)
	assert.Equal(t, "Neue Abholzeit für ein Kind: morgen um 15:00 Uhr.", push.Body)
	assert.Equal(t, "/students/77", push.DeepLink)
	assert.Equal(t, int64(501), push.RequestID)
}

// The device text names the day but never the child: it shows on a lock
// screen or a shared tablet.
func TestPickupChangeTeamNamesTheDayNotTheChild(t *testing.T) {
	t.Parallel()
	cases := map[calendar.Date]string{
		pickupTeamToday:                "Neue Abholzeit für ein Kind: heute um 15:00 Uhr.",
		pickupTeamToday.AddDays(1):     "Neue Abholzeit für ein Kind: morgen um 15:00 Uhr.",
		calendar.NewDate(2026, 11, 20): "Neue Abholzeit für ein Kind: am Fr. 20.11. um 15:00 Uhr.",
	}
	for date, want := range cases {
		reach := &pickupTeamReachDouble{responsible: []int64{11}}
		require.NoError(t, newPickupTeam(&pickupTeamStoreDouble{}, reach).NotifyPickupChangeApproved(t.Context(), pickupNotice(date)))
		require.Len(t, reach.pushes, 1)
		assert.Equal(t, want, reach.pushes[0].Body)
	}
}

func TestPickupChangeTeamAnnouncesWithdrawal(t *testing.T) {
	t.Parallel()
	reach := &pickupTeamReachDouble{responsible: []int64{11}}
	notice := pickupNotice(calendar.NewDate(2026, 11, 20))
	notice.Withdrawn, notice.PickupTime = true, ""

	require.NoError(t, newPickupTeam(&pickupTeamStoreDouble{}, reach).NotifyPickupChangeApproved(t.Context(), notice))

	require.Len(t, reach.pushes, 1)
	assert.Equal(t, "Abholzeit zurückgenommen", reach.pushes[0].Title)
	assert.Equal(t, "Für ein Kind gilt am Fr. 20.11. wieder die normale Abholzeit.", reach.pushes[0].Body)
	assert.True(t, reach.pushes[0].Withdrawn)
}

func TestPickupChangeTeamSkipsWhatIsNoNews(t *testing.T) {
	t.Parallel()
	past := pickupNotice(pickupTeamToday.AddDays(-1))
	noTime := pickupNotice(pickupTeamToday)
	noTime.PickupTime = ""
	for name, notice := range map[string]careplan.PickupChangeNotice{"past day": past, "no new time": noTime} {
		store, reach := &pickupTeamStoreDouble{}, &pickupTeamReachDouble{responsible: []int64{11}}
		require.NoError(t, newPickupTeam(store, reach).NotifyPickupChangeApproved(t.Context(), notice), name)
		assert.Empty(t, store.upserted, name)
		assert.Empty(t, reach.pushes, name)
	}

	store, reach := &pickupTeamStoreDouble{}, &pickupTeamReachDouble{responsible: []int64{9}}
	require.NoError(t, newPickupTeam(store, reach).NotifyPickupChangeApproved(t.Context(), pickupNotice(pickupTeamToday)))
	assert.Empty(t, store.upserted, "nobody but the decider is responsible")
	assert.Empty(t, reach.pushes)
}

// A failed marker write or push fails the decision, so no approval is
// reported that the team did not hear about.
func TestPickupChangeTeamFailureFailsTheDecision(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	err := newPickupTeam(&pickupTeamStoreDouble{err: boom}, &pickupTeamReachDouble{responsible: []int64{11}}).
		NotifyPickupChangeApproved(t.Context(), pickupNotice(pickupTeamToday))
	require.ErrorIs(t, err, boom)

	err = newPickupTeam(&pickupTeamStoreDouble{}, &pickupTeamReachDouble{responsible: []int64{11}, pushErr: boom}).
		NotifyPickupChangeApproved(t.Context(), pickupNotice(pickupTeamToday))
	require.ErrorIs(t, err, boom)
}

func TestPickupChangeTeamListsTheRequestsCurrentState(t *testing.T) {
	t.Parallel()
	payload := func(date string) json.RawMessage {
		return json.RawMessage(`{"date":"` + date + `","pickup_time":"15:00"}`)
	}
	informed := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	store := &pickupTeamStoreDouble{rows: []ports.PickupChangeNoticeRow{
		{RequestID: 1, StudentID: 77, StudentFirstName: "Mia", Status: "approved", Payload: payload("2026-11-13"), InformedAt: informed},
		{RequestID: 2, StudentID: 78, StudentFirstName: "Ben", Status: "rejected", Payload: payload("2026-11-20"), InformedAt: informed},
		{RequestID: 3, StudentID: 79, StudentFirstName: "Ida", Status: "approved", Payload: json.RawMessage(`{}`), InformedAt: informed},
	}}
	team := newPickupTeam(store, &pickupTeamReachDouble{})

	got, err := team.MarkPickupChangeNoticesRead(t.Context(), 11, 80)
	require.NoError(t, err)
	assert.Equal(t, [2]int64{11, 80}, store.marked)
	assert.Equal(t, pickupTeamToday, store.from, "only days from today on")
	require.Len(t, got, 2, "a request without a pickup day has nothing to show")
	assert.Equal(t, careplan.PickupChangeTeamNotice{
		RequestID: 1, StudentID: 77, StudentFirstName: "Mia", Date: calendar.NewDate(2026, 11, 13), PickupTime: "15:00", InformedAt: informed,
	}, got[0])
	assert.True(t, got[1].Withdrawn, "a corrected decision took the time back")
	assert.Empty(t, got[1].PickupTime)

	_, err = team.ListUnreadPickupChangeNotices(t.Context(), 0)
	require.ErrorIs(t, err, ErrPickupChangeNoticeInvalid)
}
