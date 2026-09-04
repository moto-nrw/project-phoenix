package activities

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ======== Supervisor Methods ========

// AddSupervisor adds a supervisor to an activity group
func (s *Service) AddSupervisor(ctx context.Context, groupID int64, staffID int64, isPrimary bool) (*activities.SupervisorPlanned, error) {
	supervisor, err := s.addSupervisorInTx(ctx, s, groupID, staffID, isPrimary)
	if err != nil {
		return nil, &ActivityError{Op: "add supervisor", Err: err}
	}

	return supervisor, nil
}

// addSupervisorInTx contains the transaction logic for adding a supervisor
func (s *Service) addSupervisorInTx(ctx context.Context, txService ActivityService, groupID, staffID int64, isPrimary bool) (*activities.SupervisorPlanned, error) {
	if _, err := txService.(*Service).findMutableActivityGroup(ctx, groupID); err != nil {
		return nil, err
	}

	if err := s.validateStaffExists(ctx, staffID); err != nil {
		return nil, err
	}

	existingSupervisors, err := txService.(*Service).supervisorRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, &ActivityError{Op: "get existing supervisors", Err: err}
	}

	if err := s.checkSupervisorNotDuplicate(staffID, existingSupervisors); err != nil {
		return nil, err
	}

	supervisor := &activities.SupervisorPlanned{
		GroupID:   groupID,
		StaffID:   staffID,
		IsPrimary: isPrimary,
	}

	if err := supervisor.Validate(); err != nil {
		return nil, &ActivityError{Op: opValidateSupervisor, Err: err}
	}

	if err := s.unsetPrimarySupervisorsInTx(ctx, txService, isPrimary, existingSupervisors); err != nil {
		return nil, err
	}

	supervisor.SetTenantID(tenant.FromContext(ctx))
	if err := txService.(*Service).supervisorRepo.Create(ctx, supervisor); err != nil {
		return nil, &ActivityError{Op: opCreateSupervisor, Err: err}
	}

	createdSupervisor, err := txService.(*Service).supervisorRepo.FindByID(ctx, supervisor.ID)
	if err != nil {
		return nil, &ActivityError{Op: "retrieve created supervisor", Err: err}
	}

	return createdSupervisor, nil
}

// checkSupervisorNotDuplicate verifies a supervisor is not already assigned to the group
func (s *Service) checkSupervisorNotDuplicate(staffID int64, existingSupervisors []*activities.SupervisorPlanned) error {
	for _, existing := range existingSupervisors {
		if existing.StaffID == staffID {
			return &ActivityError{Op: "add supervisor", Err: errors.New("supervisor already assigned to this group")}
		}
	}
	return nil
}

// unsetPrimarySupervisorsInTx unsets primary flag for other supervisors if needed
func (s *Service) unsetPrimarySupervisorsInTx(ctx context.Context, txService ActivityService, isPrimary bool, existingSupervisors []*activities.SupervisorPlanned) error {
	if !isPrimary {
		return nil
	}

	for _, existing := range existingSupervisors {
		if existing.IsPrimary {
			existing.IsPrimary = false
			if err := txService.(*Service).supervisorRepo.Update(ctx, existing); err != nil {
				return &ActivityError{Op: "update existing supervisor", Err: err}
			}
		}
	}
	return nil
}

// validateStaffExists checks if a staff member exists before creating supervisor
func (s *Service) validateStaffExists(ctx context.Context, staffID int64) error {
	if _, err := s.staffRepo.FindByID(ctx, staffID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrStaffNotFound
		}
		return &ActivityError{Op: "validate staff", Err: err}
	}
	return nil
}

// GetSupervisor retrieves a supervisor by ID
func (s *Service) GetSupervisor(ctx context.Context, id int64) (*activities.SupervisorPlanned, error) {
	supervisor, err := s.supervisorRepo.FindByID(ctx, id)
	if err != nil {
		// Convert "no rows" (bare or DatabaseError-wrapped) to our own error
		if base.IsNoRows(err) {
			return nil, &ActivityError{Op: opGetSupervisor, Err: ErrSupervisorNotFound}
		}
		return nil, &ActivityError{Op: opGetSupervisor, Err: err}
	}

	return supervisor, nil
}

// GetGroupSupervisors retrieves all supervisors for a group
func (s *Service) GetGroupSupervisors(ctx context.Context, groupID int64) ([]*activities.SupervisorPlanned, error) {
	supervisors, err := s.supervisorRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, &ActivityError{Op: "get group supervisors", Err: err}
	}

	return supervisors, nil
}

// GetSupervisorsForGroups retrieves all supervisors for multiple groups in a single query
func (s *Service) GetSupervisorsForGroups(ctx context.Context, groupIDs []int64) (map[int64][]*activities.SupervisorPlanned, error) {
	if len(groupIDs) == 0 {
		return make(map[int64][]*activities.SupervisorPlanned), nil
	}

	supervisors, err := s.supervisorRepo.FindByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, &ActivityError{Op: "get supervisors for groups", Err: err}
	}

	// Build map of group ID to supervisors
	supervisorMap := make(map[int64][]*activities.SupervisorPlanned)
	for _, supervisor := range supervisors {
		supervisorMap[supervisor.GroupID] = append(supervisorMap[supervisor.GroupID], supervisor)
	}

	return supervisorMap, nil
}

// UpdateSupervisor updates a supervisor's details
func (s *Service) UpdateSupervisor(ctx context.Context, supervisor *activities.SupervisorPlanned) (*activities.SupervisorPlanned, error) {
	existingSupervisor, err := s.findExistingSupervisor(ctx, s, supervisor.ID)
	if err != nil {
		return nil, &ActivityError{Op: opUpdateSupervisor, Err: err}
	}
	if _, err := s.findMutableActivityGroup(ctx, existingSupervisor.GroupID); err != nil {
		return nil, &ActivityError{Op: opUpdateSupervisor, Err: err}
	}
	if supervisor.GroupID != existingSupervisor.GroupID {
		return nil, &ActivityError{Op: opUpdateSupervisor, Err: errors.New("cannot change activity group for a supervisor")}
	}

	if err := supervisor.Validate(); err != nil {
		return nil, &ActivityError{Op: opUpdateSupervisor, Err: err}
	}

	if err := s.handlePrimaryStatusChangeInTx(ctx, s, supervisor, existingSupervisor); err != nil {
		return nil, &ActivityError{Op: opUpdateSupervisor, Err: err}
	}

	if err := s.supervisorRepo.Update(ctx, supervisor); err != nil {
		return nil, &ActivityError{Op: opUpdateSupervisor, Err: err}
	}

	updatedSupervisor, err := s.supervisorRepo.FindByID(ctx, supervisor.ID)
	if err != nil {
		return nil, &ActivityError{Op: opUpdateSupervisor, Err: err}
	}

	return updatedSupervisor, nil
}

// findExistingSupervisor finds and validates that a supervisor exists
func (s *Service) findExistingSupervisor(ctx context.Context, txService ActivityService, supervisorID int64) (*activities.SupervisorPlanned, error) {
	existingSupervisor, err := txService.(*Service).supervisorRepo.FindByID(ctx, supervisorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSupervisorNotFound
		}
		return nil, &ActivityError{Op: opFindSupervisor, Err: err}
	}
	return existingSupervisor, nil
}

// handlePrimaryStatusChangeInTx handles primary status changes for supervisors
func (s *Service) handlePrimaryStatusChangeInTx(ctx context.Context, txService ActivityService, supervisor, existingSupervisor *activities.SupervisorPlanned) error {
	if existingSupervisor.IsPrimary == supervisor.IsPrimary {
		return nil
	}

	if !supervisor.IsPrimary {
		return &ActivityError{Op: opUpdateSupervisor, Err: errors.New("at least one supervisor must remain primary")}
	}

	otherSupervisors, err := txService.(*Service).supervisorRepo.FindByGroupID(ctx, supervisor.GroupID)
	if err != nil {
		return &ActivityError{Op: opFindGroupSupervisors, Err: err}
	}

	for _, other := range otherSupervisors {
		if other.ID != supervisor.ID && other.IsPrimary {
			other.IsPrimary = false
			if err := txService.(*Service).supervisorRepo.Update(ctx, other); err != nil {
				return &ActivityError{Op: "update other supervisor", Err: err}
			}
		}
	}

	return nil
}

// DeleteSupervisor deletes a supervisor
func (s *Service) DeleteSupervisor(ctx context.Context, id int64) error {
	supervisor, err := s.supervisorRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &ActivityError{Op: opDeleteSupervisor, Err: ErrSupervisorNotFound}
		}
		return &ActivityError{Op: opDeleteSupervisor, Err: err}
	}
	if _, err := s.findMutableActivityGroup(ctx, supervisor.GroupID); err != nil {
		return &ActivityError{Op: opDeleteSupervisor, Err: err}
	}

	if err := s.handlePrimaryDeletionInTx(ctx, s, supervisor, id); err != nil {
		return &ActivityError{Op: opDeleteSupervisor, Err: err}
	}

	if err := s.supervisorRepo.Delete(ctx, id); err != nil {
		return &ActivityError{Op: opDeleteSupervisor, Err: err}
	}

	return nil
}

// ReplaceSupervisor removes supervisorID from the activity. With no replacement
// it delegates to DeleteSupervisor (which enforces the only-supervisor rule);
// with a replacement it recomputes the supervisor roster — promoting/reordering
// the primary lead and de-duplicating an already-assigned replacement — and
// persists it via UpdateGroupSupervisors. Run inside a tenant transaction so the
// read-compute-write commits atomically.
func (s *Service) ReplaceSupervisor(ctx context.Context, activityID, supervisorID int64, replacementStaffID *int64) error {
	if replacementStaffID == nil {
		return s.DeleteSupervisor(ctx, supervisorID)
	}

	supervisors, err := s.GetGroupSupervisors(ctx, activityID)
	if err != nil {
		return err
	}

	nextSupervisorIDs, err := buildReplacementSupervisorIDs(supervisors, supervisorID, replacementStaffID)
	if err != nil {
		return err
	}

	return s.UpdateGroupSupervisors(ctx, activityID, nextSupervisorIDs)
}

// buildReplacementSupervisorIDs computes the staff-ID roster after replacing the
// supervisor identified by supervisorID with replacementStaffID. A primary lead
// is promoted to the front; an already-assigned replacement is de-duplicated.
func buildReplacementSupervisorIDs(
	supervisors []*activities.SupervisorPlanned,
	supervisorID int64,
	replacementStaffID *int64,
) ([]int64, error) {
	if len(supervisors) == 0 {
		return nil, ErrSupervisorNotFound
	}

	var (
		targetSupervisor  *activities.SupervisorPlanned
		nextSupervisorIDs []int64
	)
	replacementAlreadyAssigned := false

	for _, supervisor := range supervisors {
		if supervisor == nil {
			continue
		}

		if supervisor.ID == supervisorID {
			targetSupervisor = supervisor
			continue
		}

		if replacementStaffID != nil && supervisor.StaffID == *replacementStaffID {
			replacementAlreadyAssigned = true
		}

		nextSupervisorIDs = append(nextSupervisorIDs, supervisor.StaffID)
	}

	if targetSupervisor == nil {
		return nil, ErrSupervisorNotFound
	}

	if replacementStaffID == nil {
		return nextSupervisorIDs, nil
	}

	if targetSupervisor.IsPrimary {
		if replacementAlreadyAssigned {
			reordered := make([]int64, 0, len(nextSupervisorIDs))
			reordered = append(reordered, *replacementStaffID)
			for _, staffID := range nextSupervisorIDs {
				if staffID == *replacementStaffID {
					continue
				}
				reordered = append(reordered, staffID)
			}
			return reordered, nil
		}

		return append([]int64{*replacementStaffID}, nextSupervisorIDs...), nil
	}

	if replacementAlreadyAssigned {
		return nextSupervisorIDs, nil
	}

	return append(nextSupervisorIDs, *replacementStaffID), nil
}

// handlePrimaryDeletionInTx ensures a new primary is promoted when deleting a primary supervisor
func (s *Service) handlePrimaryDeletionInTx(ctx context.Context, txService ActivityService, supervisor *activities.SupervisorPlanned, supervisorID int64) error {
	if !supervisor.IsPrimary {
		return nil
	}

	allSupervisors, err := txService.(*Service).supervisorRepo.FindByGroupID(ctx, supervisor.GroupID)
	if err != nil {
		return &ActivityError{Op: opFindGroupSupervisors, Err: err}
	}

	newPrimary := s.findNewPrimarySupervisor(allSupervisors, supervisorID)
	if newPrimary == nil {
		return &ActivityError{Op: opDeleteSupervisor, Err: ErrOnlySupervisorRequiresReplacement}
	}

	newPrimary.IsPrimary = true
	if err := txService.(*Service).supervisorRepo.Update(ctx, newPrimary); err != nil {
		return &ActivityError{Op: "promote new primary supervisor", Err: err}
	}

	return nil
}

// findNewPrimarySupervisor finds a suitable supervisor to promote to primary
func (s *Service) findNewPrimarySupervisor(supervisors []*activities.SupervisorPlanned, excludeID int64) *activities.SupervisorPlanned {
	for _, supervisor := range supervisors {
		if supervisor.ID != excludeID {
			return supervisor
		}
	}
	return nil
}

// SetPrimarySupervisor sets a supervisor as primary and unsets others
func (s *Service) SetPrimarySupervisor(ctx context.Context, id int64) error {
	supervisor, err := s.supervisorRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &ActivityError{Op: "set primary supervisor", Err: ErrSupervisorNotFound}
		}
		return &ActivityError{Op: "set primary supervisor", Err: err}
	}
	if _, err := s.findMutableActivityGroup(ctx, supervisor.GroupID); err != nil {
		return &ActivityError{Op: "set primary supervisor", Err: err}
	}

	supervisors, err := s.supervisorRepo.FindByGroupID(ctx, supervisor.GroupID)
	if err != nil {
		return &ActivityError{Op: "set primary supervisor", Err: err}
	}

	if err := s.updateSupervisorPrimaryStatusInTx(ctx, s, id, supervisors); err != nil {
		return &ActivityError{Op: "set primary supervisor", Err: err}
	}

	return nil
}

// updateSupervisorPrimaryStatusInTx updates primary status for all supervisors in a group
func (s *Service) updateSupervisorPrimaryStatusInTx(ctx context.Context, txService ActivityService, primaryID int64, supervisors []*activities.SupervisorPlanned) error {
	for _, sup := range supervisors {
		newPrimaryStatus := sup.ID == primaryID
		if sup.IsPrimary == newPrimaryStatus {
			continue
		}

		sup.IsPrimary = newPrimaryStatus
		if err := txService.(*Service).supervisorRepo.Update(ctx, sup); err != nil {
			return &ActivityError{Op: "update supervisor primary status", Err: err}
		}
	}
	return nil
}

// UpdateGroupSupervisors updates the supervisors for a group
// This follows the UpdateGroupEnrollments pattern but for supervisors
func (s *Service) UpdateGroupSupervisors(ctx context.Context, groupID int64, staffIDs []int64) error {
	_, err := s.findMutableActivityGroup(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "UpdateGroupSupervisors", Err: err}
	}

	supervisors, err := s.supervisorRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "update group supervisors", Err: err}
	}

	currentStaffIDs, newStaffIDs := s.buildSupervisorMaps(supervisors, staffIDs)

	if err := s.removeUnwantedSupervisorsInTx(ctx, s, currentStaffIDs, newStaffIDs, staffIDs); err != nil {
		return &ActivityError{Op: "update group supervisors", Err: err}
	}

	if err := s.addNewSupervisorsInTx(ctx, s, groupID, currentStaffIDs, newStaffIDs, staffIDs); err != nil {
		return &ActivityError{Op: "update group supervisors", Err: err}
	}

	if err := s.ensurePrimarySupervisorInTx(ctx, s, groupID, staffIDs); err != nil {
		return &ActivityError{Op: "update group supervisors", Err: err}
	}

	return nil
}

// buildSupervisorMaps creates comparison maps for current and new supervisors
func (s *Service) buildSupervisorMaps(supervisors []*activities.SupervisorPlanned, staffIDs []int64) (map[int64]int64, map[int64]bool) {
	currentStaffIDs := make(map[int64]int64) // staffID -> supervisorID
	for _, supervisor := range supervisors {
		currentStaffIDs[supervisor.StaffID] = supervisor.ID
	}

	newStaffIDs := make(map[int64]bool)
	for _, staffID := range staffIDs {
		newStaffIDs[staffID] = true
	}

	return currentStaffIDs, newStaffIDs
}

// removeUnwantedSupervisorsInTx removes supervisors that are no longer assigned
func (s *Service) removeUnwantedSupervisorsInTx(ctx context.Context, txService ActivityService, currentStaffIDs map[int64]int64, newStaffIDs map[int64]bool, staffIDs []int64) error {
	if len(currentStaffIDs) == 1 && len(staffIDs) == 0 {
		return &ActivityError{Op: "update supervisors", Err: ErrOnlySupervisorRequiresReplacement}
	}

	for staffID, supervisorID := range currentStaffIDs {
		if !newStaffIDs[staffID] {
			if err := txService.(*Service).supervisorRepo.Delete(ctx, supervisorID); err != nil {
				return &ActivityError{Op: opDeleteSupervisor, Err: err}
			}
		}
	}
	return nil
}

// addNewSupervisorsInTx adds new supervisor assignments
func (s *Service) addNewSupervisorsInTx(ctx context.Context, txService ActivityService, groupID int64, currentStaffIDs map[int64]int64, newStaffIDs map[int64]bool, staffIDs []int64) error {
	for i, staffID := range staffIDs {
		if _, exists := currentStaffIDs[staffID]; !exists {
			// Validate staff exists before creating supervisor
			if err := s.validateStaffExists(ctx, staffID); err != nil {
				return err
			}

			isPrimary := i == 0 && len(newStaffIDs) == len(staffIDs)

			supervisor := &activities.SupervisorPlanned{
				GroupID:   groupID,
				StaffID:   staffID,
				IsPrimary: isPrimary,
			}
			supervisor.SetTenantID(tenant.FromContext(ctx))

			if err := txService.(*Service).supervisorRepo.Create(ctx, supervisor); err != nil {
				return &ActivityError{Op: opCreateSupervisor, Err: err}
			}
		}
	}
	return nil
}

// ensurePrimarySupervisorInTx ensures the first supervisor in the list is primary
func (s *Service) ensurePrimarySupervisorInTx(ctx context.Context, txService ActivityService, groupID int64, staffIDs []int64) error {
	if len(staffIDs) == 0 {
		return nil
	}

	updatedSupervisors, err := txService.(*Service).supervisorRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "get updated supervisors", Err: err}
	}

	primaryStaffID := staffIDs[0]
	for _, supervisor := range updatedSupervisors {
		shouldBePrimary := supervisor.StaffID == primaryStaffID
		if supervisor.IsPrimary == shouldBePrimary {
			continue
		}

		supervisor.IsPrimary = shouldBePrimary
		if err := txService.(*Service).supervisorRepo.Update(ctx, supervisor); err != nil {
			if shouldBePrimary {
				return &ActivityError{Op: "set primary supervisor", Err: err}
			}
			return &ActivityError{Op: "remove primary status", Err: err}
		}
	}

	return nil
}
