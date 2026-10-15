package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/ports"
)

// CleanupRetention deletes a school's finished staff cleanup jobs after the
// fixed window (#4046).
type CleanupRetention struct {
	store ports.CleanupRetentionStore
	now   func() time.Time
}

func NewCleanupRetention(store ports.CleanupRetentionStore) *CleanupRetention {
	if store == nil {
		panic("workforce application: cleanup retention requires its store")
	}
	return &CleanupRetention{store: store, now: time.Now}
}

// DeleteFinished removes the staff document file cleanups and offboarding
// jobs that finished before the window.
func (r *CleanupRetention) DeleteFinished(ctx context.Context) (int, error) {
	before := r.now().Add(-domain.CleanupRetentionWindow)
	files, err := r.store.DeleteFinishedStaffDocumentFileCleanups(ctx, before)
	if err != nil {
		return 0, err
	}
	offboardings, err := r.store.DeleteCompletedOffboardingCleanups(ctx, before)
	if err != nil {
		return 0, err
	}
	return files + offboardings, nil
}
