package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/ports"
)

// CleanupRetention deletes a school's finished staff cleanup jobs after the
// fixed window (#4046).
type CleanupRetention struct {
	store ports.CleanupRetentionStore
}

func NewCleanupRetention(store ports.CleanupRetentionStore) *CleanupRetention {
	if store == nil {
		panic("workforce application: cleanup retention requires its store")
	}
	return &CleanupRetention{store: store}
}

// DeleteFinishedStaffCleanups removes the staff document file cleanups and offboarding
// jobs that finished before the window.
func (r *CleanupRetention) DeleteFinishedStaffCleanups(ctx context.Context) (int, int, error) {
	files, err := r.store.DeleteFinishedStaffDocumentFileCleanups(ctx, domain.CleanupRetentionWindow)
	if err != nil {
		return 0, 0, err
	}
	offboardings, err := r.store.DeleteCompletedOffboardingCleanups(ctx, domain.CleanupRetentionWindow)
	if err != nil {
		return 0, 0, err
	}
	return files + offboardings, int(domain.CleanupRetentionWindow.Hours() / 24), nil
}
