package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/uptrace/bun"
)

// DemoSchoolFileStore is the demo process's side of the file erasure of
// retired demo schools (#5544), on its restricted phoenix_demo connection.
type DemoSchoolFileStore struct{ db *bun.DB }

func NewDemoSchoolFileStore(db *bun.DB) *DemoSchoolFileStore { return &DemoSchoolFileStore{db: db} }

// retiredDemoSchoolDue selects a school whose files are due: hidden for at
// least an hour, so that no request of its last visit still writes, without
// a marker, not the school of an order still being seeded and never the
// standing school. Only a school with a recorded demo origin qualifies: the
// demo database is meant to hold demo schools only (ADR 0029), but that is an
// operating assumption, not a guarantee. The order of a retired school may
// already be erased (#4048), so the origin row is the proof, not the order.
// The single bind parameter is the standing school's slug.
const retiredDemoSchoolDue = `school.deleted_at < CURRENT_TIMESTAMP - INTERVAL '1 hour'
	AND EXISTS (SELECT 1 FROM platform.demo_school_origins AS origin WHERE origin.school_id = school.id)
	AND NOT EXISTS (SELECT 1 FROM platform.demo_school_states AS state
		WHERE (state.tenant_id = school.id AND state.status = 'preparing')
			OR (state.name = ? AND (state.tenant_id = school.id OR school.id = ANY(state.visitor_school_ids))))
	AND NOT EXISTS (SELECT 1 FROM platform.demo_school_file_erasures AS done WHERE done.school_id = school.id)`

// listRetiredDemoSchools binds the standing slug and the batch size.
const listRetiredDemoSchools = `SELECT school.id FROM platform.schools AS school WHERE ` + retiredDemoSchoolDue + `
	ORDER BY school.deleted_at, school.id LIMIT ?`

// recordDemoSchoolFileErasure binds the counts, the school and the standing
// slug. It checks again that the school is due, in the same statement.
const recordDemoSchoolFileErasure = `INSERT INTO platform.demo_school_file_erasures (school_id, file_counts)
	SELECT school.id, ?::jsonb FROM platform.schools AS school WHERE school.id = ? AND ` + retiredDemoSchoolDue + `
	ON CONFLICT (school_id) DO NOTHING`

func (s *DemoSchoolFileStore) RetiredDemoSchools(ctx context.Context, standingSlug string, limit int) ([]int64, error) {
	ids := []int64{}
	if err := s.db.NewRaw(listRetiredDemoSchools, standingSlug, limit).Scan(ctx, &ids); err != nil {
		return nil, fmt.Errorf("list retired demo schools: %w", err)
	}
	return ids, nil
}

func (s *DemoSchoolFileStore) RecordDemoSchoolFileErasure(ctx context.Context, standingSlug string, schoolID int64, counts map[string]int64) (bool, error) {
	data, err := json.Marshal(counts)
	if err != nil {
		return false, fmt.Errorf("encode demo school file counts: %w", err)
	}
	result, err := s.db.NewRaw(recordDemoSchoolFileErasure, string(data), schoolID, standingSlug).Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("record demo school file erasure: %w", err)
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}
