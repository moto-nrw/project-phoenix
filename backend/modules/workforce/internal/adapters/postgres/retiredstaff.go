package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// EraseRetiredStaffRecords goes through the database function of 1.15.459:
// the tenant role has no DELETE on staff documents and qualifications, and
// the function erases only a staff member of the transaction's school who
// left before the cutoff, queueing the document files that still exist.
func (s *Store) EraseRetiredStaffRecords(ctx context.Context, staffID int64, retiredBefore time.Time) (map[string]int64, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var encoded []byte
	if err := db.NewRaw(`SELECT users.erase_retired_staff_records(?, ?, ?)`, scope.TenantID, staffID, retiredBefore).Scan(ctx, &encoded); err != nil {
		return nil, fmt.Errorf("workforce postgres: erase retired staff records: %w", err)
	}
	counts := map[string]int64{}
	if err := json.Unmarshal(encoded, &counts); err != nil {
		return nil, fmt.Errorf("workforce postgres: erase retired staff records: decode counts: %w", err)
	}
	return counts, nil
}
