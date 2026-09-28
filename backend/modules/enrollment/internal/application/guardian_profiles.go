package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// resolveGuardianProfile finds an existing tenant-scoped guardian by
// account or email or creates a new one. Phone numbers from the submission
// are added by the targeted-field dispatch, not here.
func (d *Decisions) resolveGuardianProfile(
	ctx context.Context,
	request *enrollmentModels.Request,
) (*GuardianProfile, bool, error) {
	profiles := d.deps.People.GuardianProfiles
	email := strings.TrimSpace(strings.ToLower(request.GuardianEmail))
	authAccountID := int64(0)
	if request.GuardianAccountID != nil && *request.GuardianAccountID > 0 {
		authAccountID = *request.GuardianAccountID
	}

	// Authenticated submit: the JWT-derived account is authoritative over the
	// parent-editable email field. Resolve THIS account's own guardian profile
	// at the tenant first, so a parent who edited the email in the form is
	// never routed onto a different account's profile (#1663). A database
	// failure must NOT degrade into the email path: that would hand the
	// linkage decision to the parent-editable email field (or create a
	// duplicate profile) on a transient outage.
	if authAccountID > 0 {
		own, err := profiles.GuardianProfileByAccount(ctx, authAccountID)
		if err != nil {
			return nil, false, fmt.Errorf("decision: resolve guardian profile by account: %w", err)
		}
		if own != nil {
			return own, false, nil
		}
	}
	if email != "" {
		existing, err := d.existingGuardianProfileByEmail(ctx, request, email, authAccountID)
		if err != nil || existing != nil {
			return existing, false, err
		}
	}
	profile, err := d.createSubmittedGuardianProfile(ctx, request, email)
	if err != nil {
		return nil, false, err
	}
	return profile, true, nil
}

// existingGuardianProfileByEmail resolves the profile the submitted email
// already belongs to, refusing a profile the authenticated submitter may not
// claim.
func (d *Decisions) existingGuardianProfileByEmail(ctx context.Context, request *enrollmentModels.Request, email string, authAccountID int64) (*GuardianProfile, error) {
	existing, err := d.deps.People.GuardianProfiles.GuardianProfileByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("decision: resolve guardian profile by email: %w", err)
	}
	if existing == nil {
		return nil, nil
	}
	if err := d.checkGuardianProfileClaim(ctx, existing, email, authAccountID); err != nil {
		return nil, err
	}
	if err := d.applyStandaloneGuardianNameCorrection(ctx, existing, request); err != nil {
		return nil, err
	}
	return existing, nil
}

// checkGuardianProfileClaim guards against an authenticated parent claiming
// an email that already belongs to a DIFFERENT account's guardian profile at
// this school. Without it the approval would skip the by-id attach (the
// resolved profile already has an account) and link the child to that other
// account. An UNLINKED profile (account_id IS NULL) is worse, not better: the
// by-id attach would bind that whole profile — and every child already
// hanging off it — to the caller's account. guardian_email stays
// parent-editable, so only the caller's OWN address makes the profile
// claimable; anything else fails closed (#1663).
func (d *Decisions) checkGuardianProfileClaim(ctx context.Context, existing *GuardianProfile, email string, authAccountID int64) error {
	if authAccountID <= 0 {
		return nil
	}
	if existing.AccountID != nil && *existing.AccountID != authAccountID {
		return fmt.Errorf("%w: guardian_profile_id %d", enrollment.ErrGuardianAccountMismatch, existing.ID)
	}
	if existing.AccountID != nil {
		return nil
	}
	owns, err := d.submitterOwnsEmail(ctx, authAccountID, email)
	if err != nil {
		return err
	}
	if !owns {
		return fmt.Errorf("%w: guardian_profile_id %d", enrollment.ErrGuardianAccountMismatch, existing.ID)
	}
	return nil
}

// createSubmittedGuardianProfile builds a fresh profile from the request.
func (d *Decisions) createSubmittedGuardianProfile(ctx context.Context, request *enrollmentModels.Request, email string) (*GuardianProfile, error) {
	profile := &GuardianProfile{
		FirstName:              strings.TrimSpace(request.GuardianFirstName),
		LastName:               strings.TrimSpace(request.GuardianLastName),
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	if email != "" {
		emailCopy := email
		profile.Email = &emailCopy
	}
	if err := d.deps.People.Rules.ValidateGuardianProfile(profile); err != nil {
		return nil, fmt.Errorf("decision: validate guardian profile: %w", err)
	}
	if err := d.deps.People.GuardianProfiles.CreateGuardianProfile(ctx, profile); err != nil {
		return nil, fmt.Errorf("decision: create guardian profile: %w", err)
	}
	return profile, nil
}

// guardianIdentityRequest returns the request identity that may receive the
// primary guardian relationship and parent-portal access. The contact email on
// an accountless late-invite request is editable, but no verification promotes
// that replacement address to an access identity. The address the school
// invited therefore remains authoritative. Missing invite state fails closed
// instead of granting access to the editable request address.
func (d *Decisions) guardianIdentityRequest(
	ctx context.Context,
	request *enrollmentModels.Request,
) (*enrollmentModels.Request, error) {
	if request == nil {
		return nil, errors.New("decision: guardian identity request is required")
	}
	if (request.GuardianAccountID != nil && *request.GuardianAccountID > 0) ||
		enrollment.NormalizedSubmissionSource(request.SubmissionSource) != enrollmentModels.RequestSourceLateInvite {
		return request, nil
	}
	if d.deps.LateInvites == nil {
		return nil, errors.New("decision: late invite repository is not configured")
	}
	invite, err := d.deps.LateInvites.LateInviteByUsedRequestID(ctx, request.ID)
	if err != nil {
		return nil, fmt.Errorf("decision: load late invite guardian identity for request %d: %w", request.ID, err)
	}
	identityRequest := *request
	identityRequest.GuardianEmail = invite.GuardianEmail
	return &identityRequest, nil
}

func (d *Decisions) applyStandaloneGuardianNameCorrection(ctx context.Context, profile *GuardianProfile, request *enrollmentModels.Request) error {
	if request == nil {
		return nil
	}
	return d.applyStandaloneGuardianProfileNameCorrection(ctx, profile, request.GuardianFirstName, request.GuardianLastName)
}

// applyStandaloneGuardianProfileNameCorrection takes the submitted name onto
// a profile no portal account manages.
func (d *Decisions) applyStandaloneGuardianProfileNameCorrection(ctx context.Context, profile *GuardianProfile, firstName, lastName string) error {
	if profile == nil || profile.AccountID != nil || profile.HasAccount {
		return nil
	}
	first := strings.TrimSpace(firstName)
	last := strings.TrimSpace(lastName)
	if profile.FirstName == first && profile.LastName == last {
		return nil
	}
	profile.FirstName = first
	profile.LastName = last
	if err := d.deps.People.Rules.ValidateGuardianProfile(profile); err != nil {
		return fmt.Errorf("decision: validate guardian profile name correction: %w", err)
	}
	if err := d.deps.People.GuardianProfiles.UpdateGuardianProfile(ctx, profile); err != nil {
		return fmt.Errorf("decision: update guardian profile name correction: %w", err)
	}
	return nil
}

// resolveAdditionalGuardianProfile returns the guardian_profiles id for a
// co-guardian, creating the profile on first use and stamping the id back
// on the request_guardians row so the request's other children reuse it.
// A stamped id always wins (idempotent across the request's children).
// Otherwise an existing profile is preferred by email when the co-guardian
// gave one; absent that, a fresh contact-only profile is created.
func (d *Decisions) resolveAdditionalGuardianProfile(
	ctx context.Context,
	extra *enrollment.RequestGuardian,
) (int64, error) {
	if extra.GuardianProfileID != nil && *extra.GuardianProfileID > 0 {
		return *extra.GuardianProfileID, nil
	}
	email := ""
	if extra.Email != nil {
		email = strings.TrimSpace(strings.ToLower(*extra.Email))
	}
	profileID, err := d.coGuardianProfileByEmail(ctx, extra, email)
	if err != nil {
		return 0, err
	}
	if profileID == 0 {
		if profileID, err = d.createCoGuardianProfile(ctx, extra, email); err != nil {
			return 0, err
		}
	}
	// Stamp the resolved profile back so the request's other children
	// reuse it. Non-fatal on failure: the link is created regardless;
	// worst case a later child creates a duplicate email-less profile.
	if err := d.deps.Guardians.StampRequestGuardianProfile(ctx, extra.ID, profileID); err != nil {
		d.logger().Warn("decision: stamp co-guardian profile failed",
			slog.Int64("request_guardian_id", extra.ID),
			slog.Int64("guardian_profile_id", profileID),
			slog.String("error", err.Error()),
		)
	}
	return profileID, nil
}

// coGuardianProfileByEmail returns the id of the existing profile of a
// co-guardian's email, or 0.
func (d *Decisions) coGuardianProfileByEmail(ctx context.Context, extra *enrollment.RequestGuardian, email string) (int64, error) {
	if email == "" {
		return 0, nil
	}
	existing, err := d.deps.People.GuardianProfiles.GuardianProfileByEmail(ctx, email)
	if err != nil {
		return 0, fmt.Errorf("resolve co-guardian profile by email: %w", err)
	}
	if existing == nil {
		return 0, nil
	}
	if err := d.applyStandaloneGuardianProfileNameCorrection(ctx, existing, extra.FirstName, extra.LastName); err != nil {
		return 0, err
	}
	return existing.ID, nil
}

// createCoGuardianProfile creates the contact-only profile of a co-guardian.
func (d *Decisions) createCoGuardianProfile(ctx context.Context, extra *enrollment.RequestGuardian, email string) (int64, error) {
	contactMethod := "email"
	if email == "" {
		contactMethod = "phone"
	}
	profile := &GuardianProfile{
		FirstName:              strings.TrimSpace(extra.FirstName),
		LastName:               strings.TrimSpace(extra.LastName),
		PreferredContactMethod: contactMethod,
		LanguagePreference:     "de",
	}
	if email != "" {
		e := email
		profile.Email = &e
	}
	if err := d.deps.People.Rules.ValidateGuardianProfile(profile); err != nil {
		return 0, fmt.Errorf("validate co-guardian profile: %w", err)
	}
	if err := d.deps.People.GuardianProfiles.CreateGuardianProfile(ctx, profile); err != nil {
		return 0, fmt.Errorf("create co-guardian profile: %w", err)
	}
	return profile.ID, nil
}

// createGuardianPhoneNumber inserts phone as the guardian's primary mobile
// number. A blank phone or an unwired phone port is a no-op. The helper is
// idempotent because approval can relink the same guardian more than once
// while syncing targeted contact fields.
func (d *Decisions) createGuardianPhoneNumber(ctx context.Context, profileID int64, phone string) error {
	phones := d.deps.People.GuardianPhones
	phone = strings.TrimSpace(phone)
	if phone == "" || phones == nil {
		return nil
	}
	existing, err := phones.GuardianPhones(ctx, profileID)
	if err != nil {
		return fmt.Errorf("find existing guardian phone numbers: %w", err)
	}
	for _, current := range existing {
		if current != nil && strings.TrimSpace(current.PhoneNumber) == phone {
			return nil
		}
	}
	row := &GuardianPhone{
		GuardianProfileID: profileID,
		PhoneNumber:       phone,
		PhoneType:         phoneTypeMobile,
		IsPrimary:         true,
	}
	if err := phones.CreateGuardianPhone(ctx, row); err != nil {
		if isDuplicatePhoneError(err) {
			return nil
		}
		return err
	}
	return nil
}

// isDuplicatePhoneError reports the unique-index refusal of a phone number the
// guardian already has.
func isDuplicatePhoneError(err error) bool {
	return strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate")
}

// guardianProfilesByEmails resolves the tenant's profiles of the given
// addresses, keyed by the lower-cased, trimmed address.
func (d *Decisions) guardianProfilesByEmails(ctx context.Context, emails []string) (map[string]*GuardianProfile, error) {
	normalized := make([]string, 0, len(emails))
	seen := make(map[string]bool, len(emails))
	for _, email := range emails {
		email = strings.ToLower(strings.TrimSpace(email))
		if email != "" && !seen[email] {
			seen[email] = true
			normalized = append(normalized, email)
		}
	}
	result := make(map[string]*GuardianProfile, len(normalized))
	if len(normalized) == 0 {
		return result, nil
	}
	profiles, err := d.deps.People.GuardianProfiles.GuardianProfilesByEmails(ctx, normalized)
	if err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		if profile != nil && profile.Email != nil {
			result[strings.ToLower(strings.TrimSpace(*profile.Email))] = profile
		}
	}
	return result, nil
}
