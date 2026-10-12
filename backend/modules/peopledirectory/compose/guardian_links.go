package compose

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The links between guardians and students: reading them with their
// invitation state, creating, updating and removing them.

// GetStudentGuardians retrieves all guardians for a student
func (s *GuardianService) GetStudentGuardians(ctx context.Context, studentID int64) ([]*GuardianWithRelationship, error) {
	relationships, err := s.StudentGuardianRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}

	profileIDs := make([]int64, 0, len(relationships))
	for _, rel := range relationships {
		profileIDs = append(profileIDs, rel.GuardianProfileID)
	}
	profiles, err := s.GuardianProfileRepo.FindByIDs(ctx, profileIDs)
	if err != nil {
		return nil, err
	}
	phonesByProfile, err := s.GuardianPhoneNumberRepo.FindByGuardianIDs(ctx, profileIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to load guardian phone numbers: %w", err)
	}
	openInvitations, err := s.openInvitationsByProfile(ctx, profileIDs)
	if err != nil {
		return nil, err
	}

	result := make([]*GuardianWithRelationship, 0, len(relationships))
	for _, rel := range relationships {
		profile, ok := profiles[rel.GuardianProfileID]
		if !ok {
			continue // Skip if profile not found
		}

		profile.PhoneNumbers = phonesByProfile[profile.ID]

		result = append(result, &GuardianWithRelationship{
			Profile:           profile,
			Relationship:      rel,
			InvitationPending: invitationPendingForStudent(profile, rel.StudentID, openInvitations[profile.ID]),
		})
	}

	return result, nil
}

// invitationPendingForStudent decides whether the staff list should surface
// "Einladung offen" for this guardian on this child. Guardians without an
// account keep the historical profile-wide check (any open invite). Guardians
// WITH an account can still have an open invitation — a pending-approval
// role-upgrade request (#2172) — but only one anchored to this child counts,
// so a sibling's invite never marks an unrelated row as pending.
func invitationPendingForStudent(profile *users.GuardianProfile, studentID int64, invitations []GuardianInvitationRecord) bool {
	for _, inv := range invitations {
		if profile.HasAccount && (inv.StudentID == nil || *inv.StudentID != studentID) {
			continue
		}
		return true
	}
	return false
}

// openInvitationsByProfile batches the open-invitation status used by the
// guardian list. The repository excludes accepted, expired, and rejected rows.
func (s *GuardianService) openInvitationsByProfile(ctx context.Context, profileIDs []int64) (map[int64][]GuardianInvitationRecord, error) {
	byProfile := make(map[int64][]GuardianInvitationRecord)
	invitations, err := s.GuardianInvitations.ListOpen(ctx, profileIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to load open guardian invitations: %w", err)
	}
	for _, invitation := range invitations {
		byProfile[invitation.GuardianProfileID] = append(byProfile[invitation.GuardianProfileID], invitation)
	}
	return byProfile, nil
}

// GetGuardianStudents retrieves all students for a guardian
func (s *GuardianService) GetGuardianStudents(ctx context.Context, guardianProfileID int64) ([]*StudentWithRelationship, error) {
	relationships, err := s.StudentGuardianRepo.FindByGuardianProfileID(ctx, guardianProfileID)
	if err != nil {
		return nil, err
	}

	studentIDs := make([]int64, 0, len(relationships))
	for _, rel := range relationships {
		studentIDs = append(studentIDs, rel.StudentID)
	}
	students, err := s.StudentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}

	result := make([]*StudentWithRelationship, 0, len(relationships))
	for _, rel := range relationships {
		student, ok := students[rel.StudentID]
		if !ok || student.IsAlumnus() {
			continue // Skip missing and graduated students
		}

		result = append(result, &StudentWithRelationship{
			Student:      student,
			Relationship: rel,
		})
	}

	return result, nil
}

// applyRolePreset sets the relationship's role to the preset and its
// parent-portal permissions to exactly the ones the preset grants.
func (s *GuardianService) applyRolePreset(relationship *users.StudentGuardian, role string) {
	normalized, granted := s.RolePresets.Preset(role)
	permissions := make(map[string]interface{}, len(granted))
	for _, permission := range granted {
		permissions[permission] = true
	}
	relationship.SetGuardianAuthorizationData(normalized, permissions)
}

// LinkGuardianToStudent creates a relationship between guardian and student
func (s *GuardianService) LinkGuardianToStudent(ctx context.Context, req StudentGuardianCreateRequest) (*users.StudentGuardian, error) {
	// Validate guardian profile exists
	if _, err := s.GuardianProfileRepo.FindByID(ctx, req.GuardianProfileID); err != nil {
		return nil, fmt.Errorf(errMsgGuardianNotFound, err)
	}

	// Validate student exists
	if _, err := s.StudentRepo.FindByID(ctx, req.StudentID); err != nil {
		return nil, fmt.Errorf("student not found: %w", err)
	}

	role := req.GuardianRole
	if strings.TrimSpace(role) == "" {
		role = s.RolePresets.DefaultRole(req.RelationshipType, req.IsPrimary, req.IsEmergencyContact, req.CanPickup)
	}

	// Build the new relationship.
	relationship := &users.StudentGuardian{
		StudentID:          req.StudentID,
		GuardianProfileID:  req.GuardianProfileID,
		RelationshipType:   req.RelationshipType,
		IsPrimary:          req.IsPrimary,
		IsEmergencyContact: req.IsEmergencyContact,
		CanPickup:          req.CanPickup,
		PickupNotes:        req.PickupNotes,
		EmergencyPriority:  req.EmergencyPriority,
	}
	s.applyRolePreset(relationship, role)
	relationship.SetTenantID(tenant.FromContext(ctx))

	// Idempotent on an existing link: re-linking a guardian already attached to
	// this student is a pure NO-OP, never an error. LinkIfNotExists inserts via
	// ON CONFLICT DO NOTHING, so a duplicate — whether sequential, concurrent, or
	// retried — neither raises a 500 nor aborts the surrounding tenant
	// transaction (a raw duplicate INSERT would do both in PostgreSQL). This is
	// the same "re-selecting is not an error" semantics as the batch create path
	// in AddGuardiansToStudent.
	inserted, err := s.StudentGuardianRepo.LinkIfNotExists(ctx, relationship)
	if err != nil {
		return nil, fmt.Errorf("failed to create relationship: %w", err)
	}
	if inserted {
		return relationship, nil
	}

	// The link already existed. Return it UNCHANGED — linking is a create, not an
	// edit. Changing an existing relationship's flags goes through the dedicated
	// update path (UpdateStudentGuardianRelationship, the detail page's "edit"
	// action). Never upserting the flags on a re-link keeps safety-relevant fields
	// (who can pick up the child, who the emergency contact is) mutable only via
	// that explicit endpoint, with no silent overwrite from a stray re-link.
	return s.existingGuardianLink(ctx, req.StudentID, req.GuardianProfileID)
}

// existingGuardianLink returns the stored link of the guardian to the student.
func (s *GuardianService) existingGuardianLink(ctx context.Context, studentID, guardianProfileID int64) (*users.StudentGuardian, error) {
	existingLinks, err := s.StudentGuardianRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("failed to load existing guardian link: %w", err)
	}
	for _, link := range existingLinks {
		if link.GuardianProfileID == guardianProfileID {
			return link, nil
		}
	}

	return nil, fmt.Errorf("guardian link reported as existing but not found for student %d", studentID)
}

// GetStudentGuardianRelationship retrieves a student-guardian relationship by ID
func (s *GuardianService) GetStudentGuardianRelationship(ctx context.Context, relationshipID int64) (*users.StudentGuardian, error) {
	relationship, err := s.StudentGuardianRepo.FindByID(ctx, relationshipID)
	if err != nil {
		return nil, fmt.Errorf("relationship not found: %w", err)
	}
	return relationship, nil
}

// UpdateStudentGuardianRelationship updates a student-guardian relationship
func (s *GuardianService) UpdateStudentGuardianRelationship(ctx context.Context, relationshipID int64, req StudentGuardianUpdateRequest) error {
	relationship, err := s.StudentGuardianRepo.FindByID(ctx, relationshipID)
	if err != nil {
		return fmt.Errorf("relationship not found: %w", err)
	}

	// Update fields if provided
	if req.RelationshipType != nil {
		relationship.RelationshipType = *req.RelationshipType
	}
	if req.GuardianRole != nil {
		s.applyRolePreset(relationship, *req.GuardianRole)
	}
	if req.IsPrimary != nil {
		relationship.IsPrimary = *req.IsPrimary
	}
	if req.IsEmergencyContact != nil {
		relationship.IsEmergencyContact = *req.IsEmergencyContact
	}
	if req.CanPickup != nil {
		relationship.CanPickup = *req.CanPickup
	}
	if req.PickupNotes != nil {
		relationship.PickupNotes = req.PickupNotes
	}
	if req.EmergencyPriority != nil {
		relationship.EmergencyPriority = *req.EmergencyPriority
	}

	if err := relationship.Validate(); err != nil {
		return err
	}

	// Column-limited on purpose: is_payer is owned by SetStudentPayer, which
	// is gated on guardians:financial and writes an audit row. A whole-row
	// update here would carry the is_payer value this request read back into
	// the row, so a payer assigned in between (or removed in between) would be
	// silently undone by an unrelated relationship edit — without the
	// permission and without a trace.
	updated, err := s.StudentGuardianRepo.UpdateColumns(ctx, relationship,
		"relationship_type",
		"guardian_role",
		"permissions",
		"is_primary",
		"is_emergency_contact",
		"can_pickup",
		"pickup_notes",
		"emergency_priority",
	)
	if err != nil {
		return err
	}
	if updated == 0 {
		return fmt.Errorf("relationship not found: %d", relationshipID)
	}
	return nil
}

// RemoveGuardianFromStudent removes a guardian from a student.
//
// mayClearPayer says whether the caller holds guardians:financial. Unlinking
// the child's payer clears the payer mark, which is a financial decision; a
// caller without that permission gets ErrPayerRemovalRequiresFinancial and the
// relationship stays intact. The check runs under the student lock so a payer
// assigned concurrently cannot slip past it.
func (s *GuardianService) RemoveGuardianFromStudent(ctx context.Context, studentID, guardianProfileID, changedByAccountID int64, mayClearPayer bool) error {
	return s.txHandler.RunInTx(ctx, func(txCtx context.Context) error {
		return s.removeGuardianFromStudent(txCtx, studentID, guardianProfileID, changedByAccountID, mayClearPayer)
	})
}

func (s *GuardianService) removeGuardianFromStudent(ctx context.Context, studentID, guardianProfileID, changedByAccountID int64, mayClearPayer bool) error {
	// The student row serializes this deletion with SetStudentPayer, which
	// takes the same lock before it reads a relationship's is_payer state.
	if _, err := s.StudentRepo.FindByIDForUpdate(ctx, studentID); err != nil {
		return err
	}

	// Find the relationship after acquiring the lock so IsPayer is current.
	relationships, err := s.StudentGuardianRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return err
	}

	for _, rel := range relationships {
		if rel.GuardianProfileID == guardianProfileID {
			return s.deleteGuardianLink(ctx, rel, changedByAccountID, mayClearPayer)
		}
	}

	return errors.New("relationship not found")
}

// deleteGuardianLink removes one link. Clearing the payer mark with it needs
// the financial permission and writes the payer audit row first.
func (s *GuardianService) deleteGuardianLink(ctx context.Context, rel *users.StudentGuardian, changedByAccountID int64, mayClearPayer bool) error {
	if rel.IsPayer {
		if !mayClearPayer {
			return ErrPayerRemovalRequiresFinancial
		}
		if err := s.recordPayerChange(ctx, rel.GuardianProfileID, rel.StudentID, changedByAccountID, "true", "false"); err != nil {
			return err
		}
	}
	return s.StudentGuardianRepo.Delete(ctx, rel.ID)
}
