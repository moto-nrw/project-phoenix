package presence

import (
	"context"
	"log/slog"
)

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

// GetSchulhofRoomColor delegates canonical room selection to Facilities.
func (s *service) GetSchulhofRoomColor(ctx context.Context) (*string, error) {
	if s.YardRoomColor == nil {
		return nil, nil
	}
	return s.YardRoomColor(ctx)
}
