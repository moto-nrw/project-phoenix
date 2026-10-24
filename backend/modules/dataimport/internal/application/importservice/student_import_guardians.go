package importpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
)

func (c *StudentImportConfig) createGuardianRelationships(ctx context.Context, studentID int64, guardians []importModels.GuardianImportData) error {
	for i, guardianData := range guardians {
		if err := c.createSingleGuardianRelationship(ctx, studentID, guardianData, i+1); err != nil {
			return err
		}
	}
	return nil
}

func (c *StudentImportConfig) createSingleGuardianRelationship(ctx context.Context, studentID int64, guardianData importModels.GuardianImportData, index int) error {
	guardianID, err := c.createOrFindGuardian(ctx, guardianData)
	if err != nil {
		return fmt.Errorf("guardian %d: %w", index, err)
	}
	return c.createGuardianRelationship(ctx, studentID, guardianID, guardianData, index)
}

// createGuardianRelationship links the guardian to the child. The role preset
// of the row wins; an unknown or empty preset leaves the owner to derive the
// default from the relationship flags.
func (c *StudentImportConfig) createGuardianRelationship(ctx context.Context, studentID, guardianID int64, guardianData importModels.GuardianImportData, index int) error {
	link := ports.LinkGuardian{
		StudentID:          studentID,
		GuardianProfileID:  guardianID,
		RelationshipType:   ports.MapRelationshipType(guardianData.RelationshipType),
		IsPrimary:          guardianData.IsPrimary,
		IsEmergencyContact: guardianData.IsEmergencyContact,
		CanPickup:          guardianData.CanPickup,
		PickupNotes:        importModels.OptionalCell(guardianData.PickupNotes),
		EmergencyPriority:  guardianData.EmergencyPriority,
	}
	if link.EmergencyPriority < 1 {
		link.EmergencyPriority = 1
	}
	if role, ok := importModels.MapGuardianRole(guardianData.GuardianRole); ok && role != "" {
		link.GuardianRole = role
	}
	if _, err := c.Guardians.LinkGuardianToStudent(ctx, link); err != nil {
		return fmt.Errorf("create relationship %d: %w", index, err)
	}
	return nil
}

// createOrFindGuardian deduplicates guardians by e-mail: an existing profile
// with the row's e-mail is reused (and patched with the row's non-empty
// fields), otherwise a new profile is created.
func (c *StudentImportConfig) createOrFindGuardian(ctx context.Context, data importModels.GuardianImportData) (int64, error) {
	if email := strings.TrimSpace(data.Email); email != "" {
		existing, err := c.findGuardianByEmail(ctx, email)
		if err != nil {
			return 0, fmt.Errorf("database error checking existing guardian: %w", err)
		}
		if existing != nil {
			if err := c.updateExistingGuardianProfile(ctx, *existing, data); err != nil {
				return 0, copyErrorf("existing guardian profile aktualisieren: %w", err)
			}
			if err := c.createGuardianPhoneNumbers(ctx, existing.ID, data.PhoneNumbers); err != nil {
				return existing.ID, fmt.Errorf("add phone numbers to existing guardian: %w", err)
			}
			return existing.ID, nil
		}
	}

	guardian, err := c.Guardians.CreateGuardian(ctx, ports.GuardianInput{
		FirstName:          strings.TrimSpace(data.FirstName),
		LastName:           strings.TrimSpace(data.LastName),
		Email:              importModels.OptionalCell(data.Email),
		AddressStreet:      importModels.OptionalCell(data.AddressStreet),
		AddressCity:        importModels.OptionalCell(data.AddressCity),
		AddressPostalCode:  importModels.OptionalCell(data.AddressPostalCode),
		Notes:              importModels.OptionalCell(data.Notes),
		LanguagePreference: guardianLanguagePreference(data.LanguagePreference),
	})
	if err != nil {
		return 0, err
	}
	if err := c.createGuardianPhoneNumbers(ctx, guardian.ID, data.PhoneNumbers); err != nil {
		return 0, fmt.Errorf("create phone numbers: %w", err)
	}
	return guardian.ID, nil
}

// findGuardianByEmail resolves the tenant's guardian with exactly this e-mail
// (case-insensitive, like the legacy lookup); nil when none exists.
func (c *StudentImportConfig) findGuardianByEmail(ctx context.Context, email string) (*ports.Guardian, error) {
	guardian, err := c.Guardians.FindGuardianByEmail(ctx, email)
	if errors.Is(err, ports.ErrGuardianNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &guardian, nil
}

// updateExistingGuardianProfile merges non-empty import fields into an
// existing guardian. Only fields the row carries change; the owner update is
// a full replace, so the current values are carried along.
func (c *StudentImportConfig) updateExistingGuardianProfile(ctx context.Context, existing ports.Guardian, data importModels.GuardianImportData) error {
	input := ports.GuardianInput{
		FirstName:              existing.FirstName,
		LastName:               existing.LastName,
		Email:                  existing.Email,
		AddressStreet:          existing.AddressStreet,
		AddressCity:            existing.AddressCity,
		AddressPostalCode:      existing.AddressPostalCode,
		PreferredContactMethod: existing.PreferredContactMethod,
		LanguagePreference:     existing.LanguagePreference,
		Notes:                  existing.Notes,
	}
	updated := false
	if v := strings.TrimSpace(data.AddressStreet); v != "" && !ptrEquals(existing.AddressStreet, v) {
		input.AddressStreet = importModels.OptionalCell(v)
		updated = true
	}
	if v := strings.TrimSpace(data.AddressCity); v != "" && !ptrEquals(existing.AddressCity, v) {
		input.AddressCity = importModels.OptionalCell(v)
		updated = true
	}
	if v := strings.TrimSpace(data.AddressPostalCode); v != "" && !ptrEquals(existing.AddressPostalCode, v) {
		input.AddressPostalCode = importModels.OptionalCell(v)
		updated = true
	}
	if v := strings.TrimSpace(data.Notes); v != "" && !ptrEquals(existing.Notes, v) {
		input.Notes = importModels.OptionalCell(v)
		updated = true
	}
	if v := guardianLanguagePreference(data.LanguagePreference); data.LanguagePreference != "" && v != existing.LanguagePreference {
		input.LanguagePreference = v
		updated = true
	}
	if !updated {
		return nil
	}
	return c.Guardians.UpdateGuardian(ctx, existing.ID, input)
}

// createGuardianPhoneNumbers adds the phone numbers of the row the guardian
// does not have yet. Priority and the primary flag are the owner's decision.
func (c *StudentImportConfig) createGuardianPhoneNumbers(ctx context.Context, guardianID int64, phones []importModels.PhoneImportData) error {
	if len(phones) == 0 {
		return nil
	}
	existing, err := c.Guardians.ListGuardianPhones(ctx, guardianID)
	if err != nil {
		return copyErrorf("Telefonnummern laden: %w", err)
	}
	existingNumbers := make(map[string]struct{}, len(existing))
	for _, phone := range existing {
		existingNumbers[phone.PhoneNumber] = struct{}{}
	}
	for i, phoneData := range phones {
		if phoneData.PhoneNumber == "" {
			continue
		}
		if _, exists := existingNumbers[phoneData.PhoneNumber]; exists {
			continue
		}
		var label *string
		if phoneData.Label != "" {
			label = &phoneData.Label
		}
		if _, err := c.Guardians.AddGuardianPhone(ctx, guardianID, ports.GuardianPhoneInput{
			PhoneNumber: phoneData.PhoneNumber,
			PhoneType:   string(mapPhoneType(phoneData.PhoneType)),
			Label:       label,
			IsPrimary:   phoneData.IsPrimary,
		}); err != nil {
			return fmt.Errorf("phone %d: %w", i+1, err)
		}
		existingNumbers[phoneData.PhoneNumber] = struct{}{}
	}
	return nil
}

// mergeGuardianRelationships links guardians the child does not have yet and
// patches the relationship of the ones it has (role, pickup note, priority,
// relationship type when given). The Ja/Nein flags are changed only when their
// cells are supplied; an empty cell must not revoke an existing permission.
func (c *StudentImportConfig) mergeGuardianRelationships(ctx context.Context, studentID int64, guardians []importModels.GuardianImportData) error {
	if len(guardians) == 0 {
		return nil
	}
	existing, err := c.Guardians.ListStudentGuardians(ctx, studentID)
	if err != nil {
		return copyErrorf("Erziehungsberechtigte laden: %w", err)
	}
	byGuardianID := make(map[int64]ports.GuardianLink, len(existing))
	for _, entry := range existing {
		byGuardianID[entry.Link.GuardianProfileID] = entry.Link
	}
	linkedProfiles, err := c.loadLinkedGuardianProfiles(ctx, existing)
	if err != nil {
		return err
	}

	for i, data := range guardians {
		if err := c.mergeGuardianRelationship(ctx, studentID, i+1, data, linkedProfiles, byGuardianID); err != nil {
			return err
		}
	}
	return nil
}

// loadLinkedGuardianProfiles loads the phone numbers of the guardians already
// linked to the child.
func (c *StudentImportConfig) loadLinkedGuardianProfiles(ctx context.Context, existing []ports.GuardianWithLink) ([]linkedGuardianProfile, error) {
	linked := make([]linkedGuardianProfile, 0, len(existing))
	for _, entry := range existing {
		phones, err := c.Guardians.ListGuardianPhones(ctx, entry.Guardian.ID)
		if err != nil {
			return nil, copyErrorf("Telefonnummern der Erziehungsberechtigten laden: %w", err)
		}
		var normalized []string
		for _, phone := range phones {
			if value := normalizeImportPhone(phone.PhoneNumber); value != "" {
				normalized = append(normalized, value)
			}
		}
		linked = append(linked, linkedGuardianProfile{profile: entry.Guardian, phones: normalized})
	}
	return linked, nil
}

// resolveLinkedGuardian recognises an imported guardian among the guardians the
// child already has: by e-mail, otherwise by a stored phone number, otherwise
// by a unique first and last name. The match is confined to the child's own
// relationships, so a shared landline of two different children never merges
// strangers. It returns 0 when nothing matches; the caller then falls back to
// the school-wide e-mail lookup or creates the guardian. A recognised guardian
// receives the non-empty profile fields and any new phone numbers of the row.
func (c *StudentImportConfig) resolveLinkedGuardian(ctx context.Context, linked []linkedGuardianProfile, data importModels.GuardianImportData) (int64, error) {
	match := matchLinkedGuardian(linked, data)
	if match == nil {
		return 0, nil
	}
	if err := c.updateExistingGuardianProfile(ctx, *match, data); err != nil {
		return 0, copyErrorf("existing guardian profile aktualisieren: %w", err)
	}
	if err := c.createGuardianPhoneNumbers(ctx, match.ID, data.PhoneNumbers); err != nil {
		return 0, fmt.Errorf("add phone numbers to existing guardian: %w", err)
	}
	return match.ID, nil
}

// matchLinkedGuardian is the pure matching step of resolveLinkedGuardian.
func matchLinkedGuardian(linked []linkedGuardianProfile, data importModels.GuardianImportData) *ports.Guardian {
	if len(linked) == 0 {
		return nil
	}
	if email := strings.ToLower(strings.TrimSpace(data.Email)); email != "" {
		return matchGuardianByEmail(linked, email)
	}

	if guardian := matchGuardianByPhone(linked, importGuardianPhones(data)); guardian != nil {
		return guardian
	}

	first := strings.ToLower(strings.TrimSpace(data.FirstName))
	last := strings.ToLower(strings.TrimSpace(data.LastName))
	if first == "" || last == "" {
		return nil
	}
	var byName *ports.Guardian
	for i := range linked {
		g := &linked[i]
		if strings.ToLower(strings.TrimSpace(g.profile.FirstName)) != first ||
			strings.ToLower(strings.TrimSpace(g.profile.LastName)) != last {
			continue
		}
		if byName != nil {
			return nil // ambiguous: two linked guardians share the name
		}
		byName = &g.profile
	}
	return byName
}

func (c *StudentImportConfig) mergeGuardianRelationship(ctx context.Context, studentID int64, index int, data importModels.GuardianImportData, linkedProfiles []linkedGuardianProfile, byGuardianID map[int64]ports.GuardianLink) error {
	guardianID, err := c.resolveLinkedGuardian(ctx, linkedProfiles, data)
	if err != nil {
		return fmt.Errorf("guardian %d: %w", index, err)
	}
	if guardianID == 0 {
		guardianID, err = c.createOrFindGuardian(ctx, data)
		if err != nil {
			return fmt.Errorf("guardian %d: %w", index, err)
		}
	}
	link, linked := byGuardianID[guardianID]
	if !linked {
		if err := c.createGuardianRelationship(ctx, studentID, guardianID, data, index); err != nil {
			return err
		}
		return nil
	}

	update, changed := guardianLinkUpdate(link, data)
	if changed {
		if err := c.Guardians.UpdateGuardianLink(ctx, link.ID, update); err != nil {
			return fmt.Errorf("guardian %d: Zuordnung aktualisieren: %w", index, err)
		}
	}
	return nil
}

func guardianLinkUpdate(link ports.GuardianLink, data importModels.GuardianImportData) (ports.GuardianLinkUpdate, bool) {
	update := ports.GuardianLinkUpdate{}
	changed := false
	if strings.TrimSpace(data.RelationshipType) != "" {
		if mapped := ports.MapRelationshipType(data.RelationshipType); mapped != link.RelationshipType {
			update.RelationshipType = &mapped
			changed = true
		}
	}
	if role, ok := importModels.MapGuardianRole(data.GuardianRole); ok && role != "" && role != link.GuardianRole {
		update.GuardianRole = &role
		changed = true
	}
	if notes := strings.TrimSpace(data.PickupNotes); notes != "" && !ptrEquals(link.PickupNotes, notes) {
		update.PickupNotes = &notes
		changed = true
	}
	changed = applyGuardianLinkFlags(&update, link, data) || changed
	return update, changed
}

func applyGuardianLinkFlags(update *ports.GuardianLinkUpdate, link ports.GuardianLink, data importModels.GuardianImportData) bool {
	changed := false
	if data.EmergencyPriority > 0 && data.EmergencyPriority != link.EmergencyPriority {
		priority := data.EmergencyPriority
		update.EmergencyPriority = &priority
		changed = true
	}
	if data.IsPrimarySet && data.IsPrimary != link.IsPrimary {
		primary := data.IsPrimary
		update.IsPrimary = &primary
		changed = true
	}
	if data.IsEmergencyContactSet && data.IsEmergencyContact != link.IsEmergencyContact {
		emergency := data.IsEmergencyContact
		update.IsEmergencyContact = &emergency
		changed = true
	}
	if data.CanPickupSet && data.CanPickup != link.CanPickup {
		pickup := data.CanPickup
		update.CanPickup = &pickup
		changed = true
	}
	return changed
}

func matchGuardianByEmail(linked []linkedGuardianProfile, email string) *ports.Guardian {
	for i := range linked {
		g := &linked[i]
		if g.profile.Email != nil && strings.ToLower(strings.TrimSpace(*g.profile.Email)) == email {
			return &g.profile
		}
	}
	// An e-mail that matches nobody linked is resolved school-wide by the
	// caller; a phone or name match would otherwise override the e-mail.
	return nil
}

func matchGuardianByPhone(linked []linkedGuardianProfile, phones map[string]struct{}) *ports.Guardian {
	for i := range linked {
		g := &linked[i]
		for _, stored := range g.phones {
			if _, ok := phones[stored]; ok {
				return &g.profile
			}
		}
	}
	return nil
}
