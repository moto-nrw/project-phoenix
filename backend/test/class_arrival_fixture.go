package test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ClassArrivalTimeRow is one class's Unterrichtsschluss per weekday, as
// education.class_arrival_times stores it. The table belongs to the Timetable
// owner; the fixtures write and read it directly.
type ClassArrivalTimeRow struct {
	ID           int64             `bun:"id,pk,autoincrement"`
	TenantID     int64             `bun:"tenant_id,notnull"`
	CreatedAt    time.Time         `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt    time.Time         `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	SchoolClass  string            `bun:"school_class,notnull"`
	ArrivalTimes map[string]string `bun:"arrival_times,type:jsonb"`
	UpdatedBy    *int64            `bun:"updated_by,nullzero"`
}

// CreateTestClassArrivalTime creates one tenant-owned dismissal timetable.
func CreateTestClassArrivalTime(tb testing.TB, db *bun.DB, class string, times map[string]string) *ClassArrivalTimeRow {
	tb.Helper()
	row := &ClassArrivalTimeRow{SchoolClass: class, ArrivalTimes: times, TenantID: fixtureTenantID(tb)}
	_, err := db.NewInsert().Model(row).ModelTableExpr(`education.class_arrival_times`).Exec(Ctx(tb))
	require.NoError(tb, err)
	return row
}

// UpsertTestClassArrivalTime stores the weekday map of one class, replacing
// the class's row when it already has one, like the maintenance screen.
func UpsertTestClassArrivalTime(tb testing.TB, db *bun.DB, class string, times map[string]string) *ClassArrivalTimeRow {
	tb.Helper()
	row := &ClassArrivalTimeRow{SchoolClass: class, ArrivalTimes: times, TenantID: fixtureTenantID(tb)}
	_, err := db.NewInsert().
		Model(row).
		ModelTableExpr(`education.class_arrival_times`).
		On("CONFLICT (tenant_id, (LOWER(BTRIM(school_class)))) DO UPDATE").
		Set("arrival_times = EXCLUDED.arrival_times").
		Set("school_class = EXCLUDED.school_class").
		Set("updated_by = EXCLUDED.updated_by").
		Set("updated_at = NOW()").
		Returning("*").
		Exec(Ctx(tb))
	require.NoError(tb, err)
	return row
}

// ClassArrivalTimesOf returns the stored rows of one class, matched on the
// normalized class like every school_class join.
func ClassArrivalTimesOf(tb testing.TB, db *bun.DB, class string) []ClassArrivalTimeRow {
	tb.Helper()
	rows := make([]ClassArrivalTimeRow, 0)
	err := db.NewSelect().
		Model(&rows).
		ModelTableExpr(`education.class_arrival_times AS class_arrival_time_row`).
		Where(`class_arrival_time_row.tenant_id = ?`, fixtureTenantID(tb)).
		Where(`LOWER(BTRIM(class_arrival_time_row.school_class)) = LOWER(BTRIM(?))`, class).
		Scan(Ctx(tb))
	require.NoError(tb, err)
	return rows
}
