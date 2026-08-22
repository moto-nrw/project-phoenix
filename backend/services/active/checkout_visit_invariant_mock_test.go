package active

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit tests for the error branches of endOpenVisitForStudent (issue #895).
// The happy paths (open visit ended, no visit at all) are covered by the
// hermetic service and handler tests; the branches below need failure
// injection, so they use the in-package mockVisitRepository like the other
// *_mock_test.go files.

func TestEndOpenVisitForStudent_LookupErrorPropagates(t *testing.T) {
	t.Parallel()

	lookupErr := errors.New("connection reset")
	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: &mockVisitRepository{
		getCurrentByStudentIDFunc: func(context.Context, int64) (*activeModels.Visit, error) {
			return nil, lookupErr
		},
	}},
	}

	_, err := svc.endOpenVisitForStudent(context.Background(), 4711, timezone.TodayDate())

	require.Error(t, err, "a non-NotFound lookup failure must propagate so the checkout transaction rolls back")
	assert.False(t, errors.Is(err, ErrVisitNotFound))
}

func TestEndOpenVisitForStudent_AlreadyEndedIsTolerated(t *testing.T) {
	t.Parallel()

	// GetCurrentByStudentID still reports the visit as open, but by the time
	// EndVisit re-reads it the exit time is set — the concurrent-caller race.
	exitTime := time.Now()
	endedVisit := &activeModels.Visit{
		Model:     base.Model{ID: 4712},
		StudentID: 4711,
		EntryTime: time.Now().Add(-1 * time.Hour),
		ExitTime:  &exitTime,
	}
	openView := *endedVisit
	openView.ExitTime = nil

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: &mockVisitRepository{
		getCurrentByStudentIDFunc: func(context.Context, int64) (*activeModels.Visit, error) {
			return &openView, nil
		},
		findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
			return endedVisit, nil
		},
	}},
	}

	result, err := svc.endOpenVisitForStudent(context.Background(), 4711, timezone.TodayDate())

	require.NoError(t, err, "a visit ended by a concurrent caller is the desired end state, not an error")
	assert.Same(t, endedVisit, result)
}

func TestEndOpenVisitForStudent_BinaryModeStillEndsStaleVisit(t *testing.T) {
	t.Parallel()

	openVisit := &activeModels.Visit{
		Model:     base.Model{ID: 4714},
		StudentID: 4711,
		EntryTime: time.Now().Add(-1 * time.Hour),
	}
	endCalled := false

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: &mockVisitRepository{
		getCurrentByStudentIDFunc: func(context.Context, int64) (*activeModels.Visit, error) {
			return openVisit, nil
		},
		endVisitFunc: func(_ context.Context, id int64) error {
			assert.Equal(t, openVisit.ID, id)
			endCalled = true
			return nil
		},
		findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
			ended := *openVisit
			exitTime := time.Now()
			ended.ExitTime = &exitTime
			return &ended, nil
		},
	}}, settings: &stubSettingsResolver{
		stringValues: map[string]string{"operations.presence_mode": "binary"},
	},
	}

	_, err := svc.endOpenVisitForStudent(context.Background(), 4711, timezone.TodayDate())

	require.NoError(t, err)
	assert.True(t, endCalled, "checkout stale-visit healing must bypass the binary-mode EndVisit no-op")
}

func TestEndOpenVisitForStudent_EndVisitErrorPropagates(t *testing.T) {
	t.Parallel()

	openVisit := &activeModels.Visit{
		Model:     base.Model{ID: 4713},
		StudentID: 4711,
		EntryTime: time.Now().Add(-1 * time.Hour),
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: &mockVisitRepository{
		getCurrentByStudentIDFunc: func(context.Context, int64) (*activeModels.Visit, error) {
			return openVisit, nil
		},
		findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
			return openVisit, nil
		},
		endVisitFunc: func(context.Context, int64) error {
			return errors.New("disk full")
		},
	}},
	}

	_, err := svc.endOpenVisitForStudent(context.Background(), 4711, timezone.TodayDate())

	require.Error(t, err, "an EndVisit failure must propagate so attendance close and visit end stay atomic")
	assert.False(t, errors.Is(err, ErrVisitAlreadyEnded))
}

func TestEndOpenVisitForStudent_NextDayVisitIsLeftAlone(t *testing.T) {
	t.Parallel()

	// A batch checkout crossing Berlin midnight closes its snapshot day's
	// attendance; a room visit the student started AFTER that day belongs to
	// the new day's session and must stay open (review #2372).
	endCalled := false
	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: &mockVisitRepository{
		getCurrentByStudentIDFunc: func(context.Context, int64) (*activeModels.Visit, error) {
			return &activeModels.Visit{
				Model:     base.Model{ID: 4715},
				StudentID: 4711,
				EntryTime: time.Now(),
			}, nil
		},
		endVisitFunc: func(context.Context, int64) error {
			endCalled = true
			return nil
		},
	}},
	}

	result, err := svc.endOpenVisitForStudent(context.Background(), 4711, timezone.TodayDate().AddDays(-1))

	require.NoError(t, err)
	assert.Nil(t, result, "a newer-day visit reports as nothing-to-end, not as an ended row")
	assert.False(t, endCalled, "a visit entered after the checkout's day must not be ended")
}
