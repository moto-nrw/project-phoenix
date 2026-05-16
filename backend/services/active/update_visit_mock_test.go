package active

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateVisitPreloadAndTargetLookupErrors(t *testing.T) {
	ctx := context.Background()
	entryTime := time.Now()
	existingVisit := &activeModels.Visit{
		Model:         base.Model{ID: 100},
		StudentID:     200,
		ActiveGroupID: 300,
		EntryTime:     entryTime,
	}
	updatedVisit := &activeModels.Visit{
		Model:         base.Model{ID: existingVisit.ID},
		StudentID:     existingVisit.StudentID,
		ActiveGroupID: 400,
		EntryTime:     entryTime,
	}

	t.Run("returns visit not found when preload misses", func(t *testing.T) {
		svc := &service{
			visitRepo: &mockVisitRepository{
				findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
					return nil, sql.ErrNoRows
				},
			},
		}

		err := svc.UpdateVisit(ctx, updatedVisit)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrVisitNotFound), "expected ErrVisitNotFound")
	})

	t.Run("returns visit not found when preload returns nil without error", func(t *testing.T) {
		svc := &service{
			visitRepo: &mockVisitRepository{
				findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
					return nil, nil
				},
			},
		}

		err := svc.UpdateVisit(ctx, updatedVisit)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrVisitNotFound), "expected ErrVisitNotFound")
	})

	t.Run("returns active group not found when target lookup misses", func(t *testing.T) {
		svc := &service{
			visitRepo: &mockVisitRepository{
				findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
					return existingVisit, nil
				},
			},
			groupRepo: &mockGroupRepository{
				findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
					return nil, sql.ErrNoRows
				},
			},
		}

		err := svc.UpdateVisit(ctx, updatedVisit)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrActiveGroupNotFound), "expected ErrActiveGroupNotFound")
	})

	t.Run("preserves database errors from target lookup", func(t *testing.T) {
		lookupErr := errors.New("target lookup failed")
		svc := &service{
			visitRepo: &mockVisitRepository{
				findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
					return existingVisit, nil
				},
			},
			groupRepo: &mockGroupRepository{
				findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
					return nil, lookupErr
				},
			},
		}

		err := svc.UpdateVisit(ctx, updatedVisit)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrDatabaseOperation), "expected ErrDatabaseOperation")
	})

	t.Run("returns active group not found when target lookup returns nil without error", func(t *testing.T) {
		svc := &service{
			visitRepo: &mockVisitRepository{
				findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
					return existingVisit, nil
				},
			},
			groupRepo: &mockGroupRepository{
				findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
					return nil, nil
				},
			},
		}

		err := svc.UpdateVisit(ctx, updatedVisit)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrActiveGroupNotFound), "expected ErrActiveGroupNotFound")
	})

	t.Run("returns active group not found when target group is inactive", func(t *testing.T) {
		endTime := entryTime.Add(time.Hour)
		svc := &service{
			visitRepo: &mockVisitRepository{
				findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
					return existingVisit, nil
				},
			},
			groupRepo: &mockGroupRepository{
				findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
					return &activeModels.Group{
						Model:   base.Model{ID: updatedVisit.ActiveGroupID},
						EndTime: &endTime,
					}, nil
				},
			},
		}

		err := svc.UpdateVisit(ctx, updatedVisit)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrActiveGroupNotFound), "expected ErrActiveGroupNotFound")
	})

	t.Run("updates without target lookup when active group does not change", func(t *testing.T) {
		updateCalled := false
		sameGroupVisit := &activeModels.Visit{
			Model:         base.Model{ID: existingVisit.ID},
			StudentID:     existingVisit.StudentID,
			ActiveGroupID: existingVisit.ActiveGroupID,
			EntryTime:     entryTime,
		}
		svc := &service{
			visitRepo: &mockVisitRepository{
				findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
					return existingVisit, nil
				},
				updateFunc: func(context.Context, *activeModels.Visit) error {
					updateCalled = true
					return nil
				},
			},
			groupRepo: &mockGroupRepository{
				findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
					t.Fatal("target group should not be loaded when active group is unchanged")
					return nil, nil
				},
			},
		}

		err := svc.UpdateVisit(ctx, sameGroupVisit)

		require.NoError(t, err)
		assert.True(t, updateCalled, "expected visit update")
	})
}
