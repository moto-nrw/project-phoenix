package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// PresenceAutoCheckIn binds Student Presence's automatic check-in to the
// Workforce time clock. Student Presence sits below Workforce (ADR 0058,
// #3417), so it names the stamp channel in its own vocabulary and the root
// maps it onto Workforce's stamp source.
type PresenceAutoCheckIn struct {
	CheckIn workforceCompose.AutoCheckIn
}

func (a PresenceAutoCheckIn) EnsureCheckedIn(ctx context.Context, staffID int64, source studentpresence.StampSource) (bool, error) {
	channel := workforce.WorkSessionSourceApp
	if source == studentpresence.StampNFC {
		channel = workforce.WorkSessionSourceNFC
	}
	session, err := a.CheckIn.EnsureCheckedIn(ctx, staffID, channel)
	return session != nil, err
}
