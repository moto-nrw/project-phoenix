package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

const pendingOfferingChangeConstraint = "uq_offering_change_requests_pending"

type Database func(context.Context) (bun.IDB, int64, error)

type Store struct{ database Database }

func New(database Database) *Store {
	if database == nil {
		panic("care plan postgres: database runtime is required")
	}
	return &Store{database: database}
}

// LockExceptionDay takes the transaction-scoped advisory lock shared by all
// writers that can change one child's effective booking on one day.
func LockExceptionDay(ctx context.Context, db bun.IDB, tenantID, studentID int64, date string) error {
	if db == nil {
		return errors.New("care plan postgres: ambient database is required")
	}
	key := fmt.Sprintf("care-exception-day:%d:%d:%s", tenantID, studentID, date)
	_, err := db.NewRaw(`SELECT pg_advisory_xact_lock(hashtext(?))`, key).Exec(ctx)
	return err
}

type calendarDate string

type careOfferingRow struct {
	bun.BaseModel       `bun:"table:care_offerings,alias:care_offering"`
	ID                  int64             `bun:"id,pk,autoincrement"`
	TenantID            int64             `bun:"tenant_id,notnull"`
	CreatedAt           time.Time         `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt           time.Time         `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	PhaseID             int64             `bun:"phase_id,notnull"`
	ActivityGroupID     *int64            `bun:"activity_group_id"`
	Name                string            `bun:"name,notnull"`
	Description         *string           `bun:"description"`
	DaysOfWeekMode      string            `bun:"days_of_week_mode,notnull"`
	AvailableDays       []string          `bun:"available_days,type:jsonb,notnull"`
	IncludesHolidayCare bool              `bun:"includes_holiday_care,notnull"`
	IncludesLunch       bool              `bun:"includes_lunch,notnull"`
	Capacity            *int              `bun:"capacity"`
	PriceCents          *int              `bun:"price_cents"`
	IsActive            bool              `bun:"is_active,notnull"`
	IsRequired          bool              `bun:"is_required,notnull"`
	CountsAsCare        bool              `bun:"counts_as_care,notnull"`
	AutoAddGradeLevels  []int             `bun:"auto_add_grade_levels,type:jsonb,notnull"`
	AvailabilityRule    json.RawMessage   `bun:"availability_rule,type:jsonb"`
	SortOrder           int               `bun:"sort_order,notnull"`
	SelectionGroup      string            `bun:"selection_group"`
	SelectionRule       string            `bun:"selection_rule,notnull"`
	PickupTimes         map[string]string `bun:"pickup_times,type:jsonb"`
}

type autoTriggerRow struct {
	bun.BaseModel         `bun:"table:care_offering_auto_triggers,alias:care_offering_auto_trigger"`
	ID                    int64 `bun:"id,pk,autoincrement"`
	TenantID              int64 `bun:"tenant_id,notnull"`
	TargetCareOfferingID  int64 `bun:"target_care_offering_id,notnull"`
	TriggerCareOfferingID int64 `bun:"trigger_care_offering_id,notnull"`
}

type offeringChangeRow struct {
	bun.BaseModel               `bun:"table:offering_change_requests,alias:offering_change_request"`
	ID                          int64           `bun:"id,pk,autoincrement"`
	TenantID                    int64           `bun:"tenant_id,notnull"`
	CreatedAt                   time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt                   time.Time       `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID                   int64           `bun:"student_id,notnull"`
	RequestChildID              int64           `bun:"request_child_id,notnull"`
	SubmittedBy                 int64           `bun:"submitted_by,notnull"`
	CompleteWithdrawalConfirmed bool            `bun:"complete_withdrawal_confirmed,notnull"`
	WithdrawalConfirmedBy       *int64          `bun:"withdrawal_confirmed_by"`
	WithdrawalConfirmedAt       *time.Time      `bun:"withdrawal_confirmed_at"`
	ApprovedCompleteWithdrawal  bool            `bun:"approved_complete_withdrawal,notnull"`
	Payload                     json.RawMessage `bun:"payload,type:jsonb,notnull"`
	EffectiveFrom               calendarDate    `bun:"effective_from,notnull,type:date"`
	ParentNote                  *string         `bun:"parent_note"`
	Status                      string          `bun:"status,notnull"`
	DecisionReason              *string         `bun:"decision_reason"`
	DecisionSnapshot            json.RawMessage `bun:"decision_snapshot,type:jsonb"`
	ReviewedBy                  *int64          `bun:"reviewed_by"`
	ReviewedAt                  *time.Time      `bun:"reviewed_at"`
	AppliedAt                   *time.Time      `bun:"applied_at"`
}

func (s *Store) databaseForWrite(ctx context.Context, operation string) (bun.IDB, int64, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, 0, err
	}
	if tenantID <= 0 {
		return nil, 0, fmt.Errorf("care plan postgres: tenant is required to %s", operation)
	}
	return db, tenantID, nil
}

func (s *Store) FindCareOffering(ctx context.Context, id int64) (domain.CareOffering, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.CareOffering{}, false, domain.OperationStats{}, err
	}
	row := new(careOfferingRow)
	query := withTenant(careOfferingSelect(db, row).Where(`"care_offering".id = ?`, id), "care_offering", tenantID)
	found, stats, err := scanOne(ctx, query, "find care offering")
	if err != nil || !found {
		return domain.CareOffering{}, found, stats, err
	}
	result := []domain.CareOffering{careOfferingToDomain(*row)}
	triggerStats, err := s.hydrateTriggers(ctx, db, tenantID, result)
	stats.Add(triggerStats)
	if err != nil {
		return domain.CareOffering{}, false, stats, err
	}
	return result[0], true, stats, nil
}

func (s *Store) ListCareOfferings(ctx context.Context, filter domain.CareOfferingFilter) ([]domain.CareOffering, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []careOfferingRow{}
	query, empty := careOfferingListQuery(db, &rows, tenantID, filter)
	if empty {
		return []domain.CareOffering{}, domain.OperationStats{}, nil
	}
	stats, err := scanAll(ctx, query, "list care offerings")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.CareOffering, 0, len(rows))
	for _, row := range rows {
		result = append(result, careOfferingToDomain(row))
	}
	stats.Rows = int64(len(result))
	triggerStats, err := s.hydrateTriggers(ctx, db, tenantID, result)
	stats.Add(triggerStats)
	return result, stats, err
}

func careOfferingListQuery(db bun.IDB, rows *[]careOfferingRow, tenantID int64, filter domain.CareOfferingFilter) (*bun.SelectQuery, bool) {
	query := withTenant(careOfferingSelect(db, rows), "care_offering", tenantID)
	if filter.IDs != nil {
		if len(filter.IDs) == 0 {
			return query, true
		}
		query = query.Where(`"care_offering".id IN (?)`, bun.List(filter.IDs))
	}
	if filter.PhaseIDs != nil {
		if len(filter.PhaseIDs) == 0 {
			return query, true
		}
		query = query.Where(`"care_offering".phase_id IN (?)`, bun.List(filter.PhaseIDs))
	}
	if filter.ActivityGroupIDs != nil {
		if len(filter.ActivityGroupIDs) == 0 {
			return query, true
		}
		query = query.Where(`"care_offering".activity_group_id IN (?)`, bun.List(filter.ActivityGroupIDs))
	}
	if filter.ActiveOnly {
		query = query.Where(`"care_offering".is_active = TRUE`)
	}
	query = orderCareOfferings(query, filter.Order)
	if filter.LockForUpdate {
		query = query.For("UPDATE")
	}
	return query, false
}

func orderCareOfferings(query *bun.SelectQuery, order string) *bun.SelectQuery {
	switch order {
	case "id":
		return query.OrderExpr(`"care_offering".id`)
	case "phase_catalog":
		return query.OrderExpr(`"care_offering".phase_id, "care_offering".sort_order, "care_offering".id`)
	default:
		return query.OrderExpr(`"care_offering".sort_order, "care_offering".id`)
	}
}

func (s *Store) CountCareOfferingsByPhase(ctx context.Context, phaseID int64) (int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withTenant(careOfferingSelect(db, (*careOfferingRow)(nil)).Where(`"care_offering".phase_id = ?`, phaseID), "care_offering", tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	count, err := query.Count(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("care plan postgres: count care offerings by phase: %w", err)
	}
	stats.Rows = int64(count)
	return count, stats, nil
}

func (s *Store) CreateCareOffering(ctx context.Context, fields domain.CareOfferingFields) (domain.CareOffering, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "create a care offering")
	if err != nil {
		return domain.CareOffering{}, domain.OperationStats{}, err
	}
	row := careOfferingRow{TenantID: tenantID}
	applyCareOfferingFields(&row, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`enrollment.care_offerings`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.CareOffering{}, stats, fmt.Errorf("care plan postgres: create care offering: %w", err)
	}
	stats.Rows = 1
	return careOfferingToDomain(row), stats, nil
}

func (s *Store) UpdateCareOffering(ctx context.Context, id int64, fields domain.CareOfferingFields) (domain.CareOffering, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "update a care offering")
	if err != nil {
		return domain.CareOffering{}, domain.OperationStats{}, err
	}
	row := careOfferingRow{ID: id}
	applyCareOfferingFields(&row, fields)
	query := withTenant(db.NewUpdate().Model(&row).
		ModelTableExpr(`enrollment.care_offerings AS "care_offering"`).
		Column("phase_id", "activity_group_id", "name", "description", "days_of_week_mode", "available_days",
			"includes_holiday_care", "includes_lunch", "capacity", "price_cents", "is_active", "is_required",
			"counts_as_care", "auto_add_grade_levels", "availability_rule", "sort_order", "selection_group", "selection_rule", "pickup_times").
		Set("updated_at = NOW()").
		Where(`"care_offering".id = ?`, id), "care_offering", tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CareOffering{}, stats, domain.ErrCareOfferingNotFound
	}
	if err != nil {
		return domain.CareOffering{}, stats, fmt.Errorf("care plan postgres: update care offering: %w", err)
	}
	stats.Rows = 1
	return careOfferingToDomain(row), stats, nil
}

func (s *Store) DeleteCareOffering(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "delete a care offering")
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*careOfferingRow)(nil)).
		ModelTableExpr(`enrollment.care_offerings AS "care_offering"`).
		Where(`"care_offering".id = ?`, id), "care_offering", tenantID)
	return execGuarded(ctx, query, "delete care offering", domain.ErrCareOfferingNotFound)
}

func (s *Store) ReplaceAutoAddTriggers(ctx context.Context, targetID int64, triggerIDs []int64) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "replace care offering auto triggers")
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats, err := deleteAutoAddTriggers(ctx, db, tenantID, targetID)
	if err != nil || len(triggerIDs) == 0 {
		return stats, err
	}
	insertStats, err := insertAutoAddTriggers(ctx, db, tenantID, targetID, triggerIDs)
	stats.Add(insertStats)
	return stats, err
}

func deleteAutoAddTriggers(ctx context.Context, db bun.IDB, tenantID, targetID int64) (domain.OperationStats, error) {
	query := withTenant(db.NewDelete().Model((*autoTriggerRow)(nil)).
		ModelTableExpr(`enrollment.care_offering_auto_triggers AS "care_offering_auto_trigger"`).
		Where(`"care_offering_auto_trigger".target_care_offering_id = ?`, targetID), "care_offering_auto_trigger", tenantID)
	return execAny(ctx, query, "delete care offering auto triggers")
}

func insertAutoAddTriggers(ctx context.Context, db bun.IDB, tenantID, targetID int64, triggerIDs []int64) (domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	count, err := withTenant(careOfferingSelect(db, (*careOfferingRow)(nil)).
		Where(`"care_offering".id IN (?)`, bun.List(triggerIDs)), "care_offering", tenantID).Count(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("care plan postgres: validate care offering auto triggers: %w", err)
	}
	if count != len(triggerIDs) {
		return stats, domain.ErrCareOfferingTriggerInvalid
	}
	rows := make([]autoTriggerRow, 0, len(triggerIDs))
	for _, triggerID := range triggerIDs {
		rows = append(rows, autoTriggerRow{TenantID: tenantID, TargetCareOfferingID: targetID, TriggerCareOfferingID: triggerID})
	}
	stats.Queries++
	started = time.Now()
	_, err = db.NewInsert().Model(&rows).ModelTableExpr(`enrollment.care_offering_auto_triggers`).Exec(ctx)
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("care plan postgres: insert care offering auto triggers: %w", err)
	}
	stats.Rows = int64(len(rows))
	return stats, nil
}

func (s *Store) hydrateTriggers(ctx context.Context, db bun.IDB, tenantID int64, offerings []domain.CareOffering) (domain.OperationStats, error) {
	ids := make([]int64, 0, len(offerings))
	byID := make(map[int64]*domain.CareOffering, len(offerings))
	for index := range offerings {
		offerings[index].AutoAddTriggerOfferingIDs = []int64{}
		ids = append(ids, offerings[index].ID)
		byID[offerings[index].ID] = &offerings[index]
	}
	if len(ids) == 0 {
		return domain.OperationStats{}, nil
	}
	rows := []autoTriggerRow{}
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(`enrollment.care_offering_auto_triggers AS "care_offering_auto_trigger"`).
		Where(`"care_offering_auto_trigger".target_care_offering_id IN (?)`, bun.List(ids)).
		OrderExpr(`"care_offering_auto_trigger".target_care_offering_id, "care_offering_auto_trigger".trigger_care_offering_id`), "care_offering_auto_trigger", tenantID)
	stats, err := scanAll(ctx, query, "load care offering auto triggers")
	if err != nil {
		return stats, err
	}
	stats.Rows = int64(len(rows))
	for _, row := range rows {
		if offering := byID[row.TargetCareOfferingID]; offering != nil {
			offering.AutoAddTriggerOfferingIDs = append(offering.AutoAddTriggerOfferingIDs, row.TriggerCareOfferingID)
		}
	}
	return stats, nil
}

func (s *Store) FindOfferingChange(ctx context.Context, id int64, lock bool) (domain.OfferingChangeRequest, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OfferingChangeRequest{}, false, domain.OperationStats{}, err
	}
	row := new(offeringChangeRow)
	query := withTenant(offeringChangeSelect(db, row).Where(`"offering_change_request".id = ?`, id), "offering_change_request", tenantID)
	if lock {
		query = query.For("UPDATE")
	}
	found, stats, err := scanOne(ctx, query, "find offering change request")
	if err != nil || !found {
		return domain.OfferingChangeRequest{}, found, stats, err
	}
	return offeringChangeToDomain(*row), true, stats, nil
}

func (s *Store) ListOfferingChanges(ctx context.Context, filter domain.OfferingChangeFilter) ([]domain.OfferingChangeRequest, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []offeringChangeRow{}
	query, empty := offeringChangeListQuery(db, &rows, tenantID, filter)
	if empty {
		return []domain.OfferingChangeRequest{}, domain.OperationStats{}, nil
	}
	query = orderOfferingChanges(query, filter)
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.LockForUpdate {
		query = query.For("UPDATE")
	}
	stats, err := scanAll(ctx, query, "list offering change requests")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.OfferingChangeRequest, 0, len(rows))
	for _, row := range rows {
		result = append(result, offeringChangeToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func offeringChangeListQuery(db bun.IDB, rows *[]offeringChangeRow, tenantID int64, filter domain.OfferingChangeFilter) (*bun.SelectQuery, bool) {
	query := withTenant(offeringChangeSelect(db, rows), "offering_change_request", tenantID)
	if filter.IDs != nil {
		if len(filter.IDs) == 0 {
			return query, true
		}
		query = query.Where(`"offering_change_request".id IN (?)`, bun.List(filter.IDs))
	}
	if filter.StudentID > 0 {
		query = query.Where(`"offering_change_request".student_id = ?`, filter.StudentID)
	}
	if filter.StudentIDs != nil {
		if len(filter.StudentIDs) == 0 {
			return query, true
		}
		query = query.Where(`"offering_change_request".student_id IN (?)`, bun.List(filter.StudentIDs))
	}
	if len(filter.Statuses) > 0 {
		query = query.Where(`"offering_change_request".status IN (?)`, bun.List(filter.Statuses))
	}
	if filter.UrgentOnly != nil {
		if *filter.UrgentOnly {
			query = query.Where(`"offering_change_request".effective_from <= ?::date`, calendarDate(filter.UrgentDate))
		} else {
			query = query.Where(`NOT ("offering_change_request".effective_from <= ?::date)`, calendarDate(filter.UrgentDate))
		}
	}
	return query, false
}

func orderOfferingChanges(query *bun.SelectQuery, filter domain.OfferingChangeFilter) *bun.SelectQuery {
	switch filter.Order {
	case "reviewed":
		query = query.OrderExpr(`"offering_change_request".reviewed_at DESC NULLS LAST`).OrderExpr(`"offering_change_request".id DESC`)
	case "updated":
		if !filter.BeforeInstant.IsZero() {
			query = query.Where(`("offering_change_request".updated_at, "offering_change_request".id) < (?, ?)`, filter.BeforeInstant, filter.BeforeID)
		}
		query = query.OrderExpr(`"offering_change_request".updated_at DESC`).OrderExpr(`"offering_change_request".id DESC`)
	default:
		if !filter.BeforeInstant.IsZero() {
			query = query.Where(`("offering_change_request".created_at, "offering_change_request".id) < (?, ?)`, filter.BeforeInstant, filter.BeforeID)
		}
		query = query.OrderExpr(`"offering_change_request".created_at DESC`).OrderExpr(`"offering_change_request".id DESC`)
	}
	return query
}

func (s *Store) CreateOfferingChange(ctx context.Context, value domain.OfferingChangeRequest) (domain.OfferingChangeRequest, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "create an offering change request")
	if err != nil {
		return domain.OfferingChangeRequest{}, domain.OperationStats{}, err
	}
	row := offeringChangeFromDomain(value)
	row.TenantID = tenantID
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`enrollment.offering_change_requests`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.OfferingChangeRequest{}, stats, wrapOfferingChangeWrite("create", err)
	}
	stats.Rows = 1
	return offeringChangeToDomain(row), stats, nil
}

func (s *Store) UpdateOfferingChangeEffectiveFrom(ctx context.Context, id int64, date string) (domain.OperationStats, error) {
	return s.updatePending(ctx, id, "update offering change effective date", func(query *bun.UpdateQuery) *bun.UpdateQuery {
		return query.Set("effective_from = ?", calendarDate(date))
	})
}

func (s *Store) UpdateApprovedCompleteWithdrawal(ctx context.Context, id int64, complete bool) (domain.OperationStats, error) {
	return s.updatePending(ctx, id, "update approved complete withdrawal", func(query *bun.UpdateQuery) *bun.UpdateQuery {
		return query.Set("approved_complete_withdrawal = ?", complete)
	})
}

func (s *Store) UpdatePendingOfferingChange(ctx context.Context, input domain.UpdatePendingOfferingChange) (domain.OperationStats, error) {
	return s.updatePending(ctx, input.ID, "update pending offering change request", func(query *bun.UpdateQuery) *bun.UpdateQuery {
		return query.
			Set("payload = ?", input.Payload).
			Set("effective_from = ?", calendarDate(input.EffectiveFrom)).
			Set("parent_note = ?", input.ParentNote)
	})
}

func (s *Store) updatePending(ctx context.Context, id int64, operation string, change func(*bun.UpdateQuery) *bun.UpdateQuery) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, operation)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := db.NewUpdate().Model((*offeringChangeRow)(nil)).
		ModelTableExpr(`enrollment.offering_change_requests AS "offering_change_request"`).
		Set("updated_at = NOW()").
		Where(`"offering_change_request".id = ?`, id).
		Where(`"offering_change_request".status = 'pending'`)
	query = change(query)
	query = withTenant(query, "offering_change_request", tenantID)
	return execGuarded(ctx, query, operation, domain.ErrOfferingChangeNotPending)
}

func (s *Store) DecideOfferingChange(ctx context.Context, input domain.DecideOfferingChange) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "decide an offering change request")
	if err != nil {
		return domain.OperationStats{}, err
	}
	now := time.Now()
	query := db.NewUpdate().Model((*offeringChangeRow)(nil)).
		ModelTableExpr(`enrollment.offering_change_requests AS "offering_change_request"`).
		Set("status = ?", input.Status).
		Set("decision_reason = ?", input.Reason).
		Set("updated_at = ?", now).
		Where(`"offering_change_request".id = ?`, input.ID).
		Where(`"offering_change_request".status = 'pending'`)
	if input.ReviewedBy != nil && *input.ReviewedBy > 0 {
		query = query.Set("reviewed_by = ?", *input.ReviewedBy).Set("reviewed_at = ?", now)
	}
	if input.Applied {
		query = query.Set("applied_at = ?", now)
	}
	query = withTenant(query, "offering_change_request", tenantID)
	return execGuarded(ctx, query, "decide offering change request", domain.ErrOfferingChangeNotPending)
}

func (s *Store) UpdateOfferingChangeSnapshot(ctx context.Context, id int64, snapshot json.RawMessage) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "update an offering change snapshot")
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*offeringChangeRow)(nil)).
		ModelTableExpr(`enrollment.offering_change_requests AS "offering_change_request"`).
		Set("decision_snapshot = ?", snapshot).
		Set("updated_at = NOW()").
		Where(`"offering_change_request".id = ?`, id), "offering_change_request", tenantID)
	return execAny(ctx, query, "update offering change decision snapshot")
}

func (s *Store) ClosePendingOfferingChanges(ctx context.Context, studentIDs []int64, reason string, reviewedBy *int64, at time.Time) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "close pending offering changes")
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*offeringChangeRow)(nil)).
		ModelTableExpr(`enrollment.offering_change_requests AS "offering_change_request"`).
		Set("status = 'care_ended'").
		Set("decision_reason = ?", reason).
		Set("reviewed_by = ?", reviewedBy).
		Set("reviewed_at = ?", at).
		Set("updated_at = ?", at).
		Where(`"offering_change_request".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"offering_change_request".status = 'pending'`), "offering_change_request", tenantID)
	return execAny(ctx, query, "close pending offering changes")
}

func careOfferingSelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`enrollment.care_offerings AS "care_offering"`)
}

func offeringChangeSelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`enrollment.offering_change_requests AS "offering_change_request"`)
}

func applyCareOfferingFields(row *careOfferingRow, fields domain.CareOfferingFields) {
	row.PhaseID = fields.PhaseID
	row.ActivityGroupID = fields.ActivityGroupID
	row.Name = fields.Name
	row.Description = fields.Description
	row.DaysOfWeekMode = fields.DaysOfWeekMode
	row.AvailableDays = fields.AvailableDays
	row.IncludesHolidayCare = fields.IncludesHolidayCare
	row.IncludesLunch = fields.IncludesLunch
	row.Capacity = fields.Capacity
	row.PriceCents = fields.PriceCents
	row.IsActive = fields.IsActive
	row.IsRequired = fields.IsRequired
	row.CountsAsCare = fields.CountsAsCare
	row.AutoAddGradeLevels = fields.AutoAddGradeLevels
	row.AvailabilityRule = fields.AvailabilityRule
	row.SortOrder = fields.SortOrder
	row.SelectionGroup = fields.SelectionGroup
	row.SelectionRule = fields.SelectionRule
	row.PickupTimes = fields.PickupTimes
}

func careOfferingToDomain(row careOfferingRow) domain.CareOffering {
	return domain.CareOffering{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		PhaseID: row.PhaseID, ActivityGroupID: row.ActivityGroupID, Name: row.Name, Description: row.Description,
		DaysOfWeekMode: row.DaysOfWeekMode, AvailableDays: row.AvailableDays,
		IncludesHolidayCare: row.IncludesHolidayCare, IncludesLunch: row.IncludesLunch,
		Capacity: row.Capacity, PriceCents: row.PriceCents, IsActive: row.IsActive, IsRequired: row.IsRequired,
		CountsAsCare: row.CountsAsCare, AutoAddGradeLevels: row.AutoAddGradeLevels, AvailabilityRule: row.AvailabilityRule,
		SortOrder: row.SortOrder, SelectionGroup: row.SelectionGroup, SelectionRule: row.SelectionRule, PickupTimes: row.PickupTimes,
	}
}

func offeringChangeFromDomain(value domain.OfferingChangeRequest) offeringChangeRow {
	return offeringChangeRow{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		StudentID: value.StudentID, RequestChildID: value.RequestChildID, SubmittedBy: value.SubmittedBy,
		CompleteWithdrawalConfirmed: value.CompleteWithdrawalConfirmed,
		WithdrawalConfirmedBy:       value.WithdrawalConfirmedBy, WithdrawalConfirmedAt: value.WithdrawalConfirmedAt,
		ApprovedCompleteWithdrawal: value.ApprovedCompleteWithdrawal, Payload: value.Payload,
		EffectiveFrom: calendarDate(value.EffectiveFrom), ParentNote: value.ParentNote, Status: value.Status,
		DecisionReason: value.DecisionReason, DecisionSnapshot: value.DecisionSnapshot,
		ReviewedBy: value.ReviewedBy, ReviewedAt: value.ReviewedAt, AppliedAt: value.AppliedAt,
	}
}

func offeringChangeToDomain(row offeringChangeRow) domain.OfferingChangeRequest {
	return domain.OfferingChangeRequest{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StudentID: row.StudentID, RequestChildID: row.RequestChildID, SubmittedBy: row.SubmittedBy,
		CompleteWithdrawalConfirmed: row.CompleteWithdrawalConfirmed,
		WithdrawalConfirmedBy:       row.WithdrawalConfirmedBy, WithdrawalConfirmedAt: row.WithdrawalConfirmedAt,
		ApprovedCompleteWithdrawal: row.ApprovedCompleteWithdrawal, Payload: row.Payload,
		EffectiveFrom: string(row.EffectiveFrom), ParentNote: row.ParentNote, Status: row.Status,
		DecisionReason: row.DecisionReason, DecisionSnapshot: row.DecisionSnapshot,
		ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt,
	}
}

func withTenant[Q interface{ Where(string, ...any) Q }](query Q, alias string, tenantID int64) Q {
	if tenantID > 0 {
		return query.Where(`"`+alias+`".tenant_id = ?`, tenantID)
	}
	return query
}

func scanOne(ctx context.Context, query *bun.SelectQuery, operation string) (bool, domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return false, stats, nil
	}
	if err != nil {
		return false, stats, fmt.Errorf("care plan postgres: %s: %w", operation, err)
	}
	stats.Rows = 1
	return true, stats, nil
}

func scanAll(ctx context.Context, query *bun.SelectQuery, operation string) (domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("care plan postgres: %s: %w", operation, err)
	}
	return stats, nil
}

type executable interface {
	Exec(context.Context, ...any) (sql.Result, error)
}

func execGuarded(ctx context.Context, query executable, operation string, noRows error) (domain.OperationStats, error) {
	stats, rows, err := execute(ctx, query, operation)
	if err == nil && rows == 0 {
		err = noRows
	}
	return stats, err
}

func execAny(ctx context.Context, query executable, operation string) (domain.OperationStats, error) {
	stats, _, err := execute(ctx, query, operation)
	return stats, err
}

func execute(ctx context.Context, query executable, operation string) (domain.OperationStats, int64, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, 0, fmt.Errorf("care plan postgres: %s: %w", operation, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return stats, 0, fmt.Errorf("care plan postgres: %s count rows: %w", operation, err)
	}
	stats.Rows = rows
	return stats, rows, nil
}

func wrapOfferingChangeWrite(operation string, err error) error {
	var postgresError pgdriver.Error
	if errors.As(err, &postgresError) && postgresError.IntegrityViolation() && postgresError.Field('n') == pendingOfferingChangeConstraint {
		return fmt.Errorf("%w: %w", domain.ErrOfferingChangeAlreadyOpen, err)
	}
	return fmt.Errorf("care plan postgres: %s offering change request: %w", operation, err)
}

var _ interface {
	FindCareOffering(context.Context, int64) (domain.CareOffering, bool, domain.OperationStats, error)
} = (*Store)(nil)
