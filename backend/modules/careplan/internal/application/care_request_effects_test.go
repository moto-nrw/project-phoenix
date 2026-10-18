package application

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

const fakeCareRequestID int64 = 41

// careRequestMessenger records the pills and the co-guardian split.
type careRequestMessenger struct {
	fakeMessenger
	neutral []ports.RequestEvent
}

func (m *careRequestMessenger) EmitDecisionAudience(ctx context.Context, tenantID, studentID int64, audience ports.DecisionAudience, full, neutral ports.RequestEvent) error {
	if err := m.fakeMessenger.EmitDecisionAudience(ctx, tenantID, studentID, audience, full, neutral); err != nil {
		return err
	}
	m.neutral = append(m.neutral, neutral)
	return nil
}

type careRequestBroadcasts struct{ events []string }

func (b *careRequestBroadcasts) StudentUpdated(int64) error {
	b.events = append(b.events, "student_updated")
	return nil
}
func (b *careRequestBroadcasts) StudentCompanionsChanged(int64) error {
	b.events = append(b.events, "student_companions_changed")
	return nil
}
func (b *careRequestBroadcasts) ArrivalScheduleChanged() error {
	b.events = append(b.events, "arrival_schedule_changed")
	return nil
}

type careRequestRows struct {
	row carerequests.Request
	err error
}

func (r careRequestRows) FindCareScheduleRequest(context.Context, int64, bool) (carerequests.Request, error) {
	return r.row, r.err
}

type careRequestNames struct {
	name careplan.PersonName
	err  error
}

func (n careRequestNames) StudentName(context.Context, int64) (careplan.PersonName, error) {
	return n.name, n.err
}

type careRequestHarness struct {
	effects     *CareRequestEffects
	hooks       *fakeHooks
	messenger   *careRequestMessenger
	ledger      *fakeLedger
	broadcaster *careRequestBroadcasts
}

func newCareRequestHarness(t *testing.T, rows careRequestRows, names careRequestNames) *careRequestHarness {
	t.Helper()
	h := &careRequestHarness{hooks: newFakeHooks(), messenger: &careRequestMessenger{}, ledger: &fakeLedger{}, broadcaster: &careRequestBroadcasts{}}
	// The fake exposes the committed timeline, like another database reader.
	h.messenger.committedView = h.hooks.AfterCommit
	effects, err := NewCareRequestEffects(CareRequestEffectPorts{
		Records: rows, Students: names, Messenger: h.messenger, Ledger: h.ledger, Broadcaster: h.broadcaster,
	}, h.hooks)
	require.NoError(t, err)
	h.effects = effects
	return h
}

func weeklyCareRequest() *carerequests.Request {
	return &carerequests.Request{ID: fakeCareRequestID, TenantID: fakeTenantID, StudentID: fakeStudentID, SubmittedBy: fakeGuardian,
		RequestKind: "weekly_schedule", Payload: json.RawMessage(`{"weekdays":[{"weekday":1,"pickup":"15:00"},{"weekday":3,"pickup":"14:00"}]}`)}
}

func pickupCareRequest(payload string) *carerequests.Request {
	return &carerequests.Request{ID: fakeCareRequestID, TenantID: fakeTenantID, StudentID: fakeStudentID, SubmittedBy: fakeGuardian,
		RequestKind: "pickup_change", Payload: json.RawMessage(payload)}
}

const pickupTimeAndModePayload = `{"date":"2026-09-21","pickup_time":"14:30","previous_pickup_time":"16:00","departure_mode":"bus","reason":" Arzttermin "}`

func TestNewCareRequestEffectsRequiresRecordsNamesAndHooks(t *testing.T) {
	t.Parallel()
	_, err := NewCareRequestEffects(CareRequestEffectPorts{Students: careRequestNames{}}, newFakeHooks())
	require.Error(t, err)
	_, err = NewCareRequestEffects(CareRequestEffectPorts{Records: careRequestRows{}}, newFakeHooks())
	require.Error(t, err)
	_, err = NewCareRequestEffects(CareRequestEffectPorts{Records: careRequestRows{}, Students: careRequestNames{}}, nil)
	require.Error(t, err)
}

func TestCareRequestDecisionPillsNameTheRequestAndItsDay(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		request *carerequests.Request
		approve bool
		status  string
		body    string
		kind    string
	}{
		{"weekly approved", weeklyCareRequest(), true, "erledigt", "Anfrage bestätigt, Betreuungszeiten übernommen", "care_schedule"},
		{"weekly rejected", weeklyCareRequest(), false, "abgelehnt", "Anfrage abgelehnt: Kein Platz", "care_schedule"},
		{"pickup approved", pickupCareRequest(pickupTimeAndModePayload), true, "erledigt", "Abholzeit und Abholart bestätigt: 21.09.2026, 14:30 Uhr, Fährt Bus", "pickup_change"},
		{"pickup rejected", pickupCareRequest(`{"date":"2026-09-21","pickup_time":"14:30"}`), false, "abgelehnt", "Abholzeit abgelehnt: 21.09.2026, 14:30 Uhr. Grund: Kein Platz", "pickup_change"},
		{"mode rejected", pickupCareRequest(`{"date":"2026-09-21","departure_mode":"pickup"}`), false, "abgelehnt", "Abholart abgelehnt: 21.09.2026, Wird abgeholt. Grund: Kein Platz", "pickup_change"},
		{"undecodable pickup rejected", pickupCareRequest(`{`), false, "abgelehnt", "Anfrage abgelehnt: Kein Platz", "pickup_change"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newCareRequestHarness(t, careRequestRows{}, careRequestNames{})
			err := h.effects.RegisterDecision(context.Background(), tc.request,
				carerequests.DecideInput{Approve: tc.approve, ReviewedBy: fakeReviewer}, "Kein Platz", false)
			require.NoError(t, err)
			require.Len(t, h.messenger.enqueued, 1, "the durable decision intent is written in the transaction")
			assert.Empty(t, h.messenger.emitted, "the pill waits for the commit")
			pill := h.messenger.enqueued[0]
			assert.Equal(t, "request_status", pill.EventType)
			assert.Equal(t, "staff", pill.ActorKind)
			assert.Equal(t, fakeReviewer, pill.ActorAccountID)
			assert.Equal(t, tc.status, pill.RequestStatus)
			assert.Equal(t, tc.body, pill.Body)
			assert.Equal(t, tc.kind, pill.RequestType)
			assert.Equal(t, "Kein Platz", pill.DecisionReason)
			assert.Equal(t, "schedule.care_schedule_change_requests", pill.RefTable)
			assert.Equal(t, fakeCareRequestID, pill.RefID)
			h.hooks.commit()
			require.Len(t, h.messenger.emitted, 1)
			assert.Equal(t, pill, h.messenger.emitted[0])
			assert.Equal(t, 1, h.messenger.guardianWakes)
		})
	}
}

func TestCareRequestPillPayloadCarriesTheStructuredDay(t *testing.T) {
	t.Parallel()
	h := newCareRequestHarness(t, careRequestRows{}, careRequestNames{})
	require.NoError(t, h.effects.NotifySubmission(context.Background(), pickupCareRequest(pickupTimeAndModePayload)))
	require.Len(t, h.messenger.enqueued, 1)
	pill := h.messenger.enqueued[0]
	assert.Equal(t, "request_created", pill.EventType)
	assert.Equal(t, "guardian", pill.ActorKind)
	assert.Equal(t, fakeGuardian, pill.ActorAccountID)
	assert.Equal(t, "offen", pill.RequestStatus)
	assert.Equal(t, "pickup_change", pill.RequestType)
	assert.Equal(t, "Abholzeit und Abholart angefragt: 21.09.2026, 14:30 Uhr, Fährt Bus", pill.Body)
	assert.Equal(t, map[string]any{"date": "2026-09-21", "pickup_time": "14:30", "previous_pickup_time": "16:00", "departure_mode": "bus"}, pill.Payload)

	require.NoError(t, h.effects.NotifySubmission(context.Background(), weeklyCareRequest()))
	weekly := h.messenger.enqueued[1]
	assert.Equal(t, "Anfrage: Dauerhafte Betreuungszeiten ändern", weekly.Body)
	assert.Equal(t, "care_schedule", weekly.RequestType)
	assert.Nil(t, weekly.Payload)
}

func TestCareRequestClosingPillsKeepTheirWording(t *testing.T) {
	t.Parallel()
	request := pickupCareRequest(`{"date":"2026-09-21","pickup_time":"14:30"}`)
	h := newCareRequestHarness(t, careRequestRows{}, careRequestNames{})
	ctx := context.Background()
	require.NoError(t, h.effects.NotifyMarkedDone(ctx, request, fakeReviewer, "Tag vorbei"))
	require.NoError(t, h.effects.NotifyCorrection(ctx, request, fakeReviewer, true, "Doch möglich"))
	require.NoError(t, h.effects.NotifyCorrection(ctx, request, fakeReviewer, false, "Doch nicht"))
	require.Len(t, h.messenger.enqueued, 3)
	assert.Equal(t, "Anfrage abgeschlossen: 21.09.2026, 14:30 Uhr", h.messenger.enqueued[0].Body)
	assert.Equal(t, "erledigt", h.messenger.enqueued[0].RequestStatus)
	assert.Equal(t, "Entscheidung geändert. Abholzeit bestätigt: 21.09.2026, 14:30 Uhr", h.messenger.enqueued[1].Body)
	assert.Equal(t, "erledigt", h.messenger.enqueued[1].RequestStatus)
	assert.Equal(t, "Entscheidung geändert. Abholzeit abgelehnt: 21.09.2026, 14:30 Uhr", h.messenger.enqueued[2].Body)
	assert.Equal(t, "abgelehnt", h.messenger.enqueued[2].RequestStatus)
	assert.Equal(t, "Doch nicht", h.messenger.enqueued[2].DecisionReason)
}

func TestCareRequestCoGuardianNoticeHidesWhatWasNotShared(t *testing.T) {
	t.Parallel()
	h := newCareRequestHarness(t, careRequestRows{}, careRequestNames{})
	h.messenger.audience = ports.DecisionAudience{Full: []int64{fakePersonID}, Neutral: []int64{otherStudentID}}
	request := pickupCareRequest(`{"date":"2026-09-21","pickup_time":"14:30","previous_pickup_time":"16:00"}`)
	require.NoError(t, h.effects.RegisterDecision(context.Background(), request,
		carerequests.DecideInput{Approve: true, ReviewedBy: fakeReviewer}, "", false))
	assert.Zero(t, h.messenger.audienceCalls, "the notices wait for the commit")
	h.hooks.commit()
	require.Equal(t, 1, h.messenger.audienceCalls)
	assert.Equal(t, "Abholzeit bestätigt: 21.09.2026, 14:30 Uhr", h.messenger.audienceFull[0].Body)
	assert.NotNil(t, h.messenger.audienceFull[0].Payload)
	assert.Equal(t, "Betreuungsstand geändert: Abholzeit 21.09.2026", h.messenger.neutral[0].Body)
	assert.Nil(t, h.messenger.neutral[0].Payload, "the neutral line names no requested or previous time")

	weekly := newCareRequestHarness(t, careRequestRows{}, careRequestNames{})
	weekly.messenger.audience = ports.DecisionAudience{Neutral: []int64{otherStudentID}}
	require.NoError(t, weekly.effects.RegisterDecision(context.Background(), weeklyCareRequest(),
		carerequests.DecideInput{ReviewedBy: fakeReviewer}, "Kein Platz", false))
	weekly.hooks.commit()
	assert.Equal(t, "Betreuungsstand geändert: Betreuungszeiten", weekly.messenger.neutral[0].Body)
}

func TestCareRequestApprovalWakesStaffTabsAfterCommit(t *testing.T) {
	t.Parallel()
	h := newCareRequestHarness(t, careRequestRows{}, careRequestNames{})
	ctx := context.Background()
	require.NoError(t, h.effects.RegisterDecision(ctx, weeklyCareRequest(), carerequests.DecideInput{Approve: true, ReviewedBy: fakeReviewer}, "", false))
	assert.Empty(t, h.broadcaster.events, "staff tabs refetch only the committed plan")
	h.hooks.commit()
	assert.Equal(t, []string{"student_updated", "arrival_schedule_changed"}, h.broadcaster.events)

	h.broadcaster.events = nil
	require.NoError(t, h.effects.RegisterDecision(ctx, weeklyCareRequest(), carerequests.DecideInput{Approve: true, ReviewedBy: fakeReviewer}, "", true))
	h.hooks.commit()
	assert.Equal(t, []string{"student_updated", "student_companions_changed", "arrival_schedule_changed"}, h.broadcaster.events)

	h.broadcaster.events = nil
	require.NoError(t, h.effects.RegisterDecision(ctx, weeklyCareRequest(), carerequests.DecideInput{ReviewedBy: fakeReviewer}, "Kein Platz", true))
	h.hooks.commit()
	assert.Empty(t, h.broadcaster.events, "a rejection changes no plan")
}

func TestCareRequestLedgerRecordsTheRequestType(t *testing.T) {
	t.Parallel()
	h := newCareRequestHarness(t, careRequestRows{}, careRequestNames{})
	ctx := context.Background()
	updated := time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)
	pickup := pickupCareRequest(`{"date":"2026-09-21","pickup_time":"14:30"}`)
	pickup.UpdatedAt = updated
	require.NoError(t, h.effects.RecordSubmission(ctx, pickup))
	require.NoError(t, h.effects.RecordGuardianEdit(ctx, weeklyCareRequest(), fakeGuardian))
	require.NoError(t, h.effects.RecordMarkedDone(ctx, pickup, fakeReviewer, "Tag vorbei"))
	require.Len(t, h.ledger.entries, 3)
	assert.Equal(t, ports.RequestLedgerEntry{StudentID: fakeStudentID, RequestType: "pickup_change", RequestID: fakeCareRequestID,
		EventType: "submitted", ActorAccountID: fakeGuardian, UpdatedAt: updated}, h.ledger.entries[0])
	assert.Equal(t, "care_schedule", h.ledger.entries[1].RequestType)
	assert.Equal(t, "guardian_edited", h.ledger.entries[1].EventType)
	assert.Equal(t, "marked_done", h.ledger.entries[2].EventType)
	assert.Equal(t, map[string]any{"reason": "Tag vorbei"}, h.ledger.entries[2].Payload)
}

func TestCareRequestEffectsSkipUnwiredSinks(t *testing.T) {
	t.Parallel()
	hooks := newFakeHooks()
	effects, err := NewCareRequestEffects(CareRequestEffectPorts{Records: careRequestRows{}, Students: careRequestNames{}}, hooks)
	require.NoError(t, err)
	ctx := context.Background()
	request := pickupCareRequest(`{"date":"2026-09-21","pickup_time":"14:30"}`)
	require.NoError(t, effects.RecordSubmission(ctx, request))
	require.NoError(t, effects.NotifySubmission(ctx, request))
	effects.WakeGuardians(ctx, request)
	require.NoError(t, effects.RegisterDecision(ctx, request, carerequests.DecideInput{Approve: true, ReviewedBy: fakeReviewer}, "", true))
	// Only the approval's audit line waits for the commit.
	assert.Equal(t, 1, hooks.pending())
	hooks.commit()
}

func TestCareRequestReloadKeepsTheItemWhenTheNameIsMissing(t *testing.T) {
	t.Parallel()
	row := *pickupCareRequest(`{"date":"2026-09-21","pickup_time":"14:30","reason":"  Arzttermin  "}`)
	h := newCareRequestHarness(t, careRequestRows{row: row}, careRequestNames{err: errBoom})
	item, err := h.effects.ReloadDecision(context.Background(), row.ID)
	require.NoError(t, err)
	assert.Equal(t, row.ID, item.Request.ID)
	require.NotNil(t, item.Reason)
	assert.Equal(t, "Arzttermin", *item.Reason)
	assert.Empty(t, item.FirstName)

	named := newCareRequestHarness(t, careRequestRows{row: *weeklyCareRequest()}, careRequestNames{name: careplan.PersonName{FirstName: "Mia", LastName: "Muster"}})
	item, err = named.effects.ReloadDecision(context.Background(), fakeCareRequestID)
	require.NoError(t, err)
	assert.Nil(t, item.Reason)
	assert.Equal(t, "Mia", item.FirstName)
	assert.Equal(t, "Muster", item.LastName)

	failing := newCareRequestHarness(t, careRequestRows{err: errBoom}, careRequestNames{})
	_, err = failing.effects.ReloadDecision(context.Background(), fakeCareRequestID)
	require.ErrorIs(t, err, errBoom)
}

type staffProbe struct {
	staffID int64
	found   bool
	err     error
}

func (p staffProbe) CurrentStaffID(context.Context) (int64, bool, error) {
	return p.staffID, p.found, p.err
}

func TestActingStaffIDRefusesCallersWithoutStaffProfile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	staffID, err := actingStaffID(ctx, staffProbe{staffID: fakeReviewer, found: true}, "")
	require.NoError(t, err)
	assert.Equal(t, fakeReviewer, staffID)
	_, err = actingStaffID(ctx, staffProbe{}, pickupRequestStaff)
	require.ErrorIs(t, err, carerequests.ErrCareRequestForbidden)
	_, err = actingStaffID(ctx, staffProbe{err: errBoom}, pickupRequestStaff)
	require.ErrorIs(t, err, errBoom)
	assert.Equal(t, "schedule: resolve acting staff for pickup request: boom", err.Error())
}

type guardianProbe struct {
	allowed bool
	err     error
}

func (p guardianProbe) GuardianHasChildAccess(context.Context, int64, int64) (bool, error) {
	return p.allowed, p.err
}

func TestDecisionGuardianCheckSkipsWithoutMessaging(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	request := weeklyCareRequest()
	require.NoError(t, (&RequestDecisions{}).guardianStillHoldsChild(ctx, request))
	require.NoError(t, (&RequestDecisions{Guardians: guardianProbe{allowed: true}}).guardianStillHoldsChild(ctx, request))
	require.ErrorIs(t, (&RequestDecisions{Guardians: guardianProbe{}}).guardianStillHoldsChild(ctx, request), carerequests.ErrGuardianAccessRevoked)
	require.ErrorIs(t, (&RequestDecisions{Guardians: guardianProbe{err: errBoom}}).guardianStillHoldsChild(ctx, request), errBoom)
}

type departurePlan struct {
	dayMode  string
	dayFound bool
	weekly   map[string][]string
	calls    []string
}

func (p *departurePlan) PickupTime(context.Context, int64, calendar.Date) (*time.Time, error) {
	return nil, nil
}
func (p *departurePlan) DayDepartureMode(context.Context, int64, calendar.Date) (string, bool, error) {
	p.calls = append(p.calls, "day")
	return p.dayMode, p.dayFound, nil
}
func (p *departurePlan) WeeklyDepartureModes(context.Context, int64) (map[string][]string, error) {
	p.calls = append(p.calls, "weekly")
	return p.weekly, nil
}

func TestPreviousDepartureModesPreferTheDayValue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	monday := calendar.Date("2026-09-21")
	wednesday := calendar.Date("2026-09-23")
	weekly := map[string][]string{"mon": {"bus", "pickup"}}

	day := &departurePlan{dayMode: "pickup", dayFound: true, weekly: weekly}
	modes, err := previousDepartureModes(ctx, day, fakeStudentID, monday)
	require.NoError(t, err)
	assert.Equal(t, []string{"pickup"}, modes)
	assert.Equal(t, []string{"day"}, day.calls, "the OGS's day value wins without reading the week")

	plan := &departurePlan{weekly: weekly}
	modes, err = previousDepartureModes(ctx, plan, fakeStudentID, monday)
	require.NoError(t, err)
	assert.Equal(t, []string{"bus", "pickup"}, modes)

	modes, err = previousDepartureModes(ctx, &departurePlan{weekly: weekly}, fakeStudentID, wednesday)
	require.NoError(t, err)
	assert.Equal(t, []string{careplan.DepartureModeAlone}, modes, "a weekday without a set reads as alone")
}
