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
	audience, err := s.guardianAnnouncementAudience(ctx, accountID, announcementID)
	if err != nil || audience.tenantID == 0 {
		return 0, err
	}
	// Fails closed, like every other visibility check on this path: a school
	// that switched the feature off hides the attachment with the message.
	if !s.announcementVisibleForTenant(ctx, audience.tenantID, announcementID, audience.systemKind) {
		return 0, nil
	}
	return audience.tenantID, nil
}

// errAnnouncementHidden ends the school unit of WithinGuardianAnnouncementTenant
// when the school's switch hides the announcement. It never leaves this file
// and is recognised only as the unit's direct result: a rollback error joined
// to it is a failure of the unit, not a refusal.
var errAnnouncementHidden = errors.New("parent: announcement hidden by the school's switch")

// WithinGuardianAnnouncementTenant is the first pass of the same gate for the
// attachment entry (ADR 0161 step 3, #5894): it decides the audience in the
// administrative transaction, then reads the school's switch and runs read in
// ONE tenant transaction of the announcement's school, switch first. The
// checks and statements are those of GuardianAnnouncementTenant followed by
// read in a unit of its own; only the COMMIT and BEGIN between switch and
// read are gone.
//
// A refusal is tenant id 0 with no error, and read is not called. A switch
// that is off or unreadable ends the school unit before read's first
// statement. A school unit that cannot start, switch roles or commit fails
// with its error once (E9/E15): no retry, and no second way after the
// failure. A caller inside an open unit or bound to another school takes the
// unbundled way, chosen from the context before the school unit starts
// (ADR 0161 point 7).
func (s *Service) WithinGuardianAnnouncementTenant(ctx context.Context, accountID, announcementID int64, read func(ctx context.Context, tenantID int64) error) (int64, error) {
	if accountID <= 0 || announcementID <= 0 {
		return 0, nil
	}
	if read == nil {
		return 0, errors.New("parent: attachment read is required")
	}
	unitOpen, bound := s.Runtime.AmbientScope(ctx)
	audience, err := s.guardianAnnouncementAudience(ctx, accountID, announcementID)
	if err != nil || audience.tenantID == 0 {
		return 0, err
	}
	// An open unit, or a context bound to another school, cannot start this
	// school's unit without joining or refusing a foreign one; switch and
	// read then run as before, each on its own (ADR 0161 point 7).
	if unitOpen || (bound != 0 && bound != audience.tenantID) {
		return s.switchThenRead(ctx, audience, announcementID, read)
	}
	err = s.Runtime.WithinTenant(ctx, audience.tenantID, func(txCtx context.Context) error {
		if !s.announcementVisibleForTenant(txCtx, audience.tenantID, announcementID, audience.systemKind) {
			return errAnnouncementHidden
		}
		return read(txCtx, audience.tenantID)
	})
	// Only the unit's direct result is a refusal; see errAnnouncementHidden.
	if err == errAnnouncementHidden {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return audience.tenantID, nil
}

// switchThenRead is the second half of the way before ADR 0161: the switch,
// then read in a unit of its own.
func (s *Service) switchThenRead(ctx context.Context, audience announcementAudience, announcementID int64, read func(ctx context.Context, tenantID int64) error) (int64, error) {
	if !s.announcementVisibleForTenant(ctx, audience.tenantID, announcementID, audience.systemKind) {
		return 0, nil
	}
	if err := read(ctx, audience.tenantID); err != nil {
		return 0, err
	}
	return audience.tenantID, nil
}

// announcementAudience is the administrative half of the gate: the school
// and system kind of a live announcement whose audience includes the account.
// School 0 means "not for you".
type announcementAudience struct {
	tenantID   int64
	systemKind *string
}

// guardianAnnouncementAudience reads the administrative half of the gate in
// its own administrative unit.
func (s *Service) guardianAnnouncementAudience(ctx context.Context, accountID, announcementID int64) (announcementAudience, error) {
	var audience announcementAudience
	err := s.Runtime.WithinAdmin(ctx, func(adminCtx context.Context) error {
		var err error
		audience, err = s.announcementAudienceIn(adminCtx, accountID, announcementID)
		return err
	})
	if err != nil {
		return announcementAudience{}, err
	}
	return audience, nil
}

// announcementAudienceIn reads the announcement and matches the account's
// audience inside the caller's administrative transaction.
func (s *Service) announcementAudienceIn(adminCtx context.Context, accountID, announcementID int64) (announcementAudience, error) {
	a, err := s.AnnouncementRepo.FindByID(adminCtx, announcementID)
	if err != nil {
		return announcementAudience{}, fmt.Errorf("parent: load announcement for attachment: %w", err)
	}
	if a == nil || !announcementIsLive(a) {
		return announcementAudience{}, nil
	}
	matched, err := s.AnnouncementRepo.AccountMatchesAnnouncement(adminCtx, a.GetTenantID(), announcementID, accountID)
	if err != nil {
		return announcementAudience{}, fmt.Errorf("parent: match announcement audience for attachment: %w", err)
	}
	if !matched {
		return announcementAudience{}, nil
	}
	return announcementAudience{tenantID: a.GetTenantID(), systemKind: a.SystemKind}, nil
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
