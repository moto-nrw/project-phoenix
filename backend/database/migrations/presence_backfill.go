package migrations

import (
	"context"
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

// PresenceBackfillComparison compares canonical, ID-ordered field payloads in
// one repeatable-read snapshot. Target surrogate IDs are not source identities.
type PresenceBackfillComparison struct {
	SourceCount             int64   `json:"source_count"`
	TargetCount             int64   `json:"target_count"`
	SourceChecksum          string  `json:"source_checksum"`
	TargetChecksum          string  `json:"target_checksum"`
	Mismatches              int64   `json:"mismatches"`
	OldestUnmigratedSeconds float64 `json:"oldest_unmigrated_seconds"`
}

// PresenceBackfillReport is persisted atomically with each batch. Complete is
// evidence at VerifiedAt, not permission to switch callers: old writers remain
// authoritative and Cutover must run another final delta after fencing them.
type PresenceBackfillReport struct {
	TenantID                  int64                      `json:"tenant_id"`
	Phase                     string                     `json:"phase"`
	Pass                      int64                      `json:"pass"`
	SessionHighWater          int64                      `json:"session_high_water"`
	AttendanceHighWater       int64                      `json:"attendance_high_water"`
	RowsScanned               int64                      `json:"rows_scanned"`
	RowsCopied                int64                      `json:"rows_copied"`
	RowsSkipped               int64                      `json:"rows_skipped"`
	RowsRemoved               int64                      `json:"rows_removed"`
	BatchesCompleted          int64                      `json:"batches_completed"`
	Mismatches                int64                      `json:"mismatches"`
	Orphans                   int64                      `json:"orphans"`
	Complete                  bool                       `json:"complete"`
	VerifiedAt                *time.Time                 `json:"verified_at,omitempty"`
	FinalDeltaSnapshot        string                     `json:"final_delta_snapshot,omitempty"`
	Sessions                  PresenceBackfillComparison `json:"sessions"`
	Attendance                PresenceBackfillComparison `json:"attendance"`
	PlannedOccurrences        int64                      `json:"planned_occurrences"`
	PlannedAssignments        int64                      `json:"planned_assignments"`
	PlannedOccurrenceChecksum string                     `json:"planned_occurrence_checksum"`
	PlannedAssignmentChecksum string                     `json:"planned_assignment_checksum"`
}

func readPresenceCheckpoint(ctx context.Context, db bun.IDB, tenantID int64, lock bool) (PresenceBackfillReport, error) {
	var data []byte
	var err error
	if lock {
		err = db.NewRaw(`SELECT state FROM active.presence_backfill_checkpoints WHERE tenant_id = ? FOR UPDATE`, tenantID).Scan(ctx, &data)
	} else {
		err = db.NewRaw(`SELECT state FROM active.presence_backfill_checkpoints WHERE tenant_id = ?`, tenantID).Scan(ctx, &data)
	}
	if err != nil {
		return PresenceBackfillReport{}, err
	}
	var report PresenceBackfillReport
	err = json.Unmarshal(data, &report)
	return report, err
}
