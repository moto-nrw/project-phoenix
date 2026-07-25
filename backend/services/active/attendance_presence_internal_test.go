package active

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/stretchr/testify/assert"
)

func TestEnsureStaffPresenceKeepsCheckInFailureBestEffort(t *testing.T) {
	called := false
	workSessions := &workSessionServiceForSessionUnitTest{
		ensureCheckedInFunc: func(_ context.Context, _ int64, _ string) (*activeModels.WorkSession, error) {
			called = true
			return nil, errors.New("check-in failed")
		},
	}
	svc := &service{ServiceDependencies: ServiceDependencies{
		WorkSessionService: workSessions,
		Logger:             slog.New(slog.DiscardHandler),
	}}

	svc.ensureStaffPresence(context.Background(), 42, activeModels.WorkSessionSourceApp)

	assert.True(t, called)
}

func TestEnsureStaffPresenceAcceptsClosedSessionSkip(t *testing.T) {
	called := false
	workSessions := &workSessionServiceForSessionUnitTest{
		ensureCheckedInFunc: func(_ context.Context, _ int64, _ string) (*activeModels.WorkSession, error) {
			called = true
			return nil, nil
		},
	}
	svc := &service{ServiceDependencies: ServiceDependencies{
		WorkSessionService: workSessions,
		Logger:             slog.New(slog.DiscardHandler),
	}}

	svc.ensureStaffPresence(context.Background(), 42, activeModels.WorkSessionSourceApp)

	assert.True(t, called)
}
