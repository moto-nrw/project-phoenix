package application

import (
	"context"
	"errors"
	"fmt"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/userscontract"
)

// The guardians created or linked together with a new student (#1513): the
// validation that runs before the first write and the writes themselves.

// germanGuardianValidationMessage translates the English validation messages
// emitted by GuardianProfile.Validate() / GuardianPhoneNumber.Validate() into
// German for the user-facing 400 response. The model methods are shared with
// other call sites and must stay language-neutral, so the translation lives
// here. Unknown messages fall back to the original text rather than masking it.
func germanGuardianValidationMessage(err error) string {
	switch err.Error() {
	case "invalid email format":
		return "ungültiges E-Mail-Format"
	case "invalid preferred contact method":
		return "ungültige bevorzugte Kontaktmethode"
	case "phone number is required":
		return "Telefonnummer ist erforderlich"
	case "invalid phone number format":
		return "ungültiges Telefonnummer-Format"
	case "phone number must contain at least 3 digits":
		return "Telefonnummer muss mindestens 3 Ziffern enthalten"
	default:
		return err.Error()
	}
}

// newGuardianValidationError classifies a model Validate() failure as bad
// input (HTTP 400) with the German reason. The repositories run the same
// Validate() but wrap it as a plain error, which renders as a 500 (#3549), so
// write paths call this before the first repository write.
func newGuardianValidationError(err error) *userscontract.ValidationError {
	return &userscontract.ValidationError{Err: errors.New(germanGuardianValidationMessage(err))}
}

// ValidateNewGuardians checks guardian input (profile, relationship type,
// emergency priority, phone numbers, and duplicate email) WITHOUT writing
// anything. Callers that persist a student and its guardians in one
// transaction MUST call this before the first write so a ValidationError
// rolls back an empty transaction instead of committing an orphaned
// student (TenantTxMiddleware only rolls back on 5xx). Every failure is
// returned as a *ValidationError (HTTP 400) carrying a user-facing German
// message.
func (s *RetainedGuardianService) ValidateNewGuardians(ctx context.Context, guardians []peoplePorts.NewStudentGuardian) error {
	// Track emails seen within this single request so two new guardians sharing
	// one email are rejected up front rather than colliding on insert.
	seenEmails := make(map[string]struct{}, len(guardians))

	for i := range guardians {
		position := i + 1
		if guardians[i].ExistingProfileID != nil {
			if err := s.validateExistingGuardian(ctx, position, guardians[i]); err != nil {
				return err
			}
			continue
		}
		if err := s.validateNewGuardian(ctx, position, guardians[i], seenEmails); err != nil {
			return err
		}
	}

	return nil
}

// validateExistingGuardian checks the select-existing path (#1513): the entry
// links an already-existing profile instead of creating one. There is no
// profile/email/phone input to validate — only that the profile exists
// (FindByID is tenant-scoped via RLS, so a cross-tenant or stale id surfaces
// as a clean 400, not a 500) and that the relationship fields are valid,
// matching the new-guardian path and the detail-page link route.
func (s *RetainedGuardianService) validateExistingGuardian(ctx context.Context, position int, guardian peoplePorts.NewStudentGuardian) error {
	if _, err := s.GuardianProfileRepo.FindByID(ctx, *guardian.ExistingProfileID); err != nil {
		//nolint:staticcheck // ST1005: user-facing German message rendered in the 400 response
		return &userscontract.ValidationError{Err: fmt.Errorf("Erziehungsberechtigte/r %d: ausgewählte Person nicht gefunden", position)}
	}
	return validateGuardianRelationship(position, guardian.Relationship)
}

// validateNewGuardian checks a guardian the request creates: its profile,
// its relationship, its e-mail and its phone numbers, in that order.
func (s *RetainedGuardianService) validateNewGuardian(ctx context.Context, position int, guardian peoplePorts.NewStudentGuardian, seenEmails map[string]struct{}) error {
	// Profile rules (email format, contact method) via the shared model
	// Validate() — the same checks the repository runs on insert.
	probe := &peopleValues.GuardianProfileSnapshot{
		FirstName:              guardian.Profile.FirstName,
		LastName:               guardian.Profile.LastName,
		Email:                  guardian.Profile.Email,
		PreferredContactMethod: guardian.Profile.PreferredContactMethod,
	}
	if err := probe.Validate(); err != nil {
		//nolint:staticcheck // ST1005: user-facing German message rendered in the 400 response
		return &userscontract.ValidationError{Err: fmt.Errorf("Erziehungsberechtigte/r %d: %s", position, germanGuardianValidationMessage(err))}
	}
	if err := validateGuardianRelationship(position, guardian.Relationship); err != nil {
		return err
	}
	if probe.Email != nil && *probe.Email != "" {
		// already trimmed + lowercased by probe.Validate()
		if err := s.validateNewGuardianEmail(ctx, position, *probe.Email, seenEmails); err != nil {
			return err
		}
	}
	return validateNewGuardianPhones(position, guardian.PhoneNumbers)
}

// validateGuardianRelationship checks the relationship fields both paths
// share.
func validateGuardianRelationship(position int, relationship peoplePorts.StudentGuardianRelationship) error {
	// Relationship type must be one of the allowed values. Without this,
	// an unsupported value passes Bind, then fails at insert as a plain
	// error (HTTP 500). users.IsValidRelationshipType is the single source
	// of truth shared with StudentGuardian.Validate() and the detail-page
	// link path, so the allowed set cannot drift across request paths.
	if !users.IsValidRelationshipType(relationship.RelationshipType) {
		//nolint:staticcheck // ST1005: user-facing German message rendered in the 400 response
		return &userscontract.ValidationError{Err: fmt.Errorf("Erziehungsberechtigte/r %d: ungültiger Beziehungstyp", position)}
	}

	// Emergency priority must be >= 1, matching the detail-page link
	// endpoint (StudentGuardianLinkRequest.Bind rejects < 1).
	if relationship.EmergencyPriority < 1 {
		//nolint:staticcheck // ST1005: user-facing German message rendered in the 400 response
		return &userscontract.ValidationError{Err: fmt.Errorf("Erziehungsberechtigte/r %d: Notfall-Priorität muss mindestens 1 sein", position)}
	}
	return nil
}

// validateNewGuardianEmail rejects a duplicate email: a sibling sharing a
// guardian email hits the UNIQUE(tenant_id, email) index on insert and would
// otherwise surface as a 500 and roll the whole student back. Detect it as
// bad input. (Reusing the existing guardian profile is deferred to #1513.)
func (s *RetainedGuardianService) validateNewGuardianEmail(ctx context.Context, position int, email string, seenEmails map[string]struct{}) error {
	if _, dup := seenEmails[email]; dup {
		//nolint:staticcheck // ST1005: user-facing German message rendered in the 400 response
		return &userscontract.ValidationError{Err: fmt.Errorf("Erziehungsberechtigte/r %d: E-Mail-Adresse %q ist mehrfach angegeben", position, email)}
	}
	if existing, err := s.GuardianProfileRepo.FindByEmail(ctx, email); err == nil && existing != nil {
		return &userscontract.ValidationError{Err: emailInUseError{email: email, position: position}}
	}
	seenEmails[email] = struct{}{}
	return nil
}

// validateNewGuardianPhones validates phone numbers the same way
// AddPhoneNumber does on insert. GuardianProfileID is set to a placeholder (1)
// only to pass the model's "ID required" guard — the real ID is assigned
// after CreateGuardian. The phone type is coerced to mobile for unknown values
// exactly like AddPhoneNumber, so the probe never rejects a type the real path
// accepts.
func validateNewGuardianPhones(position int, phones []peoplePorts.PhoneNumberCreateRequest) error {
	for j := range phones {
		phoneType := string(phones[j].PhoneType)
		if !peopleValues.ValidPhoneType(phoneType) {
			phoneType = peopleValues.PhoneTypeMobile
		}
		phoneProbe := &peopleValues.GuardianProfilePhoneSnapshot{
			GuardianProfileID: 1,
			PhoneNumber:       phones[j].PhoneNumber,
			PhoneType:         phoneType,
		}
		if err := phoneProbe.Validate(); err != nil {
			//nolint:staticcheck // ST1005: user-facing German message rendered in the 400 response
			return &userscontract.ValidationError{Err: fmt.Errorf("Erziehungsberechtigte/r %d, Telefonnummer %d: %s", position, j+1, germanGuardianValidationMessage(err))}
		}
	}
	return nil
}

// AddGuardiansToStudent creates each guardian profile, links it to the student,
// and adds its phone numbers. It reuses CreateGuardian, LinkGuardianToStudent,
// and AddPhoneNumber, all of which join the ambient tenant transaction via the
// context, so a failure on any guardian aborts the surrounding transaction and
// leaves no partial student/guardian data behind.
func (s *RetainedGuardianService) AddGuardiansToStudent(ctx context.Context, studentID int64, guardians []peoplePorts.NewStudentGuardian) error {
	// Defense-in-depth: re-validate before any write so this method is safe
	// even if a caller forgets the pre-write ValidateNewGuardians call. Bad
	// client input surfaces as a classified ValidationError (HTTP 400) instead
	// of a generic 500.
	if err := s.ValidateNewGuardians(ctx, guardians); err != nil {
		return err
	}

	// Track profile ids already linked to this student within this request so a
	// guardian selected (or typed) twice doesn't collide on the
	// UNIQUE(student_id, guardian_profile_id) index. Re-selecting the same
	// person is not an error — the duplicate link is skipped silently (#1513).
	linked := make(map[int64]struct{}, len(guardians))

	for i := range guardians {
		if err := s.addGuardianToStudent(ctx, studentID, i, guardians[i], linked); err != nil {
			return err
		}
	}

	return nil
}

// addGuardianToStudent creates or selects one guardian at index i, links it
// to the student, and adds the phone numbers of a newly created profile.
func (s *RetainedGuardianService) addGuardianToStudent(ctx context.Context, studentID int64, i int, g peoplePorts.NewStudentGuardian, linked map[int64]struct{}) error {
	var profileID int64
	if g.ExistingProfileID != nil {
		// Select-existing path: link the existing profile as-is. Never
		// create it and never touch its phone numbers — the existing
		// profile's own data stays untouched.
		profileID = *g.ExistingProfileID
		if _, dup := linked[profileID]; dup {
			return nil
		}
	} else {
		profile, err := s.CreateGuardian(ctx, g.Profile)
		if err != nil {
			return fmt.Errorf("failed to create guardian at index %d: %w", i, err)
		}
		profileID = profile.ID
	}

	// RelationshipType and EmergencyPriority were already checked by
	// ValidateNewGuardians (>= 1, allowed type) — parity with the
	// detail-page link path (LinkGuardianToStudent).
	if _, err := s.LinkGuardianToStudent(ctx, peoplePorts.StudentGuardianCreateRequest{
		StudentID:          studentID,
		GuardianProfileID:  profileID,
		RelationshipType:   g.Relationship.RelationshipType,
		GuardianRole:       g.Relationship.GuardianRole,
		IsPrimary:          g.Relationship.IsPrimary,
		IsEmergencyContact: g.Relationship.IsEmergencyContact,
		CanPickup:          g.Relationship.CanPickup,
		PickupNotes:        g.Relationship.PickupNotes,
		EmergencyPriority:  g.Relationship.EmergencyPriority,
	}); err != nil {
		return fmt.Errorf("failed to link guardian at index %d: %w", i, err)
	}
	linked[profileID] = struct{}{}

	// Phone numbers only apply to newly created profiles. An existing
	// profile keeps its own phone numbers untouched.
	if g.ExistingProfileID != nil {
		return nil
	}
	for j := range g.PhoneNumbers {
		if _, err := s.AddPhoneNumber(ctx, profileID, g.PhoneNumbers[j]); err != nil {
			return fmt.Errorf("failed to add phone number %d for guardian at index %d: %w", j, i, err)
		}
	}
	return nil
}
