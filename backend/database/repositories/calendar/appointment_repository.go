package calendar

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	tableAppointments                = "calendar.appointments"
	tableRecurrenceRules             = "calendar.recurrence_rules"
	tableAppointmentRecipients       = "calendar.appointment_recipients"
	tableAppointmentRecipientStudent = "calendar.appointment_recipient_students"
	tableAppointmentTargets          = "calendar.appointment_targets"
	tableAppointmentOverrides        = "calendar.appointment_occurrence_overrides"

	tableExprAppointmentsAsAppointment = `calendar.appointments AS "appointment"`
)

type AppointmentRepository struct {
	*base.Repository[*calModels.Appointment]
}

func NewAppointmentRepository(db *bun.DB) calModels.AppointmentRepository {
	repo := base.NewRepository[*calModels.Appointment](db, tableAppointments, "Appointment")
	repo.TenantScoped = true
	return &AppointmentRepository{Repository: repo}
}

func (r *AppointmentRepository) FindByID(ctx context.Context, id int64) (*calModels.Appointment, error) {
	appointment, err := r.FindByIDOrNil(ctx, id)
	if err != nil {
		return nil, err
	}
	return appointment, nil
}

// FindByIDForUpdate retrieves an appointment while holding its row lock until
// the transaction completes. Lifecycle operations use the same row lock, which
// keeps a per-occurrence change from racing a reminder enqueue.
func (r *AppointmentRepository) FindByIDForUpdate(ctx context.Context, id int64) (*calModels.Appointment, error) {
	appointment := new(calModels.Appointment)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(appointment).
		ModelTableExpr(tableExprAppointmentsAsAppointment).
		Where(`"appointment".id = ?`, id).
		For("UPDATE")
	if where, val, ok := base.TenantWhere(ctx, "appointment"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Preserve the sentinel so lifecycle callers can render a concurrent
			// hard delete as their normal not-found response rather than a 500.
			return nil, fmt.Errorf("appointment %d not found: %w", id, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("lock appointment for update: %w", err)
	}
	return appointment, nil
}

// LockReminderCandidate rechecks the reminder-specific lifecycle predicates
// while obtaining the appointment row lock. A cancellation, deletion, or
// notification opt-out that committed after the scheduler's candidate scan
// therefore suppresses the later outbox insert.
func (r *AppointmentRepository) LockReminderCandidate(ctx context.Context, id int64) (*calModels.Appointment, error) {
	appointment := new(calModels.Appointment)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(appointment).
		ModelTableExpr(tableExprAppointmentsAsAppointment).
		Where(`"appointment".id = ?`, id).
		Where(`"appointment".deleted_at IS NULL`).
		Where(`"appointment".cancelled_at IS NULL`).
		Where(`"appointment".notify_guardians`).
		// A reminder delivery claim references this row. NO KEY UPDATE continues
		// to block lifecycle writes, but permits the FK's KEY SHARE lock so the
		// claim can commit before the external push is sent.
		For("NO KEY UPDATE")
	if where, val, ok := base.TenantWhere(ctx, "appointment"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("lock reminder candidate: %w", err)
	}
	return appointment, nil
}

func (r *AppointmentRepository) ListVisibleForStaff(ctx context.Context, staffID int64, from, to timezone.Date) ([]*calModels.Appointment, error) {
	var rows []*calModels.Appointment
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprAppointmentsAsAppointment).
		Where(`("appointment".organizer_staff_id = ? OR EXISTS (
			SELECT 1
			FROM calendar.appointment_recipients ar
			WHERE ar.appointment_id = "appointment".id
			  AND ar.tenant_id = "appointment".tenant_id
			  AND ar.recipient_type = ?
			  AND ar.staff_id = ?
		))`, staffID, calModels.RecipientTypeStaff, staffID).
		Where(`"appointment".deleted_at IS NULL`).
		OrderExpr(`"appointment".start_date ASC, "appointment".start_time ASC, "appointment".id ASC`)

	query = applyAppointmentWindow(query, from, to)
	if where, val, ok := base.TenantWhere(ctx, "appointment"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list visible staff calendar appointments: %w", err)
	}
	return rows, nil
}

// ListCancellationTombstonesForStaff combines organizer and recipient
// visibility with deletion and cancellation cutoffs; generic filters cannot
// express the required OR and EXISTS clauses.
func (r *AppointmentRepository) ListCancellationTombstonesForStaff(ctx context.Context, staffID int64, since time.Time) ([]*calModels.Appointment, error) {
	var rows []*calModels.Appointment
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprAppointmentsAsAppointment).
		Where(`(
			("appointment".deleted_at IS NOT NULL AND "appointment".deleted_at >= ?)
			OR ("appointment".cancelled_at IS NOT NULL AND "appointment".cancelled_at >= ?)
		)`, since, since).
		Where(`("appointment".organizer_staff_id = ? OR EXISTS (
			SELECT 1
			FROM calendar.appointment_recipients ar
			WHERE ar.appointment_id = "appointment".id
			  AND ar.tenant_id = "appointment".tenant_id
			  AND ar.recipient_type = ?
			  AND ar.staff_id = ?
		))`, staffID, calModels.RecipientTypeStaff, staffID).
		OrderExpr(`"appointment".start_date ASC, "appointment".start_time ASC, "appointment".id ASC`)
	if where, val, ok := base.TenantWhere(ctx, "appointment"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list cancellation staff calendar tombstones: %w", err)
	}
	return rows, nil
}

func (r *AppointmentRepository) ListVisibleForGuardianProfiles(ctx context.Context, guardianProfileIDs []int64, studentIDs []int64, from, to timezone.Date) ([]*calModels.Appointment, error) {
	if len(guardianProfileIDs) == 0 || len(studentIDs) == 0 {
		return []*calModels.Appointment{}, nil
	}

	var rows []*calModels.Appointment
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprAppointmentsAsAppointment).
		Where(`EXISTS (
			SELECT 1
			FROM calendar.appointment_recipients ar
			WHERE ar.appointment_id = "appointment".id
			  AND ar.tenant_id = "appointment".tenant_id
			  AND ar.recipient_type = ?
			  AND ar.guardian_profile_id IN (?)
			  AND EXISTS (
			    SELECT 1
			    FROM calendar.appointment_recipient_students ars
			    WHERE ars.recipient_id = ar.id
			      AND ars.tenant_id = ar.tenant_id
			      AND ars.student_id IN (?)
			  )
			)`, calModels.RecipientTypeGuardianProfile, bun.List(guardianProfileIDs), bun.List(studentIDs)).
		Where(`"appointment".deleted_at IS NULL`).
		OrderExpr(`"appointment".start_date ASC, "appointment".start_time ASC, "appointment".id ASC`)

	query = applyAppointmentWindow(query, from, to)
	if where, val, ok := base.TenantWhere(ctx, "appointment"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list visible guardian calendar appointments: %w", err)
	}
	return rows, nil
}

// ListCancellationTombstonesForGuardianProfiles mirrors the guardian visibility
// join of ListVisibleForGuardianProfiles but returns cancelled OR soft-deleted
// rows whose cancellation/deletion happened on/after `since`, with NO event-date
// window — the subscription feed re-exports these as STATUS:CANCELLED so a
// long-offline subscriber still receives the cancellation and purges the event,
// even when the appointment's own date has aged out of the lookback window.
func (r *AppointmentRepository) ListCancellationTombstonesForGuardianProfiles(ctx context.Context, guardianProfileIDs []int64, studentIDs []int64, since time.Time) ([]*calModels.Appointment, error) {
	if len(guardianProfileIDs) == 0 || len(studentIDs) == 0 {
		return []*calModels.Appointment{}, nil
	}

	var rows []*calModels.Appointment
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprAppointmentsAsAppointment).
		Where(`(
			("appointment".deleted_at IS NOT NULL AND "appointment".deleted_at >= ?)
			OR ("appointment".cancelled_at IS NOT NULL AND "appointment".cancelled_at >= ?)
		)`, since, since).
		Where(`EXISTS (
			SELECT 1
			FROM calendar.appointment_recipients ar
			WHERE ar.appointment_id = "appointment".id
			  AND ar.tenant_id = "appointment".tenant_id
			  AND ar.recipient_type = ?
			  AND ar.guardian_profile_id IN (?)
			  AND EXISTS (
			    SELECT 1
			    FROM calendar.appointment_recipient_students ars
			    WHERE ars.recipient_id = ar.id
			      AND ars.tenant_id = ar.tenant_id
			      AND ars.student_id IN (?)
			  )
			)`, calModels.RecipientTypeGuardianProfile, bun.List(guardianProfileIDs), bun.List(studentIDs)).
		OrderExpr(`"appointment".start_date ASC, "appointment".start_time ASC, "appointment".id ASC`)

	if where, val, ok := base.TenantWhere(ctx, "appointment"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list cancellation guardian calendar tombstones: %w", err)
	}
	return rows, nil
}

func (r *AppointmentRepository) ListOrganizedByStaff(ctx context.Context, staffID int64, from, to timezone.Date) ([]*calModels.Appointment, error) {
	var rows []*calModels.Appointment
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprAppointmentsAsAppointment).
		Where(`"appointment".organizer_staff_id = ?`, staffID).
		Where(`"appointment".deleted_at IS NULL`).
		OrderExpr(`"appointment".start_date ASC, "appointment".start_time ASC, "appointment".id ASC`)

	query = applyAppointmentWindow(query, from, to)
	if where, val, ok := base.TenantWhere(ctx, "appointment"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list organized calendar appointments: %w", err)
	}
	return rows, nil
}

// ListGuardianReminderCandidates returns the appointments a guardian reminder
// could be due for in the window: live, not cancelled, carrying the organizer's
// notify-guardians opt-in, and with at least one guardian recipient.
//
// Unlike the guardian calendar listings this is NOT scoped to one family — the
// reminder tick runs per tenant with no account of its own, so the recipient
// EXISTS check only asserts that guardians are addressed at all. Who exactly
// gets the mail is resolved per appointment afterwards, through the same
// reachability rules the published/updated/cancelled mails use.
func (r *AppointmentRepository) ListGuardianReminderCandidates(ctx context.Context, from, to timezone.Date) ([]*calModels.Appointment, error) {
	var rows []*calModels.Appointment
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprAppointmentsAsAppointment).
		Where(`"appointment".deleted_at IS NULL`).
		Where(`"appointment".cancelled_at IS NULL`).
		Where(`"appointment".notify_guardians`).
		Where(`EXISTS (
			SELECT 1
			FROM calendar.appointment_recipients ar
			WHERE ar.appointment_id = "appointment".id
			  AND ar.tenant_id = "appointment".tenant_id
			  AND ar.recipient_type = ?
			  AND ar.guardian_profile_id IS NOT NULL
			)`, calModels.RecipientTypeGuardianProfile).
		OrderExpr(`"appointment".start_date ASC, "appointment".start_time ASC, "appointment".id ASC`)

	query = query.Where(`(
		(
			"appointment".end_date >= ? AND "appointment".start_date <= ?
		)
		OR EXISTS (
			SELECT 1
			FROM calendar.recurrence_rules rr
			WHERE rr.appointment_id = "appointment".id
			  AND rr.tenant_id = "appointment".tenant_id
			  AND "appointment".start_date <= ?
			  AND (
			    rr.ends_on IS NULL
			    OR rr.ends_on + ("appointment".end_date - "appointment".start_date) >= ?
			  )
			  AND (
			    rr.occurrence_count IS NULL
			    -- Count-bounded rules have no ends_on. Use a conservative final-date
			    -- bound so historical series stop reaching the reminder expansion;
			    -- the service still decides the exact occurrence dates. The extra
			    -- monthly/yearly periods account for invalid calendar days and leap
			    -- years, so this predicate can only retain extra candidates, never
			    -- discard a live occurrence.
			    -- Searched CASE, not "CASE rr.frequency WHEN …": the branches test
			    -- two columns at once, and the simple form would compare
			    -- rr.frequency against the BOOLEAN result of that test — Postgres
			    -- casts 'daily' to boolean and the whole query dies with
			    -- "invalid input syntax for type boolean" before it reads a row.
			    -- Each branch also caps its own span. Only occurrence_count is
			    -- bounded (366); interval_count is not, so a yearly rule with 366
			    -- occurrences and interval_count 10000 aims at year 7.6 million and
			    -- the scan dies with "timestamp out of range" — for the whole tenant,
			    -- not just that appointment, on input the API accepts. The guards
			    -- multiply in bigint so they cannot overflow int themselves; a span
			    -- past the cap falls through to 'infinity', which keeps the
			    -- appointment a candidate and is the safe direction. start_date is
			    -- capped above by the window predicate, so every sum stays well
			    -- inside the date range.
			    OR CASE
			      WHEN rr.frequency = 'daily' AND rr.occurrence_count::bigint * rr.interval_count <= 1000000 THEN "appointment".start_date + (rr.occurrence_count * rr.interval_count)
			      WHEN rr.frequency = 'weekly' AND rr.occurrence_count::bigint * rr.interval_count * 7 <= 1000000 THEN "appointment".start_date + (rr.occurrence_count * rr.interval_count * 7)
			      WHEN rr.frequency = 'monthly' AND (rr.occurrence_count::bigint + 1) * rr.interval_count <= 32000 THEN ("appointment".start_date + make_interval(months => (rr.occurrence_count + 1) * rr.interval_count))::date
			      WHEN rr.frequency = 'yearly' AND (rr.occurrence_count::bigint + 401) * rr.interval_count <= 100000 THEN ("appointment".start_date + make_interval(years => (rr.occurrence_count + 401) * rr.interval_count))::date
			      ELSE 'infinity'::date
			    END + ("appointment".end_date - "appointment".start_date) >= ?
			  )
		)
		OR EXISTS (
			SELECT 1
			FROM calendar.appointment_occurrence_overrides aoo
			WHERE aoo.appointment_id = "appointment".id
			  AND aoo.tenant_id = "appointment".tenant_id
			  AND aoo.start_date BETWEEN ? AND ?
		)
	)`, from, to, to, from, from, from, to)
	if where, val, ok := base.TenantWhere(ctx, "appointment"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list guardian reminder candidates: %w", err)
	}
	return rows, nil
}

// Update overrides the generic repository Update. The appointment carries two
// TIME columns (start_time/end_time) modeled as time.Time; bun's full-model
// UPDATE re-binds those as year-0 timestamptz literals, which PostgreSQL rejects
// ("date/time field value out of range") even though the equivalent INSERT
// coerces them fine. We set the wall-clock columns as HH:MM:SS strings so there
// is no date/year to shift, mirroring the explicit-column TIME update used
// elsewhere in the codebase.
func (r *AppointmentRepository) Update(ctx context.Context, appointment *calModels.Appointment) error {
	appointment.UpdatedAt = time.Now()
	q := base.GetDB(ctx, r.DB).NewUpdate().
		TableExpr(tableExprAppointmentsAsAppointment).
		Set(`title = ?`, appointment.Title).
		Set(`description = ?`, appointment.Description).
		Set(`location = ?`, appointment.Location).
		Set(`start_date = ?`, appointment.StartDate).
		Set(`end_date = ?`, appointment.EndDate).
		Set(`start_time = ?`, timezone.WallClock(appointment.StartTime).Format("15:04:05")).
		Set(`end_time = ?`, timezone.WallClock(appointment.EndTime).Format("15:04:05")).
		Set(`all_day = ?`, appointment.AllDay).
		Set(`delivery_mode = ?`, appointment.DeliveryMode).
		Set(`overview_visibility = ?`, appointment.OverviewVisibility).
		Set(`notify_guardians = ?`, appointment.NotifyGuardians).
		Set(`updated_at = ?`, appointment.UpdatedAt).
		// Every persisted edit bumps the revision so the iCalendar SEQUENCE
		// advances and subscribers treat the event as a newer version.
		Set(`revision = revision + 1`).
		// Deliberately NOT setting cancelled_at/deleted_at: those columns are owned
		// by Cancel()/SoftDelete(). A content edit carries whatever the caller
		// loaded, so writing them here would let an edit that started before a
		// concurrent cancellation reactivate the appointment.
		Where(`"appointment".id = ?`, appointment.ID).
		// Guard the edit against a concurrent lifecycle transition: an edit that
		// began before a cancel/delete must not update a terminal appointment,
		// replace its recurrence, or fire an "updated" notice. A cancelled/deleted
		// row matches zero rows and is reported as a conflict.
		Where(`"appointment".cancelled_at IS NULL`).
		Where(`"appointment".deleted_at IS NULL`)

	if where, val, ok := base.TenantWhere(ctx, "appointment"); ok {
		q = q.Where(where, val)
	}

	// RETURNING the counter rather than incrementing the in-memory copy: the
	// database owns it (BumpRevision advances it from child-table changes too), so
	// the caller — and the PUT response it renders — must read back what was
	// actually persisted instead of a guess. Zero updated rows is the lifecycle
	// conflict guarded above; with a destination, bun reports it as ErrNoRows.
	var revision int
	result, err := q.Returning("revision").Exec(ctx, &revision)
	if errors.Is(err, sql.ErrNoRows) {
		return calModels.ErrAppointmentLifecycleConflict
	}
	if err != nil {
		return fmt.Errorf("update appointment: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update appointment: rows affected: %w", err)
	}
	if affected == 0 {
		return calModels.ErrAppointmentLifecycleConflict
	}
	appointment.Revision = revision
	return nil
}

// BumpRevision advances the appointment's revision (and updated_at) without
// touching any other field. Used when a change that alters the exported
// calendar lives in a child table (a single-occurrence cancellation override),
// so subscribers still see a newer SEQUENCE and honour the new EXDATE.
func (r *AppointmentRepository) BumpRevision(ctx context.Context, appointmentID int64) error {
	q := base.GetDB(ctx, r.DB).NewUpdate().
		TableExpr(tableExprAppointmentsAsAppointment).
		Set(`revision = revision + 1`).
		Set(`updated_at = ?`, time.Now()).
		Where(`"appointment".id = ?`, appointmentID)
	if where, val, ok := base.TenantWhere(ctx, "appointment"); ok {
		q = q.Where(where, val)
	}
	if _, err := q.Exec(ctx); err != nil {
		return fmt.Errorf("bump appointment revision: %w", err)
	}
	return nil
}

// Cancel marks the appointment cancelled (cancelled_at=now) and bumps the
// revision, in a single conditional statement so a concurrent lifecycle change
// cannot race it. The guard is `cancelled_at IS NULL AND deleted_at IS NULL`:
//   - cancelled_at IS NULL makes concurrent cancels idempotent (only the first
//     transitions);
//   - deleted_at IS NULL stops a cancel from transitioning (and mailing) an
//     appointment a concurrent delete already removed silently — a delete that
//     lands between the caller's load and this update matches zero rows here.
//
// Unlike SoftDelete the appointment stays visible in interactive calendars
// (rendered "Abgesagt"). It returns whether THIS call performed the transition
// (rows affected == 1); only the transitioning caller fires the guardian notice.
func (r *AppointmentRepository) Cancel(ctx context.Context, appointmentID int64) (bool, error) {
	now := time.Now()
	q := base.GetDB(ctx, r.DB).NewUpdate().
		TableExpr(tableExprAppointmentsAsAppointment).
		Set(`cancelled_at = ?`, now).
		Set(`revision = revision + 1`).
		Set(`updated_at = ?`, now).
		Where(`"appointment".id = ?`, appointmentID).
		Where(`"appointment".cancelled_at IS NULL`).
		Where(`"appointment".deleted_at IS NULL`)
	if where, val, ok := base.TenantWhere(ctx, "appointment"); ok {
		q = q.Where(where, val)
	}
	result, err := q.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("cancel appointment: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("cancel appointment: rows affected: %w", err)
	}
	return affected > 0, nil
}

// SoftDelete marks the appointment deleted (deleted_at=now) and bumps the
// revision so the feed re-exports it as STATUS:CANCELLED with a newer SEQUENCE.
// Interactive queries filter deleted_at IS NULL, so the row disappears from
// every staff/parent calendar while remaining a durable feed tombstone.
func (r *AppointmentRepository) SoftDelete(ctx context.Context, appointmentID int64) error {
	now := time.Now()
	q := base.GetDB(ctx, r.DB).NewUpdate().
		TableExpr(tableExprAppointmentsAsAppointment).
		Set(`deleted_at = ?`, now).
		Set(`revision = revision + 1`).
		Set(`updated_at = ?`, now).
		Where(`"appointment".id = ?`, appointmentID).
		Where(`"appointment".deleted_at IS NULL`)
	if where, val, ok := base.TenantWhere(ctx, "appointment"); ok {
		q = q.Where(where, val)
	}
	if _, err := q.Exec(ctx); err != nil {
		return fmt.Errorf("soft-delete appointment: %w", err)
	}
	return nil
}

func (r *AppointmentRepository) DeleteFeedTombstonesBefore(ctx context.Context, before time.Time) (int, error) {
	q := base.GetDB(ctx, r.DB).NewDelete().
		Model((*calModels.Appointment)(nil)).
		ModelTableExpr(tableExprAppointmentsAsAppointment).
		Where(`COALESCE("appointment".deleted_at, "appointment".cancelled_at) < ?`, before)
	if where, value, ok := base.TenantWhere(ctx, "appointment"); ok {
		q = q.Where(where, value)
	}
	result, err := q.Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete expired appointment tombstones: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count deleted appointment tombstones: %w", err)
	}
	return int(count), nil
}

func applyAppointmentWindow(query *bun.SelectQuery, from, to timezone.Date) *bun.SelectQuery {
	return query.Where(`(
		("appointment".end_date >= ? AND "appointment".start_date <= ?)
		OR EXISTS (
			SELECT 1
			FROM calendar.recurrence_rules rr
			WHERE rr.appointment_id = "appointment".id
			  AND rr.tenant_id = "appointment".tenant_id
			  AND "appointment".start_date <= ?
			  AND (
			  	rr.ends_on IS NULL
			  	OR rr.ends_on + ("appointment".end_date - "appointment".start_date) >= ?
			  )
		)
	)`, from, to, to, from)
}

type RecurrenceRuleRepository struct {
	*base.Repository[*calModels.RecurrenceRule]
}

func NewRecurrenceRuleRepository(db *bun.DB) calModels.RecurrenceRuleRepository {
	repo := base.NewRepository[*calModels.RecurrenceRule](db, tableRecurrenceRules, "RecurrenceRule")
	repo.TenantScoped = true
	return &RecurrenceRuleRepository{Repository: repo}
}

func (r *RecurrenceRuleRepository) FindByAppointmentID(ctx context.Context, appointmentID int64) (*calModels.RecurrenceRule, error) {
	row := new(calModels.RecurrenceRule)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(row).
		ModelTableExpr(`calendar.recurrence_rules AS "recurrence_rule"`).
		Where(`"recurrence_rule".appointment_id = ?`, appointmentID).
		Limit(1)
	if where, val, ok := base.TenantWhere(ctx, "recurrence_rule"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find calendar recurrence rule: %w", err)
	}
	return row, nil
}

func (r *RecurrenceRuleRepository) FindByAppointmentIDs(ctx context.Context, appointmentIDs []int64) ([]*calModels.RecurrenceRule, error) {
	if len(appointmentIDs) == 0 {
		return []*calModels.RecurrenceRule{}, nil
	}

	var rows []*calModels.RecurrenceRule
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.recurrence_rules AS "recurrence_rule"`).
		Where(`"recurrence_rule".appointment_id IN (?)`, bun.List(appointmentIDs))
	if where, val, ok := base.TenantWhere(ctx, "recurrence_rule"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find calendar recurrence rules: %w", err)
	}
	return rows, nil
}

func (r *RecurrenceRuleRepository) DeleteByAppointmentID(ctx context.Context, appointmentID int64) error {
	query := base.GetDB(ctx, r.DB).NewDelete().
		Model((*calModels.RecurrenceRule)(nil)).
		ModelTableExpr(`calendar.recurrence_rules AS "recurrence_rule"`).
		Where(`"recurrence_rule".appointment_id = ?`, appointmentID)
	if where, val, ok := base.TenantWhere(ctx, "recurrence_rule"); ok {
		query = query.Where(where, val)
	}
	if _, err := query.Exec(ctx); err != nil {
		return fmt.Errorf("delete calendar recurrence rule: %w", err)
	}
	return nil
}

type AppointmentRecipientRepository struct {
	db *bun.DB
}

func NewAppointmentRecipientRepository(db *bun.DB) calModels.AppointmentRecipientRepository {
	return &AppointmentRecipientRepository{db: db}
}

func (r *AppointmentRecipientRepository) CreateMany(ctx context.Context, recipients []*calModels.AppointmentRecipient) error {
	if len(recipients) == 0 {
		return nil
	}
	for _, recipient := range recipients {
		if err := recipient.Validate(); err != nil {
			return err
		}
		base.EnsureTenantID(ctx, recipient)
	}
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(&recipients).
		ModelTableExpr(tableAppointmentRecipients).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create calendar appointment recipients: %w", err)
	}
	return nil
}

func (r *AppointmentRecipientRepository) FindByID(ctx context.Context, id int64) (*calModels.AppointmentRecipient, error) {
	row := new(calModels.AppointmentRecipient)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(row).
		ModelTableExpr(`calendar.appointment_recipients AS "appointment_recipient"`).
		Where(`"appointment_recipient".id = ?`, id)
	if where, val, ok := base.TenantWhere(ctx, "appointment_recipient"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find calendar recipient: %w", err)
	}
	return row, nil
}

func (r *AppointmentRecipientRepository) ReplaceForAppointment(ctx context.Context, appointmentID int64, recipients []*calModels.AppointmentRecipient) error {
	deleteQuery := base.GetDB(ctx, r.db).NewDelete().
		Model((*calModels.AppointmentRecipient)(nil)).
		ModelTableExpr(`calendar.appointment_recipients AS "appointment_recipient"`).
		Where(`"appointment_recipient".appointment_id = ?`, appointmentID)
	if where, val, ok := base.TenantWhere(ctx, "appointment_recipient"); ok {
		deleteQuery = deleteQuery.Where(where, val)
	}
	if _, err := deleteQuery.Exec(ctx); err != nil {
		return fmt.Errorf("replace calendar recipients: delete existing: %w", err)
	}
	for _, recipient := range recipients {
		recipient.AppointmentID = appointmentID
	}
	return r.CreateMany(ctx, recipients)
}

func (r *AppointmentRecipientRepository) FindByAppointmentID(ctx context.Context, appointmentID int64) ([]*calModels.AppointmentRecipient, error) {
	var rows []*calModels.AppointmentRecipient
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.appointment_recipients AS "appointment_recipient"`).
		Where(`"appointment_recipient".appointment_id = ?`, appointmentID).
		OrderExpr(`"appointment_recipient".recipient_type ASC, "appointment_recipient".id ASC`)
	if where, val, ok := base.TenantWhere(ctx, "appointment_recipient"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find calendar recipients: %w", err)
	}
	return rows, nil
}

func (r *AppointmentRecipientRepository) UpdateResponse(ctx context.Context, recipientID int64, status string) error {
	switch status {
	case calModels.ResponseStatusPending, calModels.ResponseStatusAccepted, calModels.ResponseStatusDeclined, calModels.ResponseStatusInfo:
	default:
		return fmt.Errorf("invalid calendar recipient response status %q", status)
	}

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*calModels.AppointmentRecipient)(nil)).
		ModelTableExpr(`calendar.appointment_recipients AS "appointment_recipient"`).
		Set(`status = ?`, status).
		Where(`"appointment_recipient".id = ?`, recipientID)
	if status == calModels.ResponseStatusAccepted || status == calModels.ResponseStatusDeclined {
		query = query.Set(`responded_at = ?`, time.Now())
	} else {
		query = query.Set(`responded_at = NULL`)
	}
	if where, val, ok := base.TenantWhere(ctx, "appointment_recipient"); ok {
		query = query.Where(where, val)
	}
	res, err := query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("update calendar recipient response: %w", err)
	}
	return base.AssertRowsAffected(res, 1, "update calendar recipient response")
}

func (r *AppointmentRecipientRepository) ClaimReminderPush(ctx context.Context, appointmentID int64, revision int, occurrenceDate timezone.Date, guardianProfileID int64) (bool, error) {
	if appointmentID <= 0 || revision < 0 || guardianProfileID <= 0 || occurrenceDate.IsZero() {
		return false, errors.New("appointment id, revision, occurrence date, and guardian profile id are required")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return false, errors.New("tenant id is required")
	}

	var claimed bool
	err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT calendar.claim_appointment_reminder_push_delivery(?, ?, ?, ?)
	`, appointmentID, revision, occurrenceDate, guardianProfileID).Scan(ctx, &claimed)
	if err != nil {
		return false, fmt.Errorf("claim calendar appointment reminder push: %w", err)
	}
	return claimed, nil
}

func (r *AppointmentRecipientRepository) ReleaseReminderPush(ctx context.Context, appointmentID int64, revision int, occurrenceDate timezone.Date, guardianProfileID int64) error {
	if appointmentID <= 0 || revision < 0 || guardianProfileID <= 0 || occurrenceDate.IsZero() {
		return errors.New("appointment id, revision, occurrence date, and guardian profile id are required")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("tenant id is required")
	}

	_, err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT calendar.release_appointment_reminder_push_delivery(?, ?, ?, ?)
	`, appointmentID, revision, occurrenceDate, guardianProfileID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("release calendar appointment reminder push delivery: %w", err)
	}
	return nil
}

type AppointmentRecipientStudentRepository struct {
	db *bun.DB
}

func NewAppointmentRecipientStudentRepository(db *bun.DB) calModels.AppointmentRecipientStudentRepository {
	return &AppointmentRecipientStudentRepository{db: db}
}

func (r *AppointmentRecipientStudentRepository) CreateMany(ctx context.Context, links []*calModels.AppointmentRecipientStudent) error {
	if len(links) == 0 {
		return nil
	}
	for _, link := range links {
		base.EnsureTenantID(ctx, link)
	}
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(&links).
		ModelTableExpr(tableAppointmentRecipientStudent).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create calendar recipient-student links: %w", err)
	}
	return nil
}

func (r *AppointmentRecipientStudentRepository) FindByRecipientIDs(ctx context.Context, recipientIDs []int64) ([]*calModels.AppointmentRecipientStudent, error) {
	if len(recipientIDs) == 0 {
		return []*calModels.AppointmentRecipientStudent{}, nil
	}
	var rows []*calModels.AppointmentRecipientStudent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.appointment_recipient_students AS "appointment_recipient_student"`).
		Where(`"appointment_recipient_student".recipient_id IN (?)`, bun.List(recipientIDs)).
		OrderExpr(`"appointment_recipient_student".recipient_id ASC, "appointment_recipient_student".student_id ASC`)
	if where, val, ok := base.TenantWhere(ctx, "appointment_recipient_student"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find calendar recipient-student links: %w", err)
	}
	return rows, nil
}

type AppointmentTargetRepository struct {
	db *bun.DB
}

func NewAppointmentTargetRepository(db *bun.DB) calModels.AppointmentTargetRepository {
	return &AppointmentTargetRepository{db: db}
}

func (r *AppointmentTargetRepository) ReplaceForAppointment(ctx context.Context, appointmentID int64, targets []*calModels.AppointmentTarget) error {
	deleteQuery := base.GetDB(ctx, r.db).NewDelete().
		Model((*calModels.AppointmentTarget)(nil)).
		ModelTableExpr(`calendar.appointment_targets AS "appointment_target"`).
		Where(`"appointment_target".appointment_id = ?`, appointmentID)
	if where, val, ok := base.TenantWhere(ctx, "appointment_target"); ok {
		deleteQuery = deleteQuery.Where(where, val)
	}
	if _, err := deleteQuery.Exec(ctx); err != nil {
		return fmt.Errorf("replace calendar targets: delete existing: %w", err)
	}
	if len(targets) == 0 {
		return nil
	}
	for _, target := range targets {
		target.AppointmentID = appointmentID
		base.EnsureTenantID(ctx, target)
	}
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(&targets).
		ModelTableExpr(tableAppointmentTargets).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("replace calendar targets: insert: %w", err)
	}
	return nil
}

func (r *AppointmentTargetRepository) FindByAppointmentID(ctx context.Context, appointmentID int64) ([]*calModels.AppointmentTarget, error) {
	var rows []*calModels.AppointmentTarget
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.appointment_targets AS "appointment_target"`).
		Where(`"appointment_target".appointment_id = ?`, appointmentID).
		OrderExpr(`"appointment_target".id ASC`)
	if where, val, ok := base.TenantWhere(ctx, "appointment_target"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find calendar targets: %w", err)
	}
	return rows, nil
}

type AppointmentOccurrenceOverrideRepository struct {
	*base.Repository[*calModels.AppointmentOccurrenceOverride]
}

func NewAppointmentOccurrenceOverrideRepository(db *bun.DB) calModels.AppointmentOccurrenceOverrideRepository {
	repo := base.NewRepository[*calModels.AppointmentOccurrenceOverride](db, tableAppointmentOverrides, "AppointmentOccurrenceOverride")
	repo.TenantScoped = true
	return &AppointmentOccurrenceOverrideRepository{Repository: repo}
}

func (r *AppointmentOccurrenceOverrideRepository) FindCancelledByAppointmentIDs(ctx context.Context, appointmentIDs []int64) ([]*calModels.AppointmentOccurrenceOverride, error) {
	if len(appointmentIDs) == 0 {
		return []*calModels.AppointmentOccurrenceOverride{}, nil
	}
	var rows []*calModels.AppointmentOccurrenceOverride
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.appointment_occurrence_overrides AS "appointment_occurrence_override"`).
		Where(`"appointment_occurrence_override".appointment_id IN (?)`, bun.List(appointmentIDs)).
		Where(`"appointment_occurrence_override".cancelled = ?`, true).
		OrderExpr(`"appointment_occurrence_override".occurrence_date ASC, "appointment_occurrence_override".id ASC`)
	if where, val, ok := base.TenantWhere(ctx, "appointment_occurrence_override"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find cancelled calendar occurrence overrides: %w", err)
	}
	return rows, nil
}

// CancelOccurrence marks the occurrence cancelled via an INSERT ... ON CONFLICT
// DO UPDATE, so concurrent cancellations of the same occurrence converge on
// cancelled=true instead of one violating the unique constraint and returning a
// 500.
func (r *AppointmentOccurrenceOverrideRepository) CancelOccurrence(ctx context.Context, appointmentID int64, occurrenceDate timezone.Date) error {
	override := &calModels.AppointmentOccurrenceOverride{
		AppointmentID:  appointmentID,
		OccurrenceDate: occurrenceDate,
		Cancelled:      true,
	}
	base.EnsureTenantID(ctx, override)
	if _, err := base.GetDB(ctx, r.DB).NewInsert().
		Model(override).
		ModelTableExpr(`calendar.appointment_occurrence_overrides`).
		On("CONFLICT (tenant_id, appointment_id, occurrence_date) DO UPDATE").
		Set("cancelled = EXCLUDED.cancelled").
		Set("updated_at = NOW()").
		Exec(ctx); err != nil {
		return fmt.Errorf("cancel calendar occurrence: %w", err)
	}
	return nil
}

func (r *AppointmentOccurrenceOverrideRepository) DeleteByAppointmentID(ctx context.Context, appointmentID int64) error {
	query := base.GetDB(ctx, r.DB).NewDelete().
		Model((*calModels.AppointmentOccurrenceOverride)(nil)).
		ModelTableExpr(`calendar.appointment_occurrence_overrides AS "appointment_occurrence_override"`).
		Where(`"appointment_occurrence_override".appointment_id = ?`, appointmentID)
	if where, val, ok := base.TenantWhere(ctx, "appointment_occurrence_override"); ok {
		query = query.Where(where, val)
	}
	if _, err := query.Exec(ctx); err != nil {
		return fmt.Errorf("delete calendar occurrence overrides: %w", err)
	}
	return nil
}

func (r *AppointmentOccurrenceOverrideRepository) FindByAppointmentIDsAndOccurrenceDates(ctx context.Context, appointmentIDs []int64, occurrenceDates []timezone.Date) ([]*calModels.AppointmentOccurrenceOverride, error) {
	if len(appointmentIDs) == 0 || len(occurrenceDates) == 0 {
		return []*calModels.AppointmentOccurrenceOverride{}, nil
	}
	var rows []*calModels.AppointmentOccurrenceOverride
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.appointment_occurrence_overrides AS "appointment_occurrence_override"`).
		Where(`"appointment_occurrence_override".appointment_id IN (?)`, bun.List(appointmentIDs)).
		Where(`"appointment_occurrence_override".occurrence_date IN (?)`, bun.List(occurrenceDates)).
		OrderExpr(`"appointment_occurrence_override".occurrence_date ASC, "appointment_occurrence_override".id ASC`)
	if where, val, ok := base.TenantWhere(ctx, "appointment_occurrence_override"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find calendar occurrence overrides: %w", err)
	}
	return rows, nil
}

func (r *AppointmentOccurrenceOverrideRepository) FindByAppointmentIDsAndStartDates(ctx context.Context, appointmentIDs []int64, startDates []timezone.Date) ([]*calModels.AppointmentOccurrenceOverride, error) {
	if len(appointmentIDs) == 0 || len(startDates) == 0 {
		return []*calModels.AppointmentOccurrenceOverride{}, nil
	}
	var rows []*calModels.AppointmentOccurrenceOverride
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.appointment_occurrence_overrides AS "appointment_occurrence_override"`).
		Where(`"appointment_occurrence_override".appointment_id IN (?)`, bun.List(appointmentIDs)).
		Where(`"appointment_occurrence_override".start_date IN (?)`, bun.List(startDates)).
		OrderExpr(`"appointment_occurrence_override".start_date ASC, "appointment_occurrence_override".id ASC`)
	if where, val, ok := base.TenantWhere(ctx, "appointment_occurrence_override"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find calendar occurrence overrides by start date: %w", err)
	}
	return rows, nil
}
