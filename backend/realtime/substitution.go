package realtime

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// QueueSubstitutionChanged emits the tenant-wide substitution_changed
// invalidation after the surrounding tenant transaction commits.
//
// Both writers of education.group_substitution drive it — the admin
// Vertretungsplan (api/substitutions) and the self-service Gruppenübergabe
// (api/groups) — so the payload shape can never drift between them. source
// names the emitting flow ("group_transfer", "substitution_create", …) and is
// the only field carried: the event reaches every staff client of the tenant,
// so it must not name the substitute (see EventSubstitutionChanged).
//
// A zero tenant ID means the caller is outside a tenant-scoped request; there
// is nobody to broadcast to, so the call is a no-op instead of a broadcast to
// tenant 0.
func QueueSubstitutionChanged(ctx context.Context, broadcaster Broadcaster, logger *slog.Logger, source string) {
	if broadcaster == nil {
		return
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	tenant.RegisterAfterCommit(ctx, func() {
		event := NewEvent(EventSubstitutionChanged, "", EventData{Source: &source})
		if err := broadcaster.BroadcastToTenant(tenantID, event); err != nil {
			logger.Warn("SSE substitution broadcast failed",
				slog.String("source", source),
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
		}
	})
}
