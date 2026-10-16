package api

import (
	"context"
	"errors"
	"time"

	sessionsAPI "github.com/moto-nrw/project-phoenix/api/iot/sessions"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend"
)

// BindIoTSessionEnd supplies the kiosk's session-close port from the workflow.
func BindIoTSessionEnd(command sessionend.Command) sessionsAPI.EndSession {
	return func(ctx context.Context, activeGroupID int64) (time.Time, sessionsAPI.EndFailure, error) {
		result, err := command.EndSession(ctx, activeGroupID)
		failure := sessionsAPI.EndFailed
		switch {
		case errors.Is(err, sessionend.ErrSessionNotFound):
			failure = sessionsAPI.EndNotFound
		case errors.Is(err, sessionend.ErrSessionAlreadyEnded):
			failure = sessionsAPI.EndAlreadyEnded
		}
		return result.EndedAt, failure, err
	}
}
