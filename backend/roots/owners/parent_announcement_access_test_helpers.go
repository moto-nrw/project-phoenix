//go:build testhooks

package owners

import (
	"context"

	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// ParentAnnouncementAccessAfterAudience calls after once the audience match of
// an announcement has been read, still inside the audience's unit. Tests use
// it to fault the unit that follows the audience (ADR 0161 step 3, #5894).
func ParentAnnouncementAccessAfterAudience(access parentportal.AnnouncementAccess, after func()) parentportal.AnnouncementAccess {
	return announcementAccessAfterAudience{AnnouncementAccess: access, after: after}
}

type announcementAccessAfterAudience struct {
	parentportal.AnnouncementAccess
	after func()
}

func (a announcementAccessAfterAudience) AccountMatchesAnnouncement(ctx context.Context, tenantID, announcementID, accountID int64) (bool, error) {
	matched, err := a.AnnouncementAccess.AccountMatchesAnnouncement(ctx, tenantID, announcementID, accountID)
	if a.after != nil {
		a.after()
	}
	return matched, err
}
