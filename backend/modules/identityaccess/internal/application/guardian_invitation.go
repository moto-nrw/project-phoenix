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
		var (
			profile    domain.GuardianProfile
			profileErr error
		)
		profile, profileErr = l.lockInvitationProfile(txCtx, request.GuardianProfileID)
		if profileErr != nil {
			return failed(opGuardianInviteCreate, profileErr)
		}
		if _, _, _, err := l.logins.LockLoginAccountByEmail(txCtx, profile.Email); err != nil {
			return failed(opGuardianInviteCreate, err)
		}
		expiry, err := l.delivery.InvitationExpiry(txCtx)
		if err != nil {
			return failed(opGuardianInviteCreate, err)
		}
		var insertErr error
		invitation, insertErr = l.invitations.InsertGuardianInvitation(txCtx, domain.GuardianInvitation{
			TenantID:          l.runtime.TenantID(txCtx),
			Token:             uuid.Must(uuid.NewV4()).String(),
			GuardianProfileID: profile.ID,
			CreatedBy:         request.CreatedBy,
			ExpiresAt:         time.Now().Add(expiry),
			ApprovalStatus:    domain.GuardianInvitationApprovalNotRequired,
		})
		if insertErr != nil {
			return failed(opGuardianInviteCreate, insertErr)
		}
		l.logger.Info("guardian invitation created",
			slog.Int64("invitation_id", invitation.ID),
			slog.Int64("guardian_profile_id", profile.ID),
			slog.Int64("created_by", request.CreatedBy),
		)
		l.mailNewInvitation(txCtx, invitation, profile)
		return nil
	})
	if err != nil {
		return identityaccess.GuardianInvitation{}, err
	}
	return identityaccess.GuardianInvitation(invitation), nil
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

// mailNewInvitation queues the mail of a new token invitation and the
// welcome that follows it (#3534). A resend mails the link alone.
func (l *AccountLifecycle) mailNewInvitation(ctx context.Context, invitation domain.GuardianInvitation, profile domain.GuardianProfile) {
	schoolName := l.delivery.SchoolName(ctx, invitation.TenantID)
	precedingOutboxID, queued := l.delivery.EnqueueInvitationEmail(ctx, invitation, profile, schoolName)
	if !queued {
		return
	}
	l.delivery.EnqueueWelcomeEmail(ctx, profile, invitation.TenantID, schoolName, precedingOutboxID)
}

// mailExistingAccountAccess tells an account holder about access granted
// without a token and welcomes them to the school (#3534); the delivery
// sends the welcome once per guardian and school.
func (l *AccountLifecycle) mailExistingAccountAccess(ctx context.Context, profile domain.GuardianProfile, tenantID int64) {
	schoolName := l.delivery.SchoolName(ctx, tenantID)
	precedingOutboxID, queued := l.delivery.EnqueueExistingAccountEmail(ctx, profile, schoolName)
	if !queued {
		return
	}
	l.delivery.EnqueueWelcomeEmail(ctx, profile, tenantID, schoolName, precedingOutboxID)
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
		result = identityaccess.GuardianInvitationPreview{
			Email:     strings.TrimSpace(profile.Email),
			FirstName: strings.TrimSpace(profile.FirstName),
			LastName:  strings.TrimSpace(profile.LastName),
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
		if err := l.requireInvitationSchool(txCtx, invitation); err != nil {
			return err
		}
		invitationCtx := l.runtime.WithTenantID(txCtx, invitation.TenantID)
		account, acceptErr := l.acceptGuardianAtSchool(invitationCtx, invitation, registration)
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
	account, found, _, err := l.logins.LockLoginAccountByEmail(ctx, email)
	if err != nil {
		return domain.LoginAccount{}, failed(opGuardianInviteAccept, err)
	}
	if found {
		return account, nil
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
	accepted, err := l.invitations.AcceptGuardianInvitation(ctx, invitation.ID, time.Now())
	if err != nil {
		return failed(opGuardianInviteAccept, err)
	}
	if !accepted {
		// Another acceptance spent the token between the read and this write.
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
		profile, found, err := l.guardians.FindGuardianProfile(ctx, invitation.GuardianProfileID)
		if err != nil {
			return failed(opGuardianInviteResend, err)
		}
		if !found {
			return failed(opGuardianInviteResend, fmt.Errorf("guardian profile not found"))
		}
		// The delivery bookkeeping starts over; the previous attempt's outcome
		// says nothing about the send that follows.
		invitation.EmailSentAt = nil
		invitation.EmailError = nil
		if err := l.invitations.UpdateGuardianInvitation(ctx, invitation); err != nil {
			return failed(opGuardianInviteResend, err)
		}
		l.logger.Info("guardian invitation resent",
			slog.Int64("invitation_id", invitation.ID),
			slog.Int64("actor_account_id", actorAccountID),
		)
		l.delivery.EnqueueInvitationEmail(ctx, invitation, profile, l.delivery.SchoolName(ctx, invitation.TenantID))
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

func (l *AccountLifecycle) acceptGuardianAtSchool(invitationCtx context.Context, invitation domain.GuardianInvitation, registration identityaccess.GuardianRegistration) (domain.LoginAccount, error) {
	profile, found, profileErr := l.guardians.FindGuardianProfile(invitationCtx, invitation.GuardianProfileID)
	if profileErr != nil {
		return domain.LoginAccount{}, failed(opGuardianInviteAccept, profileErr)
	}
	if !found {
		return domain.LoginAccount{}, failed(opGuardianInviteAccept, fmt.Errorf("guardian profile not found"))
	}
	email := strings.ToLower(strings.TrimSpace(profile.Email))
	if email == "" {
		return domain.LoginAccount{}, failed(opGuardianInviteAccept, fmt.Errorf("guardian profile missing email"))
	}
	account, accountErr := l.provisionGuardianAccount(invitationCtx, email, registration)
	if accountErr != nil {
		return domain.LoginAccount{}, accountErr
	}
	if grantErr := l.grantGuardianAccess(invitationCtx, invitation, profile, account); grantErr != nil {
		return domain.LoginAccount{}, grantErr
	}
	if claimErr := l.claimGuardianEnrollments(invitationCtx, account.ID, email); claimErr != nil {
		return domain.LoginAccount{}, claimErr
	}
	return account, nil
}
