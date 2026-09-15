package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

type DocumentCleanupStore interface {
	EnqueueDocumentCleanup(context.Context, int64, time.Time) error
	ClaimDocumentCleanup(context.Context, int, time.Time) ([]domain.DocumentCleanupClaim, error)
	FinishDocumentCleanup(context.Context, domain.DocumentCleanupClaim, bool, time.Time) (bool, error)
	DocumentCleanupBacklog(context.Context, time.Time) (domain.DocumentCleanupBacklog, error)
}
