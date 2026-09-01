package parent

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// GuardianAnnouncementTenant answers the file storage's only question about a
// parent-facing attachment (#2890): may this guardian account see the
// attachments of this announcement, and if so, which school do they belong to.
//
// It is the same gate the read/acknowledge path uses, in the same order, and
// deliberately not a second one — a download that could be reached while the
// announcement itself cannot would be exactly the leak this whole check
// exists to prevent:
//
//  1. the announcement exists and is live (published, active, not expired),
//  2. the account is in its audience right now,
//  3. the school still has the feature switched on.
//
// A caller who fails any step gets tenant id 0 and no error. The file side
// turns that into a 404, so "outside the audience" and "no such announcement"
// are indistinguishable and announcement ids cannot be enumerated across
// schools.
//
// The lookup runs in an admin transaction because a guardian's children span
// schools; the caller then opens a tenant transaction for exactly the school
// returned here, so the attachment rows are read under that school's RLS.
func (s *service) GuardianAnnouncementTenant(ctx context.Context, accountID, announcementID int64) (int64, error) {
	if accountID <= 0 || announcementID <= 0 {
		return 0, nil
	}

	var announcementTenantID int64
	var systemKind *string
	err := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		a, err := s.AnnouncementRepo.FindByID(adminCtx, announcementID)
		if err != nil {
			return fmt.Errorf("parent: load announcement for attachment: %w", err)
		}
		if a == nil || !announcementIsLive(a) {
			return nil
		}
		matched, err := s.AnnouncementRepo.AccountMatchesAnnouncement(adminCtx, a.GetTenantID(), announcementID, accountID)
		if err != nil {
			return fmt.Errorf("parent: match announcement audience for attachment: %w", err)
		}
		if !matched {
			return nil
		}
		announcementTenantID = a.GetTenantID()
		systemKind = a.SystemKind
		return nil
	})
	if err != nil {
		return 0, err
	}
	if announcementTenantID == 0 {
		return 0, nil
	}
	// Fails closed, like every other visibility check on this path: a school
	// that switched the feature off hides the attachment with the message.
	if !s.announcementVisibleForTenant(ctx, announcementTenantID, announcementID, systemKind) {
		return 0, nil
	}
	return announcementTenantID, nil
}
