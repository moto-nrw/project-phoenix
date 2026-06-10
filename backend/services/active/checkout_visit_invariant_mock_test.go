package active

import (
	"context"
	"errors"
	"testing"
	"time"

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
	lookupErr := errors.New("connection reset")
	svc := &service{
		visitRepo: &mockVisitRepository{
			getCurrentByStudentIDFunc: func(context.Context, int64) (*activeModels.Visit, error) {
				return nil, lookupErr
			},
		},
	}

	err := svc.endOpenVisitForStudent(context.Background(), 4711)

	require.Error(t, err, "a non-NotFound lookup failure must propagate so the checkout transaction rolls back")
	assert.False(t, errors.Is(err, ErrVisitNotFound))
}

func TestEndOpenVisitForStudent_AlreadyEndedIsTolerated(t *testing.T) {
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

	svc := &service{
		visitRepo: &mockVisitRepository{
			getCurrentByStudentIDFunc: func(context.Context, int64) (*activeModels.Visit, error) {
				return &openView, nil
			},
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
				return endedVisit, nil
			},
		},
	}

	err := svc.endOpenVisitForStudent(context.Background(), 4711)

	require.NoError(t, err, "a visit ended by a concurrent caller is the desired end state, not an error")
}

func TestEndOpenVisitForStudent_EndVisitErrorPropagates(t *testing.T) {
	openVisit := &activeModels.Visit{
		Model:     base.Model{ID: 4713},
		StudentID: 4711,
		EntryTime: time.Now().Add(-1 * time.Hour),
	}

	svc := &service{
		visitRepo: &mockVisitRepository{
			getCurrentByStudentIDFunc: func(context.Context, int64) (*activeModels.Visit, error) {
				return openVisit, nil
			},
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
				return openVisit, nil
			},
			endVisitFunc: func(context.Context, int64) error {
				return errors.New("disk full")
			},
		},
	}

	err := svc.endOpenVisitForStudent(context.Background(), 4711)

	require.Error(t, err, "an EndVisit failure must propagate so attendance close and visit end stay atomic")
	assert.False(t, errors.Is(err, ErrVisitAlreadyEnded))
}
