package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
	"github.com/uptrace/bun"
)

// tenantTransitionsLockKey mirrors models/education.TenantTransitionsLockKey.
// The timetable materializer takes the same advisory key through
// services/schedule, so a materialization pass and a grade transition never
// run concurrently for one school; the two holders must agree on the exact
// string.
func tenantTransitionsLockKey(tenantID int64) string {
	return fmt.Sprintf("education.grade_transitions:%d", tenantID)
}

type transitionRow struct {
	bun.BaseModel            `bun:"table:grade_transitions,alias:transition"`
	ID                       int64      `bun:"id,pk,autoincrement"`
	TenantID                 int64      `bun:"tenant_id,notnull"`
	CreatedAt                time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt                time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	AcademicYear             string     `bun:"academic_year,notnull"`
	Status                   string     `bun:"status,notnull"`
	AppliedAt                *time.Time `bun:"applied_at"`
	AppliedBy                *int64     `bun:"applied_by"`
	RevertedAt               *time.Time `bun:"reverted_at"`
	RevertedBy               *int64     `bun:"reverted_by"`
	CreatedBy                int64      `bun:"created_by,notnull"`
	Notes                    *string    `bun:"notes"`
	RosterBaselineInstanceID *int64     `bun:"roster_baseline_instance_id"`
}

type mappingRow struct {
	bun.BaseModel `bun:"table:grade_transition_mappings,alias:mapping"`
	ID            int64   `bun:"id,pk,autoincrement"`
	TenantID      int64   `bun:"tenant_id,notnull"`
	TransitionID  int64   `bun:"transition_id,notnull"`
	FromClass     string  `bun:"from_class,notnull"`
	ToClass       *string `bun:"to_class"`
}

type historyRow struct {
	bun.BaseModel `bun:"table:grade_transition_history,alias:history"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	TransitionID  int64     `bun:"transition_id,notnull"`
	StudentID     int64     `bun:"student_id,notnull"`
	PersonName    string    `bun:"person_name,notnull"`
	FromClass     string    `bun:"from_class,notnull"`
	ToClass       *string   `bun:"to_class"`
	Action        string    `bun:"action,notnull"`
	FromStatus    *string   `bun:"from_status"`
	RFIDTag       *string   `bun:"rfid_tag"`
}

type classTeacherLedgerRow struct {
	bun.BaseModel `bun:"table:grade_transition_class_teachers,alias:ledger"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	TransitionID  int64     `bun:"transition_id,notnull"`
	StaffID       int64     `bun:"staff_id,notnull"`
	SchoolClass   string    `bun:"school_class,notnull"`
	Action        string    `bun:"action,notnull"`
}

type classListLedgerRow struct {
	bun.BaseModel `bun:"table:grade_transition_class_list_entries,alias:ledger"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	TransitionID  int64     `bun:"transition_id,notnull"`
	EntryID       *int64    `bun:"entry_id"`
	FirstName     string    `bun:"first_name,notnull"`
	LastName      string    `bun:"last_name,notnull"`
	SchoolClass   string    `bun:"school_class,notnull"`
	Action        string    `bun:"action,notnull"`
}

const (
	transitionsTable        = `education.grade_transitions AS "transition"`
	mappingsTable           = `education.grade_transition_mappings AS "mapping"`
	historyTable            = `education.grade_transition_history AS "history"`
	classTeacherLedgerTable = `education.grade_transition_class_teachers AS "ledger"`
	classListLedgerTable    = `education.grade_transition_class_list_entries AS "ledger"`
)

func storeError(op string, err error) error {
	return fmt.Errorf("school structure postgres: %s: %w", op, err)
}

func (s *Store) FindTransition(ctx context.Context, tenantID, id int64, lock string) (domain.Transition, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Transition{}, false, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	row := transitionRow{}
	query := db.NewSelect().Model(&row).ModelTableExpr(transitionsTable).
		Where(`"transition".tenant_id = ?`, tenantID).Where(`"transition".id = ?`, id)
	if lock == "update" {
		query = query.For("UPDATE")
	}
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Transition{}, false, stats, nil
	}
	if err != nil {
		return domain.Transition{}, false, stats, storeError("find transition", err)
	}
	stats.Rows = 1
	mappings, mappingStats, err := s.listMappings(ctx, db, tenantID, []int64{id})
	stats.Add(mappingStats)
	if err != nil {
		return domain.Transition{}, false, stats, err
	}
	transition := transitionToDomain(row)
	transition.Mappings = mappings[id]
	return transition, true, stats, nil
}

func (s *Store) ListTransitions(ctx context.Context, tenantID int64, filter domain.TransitionFilter) ([]domain.Transition, int, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	apply := func(query *bun.SelectQuery) *bun.SelectQuery {
		query = query.Where(`"transition".tenant_id = ?`, tenantID)
		if filter.Status != "" {
			query = query.Where(`"transition".status = ?`, filter.Status)
		}
		if filter.AcademicYear != "" {
			query = query.Where(`"transition".academic_year = ?`, filter.AcademicYear)
		}
		if filter.AfterID > 0 {
			query = query.Where(`"transition".id > ?`, filter.AfterID)
		}
		return query
	}
	started := time.Now()
	total, err := apply(db.NewSelect().Model((*transitionRow)(nil)).ModelTableExpr(transitionsTable)).Count(ctx)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return nil, 0, stats, storeError("count transitions", err)
	}
	rows := []transitionRow{}
	query := apply(db.NewSelect().Model(&rows).ModelTableExpr(transitionsTable))
	if filter.AfterID > 0 {
		query = query.OrderExpr(`"transition".id ASC`)
	} else {
		query = query.OrderExpr(`"transition".created_at DESC, "transition".id DESC`)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	started = time.Now()
	err = query.Scan(ctx)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return nil, 0, stats, storeError("list transitions", err)
	}
	stats.Rows += int64(len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	mappings, mappingStats, err := s.listMappings(ctx, db, tenantID, ids)
	stats.Add(mappingStats)
	if err != nil {
		return nil, 0, stats, err
	}
	result := make([]domain.Transition, 0, len(rows))
	for _, row := range rows {
		transition := transitionToDomain(row)
		transition.Mappings = mappings[row.ID]
		result = append(result, transition)
	}
	return result, total, stats, nil
}

func (s *Store) listMappings(ctx context.Context, db bun.IDB, tenantID int64, transitionIDs []int64) (map[int64][]domain.TransitionMapping, domain.OperationStats, error) {
	result := make(map[int64][]domain.TransitionMapping, len(transitionIDs))
	if len(transitionIDs) == 0 {
		return result, domain.OperationStats{}, nil
	}
	rows := []mappingRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := db.NewSelect().Model(&rows).ModelTableExpr(mappingsTable).
		Where(`"mapping".tenant_id = ?`, tenantID).
		Where(`"mapping".transition_id IN (?)`, bun.List(transitionIDs)).
		OrderExpr(`"mapping".transition_id ASC, "mapping".from_class ASC, "mapping".id ASC`).
		Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, storeError("list transition mappings", err)
	}
	stats.Rows = int64(len(rows))
	for _, row := range rows {
		result[row.TransitionID] = append(result[row.TransitionID], mappingToDomain(row))
	}
	return result, stats, nil
}

func (s *Store) InsertTransition(ctx context.Context, tenantID int64, draft domain.TransitionDraft) (domain.Transition, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Transition{}, domain.OperationStats{}, err
	}
	row := transitionRow{
		TenantID: tenantID, AcademicYear: draft.AcademicYear, Status: domain.TransitionStatusDraft,
		CreatedBy: draft.CreatedBy, Notes: draft.Notes,
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(&row).ModelTableExpr(`education.grade_transitions`).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.Transition{}, stats, storeError("insert transition", err)
	}
	stats.Rows = 1
	transition := transitionToDomain(row)
	if len(draft.Mappings) > 0 {
		mappings, insertStats, err := s.insertMappings(ctx, db, tenantID, row.ID, draft.Mappings)
		stats.Add(insertStats)
		if err != nil {
			return domain.Transition{}, stats, err
		}
		transition.Mappings = mappings
	}
	return transition, stats, nil
}

func (s *Store) insertMappings(ctx context.Context, db bun.IDB, tenantID, transitionID int64, inputs []domain.TransitionMappingInput) ([]domain.TransitionMapping, domain.OperationStats, error) {
	rows := make([]mappingRow, 0, len(inputs))
	for _, input := range inputs {
		rows = append(rows, mappingRow{TenantID: tenantID, TransitionID: transitionID, FromClass: input.FromClass, ToClass: input.ToClass})
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err := db.NewInsert().Model(&rows).ModelTableExpr(`education.grade_transition_mappings`).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, storeError("insert transition mappings", err)
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.TransitionMapping, 0, len(rows))
	for _, row := range rows {
		result = append(result, mappingToDomain(row))
	}
	return result, stats, nil
}

func (s *Store) UpdateTransitionFields(ctx context.Context, tenantID, id int64, academicYear string, notes *string) (int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewUpdate().TableExpr(transitionsTable).
		Set(`academic_year = ?`, academicYear).Set(`notes = ?`, notes).Set(`updated_at = NOW()`).
		Where(`"transition".tenant_id = ?`, tenantID).Where(`"transition".id = ?`, id).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, storeError("update transition", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return 0, stats, storeError("update transition: count rows", err)
	}
	return stats.Rows, stats, nil
}

func (s *Store) ReplaceMappings(ctx context.Context, tenantID, transitionID int64, mappings []domain.TransitionMappingInput) ([]domain.TransitionMapping, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewDelete().TableExpr(mappingsTable).
		Where(`"mapping".tenant_id = ?`, tenantID).Where(`"mapping".transition_id = ?`, transitionID).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, storeError("delete transition mappings", err)
	}
	if len(mappings) == 0 {
		return []domain.TransitionMapping{}, stats, nil
	}
	inserted, insertStats, err := s.insertMappings(ctx, db, tenantID, transitionID, mappings)
	stats.Add(insertStats)
	return inserted, stats, err
}

func (s *Store) DeleteTransition(ctx context.Context, tenantID, id int64) (int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewDelete().TableExpr(transitionsTable).
		Where(`"transition".tenant_id = ?`, tenantID).Where(`"transition".id = ?`, id).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, storeError("delete transition", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return 0, stats, storeError("delete transition: count rows", err)
	}
	return stats.Rows, stats, nil
}

func (s *Store) LockTransitions(ctx context.Context, tenantID int64) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewRaw(`SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`, tenantTransitionsLockKey(tenantID)).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, storeError("lock tenant transitions", err)
	}
	return stats, nil
}

func (s *Store) LockLatestApplied(ctx context.Context, tenantID int64) (domain.Transition, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Transition{}, false, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	row := transitionRow{}
	err = db.NewSelect().Model(&row).ModelTableExpr(transitionsTable).
		Where(`"transition".tenant_id = ?`, tenantID).
		Where(`"transition".status = ?`, domain.TransitionStatusApplied).
		OrderExpr(`"transition".applied_at DESC NULLS LAST, "transition".id DESC`).
		Limit(1).For("UPDATE").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Transition{}, false, stats, nil
	}
	if err != nil {
		return domain.Transition{}, false, stats, storeError("lock latest applied transition", err)
	}
	stats.Rows = 1
	return transitionToDomain(row), true, stats, nil
}

func (s *Store) MarkApplied(ctx context.Context, tenantID, id, accountID int64, at time.Time, rosterBaselineInstanceID *int64) (int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewUpdate().TableExpr(transitionsTable).
		Set(`status = ?`, domain.TransitionStatusApplied).
		Set(`applied_at = ?`, at).Set(`applied_by = ?`, accountID).
		Set(`roster_baseline_instance_id = ?`, rosterBaselineInstanceID).
		Set(`updated_at = NOW()`).
		Where(`"transition".tenant_id = ?`, tenantID).Where(`"transition".id = ?`, id).
		Where(`"transition".status = ?`, domain.TransitionStatusDraft).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, storeError("mark transition applied", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return 0, stats, storeError("mark transition applied: count rows", err)
	}
	return stats.Rows, stats, nil
}

func (s *Store) MarkReverted(ctx context.Context, tenantID, id, accountID int64, at time.Time) (int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewUpdate().TableExpr(transitionsTable).
		Set(`status = ?`, domain.TransitionStatusReverted).
		Set(`reverted_at = ?`, at).Set(`reverted_by = ?`, accountID).
		Set(`updated_at = NOW()`).
		Where(`"transition".tenant_id = ?`, tenantID).Where(`"transition".id = ?`, id).
		Where(`"transition".status = ?`, domain.TransitionStatusApplied).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, storeError("mark transition reverted", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return 0, stats, storeError("mark transition reverted: count rows", err)
	}
	return stats.Rows, stats, nil
}

func (s *Store) InsertHistory(ctx context.Context, tenantID int64, entries []domain.TransitionHistoryEntry) (domain.OperationStats, error) {
	if len(entries) == 0 {
		return domain.OperationStats{}, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	rows := make([]historyRow, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, historyRow{
			TenantID: tenantID, TransitionID: entry.TransitionID, StudentID: entry.StudentID, PersonName: entry.PersonName,
			FromClass: entry.FromClass, ToClass: entry.ToClass, Action: entry.Action, FromStatus: entry.FromStatus, RFIDTag: entry.RFIDTag,
		})
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(&rows).ModelTableExpr(`education.grade_transition_history`).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, storeError("insert transition history", err)
	}
	stats.Rows = int64(len(rows))
	return stats, nil
}

func (s *Store) ListHistory(ctx context.Context, tenantID, transitionID int64) ([]domain.TransitionHistoryEntry, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []historyRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewSelect().Model(&rows).ModelTableExpr(historyTable).
		Where(`"history".tenant_id = ?`, tenantID).Where(`"history".transition_id = ?`, transitionID).
		OrderExpr(`"history".created_at ASC, "history".id ASC`).Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, storeError("list transition history", err)
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.TransitionHistoryEntry, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.TransitionHistoryEntry{
			ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, TransitionID: row.TransitionID, StudentID: row.StudentID,
			PersonName: row.PersonName, FromClass: row.FromClass, ToClass: row.ToClass, Action: row.Action,
			FromStatus: row.FromStatus, RFIDTag: row.RFIDTag,
		})
	}
	return result, stats, nil
}

func (s *Store) InsertClassTeacherLedger(ctx context.Context, tenantID int64, entries []domain.TransitionClassTeacherEntry) (domain.OperationStats, error) {
	if len(entries) == 0 {
		return domain.OperationStats{}, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	rows := make([]classTeacherLedgerRow, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, classTeacherLedgerRow{
			TenantID: tenantID, TransitionID: entry.TransitionID, StaffID: entry.StaffID, SchoolClass: entry.SchoolClass, Action: entry.Action,
		})
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(&rows).ModelTableExpr(`education.grade_transition_class_teachers`).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, storeError("insert class teacher ledger", err)
	}
	stats.Rows = int64(len(rows))
	return stats, nil
}

func (s *Store) ListClassTeacherLedger(ctx context.Context, tenantID, transitionID int64) ([]domain.TransitionClassTeacherEntry, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []classTeacherLedgerRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewSelect().Model(&rows).ModelTableExpr(classTeacherLedgerTable).
		Where(`"ledger".tenant_id = ?`, tenantID).Where(`"ledger".transition_id = ?`, transitionID).
		OrderExpr(`"ledger".created_at ASC, "ledger".id ASC`).Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, storeError("list class teacher ledger", err)
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.TransitionClassTeacherEntry, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.TransitionClassTeacherEntry{
			ID: row.ID, TransitionID: row.TransitionID, StaffID: row.StaffID, SchoolClass: row.SchoolClass, Action: row.Action,
		})
	}
	return result, stats, nil
}

func (s *Store) InsertClassListLedger(ctx context.Context, tenantID int64, entries []domain.TransitionClassListEntry) (domain.OperationStats, error) {
	if len(entries) == 0 {
		return domain.OperationStats{}, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	rows := make([]classListLedgerRow, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, classListLedgerRow{
			TenantID: tenantID, TransitionID: entry.TransitionID, EntryID: entry.EntryID,
			FirstName: entry.FirstName, LastName: entry.LastName, SchoolClass: entry.SchoolClass, Action: entry.Action,
		})
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(&rows).ModelTableExpr(`education.grade_transition_class_list_entries`).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, storeError("insert class list ledger", err)
	}
	stats.Rows = int64(len(rows))
	return stats, nil
}

func (s *Store) ListClassListLedger(ctx context.Context, tenantID, transitionID int64) ([]domain.TransitionClassListEntry, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []classListLedgerRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewSelect().Model(&rows).ModelTableExpr(classListLedgerTable).
		Where(`"ledger".tenant_id = ?`, tenantID).Where(`"ledger".transition_id = ?`, transitionID).
		OrderExpr(`"ledger".created_at ASC, "ledger".id ASC`).Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, storeError("list class list ledger", err)
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.TransitionClassListEntry, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.TransitionClassListEntry{
			ID: row.ID, TransitionID: row.TransitionID, EntryID: row.EntryID,
			FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass, Action: row.Action,
		})
	}
	return result, stats, nil
}

func transitionToDomain(row transitionRow) domain.Transition {
	return domain.Transition{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		AcademicYear: row.AcademicYear, Status: row.Status, AppliedAt: row.AppliedAt, AppliedBy: row.AppliedBy,
		RevertedAt: row.RevertedAt, RevertedBy: row.RevertedBy, CreatedBy: row.CreatedBy, Notes: row.Notes,
		RosterBaselineInstanceID: row.RosterBaselineInstanceID,
	}
}

func mappingToDomain(row mappingRow) domain.TransitionMapping {
	return domain.TransitionMapping{ID: row.ID, TenantID: row.TenantID, TransitionID: row.TransitionID, FromClass: row.FromClass, ToClass: row.ToClass}
}
