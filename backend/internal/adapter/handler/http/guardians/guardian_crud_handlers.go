package guardians

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	guardianSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// Error messages (S1192 - avoid duplicate string literals)
const errInvalidGuardianID = "invalid guardian ID"

// listGuardians handles listing all guardians with pagination
func (rs *Resource) listGuardians(w http.ResponseWriter, r *http.Request) {
	// Create query options
	queryOptions := base.NewQueryOptions()

	// Add pagination
	page, pageSize := common.ParsePagination(r)
	queryOptions.WithPagination(page, pageSize)

	// Get guardians
	guardians, err := rs.GuardianService.ListGuardians(r.Context(), queryOptions)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response format
	responses := make([]*GuardianResponse, 0, len(guardians))
	for _, guardian := range guardians {
		responses = append(responses, newGuardianResponse(guardian))
	}

	// For now, return without total count (would need separate count query)
	common.RespondPaginated(w, r, http.StatusOK, responses, common.PaginationParams{Page: page, PageSize: pageSize, Total: len(responses)}, "Guardians retrieved successfully")
}

// getGuardian handles getting a guardian by ID
func (rs *Resource) getGuardian(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidGuardianID)))
		return
	}

	// Get guardian
	guardian, err := rs.GuardianService.GetGuardianByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("guardian not found")))
		return
	}

	common.Respond(w, r, http.StatusOK, newGuardianResponse(guardian), "Guardian retrieved successfully")
}

// createGuardian handles creating a new guardian profile
func (rs *Resource) createGuardian(w http.ResponseWriter, r *http.Request) {
	userPermissions := jwt.PermissionsFromCtx(r.Context())

	// Admin users can create guardians without additional checks
	isAdmin := hasAdminPermissions(userPermissions)

	// Non-admin users must be staff members with supervised groups
	if !isAdmin {
		// Check if user is staff member
		staff, err := rs.UserContextService.GetCurrentStaff(r.Context())
		if err != nil || staff == nil {
			common.RenderError(w, r, common.ErrorForbidden(errors.New("only staff members can create guardian profiles")))
			return
		}

		// Non-admin staff must supervise at least one group to create guardians
		educationGroups, err := rs.UserContextService.GetMyGroups(r.Context())
		if err != nil || len(educationGroups) == 0 {
			common.RenderError(w, r, common.ErrorForbidden(errors.New("only administrators or group supervisors can create guardian profiles")))
			return
		}
	}

	// Parse request
	req := &GuardianCreateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Convert to service request
	createReq := guardianSvc.GuardianCreateRequest{
		FirstName:              req.FirstName,
		LastName:               req.LastName,
		Email:                  req.Email,
		Phone:                  req.Phone,
		MobilePhone:            req.MobilePhone,
		AddressStreet:          req.AddressStreet,
		AddressCity:            req.AddressCity,
		AddressPostalCode:      req.AddressPostalCode,
		PreferredContactMethod: req.PreferredContactMethod,
		LanguagePreference:     req.LanguagePreference,
		Occupation:             req.Occupation,
		Employer:               req.Employer,
		Notes:                  req.Notes,
	}

	// Create guardian
	guardian, err := rs.GuardianService.CreateGuardian(r.Context(), createReq)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newGuardianResponse(guardian), "Guardian created successfully")
}

// updateGuardian handles updating an existing guardian
func (rs *Resource) updateGuardian(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidGuardianID)))
		return
	}

	canModify, err := rs.canModifyGuardian(r.Context(), id)
	if !canModify {
		common.RenderError(w, r, common.ErrorForbidden(err))
		return
	}

	req := &GuardianUpdateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	guardian, err := rs.GuardianService.GetGuardianByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("guardian not found")))
		return
	}

	updateReq := buildGuardianUpdateRequest(guardian, req)

	if err := rs.GuardianService.UpdateGuardian(r.Context(), id, updateReq); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	updated, err := rs.GuardianService.GetGuardianByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newGuardianResponse(updated), "Guardian updated successfully")
}

// buildGuardianUpdateRequest merges existing guardian data with partial updates
func buildGuardianUpdateRequest(guardian *users.GuardianProfile, req *GuardianUpdateRequest) guardianSvc.GuardianCreateRequest {
	updateReq := guardianSvc.GuardianCreateRequest{
		FirstName:              guardian.FirstName,
		LastName:               guardian.LastName,
		Email:                  guardian.Email,
		Phone:                  guardian.Phone,
		MobilePhone:            guardian.MobilePhone,
		AddressStreet:          guardian.AddressStreet,
		AddressCity:            guardian.AddressCity,
		AddressPostalCode:      guardian.AddressPostalCode,
		PreferredContactMethod: guardian.PreferredContactMethod,
		LanguagePreference:     guardian.LanguagePreference,
		Occupation:             guardian.Occupation,
		Employer:               guardian.Employer,
		Notes:                  guardian.Notes,
	}

	applyGuardianUpdates(&updateReq, req)
	return updateReq
}

// applyGuardianUpdates applies non-nil updates to the request
func applyGuardianUpdates(updateReq *guardianSvc.GuardianCreateRequest, req *GuardianUpdateRequest) {
	if req.FirstName != nil {
		updateReq.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		updateReq.LastName = *req.LastName
	}
	if req.Email != nil {
		updateReq.Email = req.Email
	}
	if req.Phone != nil {
		updateReq.Phone = req.Phone
	}
	if req.MobilePhone != nil {
		updateReq.MobilePhone = req.MobilePhone
	}
	if req.AddressStreet != nil {
		updateReq.AddressStreet = req.AddressStreet
	}
	if req.AddressCity != nil {
		updateReq.AddressCity = req.AddressCity
	}
	if req.AddressPostalCode != nil {
		updateReq.AddressPostalCode = req.AddressPostalCode
	}
	if req.PreferredContactMethod != nil {
		updateReq.PreferredContactMethod = *req.PreferredContactMethod
	}
	if req.LanguagePreference != nil {
		updateReq.LanguagePreference = *req.LanguagePreference
	}
	if req.Occupation != nil {
		updateReq.Occupation = req.Occupation
	}
	if req.Employer != nil {
		updateReq.Employer = req.Employer
	}
	if req.Notes != nil {
		updateReq.Notes = req.Notes
	}
}

// deleteGuardian handles deleting a guardian and all their relationships
func (rs *Resource) deleteGuardian(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidGuardianID)))
		return
	}

	// Check permissions - only supervisors of the guardian's students can delete
	canModify, err := rs.canModifyGuardian(r.Context(), id)
	if !canModify {
		common.RenderError(w, r, common.ErrorForbidden(err))
		return
	}

	// Delete guardian
	if err := rs.GuardianService.DeleteGuardian(r.Context(), id); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Guardian deleted successfully")
}

// listGuardiansWithoutAccount handles listing guardians who don't have accounts
func (rs *Resource) listGuardiansWithoutAccount(w http.ResponseWriter, r *http.Request) {
	guardians, err := rs.GuardianService.GetGuardiansWithoutAccount(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]*GuardianResponse, 0, len(guardians))
	for _, guardian := range guardians {
		responses = append(responses, newGuardianResponse(guardian))
	}

	common.Respond(w, r, http.StatusOK, responses, "Guardians without accounts retrieved successfully")
}

// listInvitableGuardians handles listing guardians who can be invited (has email, no account)
func (rs *Resource) listInvitableGuardians(w http.ResponseWriter, r *http.Request) {
	guardians, err := rs.GuardianService.GetInvitableGuardians(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]*GuardianResponse, 0, len(guardians))
	for _, guardian := range guardians {
		responses = append(responses, newGuardianResponse(guardian))
	}

	common.Respond(w, r, http.StatusOK, responses, "Invitable guardians retrieved successfully")
}
