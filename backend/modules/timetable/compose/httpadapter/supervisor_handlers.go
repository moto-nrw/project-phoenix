package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// parseSupervisorID parses supervisor ID from URL param "supervisorId".
// Returns 0 and false if parsing fails (error already rendered).
func (rs *Resource) parseSupervisorID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	supervisorID, err := common.ParseIDParam(r, "supervisorId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid supervisor ID")))
		return 0, false
	}
	return supervisorID, true
}

// checkSupervisorOwnership verifies the supervisor belongs to the specified activity.
// Returns false and renders error if ownership check fails.
func (rs *Resource) checkSupervisorOwnership(w http.ResponseWriter, r *http.Request, supervisor *activities.SupervisorPlanned, activityID int64) bool {
	if supervisor.GroupID != activityID {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("supervisor does not belong to the specified activity")))
		return false
	}
	return true
}

// fetchSupervisorsBySpecialization retrieves supervisors filtered by specialization.
func (rs *Resource) fetchSupervisorsBySpecialization(ctx context.Context, specialization string) ([]SupervisorResponse, error) {
	teachers, err := rs.UserService.GetTeachersBySpecialization(ctx, specialization)
	if err != nil {
		return nil, err
	}

	staffIDs := make([]int64, 0, len(teachers))
	for _, teacher := range teachers {
		staffIDs = append(staffIDs, teacher.StaffID)
	}
	staffByID, err := rs.UserService.GetStaffWithPersonByIDs(ctx, staffIDs)
	if err != nil {
		return nil, err
	}
	supervisors := make([]SupervisorResponse, 0, len(teachers))
	for _, teacher := range teachers {
		staff := staffByID[teacher.StaffID]
		if staff != nil && staff.Person != nil {
			supervisors = append(supervisors, SupervisorResponse{
				// Available supervisor selections are persisted back as supervisor_ids,
				// so this identifier must remain the staff ID for safe round-tripping.
				ID:        staff.ID,
				StaffID:   teacher.StaffID,
				FirstName: staff.Person.FirstName,
				LastName:  staff.Person.LastName,
				IsPrimary: false,
			})
		}
	}

	return supervisors, nil
}

// fetchAllSupervisors retrieves all staff members as potential supervisors.
func (rs *Resource) fetchAllSupervisors(ctx context.Context) ([]SupervisorResponse, error) {
	// Use batch query to avoid N+1 (fetches staff with person in single query).
	staffMembers, err := rs.UserService.ListStaffWithPerson(ctx)
	if err != nil {
		return nil, err
	}

	supervisors := make([]SupervisorResponse, 0, len(staffMembers))
	for _, staffMember := range staffMembers {
		if staffMember.Person == nil {
			slog.Default().Warn("Staff has no associated person", slog.Int64("staff_id", staffMember.ID))
			continue
		}

		supervisors = append(supervisors, SupervisorResponse{
			// Keep the public ID aligned with the staff ID expected by activity writes.
			ID:        staffMember.ID,
			StaffID:   staffMember.ID,
			FirstName: staffMember.Person.FirstName,
			LastName:  staffMember.Person.LastName,
			IsPrimary: false,
		})
	}

	return supervisors, nil
}

// SUPERVISOR ASSIGNMENT HANDLERS

// getActivitySupervisors retrieves all supervisors for a specific activity
func (rs *Resource) getActivitySupervisors(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	// Get supervisors for the activity
	supervisors, err := rs.ActivityService.GetGroupSupervisors(r.Context(), activity.ID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Convert to response objects
	responses := make([]SupervisorResponse, 0, len(supervisors))
	for _, supervisor := range supervisors {
		if supervisor == nil {
			continue // Skip nil supervisors to prevent panic
		}
		responses = append(responses, newSupervisorResponse(supervisor))
	}

	common.Respond(w, r, http.StatusOK, responses, fmt.Sprintf("Supervisors for activity '%s' retrieved successfully", activity.Name))
}

// getAvailableSupervisors retrieves available supervisors for assignment
func (rs *Resource) getAvailableSupervisors(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	specialization := r.URL.Query().Get("specialization")

	var supervisors []SupervisorResponse
	var err error

	if specialization != "" {
		supervisors, err = rs.fetchSupervisorsBySpecialization(ctx, specialization)
		if err != nil {
			slog.Default().ErrorContext(ctx, "Error fetching teachers by specialization", slog.String("error", err.Error()))
			common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to retrieve teachers")
			return
		}
	} else {
		supervisors, err = rs.fetchAllSupervisors(ctx)
		if err != nil {
			slog.Default().ErrorContext(ctx, "Error fetching staff", slog.String("error", err.Error()))
			common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to retrieve staff")
			return
		}
	}

	common.Respond(w, r, http.StatusOK, supervisors, "Available supervisors retrieved successfully")
}

type activitySupervisorReplacement struct {
	ReplacementStaffID *int64
}

func parseSupervisorReplacement(r *http.Request) (*activitySupervisorReplacement, error) {
	rawReplacement := strings.TrimSpace(r.URL.Query().Get("replacement_staff_id"))
	if rawReplacement == "" {
		return &activitySupervisorReplacement{}, nil
	}

	replacementStaffID, err := strconv.ParseInt(rawReplacement, 10, 64)
	if err != nil || replacementStaffID <= 0 {
		return nil, errors.New("replacement_staff_id must be a positive integer")
	}

	return &activitySupervisorReplacement{ReplacementStaffID: &replacementStaffID}, nil
}

// assignSupervisor assigns a supervisor to an activity
func (rs *Resource) assignSupervisor(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	// Parse request
	var req SupervisorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Validate request
	if err := req.Bind(r); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Assign supervisor
	tenantID := tenant.FromContext(r.Context())
	var supervisor *activities.SupervisorPlanned
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		supervisor, txErr = rs.ActivityService.AddSupervisor(ctx, activity.ID, req.StaffID, req.IsPrimary)
		return txErr
	}); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newSupervisorResponse(supervisor), "Supervisor assigned successfully")
}

// updateSupervisorRole updates a supervisor's role (primary/non-primary)
func (rs *Resource) updateSupervisorRole(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	supervisorID, ok := rs.parseSupervisorID(w, r)
	if !ok {
		return
	}

	// Parse request
	var req SupervisorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Get existing supervisor
	supervisor, err := rs.ActivityService.GetSupervisor(r.Context(), supervisorID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Check if supervisor belongs to the specified activity
	if !rs.checkSupervisorOwnership(w, r, supervisor, activity.ID) {
		return
	}

	// Update supervisor role within tenant transaction
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		// If making this supervisor primary, use the service method to handle it properly
		if req.IsPrimary && !supervisor.IsPrimary {
			return rs.ActivityService.SetPrimarySupervisor(ctx, supervisorID)
		} else if supervisor.IsPrimary != req.IsPrimary {
			// Only update if the primary status is changing
			supervisor.IsPrimary = req.IsPrimary
			_, txErr := rs.ActivityService.UpdateSupervisor(ctx, supervisor)
			return txErr
		}
		return nil
	}); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated supervisor
	updatedSupervisor, err := rs.ActivityService.GetSupervisor(r.Context(), supervisorID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newSupervisorResponse(updatedSupervisor), "Supervisor role updated successfully")
}

// removeSupervisor removes a supervisor from an activity
func (rs *Resource) removeSupervisor(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	supervisorID, ok := rs.parseSupervisorID(w, r)
	if !ok {
		return
	}

	// Get supervisor to verify ownership
	supervisor, err := rs.ActivityService.GetSupervisor(r.Context(), supervisorID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Check if supervisor belongs to the specified activity
	if !rs.checkSupervisorOwnership(w, r, supervisor, activity.ID) {
		return
	}

	replacement, err := parseSupervisorReplacement(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Delete supervisor
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.ActivityService.ReplaceSupervisor(ctx, activity.ID, supervisorID, replacement.ReplacementStaffID)
	}); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Supervisor removed successfully")
}
