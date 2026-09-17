package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// SchoolInvitationMaintenance are the invitation operations that need no
// flow dependencies: spending every pending invitation of a school that was
// soft-deleted, and deleting the expired ones. Every composition serves
// them, so the cleanup CLI reaches them without the invitation seams.
type SchoolInvitationMaintenance struct {
	store  ports.SchoolInvitationStore
	now    func() time.Time
	logger *slog.Logger
}

func NewSchoolInvitationMaintenance(store ports.SchoolInvitationStore, logger *slog.Logger) *SchoolInvitationMaintenance {
	if store == nil {
		panic("identity access application: school invitation maintenance requires its store")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &SchoolInvitationMaintenance{store: store, now: time.Now, logger: logger}
}

// RevokeTenantInvitations spends every pending invitation of the school, so
// a soft-deleted school hands out no access.
func (m *SchoolInvitationMaintenance) RevokeTenantInvitations(ctx context.Context, tenantID int64) (int, error) {
	revoked, _, err := m.store.RevokeSchoolInvitationsForTenant(ctx, tenantID)
	if err != nil {
		return 0, failed("invalidate invitations by tenant", err)
	}
	if revoked > 0 {
		m.logger.Info("pending invitations invalidated for deleted tenant",
			slog.Int64("tenant_id", tenantID),
			slog.Int("count", revoked))
	}
	return revoked, nil
}

// CleanupExpiredInvitations removes invitations that are no longer useful.
func (m *SchoolInvitationMaintenance) CleanupExpiredInvitations(ctx context.Context) (int, error) {
	deleted, _, err := m.store.DeleteExpiredSchoolInvitations(ctx, m.now())
	if err != nil {
		return 0, failed("cleanup invitations", err)
	}
	if deleted > 0 {
		m.logger.Info("invitation cleanup completed",
			slog.Int("records_deleted", deleted))
	}
	return deleted, nil
}
