package messaging

import (
	"context"
	"errors"
	"fmt"
)

// GuardianAnnouncementTenant answers the attachment entry's audience question
// about a parent-facing attachment (#2890): may this guardian account see the
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
// A caller who fails any step gets tenant id 0 and no error. The attachment
// entry turns that into a 404, so "outside the audience" and "no such announcement"
// are indistinguishable and announcement ids cannot be enumerated across
// schools.
//
// The lookup runs in an admin transaction because a guardian's children span
// schools; File Storage then reads the attachment rows in a tenant transaction
// for exactly the school returned here, under that school's RLS.
func (s *Service) GuardianAnnouncementTenant(ctx context.Context, accountID, announcementID int64) (int64, error) {
	if accountID <= 0 || announcementID <= 0 {
		return 0, nil
	}
	announcementTenantID, systemKind, err := s.guardianAnnouncementAudience(ctx, accountID, announcementID)
	if err != nil || announcementTenantID == 0 {
		return 0, err
	}
	// Fails closed, like every other visibility check on this path: a school
	// that switched the feature off hides the attachment with the message.
	if !s.announcementVisibleForTenant(ctx, announcementTenantID, announcementID, systemKind) {
		return 0, nil
	}
	return announcementTenantID, nil
}

// errAnnouncementHidden ends the school unit of WithinGuardianAnnouncementTenant
// when the school's switch hides the announcement; it never leaves this file.
var errAnnouncementHidden = errors.New("parent: announcement hidden by the school's switch")

// WithinGuardianAnnouncementTenant is the first pass of the same gate for the
// attachment entry (ADR 0161 step 3, #5894): it decides the audience in the
// administrative transaction, then reads the school's switch and runs read in
// ONE tenant transaction of the announcement's school, switch first. The
// order of the checks and the statements are those of
// GuardianAnnouncementTenant followed by read in a unit of its own; only the
// COMMIT and BEGIN between switch and read are gone.
//
// A refusal is tenant id 0 with no error, and read is not called. A switch
// that is off or unreadable ends the school unit before read's first
// statement, so a failed switch read never runs into an aborted transaction.
// A school unit that cannot start runs the two steps as they ran before the
// bundle (withinAnnouncementTenantUnbundled), so its outcome depends on the
// switch exactly as before and no start error is turned into a refusal the
// old order would not have made.
func (s *Service) WithinGuardianAnnouncementTenant(ctx context.Context, accountID, announcementID int64, read func(ctx context.Context, tenantID int64) error) (int64, error) {
	if accountID <= 0 || announcementID <= 0 {
		return 0, nil
	}
	if read == nil {
		return 0, errors.New("parent: attachment read is required")
	}
	tenantID, systemKind, err := s.guardianAnnouncementAudience(ctx, accountID, announcementID)
	if err != nil || tenantID == 0 {
		return 0, err
	}
	needsSwitch := systemKind == nil || *systemKind == ""
	if needsSwitch && s.Settings == nil {
		return 0, nil
	}
	started := false
	err = s.Runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
		started = true
		if !s.announcementVisibleForTenant(txCtx, tenantID, announcementID, systemKind) {
			return errAnnouncementHidden
		}
		return read(txCtx, tenantID)
	})
	switch {
	case err == nil:
		return tenantID, nil
	case errors.Is(err, errAnnouncementHidden):
		return 0, nil
	case !started:
		return s.withinAnnouncementTenantUnbundled(ctx, tenantID, announcementID, systemKind, read)
	default:
		return 0, err
	}
}

// withinAnnouncementTenantUnbundled is the order before ADR 0161 step 3, for
// a shared school unit that did not start: the switch on its own, from the
// request's settings cache or in a unit of its own and fail closed, then read
// in a unit of its own. A cached open switch therefore returns the read's
// start error as before, and an unreadable switch hides the announcement as
// before.
func (s *Service) withinAnnouncementTenantUnbundled(ctx context.Context, tenantID, announcementID int64, systemKind *string, read func(context.Context, int64) error) (int64, error) {
	if !s.announcementVisibleForTenant(ctx, tenantID, announcementID, systemKind) {
		return 0, nil
	}
	if err := s.Runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error { return read(txCtx, tenantID) }); err != nil {
		return 0, err
	}
	return tenantID, nil
}

// guardianAnnouncementAudience is the administrative half of the gate: the
// school and system kind of a live announcement whose audience includes the
// account, or school 0 when it does not.
func (s *Service) guardianAnnouncementAudience(ctx context.Context, accountID, announcementID int64) (int64, *string, error) {
	var announcementTenantID int64
	var systemKind *string
	err := s.Runtime.WithinAdmin(ctx, func(adminCtx context.Context) error {
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
		return 0, nil, err
	}
	return announcementTenantID, systemKind, nil
}

// GuardianDeclarationProofAttachmentTenant answers whether one frozen
// attachment belongs to this account's proof for this child. It intentionally
// does not require the announcement to remain live: the proof remains useful
// after expiry or withdrawal. The proof's relationship check still requires
// parent_portal.access for exactly the supplied child.
func (s *Service) GuardianDeclarationProofAttachmentTenant(ctx context.Context, accountID, announcementID, studentID, attachmentID int64) (int64, error) {
	if accountID <= 0 || announcementID <= 0 || studentID <= 0 || attachmentID <= 0 {
		return 0, nil
	}
	proof, err := s.DeclarationProof(ctx, accountID, announcementID, studentID)
	if err != nil {
		if errors.Is(err, ErrAnnouncementNotFound) {
			return 0, nil
		}
		return 0, err
	}
	for _, submission := range proof.Submissions {
		version, ok := proof.Versions[submission.VersionID]
		if !ok {
			continue
		}
		for _, attachment := range version.Attachments {
			if attachment.AttachmentID == attachmentID {
				return version.TenantID, nil
			}
		}
	}
	return 0, nil
}
