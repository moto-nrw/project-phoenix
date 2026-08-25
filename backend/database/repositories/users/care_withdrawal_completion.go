package users

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const tableExprCareWithdrawalCompletions = `users.care_withdrawal_completions AS "care_withdrawal_completion"`

type CareWithdrawalCompletionRepository struct {
	*base.Repository[*userModels.CareWithdrawalCompletion]
}

func NewCareWithdrawalCompletionRepository(db *bun.DB) userModels.CareWithdrawalCompletionRepository {
	repo := base.NewRepository[*userModels.CareWithdrawalCompletion](
		db, "users.care_withdrawal_completions", "CareWithdrawalCompletion",
	)
	repo.TenantScoped = true
	return &CareWithdrawalCompletionRepository{Repository: repo}
}

// UpsertPending implements the domain invariant of one pending task per child;
// generic Create cannot express the partial-conflict update.
func (r *CareWithdrawalCompletionRepository) UpsertPending(ctx context.Context, completion *userModels.CareWithdrawalCompletion) error {
	if completion.StudentID == nil || *completion.StudentID <= 0 {
		return errors.New("care withdrawal completion requires a student")
	}
	base.EnsureTenantID(ctx, completion)
	completion.State = userModels.CareWithdrawalStatePending
	if completion.SourceOfferings == nil {
		completion.SourceOfferings = []userModels.CareExitSourceOffering{}
	}
	_, err := base.GetDB(ctx, r.DB).NewInsert().
		Model(completion).
		ModelTableExpr(tableExprCareWithdrawalCompletions).
		On("CONFLICT (tenant_id, student_id) WHERE state = 'pending' AND student_id IS NOT NULL DO UPDATE").
		Set("first_bookingless_day = EXCLUDED.first_bookingless_day").
		Set(preserveConfirmedWithdrawal("trigger")).
		Set(preserveConfirmedWithdrawal("source_adjustment_id")).
		Set(preserveConfirmedWithdrawal("source_request_child_id")).
		Set(preserveConfirmedWithdrawal("withdrawal_confirmed_by")).
		Set(preserveConfirmedWithdrawal("withdrawal_confirmed_role")).
		Set(preserveConfirmedWithdrawal("withdrawal_confirmed_at")).
		Set(preserveConfirmedWithdrawal("source_offerings")).
		Set("updated_at = NOW()").
		Returning("id, created_at, updated_at").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "upsert pending care withdrawal completion", Err: err}
	}
	return nil
}

func preserveConfirmedWithdrawal(column string) string {
	return fmt.Sprintf(`%s = CASE
		WHEN "care_withdrawal_completion".trigger = '%s' AND EXCLUDED.trigger = '%s'
		THEN "care_withdrawal_completion".%s ELSE EXCLUDED.%s END`,
		column, userModels.CareWithdrawalTriggerDirectSchool,
		userModels.CareWithdrawalTriggerBookingExpired, column, column)
}

// FindByID delegates the generic tenant-scoped read while preserving this
// service contract's nil-on-not-found behavior.
func (r *CareWithdrawalCompletionRepository) FindByID(ctx context.Context, id int64) (*userModels.CareWithdrawalCompletion, error) {
	return r.FindByIDOrNil(ctx, id)
}

// FindByIDForUpdate delegates the generic lock read and preserves nil on a
// stale task id so concurrent completions can return a stable conflict.
func (r *CareWithdrawalCompletionRepository) FindByIDForUpdate(ctx context.Context, id int64) (*userModels.CareWithdrawalCompletion, error) {
	row, err := r.Repository.FindByIDForUpdate(ctx, id)
	if modelBase.IsNoRows(err) {
		return nil, nil
	}
	return row, err
}

// ListPending is a joined read model: generic List cannot search or hydrate
// the child's person and class fields.
func (r *CareWithdrawalCompletionRepository) ListPending(
	ctx context.Context,
	filter userModels.CareWithdrawalCompletionFilter,
) ([]*userModels.CareWithdrawalCompletion, int, error) {
	build := func() *bun.SelectQuery {
		query := base.GetDB(ctx, r.DB).NewSelect().
			TableExpr(tableExprCareWithdrawalCompletions).
			Join(`JOIN users.students AS "student" ON "student".id = "care_withdrawal_completion".student_id AND "student".tenant_id = "care_withdrawal_completion".tenant_id`).
			Join(`JOIN users.persons AS "person" ON "person".id = "student".person_id`).
			Where(`"care_withdrawal_completion".state = ?`, userModels.CareWithdrawalStatePending)
		query = base.WithTenantFilter(ctx, query, "care_withdrawal_completion")
		if search := filter.Search; search != "" {
			pattern := "%" + strings.ToLower(search) + "%"
			query = query.Where(`(LOWER("person".first_name) LIKE ? OR LOWER("person".last_name) LIKE ? OR LOWER("student".school_class) LIKE ?)`, pattern, pattern, pattern)
		}
		if filter.StudentID > 0 {
			query = query.Where(`"care_withdrawal_completion".student_id = ?`, filter.StudentID)
		}
		return query
	}
	total, err := build().Count(ctx)
	if err != nil {
		return nil, 0, &modelBase.DatabaseError{Op: "count pending care withdrawal completions", Err: err}
	}
	var rows []*userModels.CareWithdrawalCompletion
	query := build().
		ColumnExpr(`"care_withdrawal_completion".*`).
		ColumnExpr(`"person".first_name AS first_name`).
		ColumnExpr(`"person".last_name AS last_name`).
		ColumnExpr(`"student".school_class AS school_class`).
		OrderExpr(`"care_withdrawal_completion".first_bookingless_day ASC, "care_withdrawal_completion".id ASC`)
	if filter.PageSize > 0 {
		query = query.Limit(filter.PageSize)
		if filter.Page > 1 {
			query = query.Offset((filter.Page - 1) * filter.PageSize)
		}
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, 0, &modelBase.DatabaseError{Op: "list pending care withdrawal completions", Err: err}
	}
	return rows, total, nil
}

func (r *CareWithdrawalCompletionRepository) ListPendingByStudentIDs(
	ctx context.Context, studentIDs []int64,
) (map[int64]*userModels.CareWithdrawalCompletion, error) {
	result := make(map[int64]*userModels.CareWithdrawalCompletion)
	if len(studentIDs) == 0 {
		return result, nil
	}
	rows := make([]*userModels.CareWithdrawalCompletion, 0)
	query := base.GetDB(ctx, r.DB).NewSelect().Model(&rows).
		ModelTableExpr(tableExprCareWithdrawalCompletions).
		Where(`"care_withdrawal_completion".state = ?`, userModels.CareWithdrawalStatePending).
		Where(`"care_withdrawal_completion".student_id IN (?)`, bun.List(studentIDs))
	query = base.WithTenantFilter(ctx, query, "care_withdrawal_completion")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list pending care withdrawal completions by students", Err: err}
	}
	for _, row := range rows {
		if row.StudentID != nil {
			result[*row.StudentID] = row
		}
	}
	return result, nil
}

// ListResolved keeps deleted completions visible without retaining child PII.
// The left joins hydrate names only while the student still exists.
func (r *CareWithdrawalCompletionRepository) ListResolved(
	ctx context.Context,
	filter userModels.CareWithdrawalCompletionFilter,
) ([]*userModels.CareWithdrawalCompletion, int, error) {
	build := func() *bun.SelectQuery {
		query := base.GetDB(ctx, r.DB).NewSelect().
			TableExpr(tableExprCareWithdrawalCompletions).
			Join(`LEFT JOIN users.students AS "student" ON "student".id = "care_withdrawal_completion".student_id AND "student".tenant_id = "care_withdrawal_completion".tenant_id`).
			Join(`LEFT JOIN users.persons AS "person" ON "person".id = "student".person_id`).
			Where(`"care_withdrawal_completion".state = ?`, userModels.CareWithdrawalStateResolved)
		query = base.WithTenantFilter(ctx, query, "care_withdrawal_completion")
		if search := filter.Search; search != "" {
			pattern := "%" + strings.ToLower(search) + "%"
			query = query.Where(`(LOWER("person".first_name) LIKE ? OR LOWER("person".last_name) LIKE ? OR LOWER("student".school_class) LIKE ?)`, pattern, pattern, pattern)
		}
		if filter.StudentID > 0 {
			query = query.Where(`"care_withdrawal_completion".student_id = ?`, filter.StudentID)
		}
		return query
	}
	total, err := build().Count(ctx)
	if err != nil {
		return nil, 0, &modelBase.DatabaseError{Op: "count resolved care withdrawal completions", Err: err}
	}
	var rows []*userModels.CareWithdrawalCompletion
	query := build().
		ColumnExpr(`"care_withdrawal_completion".*`).
		ColumnExpr(`COALESCE("person".first_name, '') AS first_name`).
		ColumnExpr(`COALESCE("person".last_name, '') AS last_name`).
		ColumnExpr(`COALESCE("student".school_class, '') AS school_class`).
		OrderExpr(`"care_withdrawal_completion".resolved_at DESC, "care_withdrawal_completion".id DESC`).
		Limit(filter.PageSize).
		Offset((filter.Page - 1) * filter.PageSize)
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, 0, &modelBase.DatabaseError{Op: "list resolved care withdrawal completions", Err: err}
	}
	return rows, total, nil
}

// ListParticipationBoundaries returns the earliest day on which each child is
// no longer operationally participating: either the day after enrolled_until
// or a pending booking-led completion boundary.
func (r *CareWithdrawalCompletionRepository) ListParticipationBoundaries(
	ctx context.Context, studentIDs []int64, includeBookingBoundaries bool,
) (map[int64]timezone.Date, error) {
	boundaries := make(map[int64]timezone.Date, len(studentIDs))
	if len(studentIDs) == 0 {
		return boundaries, nil
	}
	type row struct {
		StudentID           int64         `bun:"student_id"`
		FirstBookinglessDay timezone.Date `bun:"first_bookingless_day"`
	}
	rows := make([]row, 0)
	err := base.GetDB(ctx, r.DB).NewRaw(`
		SELECT student.id AS student_id,
		       LEAST(student.enrolled_until + 1, completion.first_bookingless_day) AS first_bookingless_day
		FROM users.students AS student
		LEFT JOIN users.care_withdrawal_completions AS completion
		  ON completion.student_id = student.id AND completion.tenant_id = student.tenant_id
		 AND completion.state = ?
		 AND (completion.trigger <> ? OR ?)
		WHERE student.tenant_id = ? AND student.id IN (?)
		  AND (student.enrolled_until IS NOT NULL OR completion.id IS NOT NULL)
	`, userModels.CareWithdrawalStatePending, userModels.CareWithdrawalTriggerBookingExpired,
		includeBookingBoundaries, tenant.FromContext(ctx), bun.List(studentIDs)).Scan(ctx, &rows)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list care participation boundaries", Err: err}
	}
	for _, item := range rows {
		boundaries[item.StudentID] = item.FirstBookinglessDay
	}
	return boundaries, nil
}

func (r *CareWithdrawalCompletionRepository) ListPendingStudentIDs(
	ctx context.Context, studentIDs []int64,
) (map[int64]bool, error) {
	result := make(map[int64]bool)
	if len(studentIDs) == 0 {
		return result, nil
	}
	var ids []int64
	err := base.GetDB(ctx, r.DB).NewSelect().
		Model((*userModels.CareWithdrawalCompletion)(nil)).
		ModelTableExpr(tableExprCareWithdrawalCompletions).
		Column("student_id").
		Where(`"care_withdrawal_completion".tenant_id = ?`, tenant.FromContext(ctx)).
		Where(`"care_withdrawal_completion".state = ?`, userModels.CareWithdrawalStatePending).
		Where(`"care_withdrawal_completion".student_id IN (?)`, bun.In(studentIDs)).
		Scan(ctx, &ids)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list pending care withdrawal student ids", Err: err}
	}
	for _, id := range ids {
		result[id] = true
	}
	return result, nil
}

// MarkResolved is a guarded pending-to-resolved transition; generic Update
// cannot enforce that exactly one open event wins.
func (r *CareWithdrawalCompletionRepository) MarkResolved(ctx context.Context, id, actorAccountID int64, at time.Time) (bool, error) {
	outcome := userModels.CareWithdrawalOutcomeCareEnded
	result, err := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*userModels.CareWithdrawalCompletion)(nil)).
		ModelTableExpr(tableExprCareWithdrawalCompletions).
		Set("state = ?", userModels.CareWithdrawalStateResolved).
		Set("outcome = ?", outcome).
		Set("resolved_by = ?", actorAccountID).
		Set("resolved_at = ?", at).
		Set("updated_at = ?", at).
		Where(`"care_withdrawal_completion".id = ? AND "care_withdrawal_completion".state = ?`, id, userModels.CareWithdrawalStatePending).
		Where(`"care_withdrawal_completion".tenant_id = ?`, tenant.FromContext(ctx)).
		Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "resolve care withdrawal completion", Err: err}
	}
	affected, _ := result.RowsAffected()
	return affected == 1, nil
}

// MarkDeleted resolves and redacts the completion before the student cascade.
// Its caller owns the surrounding transaction, so a failed deletion restores
// the pending task as well.
func (r *CareWithdrawalCompletionRepository) MarkDeleted(ctx context.Context, id, actorAccountID int64, at time.Time) (bool, error) {
	result, err := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*userModels.CareWithdrawalCompletion)(nil)).
		ModelTableExpr(tableExprCareWithdrawalCompletions).
		Set("state = ?", userModels.CareWithdrawalStateResolved).
		Set("outcome = ?", userModels.CareWithdrawalOutcomeDeleted).
		Set("student_id = NULL").
		Set("source_adjustment_id = NULL").
		Set("source_request_child_id = NULL").
		Set("source_offerings = '[]'::jsonb").
		Set("resolved_by = ?", actorAccountID).
		Set("resolved_at = ?", at).
		Set("updated_at = ?", at).
		Where(`"care_withdrawal_completion".id = ? AND "care_withdrawal_completion".state = ?`, id, userModels.CareWithdrawalStatePending).
		Where(`"care_withdrawal_completion".tenant_id = ?`, tenant.FromContext(ctx)).
		Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "resolve and redact deleted care withdrawal completion", Err: err}
	}
	affected, _ := result.RowsAffected()
	return affected == 1, nil
}

// MarkStudentDeleted redacts every completion still linked to a student. This
// also covers deletion started from the ordinary child-data screen, so that
// route cannot orphan an invisible pending task or retain withdrawal PII.
func (r *CareWithdrawalCompletionRepository) MarkStudentDeleted(ctx context.Context, studentID, actorAccountID int64, at time.Time) (int, error) {
	result, err := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*userModels.CareWithdrawalCompletion)(nil)).
		ModelTableExpr(tableExprCareWithdrawalCompletions).
		Set("state = CASE WHEN state = ? THEN ? ELSE state END",
			userModels.CareWithdrawalStatePending, userModels.CareWithdrawalStateResolved).
		Set("outcome = CASE WHEN state = ? THEN ? ELSE outcome END",
			userModels.CareWithdrawalStatePending, userModels.CareWithdrawalOutcomeDeleted).
		Set("student_id = NULL").
		Set("source_adjustment_id = NULL").
		Set("source_request_child_id = NULL").
		Set("source_offerings = '[]'::jsonb").
		Set("obsolete_reason = CASE WHEN state = ? THEN NULL ELSE obsolete_reason END", userModels.CareWithdrawalStatePending).
		Set("resolved_by = CASE WHEN state = ? THEN ? ELSE resolved_by END", userModels.CareWithdrawalStatePending, actorAccountID).
		Set("resolved_at = CASE WHEN state = ? THEN ? ELSE resolved_at END", userModels.CareWithdrawalStatePending, at).
		Set("updated_at = ?", at).
		Where(`"care_withdrawal_completion".tenant_id = ?`, tenant.FromContext(ctx)).
		Where(`"care_withdrawal_completion".student_id = ?`, studentID).
		Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "redact care withdrawal completions for deleted student", Err: err}
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// MarkObsoleteForRebooking atomically applies the no-gap domain predicate.
func (r *CareWithdrawalCompletionRepository) MarkObsoleteForRebooking(
	ctx context.Context,
	studentID int64,
	careStartsOn timezone.Date,
	at time.Time,
) (bool, error) {
	result, err := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*userModels.CareWithdrawalCompletion)(nil)).
		ModelTableExpr(tableExprCareWithdrawalCompletions).
		Set("state = ?", userModels.CareWithdrawalStateObsolete).
		Set("obsolete_reason = ?", userModels.CareWithdrawalObsoleteRebooked).
		Set("resolved_at = ?", at).
		Set("updated_at = ?", at).
		Where(`"care_withdrawal_completion".tenant_id = ?`, tenant.FromContext(ctx)).
		Where(`"care_withdrawal_completion".student_id = ?`, studentID).
		Where(`"care_withdrawal_completion".state = ?`, userModels.CareWithdrawalStatePending).
		Where(`? <= "care_withdrawal_completion".first_bookingless_day`, careStartsOn).
		Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "obsolete care withdrawal completion after rebooking", Err: err}
	}
	affected, _ := result.RowsAffected()
	return affected == 1, nil
}

// MarkPendingObsoleteForWeeklyPlans closes booking-derived tasks when the
// school switches back to weekly-plan-driven care. Confirmed withdrawals stay.
func (r *CareWithdrawalCompletionRepository) MarkPendingObsoleteForWeeklyPlans(ctx context.Context, at time.Time) (int, error) {
	result, err := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*userModels.CareWithdrawalCompletion)(nil)).
		ModelTableExpr(tableExprCareWithdrawalCompletions).
		Set("state = ?", userModels.CareWithdrawalStateObsolete).
		Set("obsolete_reason = ?", userModels.CareWithdrawalObsoleteWeeklyPlans).
		Set("resolved_at = ?", at).
		Set("updated_at = ?", at).
		Where(`"care_withdrawal_completion".tenant_id = ?`, tenant.FromContext(ctx)).
		Where(`"care_withdrawal_completion".state = ?`, userModels.CareWithdrawalStatePending).
		Where(`"care_withdrawal_completion".trigger = ?`, userModels.CareWithdrawalTriggerBookingExpired).
		Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "obsolete pending care withdrawal completions for weekly plans", Err: err}
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// ReopenAfterCancelledExit copies one exact resolved event into a new event;
// generic CRUD must not mutate the historical outcome in place.
func (r *CareWithdrawalCompletionRepository) ReopenAfterCancelledExit(
	ctx context.Context,
	completionID, studentID int64,
	at time.Time,
) (bool, error) {
	result, err := base.GetDB(ctx, r.DB).ExecContext(ctx, `
		INSERT INTO users.care_withdrawal_completions (
			tenant_id, student_id, first_bookingless_day, trigger,
			source_adjustment_id, source_request_child_id, withdrawal_confirmed_by,
			withdrawal_confirmed_role, withdrawal_confirmed_at, source_offerings, state,
			created_at, updated_at
		)
		SELECT tenant_id, student_id, first_bookingless_day, trigger,
		       source_adjustment_id, source_request_child_id, withdrawal_confirmed_by,
		       withdrawal_confirmed_role, withdrawal_confirmed_at, source_offerings, 'pending', ?, ?
		FROM users.care_withdrawal_completions
		WHERE tenant_id = ? AND id = ? AND student_id = ?
		  AND state = 'resolved' AND outcome = 'care_ended'
		ON CONFLICT (tenant_id, student_id)
			WHERE state = 'pending' AND student_id IS NOT NULL
		DO UPDATE SET
			first_bookingless_day = EXCLUDED.first_bookingless_day,
			source_adjustment_id = EXCLUDED.source_adjustment_id,
			source_request_child_id = EXCLUDED.source_request_child_id,
			withdrawal_confirmed_by = EXCLUDED.withdrawal_confirmed_by,
			withdrawal_confirmed_role = EXCLUDED.withdrawal_confirmed_role,
			withdrawal_confirmed_at = EXCLUDED.withdrawal_confirmed_at,
			source_offerings = EXCLUDED.source_offerings,
			updated_at = EXCLUDED.updated_at
	`, at, at, tenant.FromContext(ctx), completionID, studentID)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "reopen care withdrawal after cancelled exit", Err: err}
	}
	affected, _ := result.RowsAffected()
	return affected == 1, nil
}
