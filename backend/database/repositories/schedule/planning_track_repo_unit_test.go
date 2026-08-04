package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	model "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func newPlanningTrackMockRepository(t *testing.T) (*PlanningTrackRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, db.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})
	return NewPlanningTrackRepository(db).(*PlanningTrackRepository), mock
}

func TestPlanningTrackRepositoryReadErrors(t *testing.T) {
	repo, mock := newPlanningTrackMockRepository(t)
	wantErr := errors.New("read failed")
	mock.ExpectQuery("SELECT").WillReturnError(wantErr)
	tracks, err := repo.ListAll(context.Background())
	require.Nil(t, tracks)
	require.ErrorContains(t, err, wantErr.Error())

	mock.ExpectQuery("SELECT").WillReturnError(wantErr)
	track, err := repo.FindByIDForShare(context.Background(), time.Now().UnixNano())
	require.Nil(t, track)
	require.ErrorContains(t, err, wantErr.Error())
}

func TestPlanningTrackRepositoryUpdateErrors(t *testing.T) {
	repo, mock := newPlanningTrackMockRepository(t)
	track := &model.PlanningTrack{Name: "Spur", Color: "#5080D8"}
	track.ID = time.Now().UnixNano()
	wantErr := errors.New("update failed")
	mock.ExpectExec("UPDATE").WillReturnError(wantErr)
	updated, err := repo.UpdateIfActive(context.Background(), track)
	require.False(t, updated)
	require.ErrorContains(t, err, wantErr.Error())

	mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewErrorResult(wantErr))
	updated, err = repo.UpdateIfActive(context.Background(), track)
	require.False(t, updated)
	require.ErrorContains(t, err, wantErr.Error())

	ctx := tenant.WithTenantID(context.Background(), time.Now().UnixNano())
	archivedAt := time.Now()
	track.ArchivedAt = &archivedAt
	mock.ExpectQuery("UPDATE").WillReturnError(wantErr)
	restored, err := repo.RestoreAtEnd(ctx, track)
	require.False(t, restored)
	require.ErrorContains(t, err, wantErr.Error())

	mock.ExpectQuery("UPDATE").WillReturnRows(sqlmock.NewRows([]string{"sort_order"}))
	restored, err = repo.RestoreAtEnd(ctx, track)
	require.NoError(t, err)
	require.False(t, restored)

	mock.ExpectQuery("UPDATE").WillReturnRows(sqlmock.NewRows([]string{"sort_order"}).AddRow(4))
	restored, err = repo.RestoreAtEnd(ctx, track)
	require.NoError(t, err)
	require.True(t, restored)
	require.Equal(t, 4, track.SortOrder)
	require.Nil(t, track.ArchivedAt)
}

func TestPlanningTrackRepositoryReorderErrors(t *testing.T) {
	repo, mock := newPlanningTrackMockRepository(t)
	ctx := tenant.WithTenantID(context.Background(), time.Now().UnixNano())
	wantErr := errors.New("reorder failed")
	mock.ExpectQuery("SELECT").WillReturnError(wantErr)
	require.ErrorContains(t, repo.UpdateSortOrders(ctx, []int64{time.Now().UnixNano()}), wantErr.Error())

	firstID := time.Now().UnixNano()
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(firstID))
	mock.ExpectExec("UPDATE").WillReturnError(wantErr)
	require.ErrorContains(t, repo.UpdateSortOrders(ctx, []int64{firstID}), wantErr.Error())

	secondID := firstID + 1
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(secondID))
	mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 0))
	require.Error(t, repo.UpdateSortOrders(ctx, []int64{secondID}))
}
