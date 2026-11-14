package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// linkedChildAnswersQuery reads every answered registration child of the
// school that created a student, with the pinned form version, newest
// registration first. Office matches stay out (#6053): their answers belong
// to another child's registration, as on the child's page.
const linkedChildAnswersQuery = `SELECT child.created_student_id AS student_id, request.schema_id, request.submitted_at, request.id AS request_id, child.custom_data
	FROM enrollment.request_children AS child
	JOIN enrollment.requests AS request ON request.id = child.request_id AND request.tenant_id = child.tenant_id
	WHERE child.tenant_id = ? AND child.created_student_id IS NOT NULL AND NOT child.matched_by_office
		AND request.schema_id IS NOT NULL AND child.custom_data <> '{}'::jsonb
	ORDER BY request.submitted_at DESC, request.id DESC, child.id`

type linkedChildAnswersRow struct {
	StudentID   int64           `bun:"student_id"`
	SchemaID    int64           `bun:"schema_id"`
	SubmittedAt time.Time       `bun:"submitted_at"`
	RequestID   int64           `bun:"request_id"`
	CustomData  json.RawMessage `bun:"custom_data,type:jsonb"`
}

// LinkedChildAnswers reads the answers of every registration child linked to
// a student of the school in context, in one statement.
func (r *Store) LinkedChildAnswers(ctx context.Context) ([]enrollment.LinkedChildAnswers, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []linkedChildAnswersRow
	if err := db.NewRaw(linkedChildAnswersQuery, tenantID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("failed to list linked child answers: %w", err)
	}
	out := make([]enrollment.LinkedChildAnswers, 0, len(rows))
	for _, row := range rows {
		value := enrollment.LinkedChildAnswers{StudentID: row.StudentID, SchemaID: row.SchemaID, SubmittedAt: row.SubmittedAt, RequestID: row.RequestID}
		if err := json.Unmarshal(row.CustomData, &value.CustomData); err != nil {
			return nil, fmt.Errorf("decode linked child answers of request %d: %w", row.RequestID, err)
		}
		out = append(out, value)
	}
	return out, nil
}
