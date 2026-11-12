package application

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory/internal/domain"
)

// The data import binds the new-guardian batch slice outside Owner (#5552).
var _ peopledirectory.NewGuardianBatches = (*Service)(nil)

// errGuardianBatchUnsupported refuses an entry the batch does not write the
// way the single commands would; the caller keeps those on the single path.
var errGuardianBatchUnsupported = errors.New("people directory: the guardian batch takes new guardians without e-mail only")

// CreateNewStudentGuardians joins the caller's transaction and never opens
// one (ADR 0067 addendum of 2026-11-08, point 3). The data import observes it
// (create_new_student_guardians), like CreatePersons.
func (s *Service) CreateNewStudentGuardians(ctx context.Context, links []peopledirectory.NewStudentGuardianLink) error {
	if len(links) == 0 {
		return nil
	}
	if s.guardianLifecycle == nil {
		return domain.ErrGuardianLinkOwnersUnbound
	}
	if err := s.tx.RequireTransaction(ctx); err != nil {
		return err
	}
	return s.guardianLifecycle.CreateNewStudentGuardians(ctx, links)
}

// newStudentGuardianPlan is what the five statements write.
type newStudentGuardianPlan struct {
	profiles []domain.NewGuardianProfile
	phones   [][]domain.GuardianPhoneRecord // per profile
	links    []domain.GuardianLinkRecord    // per profile, guardian id unset
}

// CreateNewStudentGuardians writes the profiles, their phones, the
// relationships with the retention reset, the pickup permissions and the
// portal accesses, one statement each. Every value is the one the single
// commands derive: CreateGuardian's defaults and validation, AddGuardianPhone's
// priority and primary flag, LinkGuardianToStudent's role preset.
func (s *GuardianLifecycle) CreateNewStudentGuardians(ctx context.Context, links []peopledirectory.NewStudentGuardianLink) error {
	if s.presets == nil {
		return errGuardianRolePresetsUnbound
	}
	g := s.guardians
	owners := g.owners
	if owners == nil {
		return domain.ErrGuardianLinkOwnersUnbound
	}
	tenantID := g.tx.TenantID(ctx)
	if tenantID <= 0 {
		return domain.ErrTenantRequired
	}
	plan, err := s.planNewStudentGuardians(links)
	if err != nil {
		return err
	}
	ids, _, err := g.store.InsertNewGuardians(ctx, plan.profiles)
	if err != nil {
		return lifecycleError(err)
	}
	var phones []domain.NewGuardianPhone
	for i, records := range plan.phones {
		for _, record := range records {
			phones = append(phones, domain.NewGuardianPhone{GuardianProfileID: ids[i], GuardianPhoneRecord: record})
		}
	}
	if _, err := g.store.InsertNewGuardianPhones(ctx, phones); err != nil {
		return lifecycleError(err)
	}
	for i := range plan.links {
		plan.links[i].GuardianProfileID = ids[i]
	}
	written, _, err := g.store.LinkNewGuardians(ctx, plan.links)
	if err != nil {
		return lifecycleError(err)
	}
	halves := make([]peopledirectory.NewGuardianLinkHalves, len(written))
	for i, link := range written {
		halves[i] = peopledirectory.NewGuardianLinkHalves{
			RelationshipID: link.ID, CanPickup: plan.links[i].CanPickup, PickupNotes: plan.links[i].PickupNotes,
			AccountID: link.AccountID, Permissions: plan.links[i].Permissions,
		}
	}
	if err := owners.CreateGuardianPickupPermissions(ctx, tenantID, halves); err != nil {
		return err
	}
	return owners.GrantGuardianStudentAccesses(ctx, tenantID, halves)
}

// planNewStudentGuardians derives every row before the first statement.
func (s *GuardianLifecycle) planNewStudentGuardians(links []peopledirectory.NewStudentGuardianLink) (newStudentGuardianPlan, error) {
	plan := newStudentGuardianPlan{
		profiles: make([]domain.NewGuardianProfile, len(links)),
		phones:   make([][]domain.GuardianPhoneRecord, len(links)),
		links:    make([]domain.GuardianLinkRecord, len(links)),
	}
	primaries := make(map[int64]bool, len(links))
	for i, entry := range links {
		in := entry.Guardian
		if entry.StudentID <= 0 || in.GuardianProfileID != nil || strings.TrimSpace(in.Email) != "" {
			return plan, errGuardianBatchUnsupported
		}
		if in.IsPrimary {
			if primaries[entry.StudentID] {
				return plan, errGuardianBatchUnsupported
			}
			primaries[entry.StudentID] = true
		}
		profile, err := newGuardianProfileRow(in)
		if err != nil {
			return plan, err
		}
		plan.profiles[i] = profile
		if plan.phones[i], err = newGuardianPhoneRows(in.PhoneNumbers); err != nil {
			return plan, err
		}
		if plan.links[i], err = s.newGuardianLinkRow(entry.StudentID, in); err != nil {
			return plan, err
		}
	}
	return plan, nil
}

// newGuardianProfileRow is the profile CreateGuardian inserts: its defaults
// and its validation, which also normalizes the names.
func newGuardianProfileRow(in peopledirectory.NewStudentGuardian) (domain.NewGuardianProfile, error) {
	snapshot := &peopledirectory.GuardianProfileSnapshot{
		FirstName: in.FirstName, LastName: in.LastName,
		AddressStreet: trimToNil(in.AddressStreet), AddressCity: trimToNil(in.AddressCity), AddressPostalCode: trimToNil(in.AddressPostalCode),
		PreferredContactMethod: in.PreferredContactMethod, LanguagePreference: in.LanguagePreference, Notes: trimToNil(in.Notes),
	}
	if snapshot.PreferredContactMethod == "" {
		snapshot.PreferredContactMethod = "phone"
	}
	if snapshot.LanguagePreference == "" {
		snapshot.LanguagePreference = "de"
	}
	if err := snapshot.Validate(); err != nil {
		return domain.NewGuardianProfile{}, newGuardianValidationError(err)
	}
	return domain.NewGuardianProfile{
		FirstName: snapshot.FirstName, LastName: snapshot.LastName,
		AddressStreet: snapshot.AddressStreet, AddressCity: snapshot.AddressCity, PostalCode: snapshot.AddressPostalCode,
		Notes: snapshot.Notes, PreferredContactMethod: snapshot.PreferredContactMethod, LanguagePreference: snapshot.LanguagePreference,
	}, nil
}

// newGuardianPhoneRows are the rows AddGuardianPhone inserts, one after the
// other, on a profile without phones: priority 1, 2, …, the first primary.
// A later phone asking to be primary would demote the first; the batch
// refuses it.
func newGuardianPhoneRows(phones []peopledirectory.NewGuardianPhone) ([]domain.GuardianPhoneRecord, error) {
	rows := make([]domain.GuardianPhoneRecord, 0, len(phones))
	for i, phone := range phones {
		if i > 0 && phone.IsPrimary {
			return nil, errGuardianBatchUnsupported
		}
		snapshot := &peopledirectory.GuardianProfilePhoneSnapshot{
			GuardianProfileID: 1, // validated before the profile exists; the id is not checked further
			PhoneNumber:       phone.PhoneNumber,
			PhoneType:         phone.PhoneType,
			IsPrimary:         phone.IsPrimary || i == 0,
			Priority:          i + 1,
		}
		if !peopledirectory.ValidPhoneType(snapshot.PhoneType) {
			snapshot.PhoneType = peopledirectory.PhoneTypeMobile
		}
		if phone.Label != "" {
			label := phone.Label
			snapshot.Label = &label
		}
		if err := snapshot.Validate(); err != nil {
			return nil, newGuardianValidationError(err)
		}
		rows = append(rows, domain.GuardianPhoneRecord{
			PhoneNumber: snapshot.PhoneNumber, PhoneType: snapshot.PhoneType, Label: snapshot.Label,
			IsPrimary: snapshot.IsPrimary, Priority: snapshot.Priority,
		})
	}
	return rows, nil
}

// newGuardianLinkRow is the relationship LinkGuardianToStudent writes: the
// row's role or the default its flags select, with the permissions that
// preset grants, normalized as the relationship validation does.
func (s *GuardianLifecycle) newGuardianLinkRow(studentID int64, in peopledirectory.NewStudentGuardian) (domain.GuardianLinkRecord, error) {
	role := in.GuardianRole
	if strings.TrimSpace(role) == "" {
		role = s.presets.DefaultRole(in.RelationshipType, in.IsPrimary, in.IsEmergencyContact, in.CanPickup)
	}
	role, permissions, err := s.preset(role)
	if err != nil {
		return domain.GuardianLinkRecord{}, err
	}
	snapshot := &peopledirectory.GuardianRelationshipSnapshot{
		StudentID: studentID, GuardianProfileID: 1, // the id is assigned by the profile insert
		RelationshipType: in.RelationshipType, GuardianRole: role, Permissions: permissions,
	}
	if err := snapshot.Validate(); err != nil {
		return domain.GuardianLinkRecord{}, err
	}
	return domain.GuardianLinkRecord{
		StudentID: studentID, RelationshipType: snapshot.RelationshipType, GuardianRole: snapshot.GuardianRole,
		IsPrimary: in.IsPrimary, IsEmergencyContact: in.IsEmergencyContact, CanPickup: in.CanPickup,
		PickupNotes: trimToNil(in.PickupNotes), EmergencyPriority: in.EmergencyPriority, Permissions: snapshot.Permissions,
	}, nil
}
