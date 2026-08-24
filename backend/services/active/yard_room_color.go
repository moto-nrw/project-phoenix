package active

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/constants"
)

// YardRoomColorResolver reports the tenant's configured Schulhof room color.
//
// Deliberately a narrow, separate interface rather than another method on the
// broad Service interface: only the location-resolution paths need it, and
// every test double that implements Service would otherwise have to grow a
// method it never calls. Same optional-capability pattern the tenant-resolve
// handler uses for ResolveManyForTenant.
type YardRoomColorResolver interface {
	// ResolveSchulhofRoomColor returns the canonical Schulhof room's color,
	// or nil when the room does not exist yet, carries no color, or the
	// lookup failed. Fails soft; the implementation owns the logging.
	ResolveSchulhofRoomColor(ctx context.Context) *string
}

// ResolveYardRoomColor returns the tenant's Schulhof room color for the
// binary-mode "Schulhof" state, or nil when it cannot be determined.
//
// In detailed mode the yard is an ordinary room visit, so the color already
// travels with the active group's room. Binary mode writes attendance only —
// there is no visit and no room to read the color from — so the one canonical
// room has to be looked up separately for the badge to follow the school's
// color scheme (#2405).
//
// Fails soft on purpose: nil means "no color configured", which the frontend
// renders as the orange Schulhof default. A colour lookup must never be able
// to fail a student list.
func ResolveYardRoomColor(ctx context.Context, svc Service) *string {
	resolver, ok := svc.(YardRoomColorResolver)
	if !ok {
		return nil
	}
	return resolver.ResolveSchulhofRoomColor(ctx)
}

// ResolveSchulhofRoomColor implements YardRoomColorResolver: the fail-soft
// wrapper around GetSchulhofRoomColor.
//
// The lookup keeps returning its error so a broken connection stays
// distinguishable from "no colour set"; swallowing it happens here, once, and
// goes through the service's injected logger so the entry carries the same
// handler, level and service attributes as every other line from active.
func (s *service) ResolveSchulhofRoomColor(ctx context.Context) *string {
	color, err := s.GetSchulhofRoomColor(ctx)
	if err != nil {
		s.getLogger().WarnContext(ctx, "yard room color lookup failed, badge falls back to default",
			slog.String("error", err.Error()),
		)
		return nil
	}
	return color
}

// GetSchulhofRoomColor looks up the canonical Schulhof room's color.
//
// Returns nil (not an error) when the room is missing: the Schulhof room is
// bootstrapped lazily on first use, so "not there yet" is a normal state for
// a tenant that has never opened the yard. Every other repository failure is
// returned, so a broken connection or a rejected query reaches the log
// instead of reading as "no colour configured" — the two are indistinguishable
// on the badge, and only one of them is worth waking up for.
//
// The name lookup is case-insensitive while the uniqueness index is not, so a
// tenant carrying a legacy "schulhof" room from before the reservation guards
// has TWO matching rows. Reading a single unordered row would hand back the
// legacy one often enough to silently drop the configured colour, so every
// match is inspected and only the exact reserved name on a system room counts
// — the same validation facilities.FindCanonicalSchulhofRoom applies.
func (s *service) GetSchulhofRoomColor(ctx context.Context) (*string, error) {
	if s.RoomRepo == nil {
		return nil, nil
	}
	rooms, err := s.RoomRepo.List(ctx, map[string]any{"name": constants.SchulhofRoomName})
	if err != nil {
		return nil, fmt.Errorf("list Schulhof rooms: %w", err)
	}
	for _, room := range rooms {
		if room == nil || room.Name != constants.SchulhofRoomName || !room.IsSystem {
			continue
		}
		if room.Color == nil || *room.Color == "" {
			return nil, nil
		}
		return room.Color, nil
	}
	return nil, nil
}
