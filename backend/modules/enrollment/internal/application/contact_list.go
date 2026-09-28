package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// dispatchContactList creates one additional guardian profile (or reuses an
// existing one matched by email, or by name and phone for a phone-only
// contact) per submitted contact, links it to the student via the
// student-guardian relationships, and inserts any submitted phone numbers.
// Mirrors the dedup-by-email behaviour of the CSV importer.
func (d *Decisions) dispatchContactList(ctx context.Context, raw any, studentID int64) (map[int64]bool, error) {
	linkedProfileIDs := map[int64]bool{}
	people := d.deps.People
	if people.GuardianProfiles == nil || people.StudentGuardians == nil {
		return linkedProfileIDs, nil
	}
	var entries []enrollment.ContactEntry
	if err := decodeStructured(raw, &entries); err != nil {
		return linkedProfileIDs, fmt.Errorf("decode contact_list: %w", err)
	}
	emails := make([]string, 0, len(entries))
	for i := range entries {
		if err := entries[i].Validate(); err != nil {
			return linkedProfileIDs, err
		}
		emails = append(emails, strings.ToLower(strings.TrimSpace(entries[i].Email)))
	}
	profilesByEmail, err := d.guardianProfilesByEmails(ctx, emails)
	if err != nil {
		return linkedProfileIDs, fmt.Errorf("find contact profiles by email: %w", err)
	}
	phoneOnlyProfiles, err := d.loadPhoneOnlyContactProfiles(ctx, studentID)
	if err != nil {
		return linkedProfileIDs, fmt.Errorf("load phone-only contact profiles: %w", err)
	}
	for i := range entries {
		profile, err := d.dispatchContact(ctx, entries[i], studentID, profilesByEmail, phoneOnlyProfiles)
		if err != nil {
			return linkedProfileIDs, err
		}
		linkedProfileIDs[profile.ID] = true
	}
	return linkedProfileIDs, nil
}

// dispatchContact resolves or creates the profile of one contact, adds its
// phone numbers and links it to the student.
func (d *Decisions) dispatchContact(
	ctx context.Context,
	c enrollment.ContactEntry,
	studentID int64,
	profilesByEmail map[string]*GuardianProfile,
	phoneOnlyProfiles *phoneOnlyContactProfiles,
) (*GuardianProfile, error) {
	emailLC := strings.ToLower(strings.TrimSpace(c.Email))
	profile, err := d.contactProfile(ctx, c, emailLC, profilesByEmail, phoneOnlyProfiles)
	if err != nil {
		return nil, err
	}
	if emailLC != "" {
		profilesByEmail[emailLC] = profile
	}
	phoneOnlyProfiles.add(profile, c)
	if err := d.createContactPhones(ctx, profile.ID, c); err != nil {
		return nil, err
	}
	// Student-guardian relationship with the parent-submitted flags. The
	// relationship type goes through the same German→enum mapping the CSV
	// importer uses; unknown values land on "other".
	rules := d.deps.People.Rules
	rel := &StudentGuardian{
		StudentID:          studentID,
		GuardianProfileID:  profile.ID,
		RelationshipType:   rules.MapRelationshipType(c.RelationshipType),
		IsPrimary:          false,
		IsEmergencyContact: c.IsEmergencyContact,
		CanPickup:          c.CanPickup,
	}
	rules.ApplyGuardianRole(rel, contactGuardianRole(c.IsEmergencyContact, c.CanPickup))
	if c.EmergencyPriority > 0 {
		rel.EmergencyPriority = c.EmergencyPriority
	}
	if err := d.upsertContactStudentGuardianLink(ctx, rel); err != nil {
		return nil, fmt.Errorf("link contact to student: %w", err)
	}
	return profile, nil
}

// contactProfile returns the profile a contact matches, creating one when it
// matches none.
func (d *Decisions) contactProfile(
	ctx context.Context,
	c enrollment.ContactEntry,
	emailLC string,
	profilesByEmail map[string]*GuardianProfile,
	phoneOnlyProfiles *phoneOnlyContactProfiles,
) (*GuardianProfile, error) {
	var profile *GuardianProfile
	if emailLC != "" {
		profile = profilesByEmail[emailLC]
	} else if matches := phoneOnlyProfiles.match(c); len(matches) > 0 {
		profile = matches[0]
	}
	if profile != nil {
		return profile, nil
	}
	profile = &GuardianProfile{
		FirstName:              c.FirstName,
		LastName:               c.LastName,
		PreferredContactMethod: "phone",
		LanguagePreference:     "de",
	}
	if emailLC != "" {
		email := emailLC
		profile.Email = &email
	}
	if err := d.deps.People.GuardianProfiles.CreateGuardianProfile(ctx, profile); err != nil {
		return nil, fmt.Errorf("create contact profile %s %s: %w", c.FirstName, c.LastName, err)
	}
	return profile, nil
}

// createContactPhones appends the contact's phone numbers; a number the
// profile already has is skipped by the unique index.
func (d *Decisions) createContactPhones(ctx context.Context, profileID int64, c enrollment.ContactEntry) error {
	phones := d.deps.People.GuardianPhones
	if phones == nil {
		return nil
	}
	for j := range c.PhoneNumbers {
		p := c.PhoneNumbers[j]
		label := p.Label
		phone := &GuardianPhone{
			GuardianProfileID: profileID,
			PhoneNumber:       p.PhoneNumber,
			PhoneType:         p.PhoneType,
			IsPrimary:         p.IsPrimary,
		}
		if label != "" {
			phone.Label = &label
		}
		if err := phones.CreateGuardianPhone(ctx, phone); err != nil && !isDuplicatePhoneError(err) {
			return fmt.Errorf("create contact phone: %w", err)
		}
	}
	return nil
}

// contactGuardianRole is the role preset of a submitted contact.
func contactGuardianRole(isEmergencyContact, canPickup bool) GuardianRole {
	if canPickup {
		return GuardianRolePickupOnly
	}
	if isEmergencyContact {
		return GuardianRoleEmergency
	}
	return GuardianRoleCustom
}

func contactIdentityName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func contactIdentityPhones(entry enrollment.ContactEntry) map[string]bool {
	phones := map[string]bool{}
	for _, phone := range entry.PhoneNumbers {
		number := strings.TrimSpace(phone.PhoneNumber)
		if number != "" {
			phones[number] = true
		}
	}
	return phones
}

// phoneOnlyContactProfiles are the contact-only profiles already linked to a
// student with their phone numbers, the identity of a contact without email.
type phoneOnlyContactProfiles struct {
	profiles        map[int64]*GuardianProfile
	phonesByProfile map[int64]map[string]bool
}

func (d *Decisions) loadPhoneOnlyContactProfiles(
	ctx context.Context,
	studentID int64,
) (*phoneOnlyContactProfiles, error) {
	result := &phoneOnlyContactProfiles{profiles: map[int64]*GuardianProfile{}, phonesByProfile: map[int64]map[string]bool{}}
	people := d.deps.People
	if studentID <= 0 || people.StudentGuardians == nil || people.GuardianProfiles == nil || people.GuardianPhones == nil {
		return result, nil
	}
	links, err := people.StudentGuardians.GuardianLinksOfStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	profileIDs := d.contactOnlyProfileIDs(links)
	result.profiles, err = people.GuardianProfiles.GuardianProfilesByID(ctx, profileIDs)
	if err != nil {
		return nil, err
	}
	phones, err := people.GuardianPhones.GuardianPhonesByGuardian(ctx, profileIDs)
	if err != nil {
		return nil, err
	}
	for profileID, rows := range phones {
		result.phonesByProfile[profileID] = guardianPhoneSet(rows)
	}
	return result, nil
}

func (d *Decisions) contactOnlyProfileIDs(links []*StudentGuardian) []int64 {
	ids := make([]int64, 0, len(links))
	seen := map[int64]bool{}
	for _, link := range links {
		if link == nil || link.IsPrimary || d.deps.People.Rules.IsFullGuardianRole(link.GuardianRole) || link.GuardianProfileID <= 0 || seen[link.GuardianProfileID] {
			continue
		}
		seen[link.GuardianProfileID] = true
		ids = append(ids, link.GuardianProfileID)
	}
	return ids
}

func guardianPhoneSet(rows []*GuardianPhone) map[string]bool {
	phones := make(map[string]bool, len(rows))
	for _, row := range rows {
		if row != nil {
			phones[strings.TrimSpace(row.PhoneNumber)] = true
		}
	}
	return phones
}

func (profiles *phoneOnlyContactProfiles) match(entry enrollment.ContactEntry) []*GuardianProfile {
	firstName := contactIdentityName(entry.FirstName)
	lastName := contactIdentityName(entry.LastName)
	phones := contactIdentityPhones(entry)
	if firstName == "" || lastName == "" || len(phones) == 0 {
		return nil
	}
	matches := make([]*GuardianProfile, 0, 1)
	for profileID, profile := range profiles.profiles {
		if profile == nil || contactIdentityName(profile.FirstName) != firstName || contactIdentityName(profile.LastName) != lastName {
			continue
		}
		if sharesPhone(profiles.phonesByProfile[profileID], phones) {
			matches = append(matches, profile)
		}
	}
	return matches
}

func sharesPhone(known, submitted map[string]bool) bool {
	for phone := range known {
		if submitted[phone] {
			return true
		}
	}
	return false
}

func (profiles *phoneOnlyContactProfiles) add(profile *GuardianProfile, entry enrollment.ContactEntry) {
	if profile == nil || profile.ID <= 0 {
		return
	}
	profiles.profiles[profile.ID] = profile
	phones := profiles.phonesByProfile[profile.ID]
	if phones == nil {
		phones = map[string]bool{}
		profiles.phonesByProfile[profile.ID] = phones
	}
	for phone := range contactIdentityPhones(entry) {
		phones[phone] = true
	}
}

// snapshotChildByID returns the child row with the id from a stored request
// snapshot.
func snapshotChildByID(snapshot map[string]any, childID int64) map[string]any {
	for _, raw := range sliceFromAny(snapshot["children"]) {
		row := mapFromAny(raw)
		if int64FromAny(row["id"]) == childID {
			return row
		}
	}
	return nil
}

func mapFromAny(v any) map[string]any {
	if m, ok := v.(map[string]any); ok && m != nil {
		return m
	}
	return map[string]any{}
}

func sliceFromAny(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	if stringsSlice, ok := v.([]string); ok {
		out := make([]any, 0, len(stringsSlice))
		for _, value := range stringsSlice {
			out = append(out, value)
		}
		return out
	}
	return nil
}

func int64FromAny(v any) int64 {
	switch typed := v.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		n, _ := typed.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return n
	default:
		return 0
	}
}
