package active

import (
	"context"
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
	// GetSchulhofRoomColor returns the canonical Schulhof room's color, or
	// nil when the room does not exist yet or carries no color.
	GetSchulhofRoomColor(ctx context.Context) (*string, error)
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
	color, err := resolver.GetSchulhofRoomColor(ctx)
	if err != nil {
		slog.WarnContext(ctx, "yard_room_color_lookup_failed", slog.String("error", err.Error()))
		return nil
	}
	return color
}

// GetSchulhofRoomColor implements YardRoomColorResolver.
//
// Returns nil (not an error) when the room is missing: the Schulhof room is
// bootstrapped lazily on first use, so "not there yet" is a normal state for
// a tenant that has never opened the yard.
//
// The canonical-name check mirrors facilities.FindCanonicalSchulhofRoom —
// FindByName matches case-insensitively, and a stray "schulhof" room created
// before the reservation guards landed must not tint the yard badge.
func (s *service) GetSchulhofRoomColor(ctx context.Context) (*string, error) {
	if s.RoomRepo == nil {
		return nil, nil
	}
	room, err := s.RoomRepo.FindByName(ctx, constants.SchulhofRoomName)
	if err != nil || room == nil {
		// A missing room surfaces as a scan error from the repository; there
		// is no sentinel to distinguish it from a real failure, and either
		// way the answer for the badge is the same.
		return nil, nil
	}
	if room.Name != constants.SchulhofRoomName || !room.IsSystem {
		return nil, nil
	}
	if room.Color == nil || *room.Color == "" {
		return nil, nil
	}
	return room.Color, nil
}
