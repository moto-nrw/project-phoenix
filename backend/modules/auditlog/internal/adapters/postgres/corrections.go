package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	"github.com/uptrace/bun"
)

type Database func(context.Context) (bun.IDB, int64, error)

type Corrections struct{ database Database }

func NewCorrections(database Database) *Corrections { return &Corrections{database: database} }

func (s *Corrections) ListDirectCorrections(ctx context.Context, filter auditlog.CorrectionFilter) ([]auditlog.DirectCorrection, auditlog.Observation, error) {
	observation := auditlog.Observation{Operation: "list_direct_corrections"}
	if filter.Limit <= 0 {
		return []auditlog.DirectCorrection{}, observation, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, observation, err
	}
	rows := []struct {
		ID                 int64           `bun:"id"`
		StudentID          int64           `bun:"student_id"`
		ActorNameSnapshot  *string         `bun:"actor_name_snapshot"`
		ActorEmailSnapshot *string         `bun:"actor_email_snapshot"`
		Reason             string          `bun:"reason"`
		Before             json.RawMessage `bun:"before_json,type:jsonb"`
		After              json.RawMessage `bun:"after_json,type:jsonb"`
		ChangedAt          time.Time       `bun:"changed_at"`
	}{}
	query := db.NewSelect().Model(&rows).TableExpr("audit.enrollment_offering_adjustments AS adjustment").
		ColumnExpr("adjustment.id, adjustment.student_id, adjustment.actor_name_snapshot, adjustment.actor_email_snapshot, adjustment.reason, adjustment.before_json, adjustment.after_json, adjustment.changed_at").
		Where("adjustment.tenant_id = ?", tenantID).Where("adjustment.source = ?", "direct").
		OrderExpr("adjustment.changed_at DESC, adjustment.id DESC").Limit(filter.Limit)
	if !filter.BeforeInstant.IsZero() {
		query = query.Where("(adjustment.changed_at, adjustment.id) < (?, ?)", filter.BeforeInstant, filter.BeforeID)
	}
	started := time.Now()
	err = query.Scan(ctx)
	observation.Queries, observation.StatementDuration = 1, time.Since(started)
	if err != nil {
		return nil, observation, fmt.Errorf("audit database error during list direct enrollment offering adjustments: %w", err)
	}
	result := make([]auditlog.DirectCorrection, 0, len(rows))
	for _, row := range rows {
		result = append(result, auditlog.DirectCorrection{ID: row.ID, StudentID: row.StudentID, ActorNameSnapshot: row.ActorNameSnapshot, ActorEmailSnapshot: row.ActorEmailSnapshot, Reason: row.Reason, Before: row.Before, After: row.After, ChangedAt: row.ChangedAt})
	}
	observation.Rows = int64(len(result))
	return result, observation, nil
}
