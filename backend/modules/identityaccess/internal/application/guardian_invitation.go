package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"

	"github.com/moto-nrw/project-phoenix/sharedkernel/textnorm"
)

// The guardian invitation lifecycle (#2722): issuing the link for a guardian
// contact, the public preview the accept page renders, the acceptance that
// provisions the parents-portal account, and the resend. The relative access
// flows next door write the same rows for one child; both decide expiry with
// one clock, and a link is redeemable exactly once.

const (
	opGuardianInviteCreate   = "create guardian invitation"
	opGuardianInviteValidate = "validate guardian invitation"
	opGuardianInviteAccept   = "accept guardian invitation"
	opGuardianInviteResend   = "resend guardian invitation"
	opGuardianInviteFetch    = "fetch guardian invitation"
)

// CreateGuardianInvitation inserts the invitation and queues its mail in one
// transaction, so the mail commits with the invitation it announces.
func (l *AccountLifecycle) CreateGuardianInvitation(ctx context.Context, request identityaccess.GuardianInvitationRequest) (_ identityaccess.GuardianInvitation, err error) {
	if l == nil {
		return identityaccess.GuardianInvitation{}, errAccountLifecycleUnavailable
	}
	defer func() { err = invitationError(err) }()
	if request.GuardianProfileID <= 0 {
		return identityaccess.GuardianInvitation{}, failed(opGuardianInviteCreate, fmt.Errorf("guardian profile ID is required"))
	}
	if request.CreatedBy <= 0 {
		return identityaccess.GuardianInvitation{}, failed(opGuardianInviteCreate, fmt.Errorf("created_by is required"))
	}
	var invitation domain.GuardianInvitation
	err = l.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		txCtx = bindRuntime(l.auth.attach, txCtx)
		profile, profileErr := l.lockInvitationAccountThenProfile(txCtx, request.GuardianProfileID)
		if profileErr != nil {
			return failed(opGuardianInviteCreate, profileErr)
		}
		expiry, err := l.delivery.InvitationExpiry(txCtx)
		if err != nil {
			return failed(opGuardianInviteCreate, err)
		}
		var insertErr error
		invitation, insertErr = l.issueGuardianInvitation(txCtx, request, profile, time.Now().Add(expiry))
		return insertErr
	})
	if err != nil {
		return identityaccess.GuardianInvitation{}, err
	}
	return identityaccess.GuardianInvitation(invitation), nil
}

// issueGuardianInvitation writes the invitation and queues its mail. A request
// that refuses a blocked address learns about the block before anything is kept.
func (l *AccountLifecycle) issueGuardianInvitation(ctx context.Context, request identityaccess.GuardianInvitationRequest, profile domain.GuardianProfile, expiresAt time.Time) (domain.GuardianInvitation, error) {
	if request.RefuseBlockedAddress {
		if err := l.refuseBlockedMail(ctx, l.runtime.TenantID(ctx), profile); err != nil {
			return domain.GuardianInvitation{}, failed(opGuardianInviteCreate, err)
		}
	}
	draft := domain.GuardianInvitation{
		TenantID:          l.runtime.TenantID(ctx),
		GuardianProfileID: profile.ID,
		CreatedBy:         request.CreatedBy,
		ExpiresAt:         expiresAt,
		ApprovalStatus:    domain.GuardianInvitationApprovalNotRequired,
	}
	if err := l.invitations.InvalidateGuardianInvitations(ctx, profile.ID, time.Now(), false); err != nil {
		return domain.GuardianInvitation{}, failed(opGuardianInviteCreate, err)
	}
	issueGuardianInvitationLink(&draft, profile)
	invitation, err := l.invitations.InsertGuardianInvitation(ctx, draft)
	if err != nil {
		return domain.GuardianInvitation{}, failed(opGuardianInviteCreate, err)
	}
	l.logger.Info("guardian invitation created",
		slog.Int64("invitation_id", invitation.ID),
		slog.Int64("guardian_profile_id", profile.ID),
		slog.Int64("created_by", request.CreatedBy),
	)
	if err := l.mailNewInvitation(ctx, invitation, profile); err != nil {
		return domain.GuardianInvitation{}, failed(opGuardianInviteCreate, err)
	}
	return invitation, nil
}

// issueGuardianInvitationLink gives the invitation a new link for the address
// the guardian has now (#6294). Only the hash of the token is stored (#4902),
// so the previous link stops working; the address travels with the token, and
// accepting requires the guardian to still have it.
func issueGuardianInvitationLink(invitation *domain.GuardianInvitation, profile domain.GuardianProfile) {
	invitation.Token = uuid.Must(uuid.NewV4()).String()
	invitation.Email = domain.GuardianInvitationAddress(profile.Email)
}

// lockInvitationAccountThenProfile locks the account of the guardian's
// address, then the school, then the guardian's profile: the order every
// acceptance and sign-in takes (#6294, #6305). Holding the profile while
// waiting for the account or the school would close a cycle with an
// acceptance or with a transaction that holds the school and then the
// profile (40P01). The address is read again under the profile lock; a
// change in between refuses the invitation instead of binding it to an
// account that was not locked.
func (l *AccountLifecycle) lockInvitationAccountThenProfile(ctx context.Context, profileID int64) (domain.GuardianProfile, error) {
	unlocked, found, err := l.guardians.FindGuardianProfile(ctx, profileID)
	if err != nil {
		return domain.GuardianProfile{}, err
	}
	if !found {
		return domain.GuardianProfile{}, fmt.Errorf("guardian profile not found")
	}
	if strings.TrimSpace(unlocked.Email) != "" {
		if _, _, _, err := l.logins.LockLoginAccountByEmail(ctx, unlocked.Email); err != nil {
			return domain.GuardianProfile{}, err
		}
	}
	if err := l.lockInvitingSchool(ctx); err != nil {
		return domain.GuardianProfile{}, err
	}
	profile, err := l.lockInvitationProfile(ctx, profileID)
	if err != nil {
		return domain.GuardianProfile{}, err
	}
	if domain.GuardianInvitationAddress(profile.Email) != domain.GuardianInvitationAddress(unlocked.Email) {
		return domain.GuardianProfile{}, domain.ErrGuardianInvitationAddressChanged
	}
	return profile, nil
}

func (l *AccountLifecycle) lockInvitationProfile(ctx context.Context, profileID int64) (domain.GuardianProfile, error) {
	profile, locked, err := l.guardians.LockGuardianProfile(ctx, profileID)
	if err != nil {
		return domain.GuardianProfile{}, err
	}
	if !locked {
		return domain.GuardianProfile{}, fmt.Errorf("guardian profile not found")
	}
	if strings.TrimSpace(profile.Email) == "" {
		return domain.GuardianProfile{}, fmt.Errorf("guardian has no email on file")
	}
	if profile.HasAccount {
		return domain.GuardianProfile{}, fmt.Errorf("guardian already has an account")
	}
	return profile, nil
}

// refuseBlockedMail fails when a new mail to the guardian would change nothing:
// the school's list holds it back, or the server still tries the last one. Nobody
// should read that as sent, so the person learns it while they are looking.
func (l *AccountLifecycle) refuseBlockedMail(ctx context.Context, tenantID int64, profile domain.GuardianProfile) error {
	blocks, err := l.delivery.GuardianMailBlocks(ctx, tenantID, []string{profile.Email})
	if err != nil {
		return fmt.Errorf("read the mail block of the guardian: %w", err)
	}
	switch blocks[strings.ToLower(strings.TrimSpace(profile.Email))] {
	case domain.GuardianMailBlockAddress:
		return domain.ErrGuardianEmailAddressBlocked
	case domain.GuardianMailBlockDelayed:
		return domain.ErrInvitationMailDelayed
	case domain.GuardianMailBlockProvider:
		return domain.ErrMailBlockedAtProvider
	case domain.GuardianMailBlockUnreadable:
		return domain.ErrMailBlockUnreadable
	default:
		return nil
	}
}

// mailWouldStayBack reports a refusal that means no mail reaches the person. A
// delay is none: that mail is still on its way.
func mailWouldStayBack(err error) bool {
	return errors.Is(err, domain.ErrGuardianEmailAddressBlocked) || errors.Is(err, domain.ErrMailBlockedAtProvider) || errors.Is(err, domain.ErrMailBlockUnreadable)
}

// mailNewInvitation queues the mail of a new token invitation and the
// welcome that follows it (#3534). A resend mails the link alone. An
// invitation whose mail the outbox did not take must not be kept, so the
// failure is returned for the caller's transaction to roll back (#4602).
func (l *AccountLifecycle) mailNewInvitation(ctx context.Context, invitation domain.GuardianInvitation, profile domain.GuardianProfile) error {
	schoolName := l.delivery.SchoolName(ctx, invitation.TenantID)
	precedingOutboxID, err := l.enqueueInvitationEmail(ctx, invitation, profile, schoolName)
	if err != nil {
		return err
	}
	l.delivery.EnqueueWelcomeEmail(ctx, profile, invitation.TenantID, schoolName, precedingOutboxID)
	return nil
}

// enqueueInvitationEmail queues the link mail alone and classifies its
// failure.
func (l *AccountLifecycle) enqueueInvitationEmail(ctx context.Context, invitation domain.GuardianInvitation, profile domain.GuardianProfile, schoolName string) (int64, error) {
	outboxID, err := l.delivery.EnqueueInvitationEmail(ctx, invitation, profile, schoolName)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", domain.ErrGuardianAccessMailNotQueued, err)
	}
	return outboxID, nil
}

// mailExistingAccountAccess tells an account holder about access granted
// without a token and welcomes them to the school (#3534); the delivery
// sends the welcome once per guardian and school. Like the invitation, the
// access rolls back when its mail cannot be queued (#4602).
func (l *AccountLifecycle) mailExistingAccountAccess(ctx context.Context, profile domain.GuardianProfile, tenantID int64) error {
	schoolName := l.delivery.SchoolName(ctx, tenantID)
	precedingOutboxID, queued, err := l.delivery.EnqueueExistingAccountEmail(ctx, profile, schoolName)
	if err != nil {
		return fmt.Errorf("%w: %w", domain.ErrGuardianAccessMailNotQueued, err)
	}
	if !queued {
		return nil
	}
	l.delivery.EnqueueWelcomeEmail(ctx, profile, tenantID, schoolName, precedingOutboxID)
	return nil
}

// ValidateGuardianInvitation returns the public details of a redeemable
// invitation. The route is public, so the lookup runs administratively; the
// guardian's own school scopes everything read afterwards.
func (l *AccountLifecycle) ValidateGuardianInvitation(ctx context.Context, token string) (result identityaccess.GuardianInvitationPreview, err error) {
	if l == nil {
		return identityaccess.GuardianInvitationPreview{}, errAccountLifecycleUnavailable
	}
	defer func() { err = invitationError(err) }()
	ctx = bindRuntime(l.auth.attach, ctx)
	err = l.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		invitation, fetchErr := l.redeemableGuardianInvitation(txCtx, token)
		if fetchErr != nil {
			return fetchErr
		}
		schoolCtx := l.runtime.WithTenantID(txCtx, invitation.TenantID)
		profile, found, profileErr := l.guardians.FindGuardianProfile(schoolCtx, invitation.GuardianProfileID)
		if profileErr != nil {
			return failed(opGuardianInviteValidate, profileErr)
		}
		if !found {
			return failed(opGuardianInviteValidate, fmt.Errorf("guardian profile not found"))
		}
		// A link issued for an earlier address shows nothing of the profile:
		// its holder may not be the guardian (#6294).
		if !identityaccess.GuardianInvitation(invitation).AddressedTo(profile.Email) {
			return failed(opGuardianInviteValidate, domain.ErrGuardianInvitationAddressChanged)
		}
		result = identityaccess.GuardianInvitationPreview{
			Email:     strings.TrimSpace(profile.Email),
			FirstName: textnorm.Name(profile.FirstName),
			LastName:  textnorm.Name(profile.LastName),
			ExpiresAt: invitation.ExpiresAt,
		}
		l.applySchoolBranding(schoolCtx, invitation.TenantID, &result)
		return nil
	})
	if err != nil {
		return identityaccess.GuardianInvitationPreview{}, err
	}
	return result, nil
}

// applySchoolBranding adds the school's name, slug and logo to the preview.
// Best-effort: an unreachable or deleted school leaves the page unbranded.
func (l *AccountLifecycle) applySchoolBranding(ctx context.Context, tenantID int64, preview *identityaccess.GuardianInvitationPreview) {
	if tenantID <= 0 {
		return
	}
	school, found, err := l.schools.FindSchool(ctx, tenantID)
	if err != nil || !found || school.Deleted {
		return
	}
	preview.SchoolName = strings.TrimSpace(school.Name)
	preview.SchoolSlug = strings.TrimSpace(school.Slug)
	preview.SchoolLogoURL = strings.TrimSpace(school.LogoURL)
}

// AcceptGuardianInvitation spends the invitation and gives the guardian
// access: the account (created or the address's existing one), its school
// mapping, the guardian role, the profile link and the spent invitation
// commit together.
func (l *AccountLifecycle) AcceptGuardianInvitation(ctx context.Context, token string, registration identityaccess.GuardianRegistration) (_ identityaccess.Account, err error) {
	if l == nil {
		return identityaccess.Account{}, errAccountLifecycleUnavailable
	}
	defer func() { err = invitationError(err) }()
	ctx = bindRuntime(l.auth.attach, ctx)
	var result domain.LoginAccount
	if registration.Password != registration.ConfirmPassword {
		return identityaccess.Account{}, failed(opGuardianInviteAccept, domain.ErrPasswordMismatch)
	}
	if err := l.passwords.ValidatePasswordStrength(registration.Password); err != nil {
		return identityaccess.Account{}, failed(opGuardianInviteAccept, err)
	}
	err = l.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		invitation, fetchErr := l.redeemableGuardianInvitation(txCtx, token)
		if fetchErr != nil {
			return fetchErr
		}
		account, acceptErr := l.acceptGuardianAtSchool(txCtx, invitation, registration)
		if acceptErr != nil {
			return acceptErr
		}
		result = account
		return nil
	})
	if err != nil {
		return identityaccess.Account{}, err
	}
	l.logger.Info("guardian invitation accepted",
		slog.Int64("account_id", result.ID))
	return identityaccess.Account{ID: result.ID, Email: result.Email}, nil
}

// provisionGuardianAccount returns the account the address already owns —
// with its credential untouched — or creates one from the registration.
func (l *AccountLifecycle) provisionGuardianAccount(ctx context.Context, email string, registration identityaccess.GuardianRegistration) (domain.LoginAccount, error) {
	if err := l.invitations.LockGuardianAccountEmail(ctx, email); err != nil {
		return domain.LoginAccount{}, failed(opGuardianInviteAccept, err)
	}
	account, found, _, err := l.logins.LockLoginAccountByEmail(ctx, email)
	if err != nil {
		return domain.LoginAccount{}, failed(opGuardianInviteAccept, err)
	}
	if found {
		return account, refuseCatererAccount(ctx, l.logins, account.ID, opGuardianInviteAccept)
	}
	hash, err := l.passwords.HashPassword(registration.Password)
	if err != nil {
		return domain.LoginAccount{}, failed(opGuardianInviteAccept, err)
	}
	created, _, err := l.invitations.InsertAccount(ctx, email, hash)
	if err != nil {
		return domain.LoginAccount{}, failed(opGuardianInviteAccept, err)
	}
	return created, nil
}

// grantGuardianAccess gives the account the school access the invitation
// promises and spends the invitation.
func (l *AccountLifecycle) grantGuardianAccess(ctx context.Context, invitation domain.GuardianInvitation, profile domain.GuardianProfile, account domain.LoginAccount) error {
	if _, err := l.sessions.GrantGuardianTenantAccess(ctx, account.ID); err != nil {
		return failed(opGuardianInviteAccept, fmt.Errorf("grant guardian school access: %w", err))
	}
	if err := l.guardians.LinkGuardianProfileToAccount(ctx, profile.ID, account.ID); err != nil {
		return failed(opGuardianInviteAccept, fmt.Errorf("link guardian profile to account: %w", err))
	}
	accepted, err := l.invitations.AcceptGuardianInvitation(ctx, invitation.ID, invitation.Token, time.Now())
	if err != nil {
		return failed(opGuardianInviteAccept, err)
	}
	if !accepted {
		// The link changed between the read and this write: another
		// acceptance spent it, or a resend, approval or new invitation
		// replaced it (#6495). A fresh lookup names the reason.
		if _, reason := l.redeemableGuardianInvitation(ctx, invitation.Token); reason != nil {
			return reason
		}
		return failed(opGuardianInviteAccept, domain.ErrInvitationUsed)
	}
	return nil
}

// claimGuardianEnrollments stamps the new account onto the enrollment
// requests the guardian filed before they had one. Best-effort: the
// acceptance is not undone because a request could not be claimed. The one
// exception is a claim that left the transaction unusable — committing the
// acceptance on top of that would be committing an unknown state.
func (l *AccountLifecycle) claimGuardianEnrollments(ctx context.Context, accountID int64, email string) error {
	if l.enrollments == nil {
		return nil
	}
	claimed, err := l.enrollments.ClaimGuardianEnrollments(ctx, accountID, email)
	if err != nil {
		if errors.Is(err, domain.ErrTransactionUnusable) {
			return failed(opGuardianInviteAccept, err)
		}
		l.logger.Warn("guardian invitation accept: enrollment backfill failed",
			slog.Int64("account_id", accountID),
			slog.Any("error", err),
		)
		return nil
	}
	if claimed > 0 {
		l.logger.Info("guardian invitation accept: claimed pre-account enrollments",
			slog.Int64("account_id", accountID),
			slog.Int("rows_claimed", claimed),
		)
	}
	return nil
}

// resendableGuardianProfile finds the guardian of an invitation and refuses a
// resend that would change nothing.
func (l *AccountLifecycle) resendableGuardianProfile(ctx context.Context, invitation domain.GuardianInvitation) (domain.GuardianProfile, error) {
	profile, found, err := l.guardians.FindGuardianProfile(ctx, invitation.GuardianProfileID)
	if err != nil {
		return domain.GuardianProfile{}, err
	}
	if !found {
		return domain.GuardianProfile{}, fmt.Errorf("guardian profile not found")
	}
	if err := l.refuseBlockedMail(ctx, invitation.TenantID, profile); err != nil {
		return domain.GuardianProfile{}, err
	}
	return profile, nil
}

func (l *AccountLifecycle) ResendGuardianInvitation(ctx context.Context, invitationID, actorAccountID int64) (err error) {
	if l == nil {
		return errAccountLifecycleUnavailable
	}
	operation := func(ctx context.Context, invitationID, actorAccountID int64) error {
		invitation, found, err := l.invitations.FindGuardianInvitation(ctx, invitationID)
		if err != nil {
			return failed(opGuardianInviteResend, err)
		}
		if !found {
			return failed(opGuardianInviteResend, domain.ErrInvitationNotFound)
		}
		if redeemErr := invitation.Redeemable(time.Now()); redeemErr != nil {
			return failed(opGuardianInviteResend, redeemErr)
		}
		profile, err := l.resendableGuardianProfile(ctx, invitation)
		if err != nil {
			return failed(opGuardianInviteResend, err)
		}
		// The delivery bookkeeping starts over; the previous attempt's outcome
		// says nothing about the send that follows. The mail carries a new
		// link for the address on file, and the previous one stops working.
		issueGuardianInvitationLink(&invitation, profile)
		invitation.EmailSentAt = nil
		invitation.EmailError = nil
		if err := l.invitations.UpdateGuardianInvitation(ctx, invitation, invitation.ApprovalStatus, invitation.ExpiresAt); err != nil {
			return failed(opGuardianInviteResend, err)
		}
		l.logger.Info("guardian invitation resent",
			slog.Int64("invitation_id", invitation.ID),
			slog.Int64("actor_account_id", actorAccountID),
		)
		if _, err := l.enqueueInvitationEmail(ctx, invitation, profile, l.delivery.SchoolName(ctx, invitation.TenantID)); err != nil {
			return failed(opGuardianInviteResend, err)
		}
		return nil
	}
	err = l.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		return invitationError(operation(bindRuntime(l.auth.attach, txCtx), invitationID, actorAccountID))
	})
	return
}

// GuardianInvitationSchoolSlug resolves the school an invitation belongs to,
// so the accepted guardian lands on their school's host. Best-effort: an
// error or a deleted school yields "".
func (l *AccountLifecycle) GuardianInvitationSchoolSlug(ctx context.Context, token string) string {
	if l == nil {
		return ""
	}
	ctx = bindRuntime(l.auth.attach, ctx)
	var slug string
	_ = l.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		invitation, found, err := l.invitations.FindGuardianInvitationByToken(txCtx, strings.TrimSpace(token))
		if err != nil || !found {
			return err
		}
		school, schoolFound, err := l.schools.FindSchool(l.runtime.WithTenantID(txCtx, invitation.TenantID), invitation.TenantID)
		if err != nil || !schoolFound || school.Deleted {
			return err
		}
		slug = school.Slug
		return nil
	})
	return slug
}

// redeemableGuardianInvitation resolves an invitation that can still be
// accepted and reports why it cannot otherwise.
func (l *AccountLifecycle) redeemableGuardianInvitation(ctx context.Context, token string) (domain.GuardianInvitation, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return domain.GuardianInvitation{}, failed(opGuardianInviteFetch, domain.ErrInvitationNotFound)
	}
	invitation, found, err := l.invitations.FindGuardianInvitationByToken(ctx, token)
	if err != nil {
		return domain.GuardianInvitation{}, failed(opGuardianInviteFetch, err)
	}
	if !found {
		return domain.GuardianInvitation{}, failed(opGuardianInviteFetch, domain.ErrInvitationNotFound)
	}
	if err := invitation.Redeemable(time.Now()); err != nil {
		return domain.GuardianInvitation{}, failed(opGuardianInviteFetch, err)
	}
	return invitation, nil
}

// lockInvitingSchool holds the school of the inviting transaction under a
// share lock, between the account and the profile (#6305). Without an
// explicit lock the transaction would reach the school only through the
// invitation's foreign key, after the profile. A school that is gone or
// switched off invites nobody.
func (l *AccountLifecycle) lockInvitingSchool(ctx context.Context) error {
	tenantID := l.runtime.TenantID(ctx)
	if tenantID <= 0 {
		return nil
	}
	school, found, err := l.schools.LockSchoolShared(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("lock the inviting school: %w", err)
	}
	if !found || !school.Live() {
		return domain.ErrInvitationTenantDeleted
	}
	return nil
}

func (l *AccountLifecycle) requireInvitationSchool(txCtx context.Context, invitation domain.GuardianInvitation) error {
	// A shared lock on the school serializes with the exclusive lock a
	// soft delete takes, so the school cannot vanish under the acceptance.
	if invitation.TenantID > 0 {
		school, found, schoolErr := l.schools.LockSchoolShared(txCtx, invitation.TenantID)
		if schoolErr != nil {
			return failed(opGuardianInviteAccept, schoolErr)
		}
		if !found || school.Deleted {
			return failed(opGuardianInviteAccept, domain.ErrInvitationTenantDeleted)
		}
	}
	return nil
}

// acceptGuardianAtSchool creates or reuses the account for the address the
// link was issued for (#6294). The profile stays locked until commit, the
// lock both contact edit paths take before they change the address: an
// address change either commits first, and the link is refused, or waits for
// the acceptance. Locks follow one order: account, then school, then profile
// (40P01 otherwise, #6294/#6333). The first address check is only an early
// refusal; the check under the profile lock decides whether access can be
// granted.
func (l *AccountLifecycle) acceptGuardianAtSchool(txCtx context.Context, invitation domain.GuardianInvitation, registration identityaccess.GuardianRegistration) (domain.LoginAccount, error) {
	invitationCtx := l.runtime.WithTenantID(txCtx, invitation.TenantID)
	profile, found, profileErr := l.guardians.FindGuardianProfile(invitationCtx, invitation.GuardianProfileID)
	if profileErr != nil {
		return domain.LoginAccount{}, failed(opGuardianInviteAccept, profileErr)
	}
	if !found {
		return domain.LoginAccount{}, failed(opGuardianInviteAccept, fmt.Errorf("guardian profile not found"))
	}
	if !identityaccess.GuardianInvitation(invitation).AddressedTo(profile.Email) {
		return domain.LoginAccount{}, failed(opGuardianInviteAccept, domain.ErrGuardianInvitationAddressChanged)
	}
	email := invitation.Email
	account, accountErr := l.provisionGuardianAccount(invitationCtx, email, registration)
	if accountErr != nil {
		return domain.LoginAccount{}, accountErr
	}
	if err := l.requireInvitationSchool(txCtx, invitation); err != nil {
		return domain.LoginAccount{}, err
	}
	profile, found, profileErr = l.guardians.LockGuardianProfile(invitationCtx, invitation.GuardianProfileID)
	if profileErr != nil {
		return domain.LoginAccount{}, failed(opGuardianInviteAccept, profileErr)
	}
	if !found {
		return domain.LoginAccount{}, failed(opGuardianInviteAccept, fmt.Errorf("guardian profile not found"))
	}
	if !identityaccess.GuardianInvitation(invitation).AddressedTo(profile.Email) {
		return domain.LoginAccount{}, failed(opGuardianInviteAccept, domain.ErrGuardianInvitationAddressChanged)
	}
	if grantErr := l.grantGuardianAccess(invitationCtx, invitation, profile, account); grantErr != nil {
		return domain.LoginAccount{}, grantErr
	}
	if claimErr := l.claimGuardianEnrollments(invitationCtx, account.ID, email); claimErr != nil {
		return domain.LoginAccount{}, claimErr
	}
	return account, nil
}
