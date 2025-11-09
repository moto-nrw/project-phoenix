package guardians

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	guardianSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// GuardianResponse represents a guardian profile response
type GuardianResponse struct {
	ID                     int64   `json:"id"`
	FirstName              string  `json:"first_name"`
	LastName               string  `json:"last_name"`
	Email                  *string `json:"email,omitempty"`
	Phone                  *string `json:"phone,omitempty"`
	MobilePhone            *string `json:"mobile_phone,omitempty"`
	AddressStreet          *string `json:"address_street,omitempty"`
	AddressCity            *string `json:"address_city,omitempty"`
	AddressPostalCode      *string `json:"address_postal_code,omitempty"`
	PreferredContactMethod string  `json:"preferred_contact_method"`
	LanguagePreference     string  `json:"language_preference"`
	Occupation             *string `json:"occupation,omitempty"`
	Employer               *string `json:"employer,omitempty"`
	Notes                  *string `json:"notes,omitempty"`
	HasAccount             bool    `json:"has_account"`
	AccountID              *int64  `json:"account_id,omitempty"`
}

// GuardianCreateRequest represents a request to create a new guardian
type GuardianCreateRequest struct {
	FirstName              string  `json:"first_name"`
	LastName               string  `json:"last_name"`
	Email                  *string `json:"email,omitempty"`
	Phone                  *string `json:"phone,omitempty"`
	MobilePhone            *string `json:"mobile_phone,omitempty"`
	AddressStreet          *string `json:"address_street,omitempty"`
	AddressCity            *string `json:"address_city,omitempty"`
	AddressPostalCode      *string `json:"address_postal_code,omitempty"`
	PreferredContactMethod string  `json:"preferred_contact_method"`
	LanguagePreference     string  `json:"language_preference"`
	Occupation             *string `json:"occupation,omitempty"`
	Employer               *string `json:"employer,omitempty"`
	Notes                  *string `json:"notes,omitempty"`
}

// GuardianUpdateRequest represents a request to update a guardian
type GuardianUpdateRequest struct {
	FirstName              *string `json:"first_name,omitempty"`
	LastName               *string `json:"last_name,omitempty"`
	Email                  *string `json:"email,omitempty"`
	Phone                  *string `json:"phone,omitempty"`
	MobilePhone            *string `json:"mobile_phone,omitempty"`
	AddressStreet          *string `json:"address_street,omitempty"`
	AddressCity            *string `json:"address_city,omitempty"`
	AddressPostalCode      *string `json:"address_postal_code,omitempty"`
	PreferredContactMethod *string `json:"preferred_contact_method,omitempty"`
	LanguagePreference     *string `json:"language_preference,omitempty"`
	Occupation             *string `json:"occupation,omitempty"`
	Employer               *string `json:"employer,omitempty"`
	Notes                  *string `json:"notes,omitempty"`
}

// StudentGuardianLinkRequest represents a request to link a guardian to a student
type StudentGuardianLinkRequest struct {
	GuardianProfileID  int64   `json:"guardian_profile_id"`
	RelationshipType   string  `json:"relationship_type"`
	IsPrimary          bool    `json:"is_primary"`
	IsEmergencyContact bool    `json:"is_emergency_contact"`
	CanPickup          bool    `json:"can_pickup"`
	PickupNotes        *string `json:"pickup_notes,omitempty"`
	EmergencyPriority  int     `json:"emergency_priority"`
}

// StudentGuardianUpdateRequest represents a request to update a student-guardian relationship
type StudentGuardianUpdateRequest struct {
	RelationshipType   *string `json:"relationship_type,omitempty"`
	IsPrimary          *bool   `json:"is_primary,omitempty"`
	IsEmergencyContact *bool   `json:"is_emergency_contact,omitempty"`
	CanPickup          *bool   `json:"can_pickup,omitempty"`
	PickupNotes        *string `json:"pickup_notes,omitempty"`
	EmergencyPriority  *int    `json:"emergency_priority,omitempty"`
}

// Bind validates the student-guardian update request
func (req *StudentGuardianUpdateRequest) Bind(r *http.Request) error {
	// All fields are optional for update
	return nil
}

// GuardianWithStudentsResponse represents a guardian with their students
type GuardianWithStudentsResponse struct {
	Guardian *GuardianResponse          `json:"guardian"`
	Students []*StudentWithRelationship `json:"students"`
}

// StudentWithRelationship represents a student with guardian relationship details
type StudentWithRelationship struct {
	StudentID          int64   `json:"student_id"`
	FirstName          string  `json:"first_name"`
	LastName           string  `json:"last_name"`
	SchoolClass        string  `json:"school_class"`
	RelationshipID     int64   `json:"relationship_id"`
	RelationshipType   string  `json:"relationship_type"`
	IsPrimary          bool    `json:"is_primary"`
	IsEmergencyContact bool    `json:"is_emergency_contact"`
	CanPickup          bool    `json:"can_pickup"`
	PickupNotes        *string `json:"pickup_notes,omitempty"`
	EmergencyPriority  int     `json:"emergency_priority"`
}

// GuardianWithRelationship represents a guardian with student relationship details
type GuardianWithRelationship struct {
	Guardian       *GuardianResponse `json:"guardian"`
	RelationshipID int64             `json:"relationship_id"`
	RelationshipType   string  `json:"relationship_type"`
	IsPrimary          bool    `json:"is_primary"`
	IsEmergencyContact bool    `json:"is_emergency_contact"`
	CanPickup          bool    `json:"can_pickup"`
	PickupNotes        *string `json:"pickup_notes,omitempty"`
	EmergencyPriority  int     `json:"emergency_priority"`
}

// Bind validates the guardian create request
func (req *GuardianCreateRequest) Bind(r *http.Request) error {
	if req.FirstName == "" {
		return errors.New("first_name is required")
	}
	if req.LastName == "" {
		return errors.New("last_name is required")
	}
	// At least one contact method is required
	if (req.Email == nil || *req.Email == "") &&
		(req.Phone == nil || *req.Phone == "") &&
		(req.MobilePhone == nil || *req.MobilePhone == "") {
		return errors.New("at least one contact method (email, phone, or mobile_phone) is required")
	}
	return nil
}

// Bind validates the guardian update request
func (req *GuardianUpdateRequest) Bind(r *http.Request) error {
	if req.FirstName != nil && *req.FirstName == "" {
		return errors.New("first_name cannot be empty")
	}
	if req.LastName != nil && *req.LastName == "" {
		return errors.New("last_name cannot be empty")
	}
	return nil
}

// Bind validates the student-guardian link request
func (req *StudentGuardianLinkRequest) Bind(r *http.Request) error {
	if req.GuardianProfileID == 0 {
		return errors.New("guardian_profile_id is required")
	}
	if req.RelationshipType == "" {
		return errors.New("relationship_type is required")
	}
	if req.EmergencyPriority < 1 {
		return errors.New("emergency_priority must be at least 1")
	}
	return nil
}

// newGuardianResponse converts a guardian profile model to a response
func newGuardianResponse(profile *users.GuardianProfile) *GuardianResponse {
	return &GuardianResponse{
		ID:                     profile.ID,
		FirstName:              profile.FirstName,
		LastName:               profile.LastName,
		Email:                  profile.Email,
		Phone:                  profile.Phone,
		MobilePhone:            profile.MobilePhone,
		AddressStreet:          profile.AddressStreet,
		AddressCity:            profile.AddressCity,
		AddressPostalCode:      profile.AddressPostalCode,
		PreferredContactMethod: profile.PreferredContactMethod,
		LanguagePreference:     profile.LanguagePreference,
		Occupation:             profile.Occupation,
		Employer:               profile.Employer,
		Notes:                  profile.Notes,
		HasAccount:             profile.HasAccount,
		AccountID:              profile.AccountID,
	}
}

// Helper function to check if user has admin permissions
func hasAdminPermissions(permissions []string) bool {
	for _, perm := range permissions {
		if perm == "admin:*" || perm == "*:*" {
			return true
		}
	}
	return false
}

// canModifyStudent checks if the current user can modify a student's guardians
func (rs *Resource) canModifyStudent(ctx context.Context, studentID int64) (bool, error) {
	userPermissions := jwt.PermissionsFromCtx(ctx)

	// Admin users have full access
	if hasAdminPermissions(userPermissions) {
		return true, nil
	}

	// Get the student
	student, err := rs.StudentRepo.FindByID(ctx, studentID)
	if err != nil {
		return false, fmt.Errorf("student not found")
	}

	// Student must have a group for non-admin operations
	if student.GroupID == nil {
		return false, fmt.Errorf("only administrators can modify guardians for students without assigned groups")
	}

	// Check if user is a staff member who supervises the student's group
	staff, err := rs.UserContextService.GetCurrentStaff(ctx)
	if err != nil || staff == nil {
		return false, fmt.Errorf("insufficient permissions to modify this student's guardians")
	}

	// Check if staff supervises the student's group
	educationGroups, err := rs.UserContextService.GetMyGroups(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get supervised groups")
	}

	for _, group := range educationGroups {
		if group.ID == *student.GroupID {
			return true, nil
		}
	}

	return false, fmt.Errorf("you can only modify guardians for students in groups you supervise")
}

// listGuardians handles listing all guardians with pagination
func (rs *Resource) listGuardians(w http.ResponseWriter, r *http.Request) {
	// Create query options
	queryOptions := base.NewQueryOptions()

	// Add pagination
	page := 1
	pageSize := 50

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	queryOptions.WithPagination(page, pageSize)

	// Get guardians
	guardians, err := rs.GuardianService.ListGuardians(r.Context(), queryOptions)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Convert to response format
	responses := make([]*GuardianResponse, 0, len(guardians))
	for _, guardian := range guardians {
		responses = append(responses, newGuardianResponse(guardian))
	}

	// For now, return without total count (would need separate count query)
	common.RespondWithPagination(w, r, http.StatusOK, responses, page, pageSize, len(responses), "Guardians retrieved successfully")
}

// getGuardian handles getting a guardian by ID
func (rs *Resource) getGuardian(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid guardian ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get guardian
	guardian, err := rs.GuardianService.GetGuardianByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, common.ErrorNotFound(errors.New("guardian not found"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newGuardianResponse(guardian), "Guardian retrieved successfully")
}

// createGuardian handles creating a new guardian profile
func (rs *Resource) createGuardian(w http.ResponseWriter, r *http.Request) {
	// Check if user is staff member
	staff, err := rs.UserContextService.GetCurrentStaff(r.Context())
	if err != nil || staff == nil {
		if err := render.Render(w, r, common.ErrorForbidden(errors.New("only staff members can create guardian profiles"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &GuardianCreateRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusCreated, newGuardianResponse(guardian), "Guardian created successfully")
}

// updateGuardian handles updating an existing guardian
func (rs *Resource) updateGuardian(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid guardian ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &GuardianUpdateRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Get existing guardian
	guardian, err := rs.GuardianService.GetGuardianByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, common.ErrorNotFound(errors.New("guardian not found"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build update request with existing values as defaults
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

	// Apply updates
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

	// Update guardian
	if err := rs.GuardianService.UpdateGuardian(r.Context(), id, updateReq); err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get updated guardian
	updated, err := rs.GuardianService.GetGuardianByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newGuardianResponse(updated), "Guardian updated successfully")
}

// deleteGuardian handles deleting a guardian and all their relationships
func (rs *Resource) deleteGuardian(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid guardian ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Delete guardian
	if err := rs.GuardianService.DeleteGuardian(r.Context(), id); err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Guardian deleted successfully")
}

// listGuardiansWithoutAccount handles listing guardians who don't have accounts
func (rs *Resource) listGuardiansWithoutAccount(w http.ResponseWriter, r *http.Request) {
	guardians, err := rs.GuardianService.GetGuardiansWithoutAccount(r.Context())
	if err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	responses := make([]*GuardianResponse, 0, len(guardians))
	for _, guardian := range guardians {
		responses = append(responses, newGuardianResponse(guardian))
	}

	common.Respond(w, r, http.StatusOK, responses, "Invitable guardians retrieved successfully")
}

// sendInvitation handles sending an invitation to a guardian
func (rs *Resource) sendInvitation(w http.ResponseWriter, r *http.Request) {
	// Parse guardian ID from URL
	guardianID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid guardian ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get current user ID
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		if err := render.Render(w, r, common.ErrorUnauthorized(errors.New("user not authenticated"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	accountID := int64(claims.ID)

	// Send invitation
	invitationReq := guardianSvc.GuardianInvitationRequest{
		GuardianProfileID: guardianID,
		CreatedBy:         accountID,
	}

	invitation, err := rs.GuardianService.SendInvitation(r.Context(), invitationReq)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Return invitation details (without token for security)
	response := map[string]interface{}{
		"id":                    invitation.ID,
		"guardian_profile_id":   invitation.GuardianProfileID,
		"expires_at":            invitation.ExpiresAt,
		"email_sent":            invitation.EmailSentAt != nil,
	}

	common.Respond(w, r, http.StatusCreated, response, "Invitation sent successfully")
}

// listPendingInvitations handles listing all pending guardian invitations
func (rs *Resource) listPendingInvitations(w http.ResponseWriter, r *http.Request) {
	invitations, err := rs.GuardianService.GetPendingInvitations(r.Context())
	if err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Convert to response format (without tokens)
	responses := make([]map[string]interface{}, 0, len(invitations))
	for _, inv := range invitations {
		responses = append(responses, map[string]interface{}{
			"id":                    inv.ID,
			"guardian_profile_id":   inv.GuardianProfileID,
			"created_at":            inv.CreatedAt,
			"expires_at":            inv.ExpiresAt,
			"email_sent_at":         inv.EmailSentAt,
			"email_error":           inv.EmailError,
			"email_retry_count":     inv.EmailRetryCount,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Pending invitations retrieved successfully")
}

// getStudentGuardians handles getting all guardians for a student (PUBLIC - everyone can view for emergency)
func (rs *Resource) getStudentGuardians(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := strconv.ParseInt(chi.URLParam(r, "studentId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get guardians with relationships
	guardiansWithRel, err := rs.GuardianService.GetStudentGuardians(r.Context(), studentID)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Convert to response format
	responses := make([]*GuardianWithRelationship, 0, len(guardiansWithRel))
	for _, gwr := range guardiansWithRel {
		responses = append(responses, &GuardianWithRelationship{
			Guardian:           newGuardianResponse(gwr.Profile),
			RelationshipID:     gwr.Relationship.ID,
			RelationshipType:   gwr.Relationship.RelationshipType,
			IsPrimary:          gwr.Relationship.IsPrimary,
			IsEmergencyContact: gwr.Relationship.IsEmergencyContact,
			CanPickup:          gwr.Relationship.CanPickup,
			PickupNotes:        gwr.Relationship.PickupNotes,
			EmergencyPriority:  gwr.Relationship.EmergencyPriority,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Student guardians retrieved successfully")
}

// getGuardianStudents handles getting all students for a guardian
func (rs *Resource) getGuardianStudents(w http.ResponseWriter, r *http.Request) {
	// Parse guardian ID from URL
	guardianID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid guardian ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get students with relationships
	studentsWithRel, err := rs.GuardianService.GetGuardianStudents(r.Context(), guardianID)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Convert to response format
	responses := make([]*StudentWithRelationship, 0, len(studentsWithRel))
	for _, swr := range studentsWithRel {
		// Get person data for student
		person, err := rs.PersonService.Get(r.Context(), swr.Student.PersonID)
		if err != nil {
			log.Printf("Failed to get person for student %d: %v", swr.Student.ID, err)
			continue
		}

		responses = append(responses, &StudentWithRelationship{
			StudentID:          swr.Student.ID,
			FirstName:          person.FirstName,
			LastName:           person.LastName,
			SchoolClass:        swr.Student.SchoolClass,
			RelationshipID:     swr.Relationship.ID,
			RelationshipType:   swr.Relationship.RelationshipType,
			IsPrimary:          swr.Relationship.IsPrimary,
			IsEmergencyContact: swr.Relationship.IsEmergencyContact,
			CanPickup:          swr.Relationship.CanPickup,
			PickupNotes:        swr.Relationship.PickupNotes,
			EmergencyPriority:  swr.Relationship.EmergencyPriority,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Guardian students retrieved successfully")
}

// linkGuardianToStudent handles linking a guardian to a student (SUPERVISOR only)
func (rs *Resource) linkGuardianToStudent(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := strconv.ParseInt(chi.URLParam(r, "studentId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check permissions - only supervisors of the student's group can link guardians
	canModify, err := rs.canModifyStudent(r.Context(), studentID)
	if !canModify {
		if err := render.Render(w, r, common.ErrorForbidden(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &StudentGuardianLinkRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Convert to service request
	linkReq := guardianSvc.StudentGuardianCreateRequest{
		StudentID:          studentID,
		GuardianProfileID:  req.GuardianProfileID,
		RelationshipType:   req.RelationshipType,
		IsPrimary:          req.IsPrimary,
		IsEmergencyContact: req.IsEmergencyContact,
		CanPickup:          req.CanPickup,
		PickupNotes:        req.PickupNotes,
		EmergencyPriority:  req.EmergencyPriority,
	}

	// Link guardian to student
	relationship, err := rs.GuardianService.LinkGuardianToStudent(r.Context(), linkReq)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusCreated, relationship, "Guardian linked to student successfully")
}

// updateStudentGuardianRelationship handles updating a student-guardian relationship (SUPERVISOR only)
func (rs *Resource) updateStudentGuardianRelationship(w http.ResponseWriter, r *http.Request) {
	// Parse relationship ID from URL
	relationshipID, err := strconv.ParseInt(chi.URLParam(r, "relationshipId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid relationship ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get the relationship to find the student ID
	relationship, err := rs.GuardianService.GetStudentGuardianRelationship(r.Context(), relationshipID)
	if err != nil {
		if err := render.Render(w, r, common.ErrorNotFound(errors.New("relationship not found"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check permissions - only supervisors of the student's group can update relationships
	canModify, err := rs.canModifyStudent(r.Context(), relationship.StudentID)
	if !canModify {
		if err := render.Render(w, r, common.ErrorForbidden(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &StudentGuardianUpdateRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Convert to service request
	updateReq := guardianSvc.StudentGuardianUpdateRequest{
		RelationshipType:   req.RelationshipType,
		IsPrimary:          req.IsPrimary,
		IsEmergencyContact: req.IsEmergencyContact,
		CanPickup:          req.CanPickup,
		PickupNotes:        req.PickupNotes,
		EmergencyPriority:  req.EmergencyPriority,
	}

	// Update relationship
	if err := rs.GuardianService.UpdateStudentGuardianRelationship(r.Context(), relationshipID, updateReq); err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Relationship updated successfully")
}

// removeGuardianFromStudent handles removing a guardian from a student (SUPERVISOR only)
func (rs *Resource) removeGuardianFromStudent(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := strconv.ParseInt(chi.URLParam(r, "studentId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse guardian ID from URL
	guardianID, err := strconv.ParseInt(chi.URLParam(r, "guardianId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid guardian ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check permissions - only supervisors of the student's group can remove guardians
	canModify, err := rs.canModifyStudent(r.Context(), studentID)
	if !canModify {
		if err := render.Render(w, r, common.ErrorForbidden(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Remove guardian from student
	if err := rs.GuardianService.RemoveGuardianFromStudent(r.Context(), studentID, guardianID); err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Guardian removed from student successfully")
}

// PUBLIC INVITATION ENDPOINTS (No authentication required)

// GuardianInvitationAcceptRequest represents a request to accept a guardian invitation
type GuardianInvitationAcceptRequest struct {
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

// Bind validates the invitation accept request
func (req *GuardianInvitationAcceptRequest) Bind(r *http.Request) error {
	if req.Password == "" {
		return errors.New("password is required")
	}
	if req.ConfirmPassword == "" {
		return errors.New("confirm_password is required")
	}
	if req.Password != req.ConfirmPassword {
		return errors.New("passwords do not match")
	}
	return nil
}

// validateGuardianInvitation handles validating a guardian invitation token (PUBLIC)
func (rs *Resource) validateGuardianInvitation(w http.ResponseWriter, r *http.Request) {
	// Get token from URL
	token := chi.URLParam(r, "token")
	if token == "" {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invitation token is required"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Validate invitation
	result, err := rs.GuardianService.ValidateInvitation(r.Context(), token)
	if err != nil {
		if err := render.Render(w, r, common.ErrorNotFound(errors.New("invitation not found or expired"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, result, "Invitation is valid")
}

// acceptGuardianInvitation handles accepting a guardian invitation and creating an account (PUBLIC)
func (rs *Resource) acceptGuardianInvitation(w http.ResponseWriter, r *http.Request) {
	// Get token from URL
	token := chi.URLParam(r, "token")
	if token == "" {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invitation token is required"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &GuardianInvitationAcceptRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Convert to service request
	acceptReq := guardianSvc.GuardianInvitationAcceptRequest{
		Token:           token,
		Password:        req.Password,
		ConfirmPassword: req.ConfirmPassword,
	}

	// Accept invitation
	account, err := rs.GuardianService.AcceptInvitation(r.Context(), acceptReq)
	if err != nil {
		// Log the full error for debugging
		log.Printf("Error accepting invitation: %v", err)

		// Return appropriate error
		if err.Error() == "invitation not found" || err.Error() == "invitation has expired" {
			if err := render.Render(w, r, common.ErrorNotFound(err)); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
		} else {
			if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
		}
		return
	}

	// Return account details (without password hash)
	response := map[string]interface{}{
		"id":       account.ID,
		"email":    account.Email,
		"username": account.Username,
		"message":  "Account created successfully. You can now log in to the parent portal.",
	}

	common.Respond(w, r, http.StatusCreated, response, "Invitation accepted and account created successfully")
}
