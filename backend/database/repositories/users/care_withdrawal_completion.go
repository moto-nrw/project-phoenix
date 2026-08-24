package users

import (
	"context"
	"errors"
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
		Set(`first_bookingless_day = LEAST("care_withdrawal_completion".first_bookingless_day, EXCLUDED.first_bookingless_day)`).
		Set("trigger = EXCLUDED.trigger").
		Set("source_adjustment_id = EXCLUDED.source_adjustment_id").
		Set("withdrawal_confirmed_by = EXCLUDED.withdrawal_confirmed_by").
		Set("withdrawal_confirmed_role = EXCLUDED.withdrawal_confirmed_role").
		Set("withdrawal_confirmed_at = EXCLUDED.withdrawal_confirmed_at").
		Set("source_offerings = EXCLUDED.source_offerings").
		Set("updated_at = NOW()").
		Returning("id, created_at, updated_at").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "upsert pending care withdrawal completion", Err: err}
	}
	return nil
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
// school switches back to weekly-plan-driven care.
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
			source_adjustment_id, withdrawal_confirmed_by,
			withdrawal_confirmed_role, withdrawal_confirmed_at, source_offerings, state,
			created_at, updated_at
		)
		SELECT tenant_id, student_id, first_bookingless_day, trigger,
		       source_adjustment_id, withdrawal_confirmed_by,
		       withdrawal_confirmed_role, withdrawal_confirmed_at, source_offerings, 'pending', ?, ?
		FROM users.care_withdrawal_completions
		WHERE tenant_id = ? AND id = ? AND student_id = ?
		  AND state = 'resolved' AND outcome = 'care_ended'
		ON CONFLICT (tenant_id, student_id)
			WHERE state = 'pending' AND student_id IS NOT NULL
		DO UPDATE SET
			first_bookingless_day = EXCLUDED.first_bookingless_day,
			source_adjustment_id = EXCLUDED.source_adjustment_id,
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
