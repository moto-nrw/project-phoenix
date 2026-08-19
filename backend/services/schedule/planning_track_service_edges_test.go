package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func TestPlanningTrackServiceValidationAndMissingEdges(t *testing.T) {
	repo := newPlanningTrackRepoStub()
	service := NewPlanningTrackService(repo, nil)
	invalid := PlanningTrackInput{Name: "", Color: "blue", SortOrder: -1}

	if _, err := service.CreatePlanningTrack(context.Background(), invalid); !errors.Is(err, ErrPlanningTrackInvalid) {
		t.Fatalf("create invalid planning track: got %v", err)
	}
	if _, err := service.UpdatePlanningTrack(context.Background(), 0, invalid); !errors.Is(err, ErrPlanningTrackNotFound) {
		t.Fatalf("update missing planning track: got %v", err)
	}
	if _, err := service.ArchivePlanningTrack(context.Background(), 0); !errors.Is(err, ErrPlanningTrackNotFound) {
		t.Fatalf("archive missing planning track: got %v", err)
	}
	if _, err := service.RestorePlanningTrack(context.Background(), 0); !errors.Is(err, ErrPlanningTrackNotFound) {
		t.Fatalf("restore missing planning track: got %v", err)
	}
}

func TestPlanningTrackServiceReorderReturnsRepositoryError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create database mock: %v", err)
	}
	db := bun.NewDB(sqlDB, pgdialect.New())
	wantErr := errors.New("reorder failed")
	repo := newPlanningTrackRepoStub()
	repo.updateSortOrdersErr = wantErr
	tenantID := time.Now().UnixNano()
	ctx := tenant.WithTenantID(context.Background(), tenantID)

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_tenant").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SELECT set_config").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	err = NewPlanningTrackService(repo, db).ReorderPlanningTracks(ctx, []int64{tenantID})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("database expectations: %v", err)
	}
}
