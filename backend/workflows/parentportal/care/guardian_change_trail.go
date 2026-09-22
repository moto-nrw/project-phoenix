package care

import (
	"context"
	"strings"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

// guardianChangeEntry is one pending audit row: change type, field, and the
// before/after values (nil = NULL, e.g. a cleared field).
type guardianChangeEntry struct {
	changeType string
	field      string
	oldValue   *string
	newValue   *string
}

// writeGuardianChanges appends one append-only audit row per supplied change,
// snapshotting the acting guardian once. A nil repo or empty change set is a
// no-op. Must run inside the same tenant transaction as the change so the trail
// is atomic with it.
func (s *Service) writeGuardianChanges(ctx context.Context, accountID, studentID, guardianProfileID int64, changes []guardianChangeEntry) error {
	if s.GuardianChanges == nil || len(changes) == 0 {
		return nil
	}
	actorName, actorEmail := s.actorSnapshot(ctx, accountID)
	actorID := accountID
	entries := make([]GuardianChange, 0, len(changes))
	for _, c := range changes {
		entries = append(entries, GuardianChange{
			StudentID:          studentID,
			GuardianProfileID:  guardianProfileID,
			ActorAccountID:     &actorID,
			ActorNameSnapshot:  actorName,
			ActorEmailSnapshot: actorEmail,
			ChangeType:         c.changeType,
			FieldName:          c.field,
			OldValue:           c.oldValue,
			NewValue:           c.newValue,
		})
	}
	return s.GuardianChanges.RecordGuardianChanges(ctx, entries)
}

// auditPickupFlagChanges records one row per safety-critical flag (can_pickup /
// is_emergency_contact) that actually changed value. Notes and priority are
// annotations and are not audited.
func (s *Service) auditPickupFlagChanges(ctx context.Context, accountID, studentID, guardianProfileID int64, input GuardianRelationshipInput, oldCanPickup, oldEmergency bool) error {
	var changes []guardianChangeEntry
	if input.CanPickup != nil && *input.CanPickup != oldCanPickup {
		changes = append(changes, guardianChangeEntry{GuardianChangeTypePickup, GuardianFieldCanPickup, boolAuditValue(oldCanPickup), boolAuditValue(*input.CanPickup)})
	}
	if input.IsEmergencyContact != nil && *input.IsEmergencyContact != oldEmergency {
		changes = append(changes, guardianChangeEntry{GuardianChangeTypePickup, GuardianFieldEmergencyContact, boolAuditValue(oldEmergency), boolAuditValue(*input.IsEmergencyContact)})
	}
	return s.writeGuardianChanges(ctx, accountID, studentID, guardianProfileID, changes)
}

// guardianContactSnapshot captures the auditable contact fields of a profile
// (plus a stable rendering of its phone list) taken before an edit, so the
// change can be diffed against the post-edit state.
type guardianContactSnapshot struct {
	firstName  string
	lastName   string
	email      string
	street     string
	city       string
	postalCode string
	phones     string
}

func snapshotGuardianContact(profile *usersModels.GuardianProfile, phones []*usersModels.GuardianPhoneNumber) guardianContactSnapshot {
	return guardianContactSnapshot{
		firstName:  profile.FirstName,
		lastName:   profile.LastName,
		email:      deref(profile.Email),
		street:     deref(profile.AddressStreet),
		city:       deref(profile.AddressCity),
		postalCode: deref(profile.AddressPostalCode),
		phones:     formatPhonesForAudit(phones),
	}
}

// auditContactChanges records one row per contact field that actually changed
// between the pre-edit snapshot and the post-edit profile (incl. the phone
// list as a single rendered field).
//
// The before/after VALUES are deliberately not persisted (old/new left NULL):
// contact fields (name, email, phone, address) are third-party PII, and keeping
// their history in an append-only log is a Datenminimierung problem — the live
// profile always holds the current value, and the trail's purpose is "who
// changed which field, when", not a value diff. This mirrors the pickup-notes
// decision (notes content is never logged). Because no values are stored, the
// rows die with the child/guardian via the table's ON DELETE CASCADE and need
// no separate retention window.
func (s *Service) auditContactChanges(ctx context.Context, accountID, studentID, guardianProfileID int64, before guardianContactSnapshot, profile *usersModels.GuardianProfile, newPhones []*usersModels.GuardianPhoneNumber) error {
	var changes []guardianChangeEntry
	add := func(field, oldVal, newVal string) {
		if oldVal != newVal {
			changes = append(changes, guardianChangeEntry{GuardianChangeTypeContact, field, nil, nil})
		}
	}
	add(GuardianFieldFirstName, before.firstName, profile.FirstName)
	add(GuardianFieldLastName, before.lastName, profile.LastName)
	add(GuardianFieldEmail, before.email, deref(profile.Email))
	add(GuardianFieldAddressStreet, before.street, deref(profile.AddressStreet))
	add(GuardianFieldAddressCity, before.city, deref(profile.AddressCity))
	add(GuardianFieldAddressPostalCode, before.postalCode, deref(profile.AddressPostalCode))
	add(GuardianFieldPhones, before.phones, formatPhonesForAudit(newPhones))
	return s.writeGuardianChanges(ctx, accountID, studentID, guardianProfileID, changes)
}

func boolAuditValue(b bool) *string {
	v := "false"
	if b {
		v = "true"
	}
	return &v
}

// formatPhonesForAudit renders a guardian's phone list to a stable string for
// the audit trail. Old and new lists arrive from the repo in the same order
// (primary first), so an unchanged list formats identically and is not logged.
// The label is part of the rendering: a label-only edit (same number/type/
// primary) is still an accepted change and must produce a distinct string so it
// writes an audit row rather than diffing identically and being dropped.
func formatPhonesForAudit(phones []*usersModels.GuardianPhoneNumber) string {
	if len(phones) == 0 {
		return ""
	}
	parts := make([]string, 0, len(phones))
	for _, p := range phones {
		entry := p.PhoneNumber + " (" + string(p.PhoneType)
		if p.IsPrimary {
			entry += ", primär"
		}
		entry += ")"
		if p.Label != nil {
			if label := strings.TrimSpace(*p.Label); label != "" {
				entry += " [" + label + "]"
			}
		}
		parts = append(parts, entry)
	}
	return strings.Join(parts, "; ")
}

// actorSnapshot returns the acting guardian's name/email for the audit trail so
// it survives a later account deletion. Best-effort: a missing profile yields
// nils rather than failing the audited operation.
func (s *Service) actorSnapshot(ctx context.Context, accountID int64) (*string, *string) {
	actor, err := s.GuardianProfileRepo.FindByAccountID(ctx, accountID)
	if err != nil || actor == nil {
		return nil, nil
	}
	var name *string
	if full := strings.TrimSpace(actor.FirstName + " " + actor.LastName); full != "" {
		name = &full
	}
	return name, actor.Email
}
