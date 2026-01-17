package auth

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/audit"
	"github.com/moto-nrw/project-phoenix/internal/core/logger"
)

// logAuthEvent logs an authentication event for audit purposes
func (s *Service) logAuthEvent(ctx context.Context, accountID int64, eventType string, success bool, ipAddress, userAgent string, errorMessage string) {
	event := audit.NewAuthEvent(accountID, eventType, success, ipAddress)
	event.UserAgent = userAgent
	if errorMessage != "" {
		event.ErrorMessage = errorMessage
	}

	// Log asynchronously to avoid blocking auth operations
	go func() {
		// Create a new context with timeout for the logging operation
		// Use WithoutCancel to detach from parent cancellation while preserving context values
		logCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()

		if err := s.repos.AuthEvent.Create(logCtx, event); err != nil {
			// Log the error but don't fail the auth operation
			if logger.Logger != nil {
				logger.Logger.WithError(err).Warn("Failed to log auth event")
			}
		}
	}()
}
