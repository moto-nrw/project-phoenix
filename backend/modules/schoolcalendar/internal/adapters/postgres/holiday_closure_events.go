package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Proposals and commands of the notices after a school closing day changed
// (ADR 0196). A school statement runs in the school's transaction and names
// the school in its own filter; a Träger statement runs in the Träger
// transaction and names the Träger and, where it reads a school's rows, the
// school's current membership. The filters hold below row security too.

const (
	bumpHolidayClosureRevisionSQL = `INSERT INTO calendar.holiday_closure_revisions (tenant_id, revision)
		VALUES (?, 1)
		ON CONFLICT (tenant_id) DO UPDATE
			SET revision = calendar.holiday_closure_revisions.revision + 1, updated_at = CURRENT_TIMESTAMP
		RETURNING revision`
	openHolidayClosureProposalSQL = `SELECT id, previous_start, previous_end, span_from, span_to, supersedes_event_id
		FROM calendar.holiday_closure_proposals
		WHERE tenant_id = ? AND source_kind = ? AND source_id = ? AND state = 'open'
		FOR UPDATE`
	unprocessedHolidayClosureEventSQL = `SELECT id, span_from, span_to FROM calendar.holiday_closure_events
		WHERE tenant_id = ? AND source_kind = ? AND source_id = ? AND state IN ('pending', 'leased')
		ORDER BY id DESC LIMIT 1`
	insertHolidayClosureProposalSQL = `INSERT INTO calendar.holiday_closure_proposals
			(tenant_id, organization_id, source_kind, source_id, operation_id, reason, previous_start, previous_end,
			current_start, current_end, span_from, span_to, revision, supersedes_event_id)
		VALUES (?, 0, ?, ?, gen_random_uuid(), ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	mergeHolidayClosureProposalSQL = `UPDATE calendar.holiday_closure_proposals
		SET reason = ?, current_start = ?, current_end = ?, span_from = LEAST(span_from, ?), span_to = GREATEST(span_to, ?), revision = ?
		WHERE tenant_id = ? AND id = ? AND state = 'open'`
	dropHolidayClosureProposalSQL = `DELETE FROM calendar.holiday_closure_proposals
		WHERE tenant_id = ? AND id = ? AND state = 'open'`

	holidayClosureNoticeColumns = `proposal.id AS proposal_id, proposal.tenant_id, proposal.organization_id, proposal.source_id,
		proposal.reason, proposal.previous_start, proposal.previous_end, proposal.current_start, proposal.current_end,
		proposal.span_from, proposal.span_to, proposal.revision, proposal.operation_id::text AS operation_id,
		proposal.state AS proposal_state, proposal.supersedes_event_id, proposal.updated_at,
		event.id AS event_id, event.state AS event_state, event.confirmed_at`
	// A notice stays listed while it is open or not finished; a finished
	// one for two weeks, a superseded command never (its proposal shows).
	holidayClosureNoticeVisible = `(proposal.state = 'open' AND proposal.span_to >= ?)
		OR (proposal.state = 'confirmed' AND event.state NOT IN ('completed', 'no_mail', 'needs_confirmation'))
		OR (proposal.state = 'confirmed' AND event.state IN ('completed', 'no_mail')
			AND COALESCE(event.completed_at, event.processed_at) > CURRENT_TIMESTAMP - interval '14 days')`
	schoolHolidayClosureNoticesSQL = `SELECT ` + holidayClosureNoticeColumns + `
		FROM calendar.holiday_closure_proposals AS proposal
		LEFT JOIN calendar.holiday_closure_events AS event ON event.proposal_id = proposal.id AND event.tenant_id = proposal.tenant_id
		WHERE proposal.tenant_id = ? AND (` + holidayClosureNoticeVisible + `)
		ORDER BY proposal.span_from, proposal.id`
	traegerHolidayClosureNoticesSQL = `SELECT ` + holidayClosureNoticeColumns + `
		FROM calendar.holiday_closure_proposals AS proposal
		LEFT JOIN calendar.holiday_closure_events AS event
			ON event.proposal_id = proposal.id AND event.organization_id = proposal.organization_id
		WHERE proposal.organization_id = ? AND (` + holidayClosureNoticeVisible + `)
		ORDER BY proposal.span_from, proposal.tenant_id, proposal.id`
	lockHolidayClosureProposalSQL = `SELECT ` + holidayClosureNoticeColumns + `
		FROM calendar.holiday_closure_proposals AS proposal
		LEFT JOIN calendar.holiday_closure_events AS event ON event.proposal_id = proposal.id AND event.tenant_id = proposal.tenant_id
		WHERE proposal.tenant_id = ? AND proposal.id = ?
		FOR UPDATE OF proposal`
	insertHolidayClosureEventSQL = `INSERT INTO calendar.holiday_closure_events
			(tenant_id, organization_id, proposal_id, operation_id, source_kind, source_id, revision, span_from, span_to,
			confirmed_channel)
		SELECT proposal.tenant_id, proposal.organization_id, proposal.id, proposal.operation_id, proposal.source_kind,
			proposal.source_id, proposal.revision, proposal.span_from, proposal.span_to, ?
		FROM calendar.holiday_closure_proposals AS proposal
		WHERE proposal.tenant_id = ? AND proposal.id = ? AND proposal.state = 'open'
		ON CONFLICT (proposal_id) DO NOTHING`
	confirmHolidayClosureProposalSQL = `UPDATE calendar.holiday_closure_proposals
		SET state = 'confirmed', confirmed_at = CURRENT_TIMESTAMP
		WHERE tenant_id = ? AND id = ? AND state = 'open'`
	retryHolidayClosureEventSQL = `UPDATE calendar.holiday_closure_events
		SET state = 'pending', next_attempt_at = CURRENT_TIMESTAMP, error_code = NULL,
			requested_channel = ?, requested_at = CURRENT_TIMESTAMP
		WHERE tenant_id = ? AND proposal_id = ? AND state IN ('failed', 'blocked')`

	leaseHolidayClosureEventSQL = `UPDATE calendar.holiday_closure_events AS event
		SET state = 'leased', lease_token = gen_random_uuid(), lease_until = CURRENT_TIMESTAMP + ?::interval,
			attempts = event.attempts + 1
		WHERE event.id = (
			SELECT due.id FROM calendar.holiday_closure_events AS due
			WHERE due.organization_id = ?
				AND ((due.state = 'pending' AND due.next_attempt_at <= CURRENT_TIMESTAMP)
					OR (due.state = 'leased' AND due.lease_until < CURRENT_TIMESTAMP))
			ORDER BY due.next_attempt_at, due.id
			LIMIT 1
			FOR UPDATE SKIP LOCKED)
			AND event.organization_id = ?
		RETURNING event.id, event.organization_id, event.tenant_id, event.operation_id::text AS operation_id, event.revision,
			event.span_from, event.span_to, event.lease_token::text AS lease_token, event.attempts, event.state`
	lockLeasedHolidayClosureEventSQL = `SELECT id FROM calendar.holiday_closure_events
		WHERE organization_id = ? AND id = ? AND state = 'leased' AND lease_token = ?::uuid
		FOR UPDATE`
	finishHolidayClosureEventSQL = `UPDATE calendar.holiday_closure_events
		SET state = ?, error_code = NULLIF(?, ''), lease_token = NULL, lease_until = NULL,
			processed_at = CURRENT_TIMESTAMP,
			completed_at = CASE WHEN ? IN ('completed', 'no_mail') THEN CURRENT_TIMESTAMP ELSE NULL END,
			next_attempt_at = COALESCE(?, next_attempt_at)
		WHERE organization_id = ? AND id = ? AND state = 'leased' AND lease_token = ?::uuid`
	holidayClosureCalendarSQL = `SELECT kind, source_id, start_date, end_date, reason, school_name, federal_state, revision
		FROM calendar.traeger_holiday_closure_calendar(?, ?, ?)`
	supersededHolidayClosureEventSQL = `SELECT EXISTS (SELECT 1 FROM calendar.holiday_closure_proposals
		WHERE organization_id = ? AND supersedes_event_id = ?)`
	waitingHolidayClosureEventsSQL = `SELECT event.id, event.organization_id, event.tenant_id, event.operation_id::text AS operation_id,
			event.revision, event.span_from, event.span_to, '' AS lease_token, event.attempts, event.state
		FROM calendar.holiday_closure_events AS event
		WHERE event.organization_id = ? AND event.state IN ('prepared', 'unknown')
		ORDER BY event.id`
	settleHolidayClosureEventSQL = `UPDATE calendar.holiday_closure_events
		SET state = ?, completed_at = CASE WHEN ? = 'completed' THEN CURRENT_TIMESTAMP ELSE completed_at END
		WHERE organization_id = ? AND id = ? AND state IN ('prepared', 'unknown')`
	retireHolidayClosureEventsSQL = `DELETE FROM calendar.holiday_closure_proposals AS proposal
		WHERE proposal.organization_id = ?
			AND ((proposal.state = 'open' AND proposal.span_to < ?)
				OR (proposal.state = 'confirmed' AND EXISTS (SELECT 1 FROM calendar.holiday_closure_events AS event
					WHERE event.proposal_id = proposal.id AND event.organization_id = ?
						AND event.state IN ('completed', 'no_mail', 'needs_confirmation')
						AND COALESCE(event.completed_at, event.processed_at) < CURRENT_TIMESTAMP - interval '30 days')))`
	dueHolidayClosureTraegerSQL = `SELECT id FROM calendar.holiday_closure_due_traeger_ids() AS id`
)

// HolidayClosureEventStore persists the proposals and commands and serves
// School Calendar's proposal and worker side of them.
type HolidayClosureEventStore struct {
	school Database
	today  func() calendar.Date
}

var (
	_ schoolcalendar.HolidayClosureProposals  = (*HolidayClosureEventStore)(nil)
	_ schoolcalendar.HolidayClosureEventQueue = (*HolidayClosureEventStore)(nil)
)

// NewHolidayClosureEventStore builds the store; school resolves the
// caller's school transaction, today names the Berlin day (nil: today).
func NewHolidayClosureEventStore(school Database, today func() calendar.Date) *HolidayClosureEventStore {
	if school == nil {
		panic("school calendar postgres: the school database is required")
	}
	if today == nil {
		today = calendar.TodayDate
	}
	return &HolidayClosureEventStore{school: school, today: today}
}

func (s *HolidayClosureEventStore) schoolDB(ctx context.Context) (bun.IDB, int64, error) {
	db, schoolID, err := s.school(ctx)
	if err != nil {
		return nil, 0, err
	}
	if schoolID <= 0 {
		return nil, 0, errors.New("school calendar postgres: the school transaction is required")
	}
	return db, schoolID, nil
}

// BumpHolidayClosureRevision raises the school's revision and holds its row
// until the school transaction ends: the calendar lock of ADR 0196.
func BumpHolidayClosureRevision(ctx context.Context, db bun.IDB, schoolID int64) (int64, error) {
	var revision int64
	if err := db.NewRaw(bumpHolidayClosureRevisionSQL, schoolID).Scan(ctx, &revision); err != nil {
		return 0, fmt.Errorf("school calendar postgres: raise the closing day revision: %w", err)
	}
	return revision, nil
}

type openProposalRow struct {
	ID                int64          `bun:"id"`
	PreviousStart     *calendar.Date `bun:"previous_start"`
	PreviousEnd       *calendar.Date `bun:"previous_end"`
	SpanFrom          calendar.Date  `bun:"span_from"`
	SpanTo            calendar.Date  `bun:"span_to"`
	SupersedesEventID *int64         `bun:"supersedes_event_id"`
}

type eventSpanRow struct {
	ID       int64         `bun:"id"`
	SpanFrom calendar.Date `bun:"span_from"`
	SpanTo   calendar.Date `bun:"span_to"`
}

// RecordHolidayClosureChange merges a change of a school closing day into
// the open proposal of that closing day, or opens one. A change back to the
// state before the open proposal drops it again.
func RecordHolidayClosureChange(ctx context.Context, db bun.IDB, schoolID int64, change domain.HolidayClosureChange) error {
	var open openProposalRow
	err := db.NewRaw(openHolidayClosureProposalSQL, schoolID, change.SourceKind, change.SourceID).Scan(ctx, &open)
	switch {
	case err == nil:
		previous := rangeOf(open.PreviousStart, open.PreviousEnd)
		if sameRange(previous, change.Current) && open.SupersedesEventID == nil {
			_, err = db.NewRaw(dropHolidayClosureProposalSQL, schoolID, open.ID).Exec(ctx)
			return wrapProposal(err)
		}
		from, to := change.Span()
		_, err = db.NewRaw(mergeHolidayClosureProposalSQL, change.Reason, startOf(change.Current), endOf(change.Current), from, to,
			change.Revision, schoolID, open.ID).Exec(ctx)
		return wrapProposal(err)
	case !errors.Is(err, sql.ErrNoRows):
		return wrapProposal(err)
	}
	if sameRange(change.Previous, change.Current) {
		return nil
	}
	from, to := change.Span()
	var supersedes *int64
	var unprocessed eventSpanRow
	switch err := db.NewRaw(unprocessedHolidayClosureEventSQL, schoolID, change.SourceKind, change.SourceID).Scan(ctx, &unprocessed); {
	case err == nil:
		supersedes = &unprocessed.ID
		from, to = min(from, unprocessed.SpanFrom), max(to, unprocessed.SpanTo)
	case !errors.Is(err, sql.ErrNoRows):
		return wrapProposal(err)
	}
	_, err = db.NewRaw(insertHolidayClosureProposalSQL, schoolID, change.SourceKind, change.SourceID, change.Reason,
		startOf(change.Previous), endOf(change.Previous), startOf(change.Current), endOf(change.Current), from, to,
		change.Revision, supersedes).Exec(ctx)
	return wrapProposal(err)
}

func wrapProposal(err error) error {
	if err != nil {
		return fmt.Errorf("school calendar postgres: record the holiday closure proposal: %w", err)
	}
	return nil
}

func rangeOf(start, end *calendar.Date) *domain.HolidayClosureRange {
	if start == nil || end == nil {
		return nil
	}
	return &domain.HolidayClosureRange{StartsOn: *start, EndsOn: *end}
}

func sameRange(a, b *domain.HolidayClosureRange) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func startOf(r *domain.HolidayClosureRange) *calendar.Date {
	if r == nil {
		return nil
	}
	return &r.StartsOn
}

func endOf(r *domain.HolidayClosureRange) *calendar.Date {
	if r == nil {
		return nil
	}
	return &r.EndsOn
}

type holidayClosureNoticeRow struct {
	bun.BaseModel     `bun:"table:calendar.holiday_closure_proposals"`
	ProposalID        int64          `bun:"proposal_id"`
	TenantID          int64          `bun:"tenant_id"`
	OrganizationID    int64          `bun:"organization_id"`
	SourceID          int64          `bun:"source_id"`
	Reason            string         `bun:"reason"`
	PreviousStart     *calendar.Date `bun:"previous_start"`
	PreviousEnd       *calendar.Date `bun:"previous_end"`
	CurrentStart      *calendar.Date `bun:"current_start"`
	CurrentEnd        *calendar.Date `bun:"current_end"`
	SpanFrom          calendar.Date  `bun:"span_from"`
	SpanTo            calendar.Date  `bun:"span_to"`
	Revision          int64          `bun:"revision"`
	OperationID       string         `bun:"operation_id"`
	ProposalState     string         `bun:"proposal_state"`
	SupersedesEventID *int64         `bun:"supersedes_event_id"`
	UpdatedAt         time.Time      `bun:"updated_at"`
	EventID           *int64         `bun:"event_id"`
	EventState        *string        `bun:"event_state"`
	ConfirmedAt       *time.Time     `bun:"confirmed_at"`
}

func (row holidayClosureNoticeRow) notice() schoolcalendar.HolidayClosureNotice {
	notice := schoolcalendar.HolidayClosureNotice{
		ProposalID: row.ProposalID, SchoolID: row.TenantID, OrganizationID: row.OrganizationID, ClosingDayID: row.SourceID,
		Reason: row.Reason, SpanFrom: row.SpanFrom, SpanTo: row.SpanTo, Revision: row.Revision, OperationID: row.OperationID,
		Changed: row.SupersedesEventID != nil, ConfirmedAt: row.ConfirmedAt, UpdatedAt: row.UpdatedAt,
	}
	if r := rangeOf(row.PreviousStart, row.PreviousEnd); r != nil {
		notice.Previous = &schoolcalendar.HolidayClosureRange{StartsOn: r.StartsOn, EndsOn: r.EndsOn}
	}
	if r := rangeOf(row.CurrentStart, row.CurrentEnd); r != nil {
		notice.Current = &schoolcalendar.HolidayClosureRange{StartsOn: r.StartsOn, EndsOn: r.EndsOn}
	}
	if row.EventID != nil {
		notice.EventID = *row.EventID
	}
	state := ""
	if row.EventState != nil {
		state = *row.EventState
	}
	notice.Status = holidayClosureStatusOf(row.ProposalState, state)
	return notice
}

func notices(rows []holidayClosureNoticeRow) []schoolcalendar.HolidayClosureNotice {
	result := make([]schoolcalendar.HolidayClosureNotice, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.notice())
	}
	return result
}

// SchoolHolidayClosureNotices lists the visible notices of the school.
func (s *HolidayClosureEventStore) SchoolHolidayClosureNotices(ctx context.Context) ([]schoolcalendar.HolidayClosureNotice, error) {
	db, schoolID, err := s.schoolDB(ctx)
	if err != nil {
		return nil, err
	}
	var rows []holidayClosureNoticeRow
	if err := db.NewRaw(schoolHolidayClosureNoticesSQL, schoolID, s.today()).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("school calendar postgres: list the holiday closure notices of a school: %w", err)
	}
	return notices(rows), nil
}

// TraegerHolidayClosureNotices lists the visible notices of the Träger. Row
// security keeps them to its current schools; the caller checks the
// membership once more against Organisation & Tenancy.
func (s *HolidayClosureEventStore) TraegerHolidayClosureNotices(ctx context.Context, organizationID int64) ([]schoolcalendar.HolidayClosureNotice, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []holidayClosureNoticeRow
	if err := db.NewRaw(traegerHolidayClosureNoticesSQL, organizationID, s.today()).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("school calendar postgres: list the holiday closure notices of a Träger: %w", err)
	}
	return notices(rows), nil
}

// ConfirmHolidayClosureNotice confirms the proposal under its row lock or
// takes its failed command up again. The lock serializes the clicks of the
// school and the Träger-Büro: the second finds the command of the first.
func (s *HolidayClosureEventStore) ConfirmHolidayClosureNotice(ctx context.Context, proposalID, revision int64, actor schoolcalendar.HolidayClosureActor) (schoolcalendar.HolidayClosureNotice, error) {
	if proposalID <= 0 {
		return schoolcalendar.HolidayClosureNotice{}, schoolcalendar.ErrHolidayClosureNoticeNotFound
	}
	if actor.Channel != schoolcalendar.HolidayClosureChannelSchool && actor.Channel != schoolcalendar.HolidayClosureChannelTraeger {
		return schoolcalendar.HolidayClosureNotice{}, errors.New("school calendar postgres: the channel of the click is required")
	}
	db, schoolID, err := s.schoolDB(ctx)
	if err != nil {
		return schoolcalendar.HolidayClosureNotice{}, err
	}
	var row holidayClosureNoticeRow
	if err := db.NewRaw(lockHolidayClosureProposalSQL, schoolID, proposalID).Scan(ctx, &row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return schoolcalendar.HolidayClosureNotice{}, schoolcalendar.ErrHolidayClosureNoticeNotFound
		}
		return schoolcalendar.HolidayClosureNotice{}, fmt.Errorf("school calendar postgres: lock the holiday closure proposal: %w", err)
	}
	if actor.Channel == schoolcalendar.HolidayClosureChannelTraeger && row.OrganizationID != actor.OrganizationID {
		return schoolcalendar.HolidayClosureNotice{}, schoolcalendar.ErrHolidayClosureNoticeNotFound
	}
	if err := confirmOrRetry(ctx, db, schoolID, proposalID, revision, actor, row); err != nil {
		if errors.Is(err, schoolcalendar.ErrHolidayClosureNoticeChanged) {
			return row.notice(), err
		}
		return schoolcalendar.HolidayClosureNotice{}, err
	}
	var confirmed holidayClosureNoticeRow
	if err := db.NewRaw(lockHolidayClosureProposalSQL, schoolID, proposalID).Scan(ctx, &confirmed); err != nil {
		return schoolcalendar.HolidayClosureNotice{}, fmt.Errorf("school calendar postgres: read the confirmed proposal: %w", err)
	}
	return confirmed.notice(), nil
}

// confirmOrRetry writes the command of an open proposal at its revision, or
// takes the failed command of a confirmed one up again.
func confirmOrRetry(ctx context.Context, db bun.IDB, schoolID, proposalID, revision int64, actor schoolcalendar.HolidayClosureActor, row holidayClosureNoticeRow) error {
	if row.ProposalState != "open" {
		if _, err := db.NewRaw(retryHolidayClosureEventSQL, actor.Channel, schoolID, proposalID).Exec(ctx); err != nil {
			return fmt.Errorf("school calendar postgres: take the holiday closure event up again: %w", err)
		}
		return nil
	}
	if row.Revision != revision {
		return schoolcalendar.ErrHolidayClosureNoticeChanged
	}
	if _, err := db.NewRaw(insertHolidayClosureEventSQL, actor.Channel, schoolID, proposalID).Exec(ctx); err != nil {
		return fmt.Errorf("school calendar postgres: write the holiday closure event: %w", err)
	}
	if _, err := db.NewRaw(confirmHolidayClosureProposalSQL, schoolID, proposalID).Exec(ctx); err != nil {
		return fmt.Errorf("school calendar postgres: confirm the holiday closure proposal: %w", err)
	}
	return nil
}

type holidayClosureEventRow struct {
	bun.BaseModel  `bun:"table:calendar.holiday_closure_events"`
	ID             int64         `bun:"id"`
	OrganizationID int64         `bun:"organization_id"`
	TenantID       int64         `bun:"tenant_id"`
	OperationID    string        `bun:"operation_id"`
	Revision       int64         `bun:"revision"`
	SpanFrom       calendar.Date `bun:"span_from"`
	SpanTo         calendar.Date `bun:"span_to"`
	LeaseToken     string        `bun:"lease_token"`
	Attempts       int           `bun:"attempts"`
	State          string        `bun:"state"`
}

func (row holidayClosureEventRow) event() schoolcalendar.HolidayClosureEvent {
	return schoolcalendar.HolidayClosureEvent{ID: row.ID, OrganizationID: row.OrganizationID, SchoolID: row.TenantID,
		OperationID: row.OperationID, Revision: row.Revision, SpanFrom: row.SpanFrom, SpanTo: row.SpanTo,
		LeaseToken: row.LeaseToken, Attempts: row.Attempts, State: row.State}
}

// DueHolidayClosureTraeger names the Träger with work through the narrow
// definer function; it needs a transaction of any kind.
func (s *HolidayClosureEventStore) DueHolidayClosureTraeger(ctx context.Context) ([]int64, error) {
	raw, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("school calendar postgres: a transaction is required")
	}
	tx, ok := raw.(bun.Tx)
	if !ok {
		return nil, fmt.Errorf("school calendar postgres: unsupported transaction %T", raw)
	}
	var ids []int64
	if err := tx.NewRaw(dueHolidayClosureTraegerSQL).Scan(ctx, &ids); err != nil {
		return nil, fmt.Errorf("school calendar postgres: list the Träger with due holiday closure events: %w", err)
	}
	return ids, nil
}

// LeaseHolidayClosureEvent leases the next due command of the Träger.
func (s *HolidayClosureEventStore) LeaseHolidayClosureEvent(ctx context.Context, organizationID int64, leaseFor time.Duration) (schoolcalendar.HolidayClosureEvent, bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return schoolcalendar.HolidayClosureEvent{}, false, err
	}
	var row holidayClosureEventRow
	err = db.NewRaw(leaseHolidayClosureEventSQL, fmt.Sprintf("%d seconds", int(leaseFor.Seconds())), organizationID, organizationID).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return schoolcalendar.HolidayClosureEvent{}, false, nil
	}
	if err != nil {
		return schoolcalendar.HolidayClosureEvent{}, false, fmt.Errorf("school calendar postgres: lease a holiday closure event: %w", err)
	}
	return row.event(), true, nil
}

// LockLeasedHolidayClosureEvent locks the command under its lease.
func (s *HolidayClosureEventStore) LockLeasedHolidayClosureEvent(ctx context.Context, organizationID, eventID int64, leaseToken string) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	var id int64
	err = db.NewRaw(lockLeasedHolidayClosureEventSQL, organizationID, eventID, leaseToken).Scan(ctx, &id)
	if errors.Is(err, sql.ErrNoRows) {
		return schoolcalendar.ErrHolidayClosureLeaseLost
	}
	if err != nil {
		return fmt.Errorf("school calendar postgres: lock the leased holiday closure event: %w", err)
	}
	return nil
}

type holidayClosureCalendarRow struct {
	Kind         string          `bun:"kind"`
	SourceID     int64           `bun:"source_id"`
	StartDate    *calendar.Date  `bun:"start_date"`
	EndDate      *calendar.Date  `bun:"end_date"`
	Reason       *string         `bun:"reason"`
	SchoolName   *string         `bun:"school_name"`
	FederalState json.RawMessage `bun:"federal_state"`
	Revision     *int64          `bun:"revision"`
}

// HolidayClosureCalendar reads a current school of the Träger through the
// read channel of the worker.
func (s *HolidayClosureEventStore) HolidayClosureCalendar(ctx context.Context, organizationID, schoolID int64, from, to calendar.Date) (schoolcalendar.HolidayClosureCalendar, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return schoolcalendar.HolidayClosureCalendar{}, err
	}
	var rows []holidayClosureCalendarRow
	if err := db.NewRaw(holidayClosureCalendarSQL, schoolID, from, to).Scan(ctx, &rows); err != nil {
		return schoolcalendar.HolidayClosureCalendar{}, fmt.Errorf("school calendar postgres: read the calendar of a holiday closure: %w", err)
	}
	result, err := holidayClosureCalendarOf(rows)
	if err != nil {
		return schoolcalendar.HolidayClosureCalendar{}, err
	}
	if result.SchoolID != schoolID {
		return schoolcalendar.HolidayClosureCalendar{}, schoolcalendar.ErrHolidayClosureNoticeNotFound
	}
	return result, nil
}

// holidayClosureCalendarOf reads the header row and the closing days of the
// read channel.
func holidayClosureCalendarOf(rows []holidayClosureCalendarRow) (schoolcalendar.HolidayClosureCalendar, error) {
	result := schoolcalendar.HolidayClosureCalendar{}
	for _, row := range rows {
		if row.Kind != "school" {
			if day, ok := row.closingDay(); ok {
				result.ClosingDays = append(result.ClosingDays, day)
			}
			continue
		}
		result.SchoolID = row.SourceID
		result.SchoolName = valueOf(row.SchoolName)
		if row.Revision != nil {
			result.Revision = *row.Revision
		}
		if len(row.FederalState) > 0 && string(row.FederalState) != "null" {
			if err := json.Unmarshal(row.FederalState, &result.FederalState); err != nil {
				return schoolcalendar.HolidayClosureCalendar{}, fmt.Errorf("%w: %w", schoolcalendar.ErrFederalStateUnavailable, err)
			}
		}
	}
	return result, nil
}

func (row holidayClosureCalendarRow) closingDay() (schoolcalendar.HolidayClosureCalendarDay, bool) {
	if (row.Kind != "closing" && row.Kind != "traeger_closing") || row.StartDate == nil || row.EndDate == nil {
		return schoolcalendar.HolidayClosureCalendarDay{}, false
	}
	return schoolcalendar.HolidayClosureCalendarDay{SourceID: row.SourceID, Traeger: row.Kind == "traeger_closing",
		StartsOn: *row.StartDate, EndsOn: *row.EndDate, Reason: valueOf(row.Reason)}, true
}

func valueOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// HolidayClosureEventSuperseded reports a later change of the same closing
// day that replaced the command before it was processed.
func (s *HolidayClosureEventStore) HolidayClosureEventSuperseded(ctx context.Context, organizationID, eventID int64) (bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return false, err
	}
	var superseded bool
	if err := db.NewRaw(supersededHolidayClosureEventSQL, organizationID, eventID).Scan(ctx, &superseded); err != nil {
		return false, fmt.Errorf("school calendar postgres: check the holiday closure event: %w", err)
	}
	return superseded, nil
}

// FinishHolidayClosureEvent writes the outcome of a leased command.
func (s *HolidayClosureEventStore) FinishHolidayClosureEvent(ctx context.Context, organizationID, eventID int64, leaseToken string, outcome schoolcalendar.HolidayClosureOutcome) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	var retryAt *time.Time
	if !outcome.RetryAt.IsZero() {
		retryAt = &outcome.RetryAt
	}
	result, err := db.NewRaw(finishHolidayClosureEventSQL, outcome.State, outcome.ErrorCode, outcome.State, retryAt,
		organizationID, eventID, leaseToken).Exec(ctx)
	if err != nil {
		return fmt.Errorf("school calendar postgres: finish the holiday closure event: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return schoolcalendar.ErrHolidayClosureLeaseLost
	}
	return nil
}

// WaitingHolidayClosureEvents lists the prepared and unknown commands.
func (s *HolidayClosureEventStore) WaitingHolidayClosureEvents(ctx context.Context, organizationID int64) ([]schoolcalendar.HolidayClosureEvent, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []holidayClosureEventRow
	if err := db.NewRaw(waitingHolidayClosureEventsSQL, organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("school calendar postgres: list the waiting holiday closure events: %w", err)
	}
	events := make([]schoolcalendar.HolidayClosureEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, row.event())
	}
	return events, nil
}

// SettleHolidayClosureEvent moves a prepared or unknown command on.
func (s *HolidayClosureEventStore) SettleHolidayClosureEvent(ctx context.Context, organizationID, eventID int64, state string) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(settleHolidayClosureEventSQL, state, state, organizationID, eventID).Exec(ctx); err != nil {
		return fmt.Errorf("school calendar postgres: settle the holiday closure event: %w", err)
	}
	return nil
}

// RetireHolidayClosureEvents removes finished commands after 30 days with
// their proposals, and proposals nobody confirmed 30 days after their last
// day. A failed command stays until it is taken up.
func (s *HolidayClosureEventStore) RetireHolidayClosureEvents(ctx context.Context, organizationID int64) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	result, err := db.NewRaw(retireHolidayClosureEventsSQL, organizationID, s.today().AddDays(-30), organizationID).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("school calendar postgres: retire holiday closure events: %w", err)
	}
	n, err := result.RowsAffected()
	return n, err
}

// LockHolidayClosureRevision raises the school's closing day revision in
// the school transaction (ADR 0196).
func (s *Store) LockHolidayClosureRevision(ctx context.Context) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.tenantDatabase(ctx, "change a closing day")
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	started := time.Now()
	revision, err := BumpHolidayClosureRevision(ctx, db, tenantID)
	return revision, domain.OperationStats{Queries: 1, Rows: 1, StatementDuration: time.Since(started)}, err
}

// RecordHolidayClosureChange proposes the notice of a changed closing day.
func (s *Store) RecordHolidayClosureChange(ctx context.Context, change domain.HolidayClosureChange) (domain.OperationStats, error) {
	db, tenantID, err := s.tenantDatabase(ctx, "propose a holiday closure notice")
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	err = RecordHolidayClosureChange(ctx, db, tenantID, change)
	return domain.OperationStats{Queries: 3, StatementDuration: time.Since(started)}, err
}

// holidayClosureStatusOf names what both places show of a proposal and the
// state of its command (ADR 0196 section 5).
func holidayClosureStatusOf(proposalState, eventState string) schoolcalendar.HolidayClosureStatus {
	if proposalState == "open" || eventState == "" {
		return schoolcalendar.HolidayClosureSuggested
	}
	switch eventState {
	case schoolcalendar.HolidayClosureEventCompleted:
		return schoolcalendar.HolidayClosureSent
	case schoolcalendar.HolidayClosureEventNoMail, schoolcalendar.HolidayClosureEventNeedsConfirmation:
		return schoolcalendar.HolidayClosureNoMail
	case schoolcalendar.HolidayClosureEventFailed, schoolcalendar.HolidayClosureEventBlocked:
		return schoolcalendar.HolidayClosureFailed
	case schoolcalendar.HolidayClosureEventUnknown:
		return schoolcalendar.HolidayClosureUnknown
	default:
		return schoolcalendar.HolidayClosureSending
	}
}
