package postgres

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

type visitRow struct {
	bun.BaseModel `bun:"table:active.visits,alias:visit"`
	ID            int64      `bun:"id,pk,autoincrement"`
	TenantID      int64      `bun:"tenant_id,notnull"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID     int64      `bun:"student_id,notnull"`
	ActiveGroupID int64      `bun:"active_group_id,notnull"`
	EntryTime     time.Time  `bun:"entry_time,notnull"`
	ExitTime      *time.Time `bun:"exit_time"`
}

func visitRowFromRecord(row *ports.Visit) *visitRow {
	return &visitRow{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StudentID: row.StudentID, ActiveGroupID: row.ActiveGroupID, EntryTime: row.EntryTime, ExitTime: row.ExitTime}
}

func (row *visitRow) record() *ports.Visit {
	return &ports.Visit{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StudentID: row.StudentID, ActiveGroupID: row.ActiveGroupID, EntryTime: row.EntryTime, ExitTime: row.ExitTime}
}

func visitRecordsFromRows(rows []*visitRow) []*ports.Visit {
	result := make([]*ports.Visit, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.record())
	}
	return result
}
