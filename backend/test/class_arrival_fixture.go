package test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// CreateTestClassArrivalTime creates one tenant-owned dismissal timetable.
func CreateTestClassArrivalTime(tb testing.TB, db *bun.DB, class string, times map[string]string) *education.ClassArrivalTime {
	tb.Helper()
	row := &education.ClassArrivalTime{SchoolClass: class, ArrivalTimes: times}
	row.SetTenantID(fixtureTenantID(tb))
	_, err := db.NewInsert().Model(row).ModelTableExpr(`education.class_arrival_times`).Exec(Ctx(tb))
	require.NoError(tb, err)
	return row
}
