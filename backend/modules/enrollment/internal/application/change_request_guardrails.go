package application

import (
	"context"
	"fmt"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Guardians with a parent-portal account maintain their own details in the
// portal; a change request may not rewrite them.

// validateAccountLinkedGuardianEdits refuses a proposal that changes the
// details of an account-linked primary guardian or co-guardian, or that adds
// an account-linked co-guardian whose details differ from the portal profile.
func (s *ChangeRequests) validateAccountLinkedGuardianEdits(ctx context.Context, req *enrollmentModels.Request, editReq SubmitRequest) error {
	if s.deps.People.GuardianProfiles == nil {
		return nil
	}
	profile, err := s.primaryGuardianProfile(ctx, req)
	if err != nil {
		return fmt.Errorf("change request: load primary guardian profile for account guardrail: %w", err)
	}
	if profile != nil && guardianHasPortalAccount(profile) {
		if !sameTrimmedString(editReq.GuardianFirstName, req.GuardianFirstName) ||
			!sameTrimmedString(editReq.GuardianLastName, req.GuardianLastName) ||
			!sameOptionalString(editReq.GuardianPhone, req.GuardianPhone) {
			return fmt.Errorf("%w: account-linked guardian profile details must be changed in the parent portal", enrollment.ErrChangeRequestInvalidData)
		}
	}
	if len(editReq.AdditionalGuardians) == 0 || s.deps.Guardians == nil {
		return nil
	}
	existing, err := s.deps.Guardians.RequestGuardians(ctx, []int64{req.ID})
	if err != nil {
		return fmt.Errorf("change request: list guardians for account guardrail: %w", err)
	}
	emails := make([]string, 0, len(editReq.AdditionalGuardians))
	for _, guardian := range editReq.AdditionalGuardians {
		emails = append(emails, lowerTrim(optionalString(guardian.Email)))
	}
	profiles, err := s.guardianProfilesByEmails(ctx, emails)
	if err != nil {
		return fmt.Errorf("change request: load co-guardian profiles for account guardrail: %w", err)
	}
	for i, guardian := range editReq.AdditionalGuardians {
		if err := s.checkAccountLinkedCoGuardian(ctx, i, guardian, existing, profiles); err != nil {
			return err
		}
	}
	return nil
}

func (s *ChangeRequests) checkAccountLinkedCoGuardian(ctx context.Context, i int, guardian SubmitGuardian, existing []*enrollment.RequestGuardian, profiles map[string]*GuardianProfile) error {
	email := lowerTrim(optionalString(guardian.Email))
	if email == "" {
		return nil
	}
	profile := profiles[email]
	if profile == nil || !guardianHasPortalAccount(profile) {
		return nil
	}
	if old := matchingRequestGuardian(existing, profile.ID, email); old != nil {
		if !sameTrimmedString(guardian.FirstName, old.FirstName) ||
			!sameTrimmedString(guardian.LastName, old.LastName) ||
			!sameOptionalString(guardian.Phone, old.Phone) {
			return fmt.Errorf("%w: account-linked co-guardian %d details must be changed in the parent portal", enrollment.ErrChangeRequestInvalidData, i)
		}
		return nil
	}
	if !sameTrimmedString(guardian.FirstName, profile.FirstName) ||
		!sameTrimmedString(guardian.LastName, profile.LastName) ||
		!s.submittedPhoneMatchesProfile(ctx, profile.ID, guardian.Phone) {
		return fmt.Errorf("%w: account-linked co-guardian %d details must match the parent portal profile", enrollment.ErrChangeRequestInvalidData, i)
	}
	return nil
}

// guardianHasPortalAccount reports whether the profile belongs to a
// parent-portal account.
func guardianHasPortalAccount(profile *GuardianProfile) bool {
	return profile.HasAccount || profile.AccountID != nil
}

// primaryGuardianProfile resolves the request's primary guardian profile: by
// the submitting account first, by the email otherwise.
func (s *ChangeRequests) primaryGuardianProfile(ctx context.Context, req *enrollmentModels.Request) (*GuardianProfile, error) {
	profiles := s.deps.People.GuardianProfiles
	if profiles == nil || req == nil {
		return nil, nil
	}
	if req.GuardianAccountID != nil && *req.GuardianAccountID > 0 {
		profile, err := profiles.GuardianProfileByAccount(ctx, *req.GuardianAccountID)
		if err != nil {
			return nil, err
		}
		if profile != nil {
			return profile, nil
		}
	}
	email := lowerTrim(req.GuardianEmail)
	if email == "" {
		return nil, nil
	}
	return profiles.GuardianProfileByEmail(ctx, email)
}

func matchingRequestGuardian(rows []*enrollment.RequestGuardian, profileID int64, email string) *enrollment.RequestGuardian {
	email = lowerTrim(email)
	for _, row := range rows {
		if row == nil {
			continue
		}
		if row.GuardianProfileID != nil && *row.GuardianProfileID == profileID {
			return row
		}
		if email != "" && lowerTrim(optionalString(row.Email)) == email {
			return row
		}
	}
	return nil
}

func (s *ChangeRequests) submittedPhoneMatchesProfile(ctx context.Context, profileID int64, submitted *string) bool {
	phone := trimmedOptionalString(submitted)
	if phone == "" {
		return true
	}
	if s.deps.People.GuardianPhones == nil {
		return false
	}
	rows, err := s.deps.People.GuardianPhones.GuardianPhones(ctx, profileID)
	if err != nil {
		return false
	}
	for _, row := range rows {
		if strings.TrimSpace(row.PhoneNumber) == phone {
			return true
		}
	}
	return false
}

// guardianProfilesByEmails resolves the tenant's profiles of the given
// addresses in one read, keyed by the lower-cased, trimmed address.
func (s *ChangeRequests) guardianProfilesByEmails(ctx context.Context, emails []string) (map[string]*GuardianProfile, error) {
	normalized := make([]string, 0, len(emails))
	seen := make(map[string]bool, len(emails))
	for _, email := range emails {
		email = lowerTrim(email)
		if email != "" && !seen[email] {
			seen[email] = true
			normalized = append(normalized, email)
		}
	}
	result := make(map[string]*GuardianProfile, len(normalized))
	if len(normalized) == 0 {
		return result, nil
	}
	profiles, err := s.deps.People.GuardianProfiles.GuardianProfilesByEmails(ctx, normalized)
	if err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		if profile != nil && profile.Email != nil {
			result[lowerTrim(*profile.Email)] = profile
		}
	}
	return result, nil
}

func sameTrimmedString(left, right string) bool {
	return strings.TrimSpace(left) == strings.TrimSpace(right)
}

func sameOptionalString(left, right *string) bool {
	return trimmedOptionalString(left) == trimmedOptionalString(right)
}
