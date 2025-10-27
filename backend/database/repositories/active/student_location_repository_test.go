package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func newMockDB(t *testing.T) (*bun.DB, sqlmock.Sqlmock, func()) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	cleanup := func() {
		_ = bunDB.Close()
		_ = sqlDB.Close()
	}

	return bunDB, mock, cleanup
}

func TestAttendanceRepository_GetTodayByStudentIDs(t *testing.T) {
	ctx := context.Background()
	studentIDs := []int64{101, 202, 101} // includes duplicate to ensure de-duplication

	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	repo := activeRepo.NewAttendanceRepository(db)
	today := time.Now().UTC().Truncate(24 * time.Hour)

	rows := sqlmock.NewRows([]string{
		"id", "created_at", "updated_at",
		"student_id", "date", "check_in_time", "check_out_time",
		"checked_in_by", "checked_out_by", "device_id",
	}).
		AddRow(int64(1), today, today, int64(101), today, today.Add(-1*time.Hour), nil, int64(11), nil, int64(501)).
		AddRow(int64(2), today, today, int64(101), today, today.Add(-3*time.Hour), nil, int64(11), nil, int64(501)).
		AddRow(int64(3), today, today, int64(202), today, today.Add(-30*time.Minute), today.Add(-5*time.Minute), int64(12), nil, int64(502))

	mock.ExpectQuery(`SELECT .* FROM active\.attendance AS "attendance"`).
		WillReturnRows(rows)

	result, err := repo.GetTodayByStudentIDs(ctx, studentIDs)
	require.NoError(t, err)
	require.Len(t, result, 2)

	if attendance, ok := result[int64(101)]; assert.True(t, ok) {
		assert.Equal(t, int64(1), attendance.ID)
		assert.Equal(t, today.Add(-1*time.Hour), attendance.CheckInTime)
	}
	if attendance, ok := result[int64(202)]; assert.True(t, ok) {
		assert.NotNil(t, attendance.CheckOutTime)
	}

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVisitRepository_GetCurrentByStudentIDs(t *testing.T) {
	ctx := context.Background()
	studentIDs := []int64{301, 302}

	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	repo := activeRepo.NewVisitRepository(db)
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "created_at", "updated_at",
		"student_id", "active_group_id", "entry_time", "exit_time",
	}).
		AddRow(int64(10), now, now, int64(301), int64(401), now.Add(-15*time.Minute), nil).
		AddRow(int64(11), now, now, int64(302), int64(402), now.Add(-10*time.Minute), nil)

	mock.ExpectQuery(`SELECT .* FROM active\.visits AS "visit"`).
		WillReturnRows(rows)

	visits, err := repo.GetCurrentByStudentIDs(ctx, studentIDs)
	require.NoError(t, err)
	require.Len(t, visits, 2)
	assert.Equal(t, int64(401), visits[int64(301)].ActiveGroupID)
	assert.Equal(t, int64(402), visits[int64(302)].ActiveGroupID)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGroupRepository_FindByIDs(t *testing.T) {
	ctx := context.Background()
	groupIDs := []int64{501, 502}

	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	repo := activeRepo.NewGroupRepository(db)
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "created_at", "updated_at",
		"start_time", "end_time", "last_activity", "timeout_minutes",
		"group_id", "device_id", "room_id",
	}).
		AddRow(int64(501), now, now, now, nil, now, 30, int64(1001), nil, int64(9001)).
		AddRow(int64(502), now, now, now, nil, now, 45, int64(1002), nil, int64(9002))

	mock.ExpectQuery(`SELECT .* FROM active\.groups AS "group"`).
		WillReturnRows(rows)

	roomRows := sqlmock.NewRows([]string{
		"id", "created_at", "updated_at",
		"name", "building", "floor", "capacity", "category", "color",
	}).
		AddRow(int64(9001), now, now, "Library", "Main", 1, 20, "Study", "#FFFFFF").
		AddRow(int64(9002), now, now, "Lab", "Annex", 2, 25, "Science", "#000000")

	mock.ExpectQuery(`SELECT .* FROM facilities\.rooms AS "room"`).
		WillReturnRows(roomRows)

	groups, err := repo.FindByIDs(ctx, groupIDs)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	assert.Equal(t, "Library", groups[int64(501)].Room.Name)
	assert.Equal(t, int64(9002), groups[int64(502)].RoomID)

	require.NoError(t, mock.ExpectationsWereMet())
}
