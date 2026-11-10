package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Criteria of the allocation of holiday places (#4700). Every statement runs
// in the Träger transaction of the Träger it names and names it once more in
// its filter, below the Träger policy.

// The sort key of a booking by the criteria of its period (#4700) is one
// flag per active criterion in the order of their priority, false when the
// answer that counts (the Büro's check, else the parents' statement) says
// yes. Arrays sort element by element, so a booking that meets a criterion
// of higher priority comes first; an unknown answer counts as not met. A
// period by arrival has the empty key, which keeps the order of arrival
// (#4699). The surrounding statement names the registration "registration";
// holidayCriteriaOrderHead + <alias of the child> + holidayCriteriaOrderTail
// is the key.
const (
	holidayCriteriaOrderHead = `(SELECT COALESCE(
			array_agg(NOT COALESCE(answer.checked, answer.stated, false) ORDER BY criterion.position, criterion.id), '{}'::boolean[])
		FROM enrollment.holiday_periods AS ordered_period
		JOIN enrollment.holiday_allocation_criteria AS criterion
			ON criterion.organization_id = ordered_period.organization_id AND criterion.active AND criterion.archived_at IS NULL
		LEFT JOIN enrollment.holiday_registration_criteria AS answer
			ON answer.organization_id = criterion.organization_id AND answer.criterion_id = criterion.id
			AND answer.holiday_registration_id = registration.id
			AND answer.holiday_registration_child_id IS NOT DISTINCT FROM (CASE WHEN criterion.scope = 'child' THEN `
	holidayCriteriaOrderTail = `.id END)
		WHERE ordered_period.organization_id = registration.organization_id AND ordered_period.id = registration.holiday_period_id
			AND ordered_period.allocation_mode = 'criteria')`
	// holidayCriteriaOrderKey is the key of a child named "child".
	holidayCriteriaOrderKey = holidayCriteriaOrderHead + "child" + holidayCriteriaOrderTail
)

const (
	holidayCriteriaQuery = `SELECT criterion.id, criterion.name, criterion.question, criterion.scope, criterion.active, criterion.position,
			EXISTS (SELECT 1 FROM enrollment.holiday_registration_criteria AS answer
				WHERE answer.organization_id = criterion.organization_id AND answer.criterion_id = criterion.id) AS answered
		FROM enrollment.holiday_allocation_criteria AS criterion
		WHERE criterion.organization_id = ? AND criterion.archived_at IS NULL
		ORDER BY criterion.position, criterion.id`
	// archiveHolidayCriteriaQuery archives every criterion the list leaves
	// out; answers and decisions keep naming it.
	archiveHolidayCriteriaQuery = `UPDATE enrollment.holiday_allocation_criteria
		SET archived_at = statement_timestamp(), active = false, updated_at = statement_timestamp(), updated_by_account_id = NULLIF(?::bigint, 0)
		WHERE organization_id = ? AND archived_at IS NULL AND NOT (id = ANY(?::bigint[]))`
	updateHolidayCriterionQuery = `UPDATE enrollment.holiday_allocation_criteria
		SET name = ?, question = ?, scope = ?, active = ?, position = ?, updated_at = statement_timestamp(), updated_by_account_id = NULLIF(?::bigint, 0)
		WHERE organization_id = ? AND id = ? AND archived_at IS NULL`
	insertHolidayCriterionQuery = `INSERT INTO enrollment.holiday_allocation_criteria
			(organization_id, name, question, scope, active, position, updated_by_account_id)
		VALUES (?, ?, ?, ?, ?, ?, NULLIF(?::bigint, 0))`
	// lockHolidayCriteriaQuery holds the criteria of the Träger while they
	// are written, so two writes of the list run one after the other.
	lockHolidayCriteriaQuery = `SELECT id FROM enrollment.holiday_allocation_criteria
		WHERE organization_id = ? AND archived_at IS NULL ORDER BY id FOR UPDATE`
	insertHolidayCriterionAnswersQuery = `INSERT INTO enrollment.holiday_registration_criteria
			(organization_id, holiday_period_id, holiday_registration_id, holiday_registration_child_id, criterion_id, stated)
		SELECT ?, ?, ?, NULLIF(answer.child_id, 0), answer.criterion_id, answer.stated
		FROM unnest(?::bigint[], ?::bigint[], ?::boolean[]) AS answer(child_id, criterion_id, stated)
		JOIN enrollment.holiday_allocation_criteria AS criterion
			ON criterion.organization_id = ? AND criterion.id = answer.criterion_id AND criterion.archived_at IS NULL`
	holidayCriterionAnswersQuery = `SELECT answer.holiday_registration_id, answer.holiday_registration_child_id, answer.criterion_id,
			answer.stated, answer.checked, answer.checked_at
		FROM enrollment.holiday_registration_criteria AS answer
		WHERE answer.organization_id = ? AND answer.holiday_period_id = ? AND (?::bigint = 0 OR answer.holiday_registration_id = ?)
		ORDER BY answer.holiday_registration_id, answer.criterion_id, answer.holiday_registration_child_id NULLS FIRST`
	// checkHolidayCriterionQuery writes the Büro's check of one answer; a
	// check of an answer the parents never gave adds it without a
	// statement. The child must be one of the registration.
	checkHolidayCriterionQuery = `INSERT INTO enrollment.holiday_registration_criteria AS answer
			(organization_id, holiday_period_id, holiday_registration_id, holiday_registration_child_id, criterion_id,
			 checked, checked_by_account_id, checked_at)
		SELECT registration.organization_id, registration.holiday_period_id, registration.id, NULLIF(?::bigint, 0), criterion.id,
			?::boolean, NULLIF(?::bigint, 0), statement_timestamp()
		FROM enrollment.holiday_registrations AS registration
		JOIN enrollment.holiday_allocation_criteria AS criterion
			ON criterion.organization_id = registration.organization_id AND criterion.id = ?
		WHERE registration.organization_id = ? AND registration.holiday_period_id = ? AND registration.id = ?
			AND (?::bigint = 0 OR EXISTS (SELECT 1 FROM enrollment.holiday_registration_children AS child
				WHERE child.organization_id = registration.organization_id AND child.holiday_registration_id = registration.id AND child.id = ?))
		ON CONFLICT (holiday_registration_id, criterion_id, holiday_registration_child_id)
			DO UPDATE SET checked = EXCLUDED.checked, checked_by_account_id = EXCLUDED.checked_by_account_id, checked_at = EXCLUDED.checked_at`
	// uncheckHolidayCriterionQuery takes a check back; an answer the parents
	// never gave goes with it.
	uncheckHolidayCriterionQuery = `WITH unchecked AS (
			UPDATE enrollment.holiday_registration_criteria
			SET checked = NULL, checked_by_account_id = NULL, checked_at = NULL
			WHERE organization_id = ? AND holiday_period_id = ? AND holiday_registration_id = ? AND criterion_id = ?
				AND holiday_registration_child_id IS NOT DISTINCT FROM NULLIF(?::bigint, 0) AND stated IS NOT NULL
		)
		DELETE FROM enrollment.holiday_registration_criteria
		WHERE organization_id = ? AND holiday_period_id = ? AND holiday_registration_id = ? AND criterion_id = ?
			AND holiday_registration_child_id IS NOT DISTINCT FROM NULLIF(?::bigint, 0) AND stated IS NULL`
)

type holidayCriterionRow struct {
	ID       int64  `bun:"id"`
	Name     string `bun:"name"`
	Question string `bun:"question"`
	Scope    string `bun:"scope"`
	Active   bool   `bun:"active"`
	Position int    `bun:"position"`
	Answered bool   `bun:"answered"`
}

// ReadHolidayCriteria reads the criteria of the Träger by priority, archived
// ones left out.
func (s *HolidayRegistrationStore) ReadHolidayCriteria(ctx context.Context, organizationID int64) ([]enrollment.HolidayCriterion, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []holidayCriterionRow
	if err := db.NewRaw(holidayCriteriaQuery, organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday criteria: %w", err)
	}
	criteria := make([]enrollment.HolidayCriterion, len(rows))
	for i, row := range rows {
		criteria[i] = enrollment.HolidayCriterion(row)
	}
	return criteria, nil
}

// WriteHolidayCriteria makes criteria the criteria of the Träger in their
// order: a known ID is changed, a zero ID added, every other one archived.
// An ID of another Träger or an archived one changes nothing.
func (s *HolidayRegistrationStore) WriteHolidayCriteria(ctx context.Context, organizationID int64, criteria []enrollment.HolidayCriterionInput, actorAccountID int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	var locked []int64
	if err := db.NewRaw(lockHolidayCriteriaQuery, organizationID).Scan(ctx, &locked); err != nil {
		return fmt.Errorf("enrollment postgres: lock holiday criteria: %w", err)
	}
	kept := []int64{}
	for _, criterion := range criteria {
		if criterion.ID > 0 {
			kept = append(kept, criterion.ID)
		}
	}
	if _, err := db.NewRaw(archiveHolidayCriteriaQuery, actorAccountID, organizationID, pgdialect.Array(kept)).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: archive holiday criteria: %w", err)
	}
	for i, criterion := range criteria {
		query, args := insertHolidayCriterionQuery, []any{organizationID, criterion.Name, criterion.Question, criterion.Scope, criterion.Active, i + 1, actorAccountID}
		if criterion.ID > 0 {
			query, args = updateHolidayCriterionQuery, []any{criterion.Name, criterion.Question, criterion.Scope, criterion.Active, i + 1, actorAccountID, organizationID, criterion.ID}
		}
		if _, err := db.NewRaw(query, args...).Exec(ctx); err != nil {
			return fmt.Errorf("enrollment postgres: write holiday criterion: %w", err)
		}
	}
	return nil
}

// insertHolidayCriterionAnswers stores the parents' answers of a new
// registration (#4700): the family's, then each child's by the child IDs in
// the order of the form, in one statement. An answer to an archived
// criterion or one of another Träger is left out.
func insertHolidayCriterionAnswers(ctx context.Context, db bun.IDB, organizationID, periodID, registrationID int64, input enrollment.HolidayRegistrationInput, childIDs []int64) error {
	var children, criteria []int64
	var stated []bool
	add := func(childID int64, answers map[int64]bool) {
		for _, criterionID := range slices.Sorted(maps.Keys(answers)) {
			children = append(children, childID)
			criteria = append(criteria, criterionID)
			stated = append(stated, answers[criterionID])
		}
	}
	add(0, input.Criteria)
	for i, child := range input.Children {
		add(childIDs[i], child.Criteria)
	}
	if len(criteria) == 0 {
		return nil
	}
	if _, err := db.NewRaw(insertHolidayCriterionAnswersQuery, organizationID, periodID, registrationID,
		pgdialect.Array(children), pgdialect.Array(criteria), pgdialect.Array(stated), organizationID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: insert holiday criterion answers: %w", err)
	}
	return nil
}

type holidayCriterionAnswerRow struct {
	RegistrationID int64      `bun:"holiday_registration_id"`
	ChildID        *int64     `bun:"holiday_registration_child_id"`
	CriterionID    int64      `bun:"criterion_id"`
	Stated         *bool      `bun:"stated"`
	Checked        *bool      `bun:"checked"`
	CheckedAt      *time.Time `bun:"checked_at"`
}

// ReadHolidayCriterionAnswers reads the answers of the registrations of a
// period, or of the one registrationID names, archived criteria included.
func (s *HolidayRegistrationStore) ReadHolidayCriterionAnswers(ctx context.Context, organizationID, periodID, registrationID int64) ([]enrollment.HolidayCriterionAnswer, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []holidayCriterionAnswerRow
	if err := db.NewRaw(holidayCriterionAnswersQuery, organizationID, periodID, registrationID, registrationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday criterion answers: %w", err)
	}
	answers := make([]enrollment.HolidayCriterionAnswer, len(rows))
	for i, row := range rows {
		answers[i] = enrollment.HolidayCriterionAnswer(row)
	}
	return answers, nil
}

// CheckHolidayCriterion writes or takes back the Büro's check of one answer
// of a registration of the period; childID is zero for the family. found is
// false when the registration, the criterion or the child is not one of the
// Träger's.
func (s *HolidayRegistrationStore) CheckHolidayCriterion(ctx context.Context, organizationID, periodID, registrationID, criterionID, childID int64, checked *bool, actorAccountID int64) (bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return false, err
	}
	if checked == nil {
		if _, err := db.NewRaw(uncheckHolidayCriterionQuery, organizationID, periodID, registrationID, criterionID, childID,
			organizationID, periodID, registrationID, criterionID, childID).Exec(ctx); err != nil {
			return false, fmt.Errorf("enrollment postgres: take back holiday criterion check: %w", err)
		}
		return true, nil
	}
	result, err := db.NewRaw(checkHolidayCriterionQuery, childID, *checked, actorAccountID, criterionID,
		organizationID, periodID, registrationID, childID, childID).Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("enrollment postgres: check holiday criterion: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("enrollment postgres: check holiday criterion: %w", err)
	}
	return rows > 0, nil
}

// holidayPendingBookingsQuery reads the pending bookings of the counting
// registrations of a period in the order of the proposal of each week: the
// criteria, then arrival, number and position (#4700).
const holidayPendingBookingsQuery = `SELECT booking.id AS booking_id, booking.holiday_period_week_id,
		row_number() OVER (PARTITION BY booking.holiday_period_week_id
			ORDER BY ` + holidayCriteriaOrderKey + `, registration.submitted_at, registration.id, child.position) AS rank,
		registration.id AS registration_id, registration.submitted_at, child.id AS child_id, child.position,
		child.first_name, child.last_name, model.name AS time_model_name, booking.days::text AS days
	FROM enrollment.holiday_registration_bookings AS booking
	JOIN enrollment.holiday_registration_children AS child
		ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
	JOIN enrollment.holiday_registrations AS registration
		ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
	JOIN enrollment.holiday_week_time_models AS model
		ON model.organization_id = booking.organization_id AND model.id = booking.holiday_week_time_model_id
	WHERE booking.organization_id = ? AND booking.holiday_period_id = ?
		AND booking.placement = 'pending' AND registration.status IN ` + holidayCountingStatuses + `
	ORDER BY booking.holiday_period_week_id, rank`

type holidayPendingBookingRow struct {
	BookingID      int64     `bun:"booking_id"`
	WeekID         int64     `bun:"holiday_period_week_id"`
	Rank           int       `bun:"rank"`
	RegistrationID int64     `bun:"registration_id"`
	SubmittedAt    time.Time `bun:"submitted_at"`
	ChildID        int64     `bun:"child_id"`
	Position       int       `bun:"position"`
	FirstName      string    `bun:"first_name"`
	LastName       string    `bun:"last_name"`
	TimeModelName  string    `bun:"time_model_name"`
	Days           *string   `bun:"days"`
}

// ReadHolidayPendingBookings reads the pending bookings of the period by
// week in the order of the proposal: one statement for any number of weeks.
func (s *HolidayRegistrationStore) ReadHolidayPendingBookings(ctx context.Context, organizationID, periodID int64) ([]enrollment.HolidayPendingBooking, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []holidayPendingBookingRow
	if err := db.NewRaw(holidayPendingBookingsQuery, organizationID, periodID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read pending holiday bookings: %w", err)
	}
	pending := make([]enrollment.HolidayPendingBooking, len(rows))
	for i, row := range rows {
		days, err := holidayDaysOf(row.Days)
		if err != nil {
			return nil, err
		}
		pending[i] = enrollment.HolidayPendingBooking{
			BookingID: row.BookingID, WeekID: row.WeekID, Rank: row.Rank, RegistrationID: row.RegistrationID,
			SubmittedAt: row.SubmittedAt, ChildID: row.ChildID, ChildPosition: row.Position,
			ChildFirstName: row.FirstName, ChildLastName: row.LastName, TimeModelName: row.TimeModelName, Days: days,
		}
	}
	return pending, nil
}

// decideHolidayBookingsQuery gives the named pending bookings of a locked
// week their place or puts them on the waiting list; a booking that is no
// longer pending stays as it is and is not returned.
const decideHolidayBookingsQuery = `UPDATE enrollment.holiday_registration_bookings AS booking
	SET placement = CASE WHEN booking.id = ANY(?::bigint[]) THEN 'placed' ELSE 'waitlisted' END, promoted_at = NULL
	WHERE booking.organization_id = ? AND booking.holiday_period_week_id = ? AND booking.placement = 'pending'
		AND (booking.id = ANY(?::bigint[]) OR booking.id = ANY(?::bigint[]))
	RETURNING booking.id`

// DecideHolidayBookings gives placed their place and puts waitlisted on the
// waiting list in a week the caller holds, and returns the bookings it
// changed: only pending ones change.
func (s *HolidayRegistrationStore) DecideHolidayBookings(ctx context.Context, organizationID, weekID int64, placed, waitlisted []int64) ([]int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	if placed == nil {
		placed = []int64{}
	}
	if waitlisted == nil {
		waitlisted = []int64{}
	}
	var changed []int64
	if err := db.NewRaw(decideHolidayBookingsQuery, pgdialect.Array(placed), organizationID, weekID,
		pgdialect.Array(placed), pgdialect.Array(waitlisted)).Scan(ctx, &changed); err != nil {
		return nil, fmt.Errorf("enrollment postgres: decide holiday bookings: %w", err)
	}
	return changed, nil
}

const (
	insertHolidayAllocationDecisionsQuery = `INSERT INTO enrollment.holiday_allocation_decisions
			(organization_id, holiday_period_id, holiday_registration_id, holiday_registration_child_id, holiday_period_week_id,
			 week_starts_on, week_ends_on, placement, proposal, rank, criteria_met, reason, decided_by_account_id)
		SELECT ?, ?, decision.registration_id, decision.child_id, decision.week_id, decision.starts_on, decision.ends_on,
			decision.placement, decision.proposal, decision.rank,
			COALESCE((SELECT array_agg(met.name ORDER BY met.ordinality) FROM jsonb_array_elements_text(decision.criteria_met::jsonb)
				WITH ORDINALITY AS met(name, ordinality)), '{}'),
			decision.reason, NULLIF(?::bigint, 0)
		FROM unnest(?::bigint[], ?::bigint[], ?::bigint[], ?::date[], ?::date[], ?::text[], ?::text[], ?::int[], ?::text[], ?::text[])
			AS decision(registration_id, child_id, week_id, starts_on, ends_on, placement, proposal, rank, criteria_met, reason)`
	holidayAllocationDecisionsQuery = `SELECT decision.id, decision.holiday_period_week_id, decision.week_starts_on, decision.week_ends_on,
			decision.holiday_registration_id, decision.holiday_registration_child_id, child.first_name, child.last_name,
			decision.placement, decision.proposal, decision.rank, decision.criteria_met, decision.reason,
			decision.decided_at, decision.decided_by_account_id
		FROM enrollment.holiday_allocation_decisions AS decision
		JOIN enrollment.holiday_registration_children AS child
			ON child.organization_id = decision.organization_id AND child.id = decision.holiday_registration_child_id
		WHERE decision.organization_id = ? AND decision.holiday_period_id = ?
		ORDER BY decision.decided_at DESC, decision.id DESC
		LIMIT 500`
)

// InsertHolidayAllocationDecisions records decisions of the Büro of the
// period in one statement.
func (s *HolidayRegistrationStore) InsertHolidayAllocationDecisions(ctx context.Context, organizationID, periodID int64, decisions []enrollment.HolidayAllocationDecision, actorAccountID int64) error {
	if len(decisions) == 0 {
		return nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	n := len(decisions)
	registrations, children, weeks, ranks := make([]int64, n), make([]int64, n), make([]int64, n), make([]int, n)
	startsOn, endsOn := make([]string, n), make([]string, n)
	placements, proposals, met, reasons := make([]string, n), make([]string, n), make([]string, n), make([]string, n)
	for i, decision := range decisions {
		encoded, err := json.Marshal(decision.CriteriaMet)
		if err != nil {
			return fmt.Errorf("enrollment postgres: encode criteria met: %w", err)
		}
		registrations[i], children[i], weeks[i], ranks[i] = decision.RegistrationID, decision.ChildID, decision.WeekID, decision.Rank
		startsOn[i], endsOn[i] = decision.WeekStartsOn.String(), decision.WeekEndsOn.String()
		placements[i], proposals[i], met[i], reasons[i] = decision.Placement, decision.Proposal, string(encoded), decision.Reason
	}
	if _, err := db.NewRaw(insertHolidayAllocationDecisionsQuery, organizationID, periodID, actorAccountID,
		pgdialect.Array(registrations), pgdialect.Array(children), pgdialect.Array(weeks), pgdialect.Array(startsOn),
		pgdialect.Array(endsOn), pgdialect.Array(placements), pgdialect.Array(proposals), pgdialect.Array(ranks),
		pgdialect.Array(met), pgdialect.Array(reasons)).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: insert holiday allocation decisions: %w", err)
	}
	return nil
}

type holidayAllocationDecisionRow struct {
	ID                 int64         `bun:"id"`
	WeekID             int64         `bun:"holiday_period_week_id"`
	WeekStartsOn       calendar.Date `bun:"week_starts_on,type:date"`
	WeekEndsOn         calendar.Date `bun:"week_ends_on,type:date"`
	RegistrationID     int64         `bun:"holiday_registration_id"`
	ChildID            int64         `bun:"holiday_registration_child_id"`
	ChildFirstName     string        `bun:"first_name"`
	ChildLastName      string        `bun:"last_name"`
	Placement          string        `bun:"placement"`
	Proposal           string        `bun:"proposal"`
	Rank               int           `bun:"rank"`
	CriteriaMet        []string      `bun:"criteria_met,array"`
	Reason             string        `bun:"reason"`
	DecidedAt          time.Time     `bun:"decided_at"`
	DecidedByAccountID *int64        `bun:"decided_by_account_id"`
}

// ReadHolidayAllocationDecisions reads the newest 500 decisions of the
// period, newest first.
func (s *HolidayRegistrationStore) ReadHolidayAllocationDecisions(ctx context.Context, organizationID, periodID int64) ([]enrollment.HolidayAllocationDecision, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []holidayAllocationDecisionRow
	if err := db.NewRaw(holidayAllocationDecisionsQuery, organizationID, periodID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday allocation decisions: %w", err)
	}
	decisions := make([]enrollment.HolidayAllocationDecision, len(rows))
	for i, row := range rows {
		met := row.CriteriaMet
		if met == nil {
			met = []string{}
		}
		decisions[i] = enrollment.HolidayAllocationDecision{
			ID: row.ID, WeekID: row.WeekID, WeekStartsOn: row.WeekStartsOn, WeekEndsOn: row.WeekEndsOn,
			RegistrationID: row.RegistrationID, ChildID: row.ChildID, ChildFirstName: row.ChildFirstName, ChildLastName: row.ChildLastName,
			Placement: row.Placement, Proposal: row.Proposal, Rank: row.Rank, CriteriaMet: met, Reason: row.Reason,
			DecidedAt: row.DecidedAt, DecidedByAccountID: row.DecidedByAccountID,
		}
	}
	return decisions, nil
}
