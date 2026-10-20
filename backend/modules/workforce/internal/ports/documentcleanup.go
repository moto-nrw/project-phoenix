package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

type DocumentCleanupStore interface {
	EnqueueDocumentCleanup(context.Context, int64, time.Time) error
	ClaimDocumentCleanup(context.Context, int, time.Time) ([]workforce.DocumentCleanupClaim, error)
	FinishDocumentCleanup(context.Context, workforce.DocumentCleanupClaim, bool, time.Time) (bool, error)
	DocumentCleanupBacklog(context.Context, time.Time) (workforce.DocumentCleanupBacklog, error)
}
