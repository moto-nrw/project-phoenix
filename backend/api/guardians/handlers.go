package guardians

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/seedtoken"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	guardianSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/spf13/viper"
	"github.com/uptrace/bun"
)

// Error messages (S1192 - avoid duplicate string literals)
const (
	errInvalidGuardianID    = "invalid guardian ID"
	errInvalidPhoneID       = "invalid phone ID"
	errPhoneNotFound        = "phone number not found"
	errPhoneNotBelongsGuard = "phone number does not belong to this guardian"
	maxDeliveryProfileIDs   = 100
)

// Note: "log" import kept for non-RenderError logging (e.g., line 741)

// PhoneNumberResponse represents a guardian phone number in API responses
type PhoneNumberResponse struct {
	ID          int64   `json:"id"`
	PhoneNumber string  `json:"phone_number"`
	PhoneType   string  `json:"phone_type"`
	Label       *string `json:"label,omitempty"`
	IsPrimary   bool    `json:"is_primary"`
	Priority    int     `json:"priority"`
}

// GuardianResponse represents a guardian profile response
type GuardianResponse struct {
	ID                     int64                  `json:"id"`
	FirstName              string                 `json:"first_name"`
	LastName               string                 `json:"last_name"`
	Email                  *string                `json:"email,omitempty"`
	PhoneNumbers           []*PhoneNumberResponse `json:"phone_numbers,omitempty"`
	AddressStreet          *string                `json:"address_street,omitempty"`
	AddressCity            *string                `json:"address_city,omitempty"`
	AddressPostalCode      *string                `json:"address_postal_code,omitempty"`
	PreferredContactMethod string                 `json:"preferred_contact_method"`
	LanguagePreference     string                 `json:"language_preference"`
	Notes                  *string                `json:"notes,omitempty"`
	HasAccount             bool                   `json:"has_account"`
	AccountID              *int64                 `json:"account_id,omitempty"`
}

// GuardianPickerResponse is the MINIMAL, GDPR-safe projection returned by the
// guardian picker search (GET /guardians/search). Unlike the admin guardian list
// (full profiles) the projection here is deliberately the smallest thing that
// still lets someone recognise a contact they already know — and nothing they
// could use to profile a family they have no relationship to (#1513):
//   - Only a COUNT of other linked children is returned, never their names —
//     a guardian's other children belong to other families the searcher may
//     not supervise. Full child names stay on the guardian detail view.
//     Address, notes, language, contact method, account_id are all omitted.
type GuardianPickerResponse struct {
	ID                  int64   `json:"id"`
	FirstName           string  `json:"first_name"`
	LastName            string  `json:"last_name"`
	Email               *string `json:"email,omitempty"`
	LinkedChildrenCount int     `json:"linked_children_count"`
}

// GuardianCreateRequest represents a request to create a new guardian
// Note: Phone numbers should be added separately via POST /guardians/{id}/phone-numbers
type GuardianCreateRequest struct {
	FirstName              string  `json:"first_name"`
	LastName               string  `json:"last_name"`
	Email                  *string `json:"email,omitempty"`
	AddressStreet          *string `json:"address_street,omitempty"`
	AddressCity            *string `json:"address_city,omitempty"`
	AddressPostalCode      *string `json:"address_postal_code,omitempty"`
	PreferredContactMethod string  `json:"preferred_contact_method"`
	LanguagePreference     string  `json:"language_preference"`
	Notes                  *string `json:"notes,omitempty"`
}

// GuardianUpdateRequest represents a request to update a guardian
// Note: Phone numbers are updated via separate phone number endpoints
type GuardianUpdateRequest struct {
	FirstName              *string `json:"first_name,omitempty"`
	LastName               *string `json:"last_name,omitempty"`
	Email                  *string `json:"email,omitempty"`
	AddressStreet          *string `json:"address_street,omitempty"`
	AddressCity            *string `json:"address_city,omitempty"`
	AddressPostalCode      *string `json:"address_postal_code,omitempty"`
	PreferredContactMethod *string `json:"preferred_contact_method,omitempty"`
	LanguagePreference     *string `json:"language_preference,omitempty"`
	Notes                  *string `json:"notes,omitempty"`
}

// StudentGuardianLinkRequest represents a request to link a guardian to a student
type StudentGuardianLinkRequest struct {
	GuardianProfileID  int64   `json:"guardian_profile_id"`
	RelationshipType   string  `json:"relationship_type"`
	GuardianRole       string  `json:"guardian_role,omitempty"`
	IsPrimary          bool    `json:"is_primary"`
	IsEmergencyContact bool    `json:"is_emergency_contact"`
	CanPickup          bool    `json:"can_pickup"`
	PickupNotes        *string `json:"pickup_notes,omitempty"`
	EmergencyPriority  int     `json:"emergency_priority"`
}

// StudentGuardianUpdateRequest represents a request to update a student-guardian relationship
type StudentGuardianUpdateRequest struct {
	RelationshipType   *string `json:"relationship_type,omitempty"`
	GuardianRole       *string `json:"guardian_role,omitempty"`
	IsPrimary          *bool   `json:"is_primary,omitempty"`
	IsEmergencyContact *bool   `json:"is_emergency_contact,omitempty"`
	CanPickup          *bool   `json:"can_pickup,omitempty"`
	PickupNotes        *string `json:"pickup_notes,omitempty"`
	EmergencyPriority  *int    `json:"emergency_priority,omitempty"`
}

// Bind validates the student-guardian update request
func (req *StudentGuardianUpdateRequest) Bind(_ *http.Request) error {
	// All fields are optional for update
	return nil
}

// PhoneNumberCreateRequest represents a request to add a phone number
type PhoneNumberCreateRequest struct {
	PhoneNumber string  `json:"phone_number"`
	PhoneType   string  `json:"phone_type"` // mobile, home, work, other
	Label       *string `json:"label,omitempty"`
	IsPrimary   bool    `json:"is_primary"`
}

// Bind validates the phone number create request
func (req *PhoneNumberCreateRequest) Bind(_ *http.Request) error {
	if req.PhoneNumber == "" {
		return errors.New("phone_number is required")
	}
	// Validate phone type
	validTypes := map[string]bool{"mobile": true, "home": true, "work": true, "other": true}
	if req.PhoneType != "" && !validTypes[req.PhoneType] {
		return errors.New("phone_type must be one of: mobile, home, work, other")
	}
	if req.PhoneType == "" {
		req.PhoneType = "mobile" // Default to mobile
	}
	return nil
}

// PhoneNumberUpdateRequest represents a request to update a phone number
type PhoneNumberUpdateRequest struct {
	PhoneNumber *string `json:"phone_number,omitempty"`
	PhoneType   *string `json:"phone_type,omitempty"`
	Label       *string `json:"label,omitempty"`
	IsPrimary   *bool   `json:"is_primary,omitempty"`
	Priority    *int    `json:"priority,omitempty"`
}

// Bind validates the phone number update request
func (req *PhoneNumberUpdateRequest) Bind(_ *http.Request) error {
	if req.PhoneNumber != nil && *req.PhoneNumber == "" {
		return errors.New("phone_number cannot be empty")
	}
	if req.PhoneType != nil {
		validTypes := map[string]bool{"mobile": true, "home": true, "work": true, "other": true}
		if !validTypes[*req.PhoneType] {
			return errors.New("phone_type must be one of: mobile, home, work, other")
		}
	}
	return nil
}

// StudentWithRelationship represents a student with guardian relationship details
type StudentWithRelationship struct {
	StudentID          int64   `json:"student_id"`
	FirstName          string  `json:"first_name"`
	LastName           string  `json:"last_name"`
	SchoolClass        string  `json:"school_class"`
	RelationshipID     int64   `json:"relationship_id"`
	RelationshipType   string  `json:"relationship_type"`
	GuardianRole       string  `json:"guardian_role"`
	IsPrimary          bool    `json:"is_primary"`
	IsEmergencyContact bool    `json:"is_emergency_contact"`
	CanPickup          bool    `json:"can_pickup"`
	PickupNotes        *string `json:"pickup_notes,omitempty"`
	EmergencyPriority  int     `json:"emergency_priority"`
}

// GuardianWithRelationship represents a guardian with student relationship details
type GuardianWithRelationship struct {
	Guardian           *GuardianResponse `json:"guardian"`
	RelationshipID     int64             `json:"relationship_id"`
	RelationshipType   string            `json:"relationship_type"`
	GuardianRole       string            `json:"guardian_role"`
	IsPrimary          bool              `json:"is_primary"`
	IsEmergencyContact bool              `json:"is_emergency_contact"`
	CanPickup          bool              `json:"can_pickup"`
	PickupNotes        *string           `json:"pickup_notes,omitempty"`
	EmergencyPriority  int               `json:"emergency_priority"`
	// AccountStatus is the portal-access state of this guardian for the staff
	// "Erziehungsberechtigte" tab: "active" (has login), "pending" (invited,
	// not yet accepted), or "none" (info on file, no account, can be invited).
	AccountStatus string `json:"account_status"`
}

// guardianAccountStatus derives the staff-facing account-status string.
func guardianAccountStatus(hasAccount, invitationPending bool) string {
	switch {
	case hasAccount:
		return "active"
	case invitationPending:
		return "pending"
	default:
		return "none"
	}
}

// Bind validates the guardian create request
// Note: Contact method validation (email or phone) is done at the service/handler level
// after phone numbers are added separately
// Note: FirstName and LastName are optional (e.g., CSV imports may only have relationship type)
func (req *GuardianCreateRequest) Bind(_ *http.Request) error {
	return nil
}

// Bind validates the guardian update request
// Note: FirstName and LastName are optional and can be set to empty
func (req *GuardianUpdateRequest) Bind(_ *http.Request) error {
	return nil
}

// Bind validates the student-guardian link request
func (req *StudentGuardianLinkRequest) Bind(_ *http.Request) error {
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

// GuardianPhoneInput is one phone number for a guardian created in the atomic
// add-guardians-to-student request. Plain DTO (no per-element Bind): the service
// layer validates via ValidateNewGuardians.
type GuardianPhoneInput struct {
	PhoneNumber string `json:"phone_number"`
	PhoneType   string `json:"phone_type,omitempty"` // mobile, home, work, other
	Label       string `json:"label,omitempty"`
	IsPrimary   bool   `json:"is_primary,omitempty"`
}

// GuardianWithRelationshipInput is one guardian to attach to an EXISTING student
// in a single atomic request: either a NEW profile (name/contact + phone
// numbers) or an EXISTING one (GuardianProfileID, sibling case), plus the
// relationship flags for THIS student. Mirrors one row of the student detail
// page's guardian form and the GuardianInput used by the student-create flow.
type GuardianWithRelationshipInput struct {
	// GuardianProfileID, when set, links an EXISTING guardian profile instead of
	// creating a new one (sibling case, #1513). The profile/phone fields below
	// are then ignored and the existing profile is never mutated — only the
	// relationship flags apply to the new link.
	GuardianProfileID *int64 `json:"guardian_profile_id,omitempty"`

	// Profile (used only when GuardianProfileID is nil)
	FirstName              string `json:"first_name"`
	LastName               string `json:"last_name"`
	Email                  string `json:"email,omitempty"`
	AddressStreet          string `json:"address_street,omitempty"`
	AddressCity            string `json:"address_city,omitempty"`
	AddressPostalCode      string `json:"address_postal_code,omitempty"`
	PreferredContactMethod string `json:"preferred_contact_method,omitempty"`
	LanguagePreference     string `json:"language_preference,omitempty"`
	Notes                  string `json:"notes,omitempty"`

	// Relationship to the student
	RelationshipType   string `json:"relationship_type"`
	GuardianRole       string `json:"guardian_role,omitempty"`
	IsPrimary          bool   `json:"is_primary,omitempty"`
	IsEmergencyContact bool   `json:"is_emergency_contact,omitempty"`
	CanPickup          bool   `json:"can_pickup,omitempty"`
	PickupNotes        string `json:"pickup_notes,omitempty"`
	EmergencyPriority  int    `json:"emergency_priority,omitempty"`

	PhoneNumbers []GuardianPhoneInput `json:"phone_numbers,omitempty"`
}

// CreateStudentGuardiansRequest is the body of POST /students/{studentId}/guardians/batch.
type CreateStudentGuardiansRequest struct {
	Guardians []GuardianWithRelationshipInput `json:"guardians"`
}

// Bind validates the atomic add-guardians request.
func (req *CreateStudentGuardiansRequest) Bind(_ *http.Request) error {
	if len(req.Guardians) == 0 {
		return errors.New("at least one guardian is required")
	}
	return nil
}

// newGuardianResponse converts a guardian profile model to a response
func newGuardianResponse(profile *users.GuardianProfile) *GuardianResponse {
	response := &GuardianResponse{
		ID:                     profile.ID,
		FirstName:              profile.FirstName,
		LastName:               profile.LastName,
		Email:                  profile.Email,
		AddressStreet:          profile.AddressStreet,
		AddressCity:            profile.AddressCity,
		AddressPostalCode:      profile.AddressPostalCode,
		PreferredContactMethod: profile.PreferredContactMethod,
		LanguagePreference:     profile.LanguagePreference,
		Notes:                  profile.Notes,
		HasAccount:             profile.HasAccount,
		AccountID:              profile.AccountID,
	}

	// Convert phone numbers if present
	if len(profile.PhoneNumbers) > 0 {
		response.PhoneNumbers = make([]*PhoneNumberResponse, 0, len(profile.PhoneNumbers))
		for _, phone := range profile.PhoneNumbers {
			response.PhoneNumbers = append(response.PhoneNumbers, newPhoneNumberResponse(phone))
		}
	}

	return response
}

// newPhoneNumberResponse converts a phone number model to a response
func newPhoneNumberResponse(phone *users.GuardianPhoneNumber) *PhoneNumberResponse {
	return &PhoneNumberResponse{
		ID:          phone.ID,
		PhoneNumber: phone.PhoneNumber,
		PhoneType:   string(phone.PhoneType),
		Label:       phone.Label,
		IsPrimary:   phone.IsPrimary,
		Priority:    phone.Priority,
	}
}

// canModifyStudent checks if the current user can modify a student's guardians
func (rs *Resource) canModifyStudent(ctx context.Context, studentID int64) (bool, error) {
	userPermissions := jwt.PermissionsFromCtx(ctx)

	// Admin users have full access
	if authorize.HasAdminWildcard(userPermissions) {
		return true, nil
	}

	// Get the student
	student, err := rs.PersonService.GetStudentByID(ctx, studentID)
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

// canModifyGuardian checks if the current user can modify a guardian profile
// User can modify if they are admin OR if they supervise at least one student linked to this guardian
func (rs *Resource) canModifyGuardian(ctx context.Context, guardianID int64) (bool, error) {
	userPermissions := jwt.PermissionsFromCtx(ctx)

	// Admin users have full access
	if authorize.HasAdminWildcard(userPermissions) {
		return true, nil
	}

	// Check if user is a staff member
	staff, err := rs.UserContextService.GetCurrentStaff(ctx)
	if err != nil || staff == nil {
		return false, fmt.Errorf("only staff members can modify guardian profiles")
	}

	// Get students linked to this guardian
	studentsWithRel, err := rs.GuardianService.GetGuardianStudents(ctx, guardianID)
	if err != nil {
		return false, fmt.Errorf("failed to get guardian's students")
	}

	// If guardian has no linked students, only admins can modify
	if len(studentsWithRel) == 0 {
		return false, fmt.Errorf("only administrators can modify guardians with no linked students")
	}

	// Check if staff supervises at least one of the guardian's students
	educationGroups, err := rs.UserContextService.GetMyGroups(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get supervised groups")
	}

	// Build map of supervised group IDs for efficient lookup
	supervisedGroupIDs := make(map[int64]bool)
	for _, group := range educationGroups {
		supervisedGroupIDs[group.ID] = true
	}

	// Check if any of the guardian's students are in supervised groups
	for _, studentRel := range studentsWithRel {
		// Get full student details to check their group
		student, err := rs.PersonService.GetStudentByID(ctx, studentRel.Student.ID)
		if err != nil {
			continue
		}

		// Check if this student's group is supervised by current user
		if student.GroupID != nil && supervisedGroupIDs[*student.GroupID] {
			return true, nil
		}
	}

	return false, fmt.Errorf("you can only modify guardians for students in groups you supervise")
}

// listGuardians handles listing all guardians with pagination. This is the
// admin (users:read) full-profile list. The picker search lives in its own
// handler (searchGuardiansForPicker) with a lower gate + minimal projection.
func (rs *Resource) listGuardians(w http.ResponseWriter, r *http.Request) {
	page, pageSize := common.ParsePagination(r)

	queryOptions := base.NewQueryOptions()
	queryOptions.WithPagination(page, pageSize)

	guardians, err := rs.GuardianService.ListGuardians(r.Context(), queryOptions)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]*GuardianResponse, 0, len(guardians))
	for _, guardian := range guardians {
		responses = append(responses, newGuardianResponse(guardian))
	}

	// For now, return without total count (would need separate count query)
	common.RespondPaginated(w, r, http.StatusOK, responses, common.PaginationParams{Page: page, PageSize: pageSize, Total: len(responses)}, "Guardians retrieved successfully")
}

// minGuardianPickerQueryLength is the server-side floor for picker searches. It
// mirrors the client minimum so a non-admin can't enumerate the guardian pool
// one character at a time. It is a fixed data-protection guardrail, not a
// per-school business rule, so it lives here rather than in the settings system.
const minGuardianPickerQueryLength = 3

// maxGuardianPickerResults caps how many matches a single picker search can
// return. common.ParsePagination enforces NO upper bound on page_size, so
// without this clamp a caller could pass ?q=ann&page_size=100000 and pull most
// of the tenant's guardian pool (name + email + linked-children count) in one
// request — defeating the minimal, enumeration-resistant projection. The picker
// is a type-to-narrow lookup; a staff member never needs more than a screenful
// of candidates, so 50 is the hard ceiling regardless of the requested
// page_size. There is no OFFSET: the endpoint deliberately returns only this
// first capped slice, not real pages.
const maxGuardianPickerResults = 50

// searchGuardiansForPicker backs the existing-guardian picker (#1513). It shares
// the users:read gate with the other guardian reads, so anyone who can reach the
// student create/detail flows can link a sibling's already-existing guardian
// instead of creating a duplicate. The projection is deliberately minimal and
// enumeration-resistant — name, email, and only a COUNT of other linked children
// (see GuardianPickerResponse). A query shorter than minGuardianPickerQueryLength
// returns an empty page rather than 400, so the client's "type at least N
// characters" hint stays the single source of truth.
func (rs *Resource) searchGuardiansForPicker(w http.ResponseWriter, r *http.Request) {
	page, pageSize := common.ParsePagination(r)
	search := strings.TrimSpace(r.URL.Query().Get("q"))

	if len([]rune(search)) < minGuardianPickerQueryLength {
		common.RespondPaginated(w, r, http.StatusOK, []*GuardianPickerResponse{}, common.PaginationParams{Page: page, PageSize: pageSize, Total: 0}, "Guardians retrieved successfully")
		return
	}

	// Clamp the caller-supplied page_size to a hard ceiling — ParsePagination
	// itself imposes no maximum.
	limit := pageSize
	if limit > maxGuardianPickerResults {
		limit = maxGuardianPickerResults
	}

	matches, err := rs.GuardianService.SearchGuardiansForPicker(r.Context(), search, limit)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]*GuardianPickerResponse, 0, len(matches))
	for _, m := range matches {
		responses = append(responses, &GuardianPickerResponse{
			ID:                  m.Profile.ID,
			FirstName:           m.Profile.FirstName,
			LastName:            m.Profile.LastName,
			Email:               m.Profile.Email,
			LinkedChildrenCount: len(m.Children),
		})
	}

	// Report the clamped limit as the page size so the envelope reflects what
	// was actually applied, not the (possibly larger) requested page_size. The
	// endpoint returns a single capped slice with no OFFSET — there are no
	// further pages to fetch — so Total is the size of this slice.
	common.RespondPaginated(w, r, http.StatusOK, responses, common.PaginationParams{Page: page, PageSize: limit, Total: len(responses)}, "Guardians retrieved successfully")
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
	isAdmin := authorize.HasAdminWildcard(userPermissions)

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

	// Convert to service request (phone numbers are added separately)
	createReq := guardianSvc.GuardianCreateRequest{
		FirstName:              req.FirstName,
		LastName:               req.LastName,
		Email:                  req.Email,
		AddressStreet:          req.AddressStreet,
		AddressCity:            req.AddressCity,
		AddressPostalCode:      req.AddressPostalCode,
		PreferredContactMethod: req.PreferredContactMethod,
		LanguagePreference:     req.LanguagePreference,
		Notes:                  req.Notes,
	}

	// Create guardian
	var guardian *users.GuardianProfile
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		guardian, txErr = rs.GuardianService.CreateGuardian(ctx, createReq)
		return txErr
	}); err != nil {
		// A duplicate email (or other bad input) comes back as a ValidationError
		// carrying a user-facing German message — render it as a 400 so the
		// frontend surfaces "bereits vergeben – über die Suche auswählen"
		// instead of the generic 500 catch-all. The tenant tx already rolled
		// back, so no partial guardian survives.
		var validationErr *guardianSvc.ValidationError
		if errors.As(err, &validationErr) {
			common.RenderError(w, r, common.ErrorInvalidRequest(validationErr))
			return
		}
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

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.GuardianService.UpdateGuardian(ctx, id, updateReq)
	}); err != nil {
		// A duplicate email comes back as a ValidationError carrying a German
		// message — render it as 400 (mirrors createGuardian), not the 500
		// catch-all. The tenant tx already rolled back.
		var validationErr *guardianSvc.ValidationError
		if errors.As(err, &validationErr) {
			common.RenderError(w, r, common.ErrorInvalidRequest(validationErr))
			return
		}
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
// Note: Phone numbers are managed separately via phone number endpoints
func buildGuardianUpdateRequest(guardian *users.GuardianProfile, req *GuardianUpdateRequest) guardianSvc.GuardianCreateRequest {
	updateReq := guardianSvc.GuardianCreateRequest{
		FirstName:              guardian.FirstName,
		LastName:               guardian.LastName,
		Email:                  guardian.Email,
		AddressStreet:          guardian.AddressStreet,
		AddressCity:            guardian.AddressCity,
		AddressPostalCode:      guardian.AddressPostalCode,
		PreferredContactMethod: guardian.PreferredContactMethod,
		LanguagePreference:     guardian.LanguagePreference,
		Notes:                  guardian.Notes,
	}

	applyGuardianUpdates(&updateReq, req)
	return updateReq
}

// applyGuardianUpdates applies non-nil updates to the request
// Note: Phone numbers are managed separately via phone number endpoints
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
	if req.Notes != nil {
		updateReq.Notes = req.Notes
	}
}

// GuardianDeletePreview is the read-only answer to "what would a full delete of
// this guardian affect?" — the children that would lose the guardian plus a
// ready-to-show German warning. It backs GET /guardians/{id}/delete-preview.
type GuardianDeletePreview struct {
	LinkedCount   int      `json:"linked_count"`
	AffectedNames []string `json:"affected_names"`
	// AffectedLinkIDs are serialized as strings, not JSON numbers: students_guardians.id
	// is an int64 and could exceed JavaScript's safe-integer range, where the frontend
	// would silently round the value before echoing it back as expected_link_ids — making
	// the confirm look like a stale preview. Strings cross the API boundary losslessly
	// (repo convention: int64 IDs are strings to the client).
	AffectedLinkIDs []string `json:"affected_link_ids"`
	Warning         string   `json:"warning"`
}

// stringifyLinkIDs renders int64 link IDs as decimal strings for the API boundary.
func stringifyLinkIDs(ids []int64) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = strconv.FormatInt(id, 10)
	}
	return out
}

// guardianFullDeleteWarning builds the German warning describing the blast
// radius of a deliberate full delete. It is shown in the confirmation step
// (via the delete-preview endpoint) and as the body of the admin 409 a blind
// delete returns. The wording is phrased as a warning about what the full
// delete WOULD do — never as if the deletion has already happened — so it reads
// correctly both as a pre-confirmation hint and as a 409 error body.
func guardianFullDeleteWarning(names []string) string {
	switch len(names) {
	case 0:
		return "Die Person ist mit keinem Kind verknüpft und wird mit dem Profil vollständig gelöscht."
	case 1:
		return "Die Person ist nur mit diesem Kind verknüpft und wird mit dem Profil vollständig gelöscht."
	default:
		return fmt.Sprintf(
			"Die Person ist mit %d Kindern verknüpft und wird bei allen entfernt: %s.",
			len(names), strings.Join(names, ", "))
	}
}

// guardianDeletePreview returns the children a full delete would affect, plus a
// ready-to-show German warning. Read-only: it replaces the old destructive
// "probe DELETE" the frontend used to discover the affected children (#819), so
// opening the full-delete confirmation never deletes anything by itself.
//
// Admin-only, mirroring who may actually perform the full delete — a full
// delete reaches across siblings the caller may not supervise, so non-admins
// have no use for (and must not see) the affected children.
func (rs *Resource) guardianDeletePreview(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidGuardianID)))
		return
	}

	if !authorize.HasAdminWildcard(jwt.PermissionsFromCtx(r.Context())) {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("only administrators can preview a full guardian delete")))
		return
	}

	if _, err := rs.GuardianService.GetGuardianByID(r.Context(), id); err != nil {
		if errors.Is(err, users.ErrGuardianProfileNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("guardian not found")))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	impact, err := rs.GuardianService.GetGuardianDeleteImpact(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, &GuardianDeletePreview{
		LinkedCount:     len(impact.StudentNames),
		AffectedNames:   impact.StudentNames,
		AffectedLinkIDs: stringifyLinkIDs(impact.LinkIDs),
		Warning:         guardianFullDeleteWarning(impact.StudentNames),
	}, "Guardian delete preview retrieved successfully")
}

func parseExpectedGuardianLinkIDs(r *http.Request) ([]int64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("expected_link_ids"))
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			return nil, errors.New("expected_link_ids must contain only numeric IDs")
		}
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return nil, errors.New("expected_link_ids must contain only positive numeric IDs")
		}
		ids = append(ids, id)
	}
	return ids, nil
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

	// "force" requests the deliberate full delete (guardian + all links). Without
	// it, a guardian still linked to students is refused with a 409 listing the
	// affected children — the single-student unlink lives at
	// DELETE /students/{id}/guardians/{gid}.
	force := r.URL.Query().Get("force") == "true"
	isAdmin := authorize.HasAdminWildcard(jwt.PermissionsFromCtx(r.Context()))

	hasLinks, err := rs.GuardianService.EvaluateGuardianDelete(r.Context(), id, force, isAdmin)
	if err != nil {
		rs.renderGuardianDeleteError(w, r, err, isAdmin)
		return
	}

	expectedLinkIDs, err := parseExpectedGuardianLinkIDs(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Delete guardian. With links present (force+admin) remove the links first
	// to satisfy the RESTRICT FK; otherwise a plain delete. Both run in one
	// tenant transaction so a failure leaves guardian and links intact.
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if hasLinks {
			return rs.GuardianService.DeleteGuardianWithLinks(ctx, id, expectedLinkIDs)
		}
		return rs.GuardianService.DeleteGuardian(ctx, id)
	}); err != nil {
		rs.renderGuardianDeleteError(w, r, err, isAdmin)
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Guardian deleted successfully")
}

// renderGuardianDeleteError maps the errors from EvaluateGuardianDelete and the
// delete transaction onto their HTTP responses, preserving the two distinct
// German conflict messages (the admin warning lists the affected children; the
// non-admin one does not).
func (rs *Resource) renderGuardianDeleteError(w http.ResponseWriter, r *http.Request, err error, isAdmin bool) {
	var stillLinked *guardianSvc.GuardianStillLinkedError
	switch {
	case errors.As(err, &stillLinked):
		message := "Erziehungsberechtigte/r kann nicht gelöscht werden: Noch mit Kindern verknüpft"
		if isAdmin {
			message = guardianFullDeleteWarning(stillLinked.StudentNames)
		}
		common.RenderError(w, r, common.ErrorConflictMessage(message))
	case errors.Is(err, guardianSvc.ErrGuardianForceDeleteRequiresAdmin):
		// A full delete reaches across every linked student — including siblings
		// in groups the caller may not supervise. Restrict that blast radius to
		// admins; group supervisors must use the per-student unlink instead.
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, guardianSvc.ErrGuardianDeletePreviewChanged):
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, common.ErrorConflictMessage(err.Error()))
	case common.IsConstraintViolation(err):
		// Safety net: a link added between the check above and the delete trips
		// the RESTRICT FK — surface it as the same 409 rather than a 500.
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, common.ErrorConflictMessage("Erziehungsberechtigte/r kann nicht gelöscht werden: Noch mit Kindern verknüpft"))
	case strings.Contains(err.Error(), "not found"):
		common.RenderError(w, r, common.ErrorNotFound(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
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

// sendInvitation handles sending an invitation to a guardian
func (rs *Resource) sendInvitation(w http.ResponseWriter, r *http.Request) {
	// Parse guardian ID from URL
	guardianID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidGuardianID)))
		return
	}

	// Get current user ID
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("user not authenticated")))
		return
	}
	accountID := int64(claims.ID)

	// Outbox path (#1937). The old GuardianService.SendInvitation sent
	// synchronously and stamped email_sent_at, which is why this endpoint used
	// to claim "gesendet" for mail that had not left the building — and for
	// mail whose enqueue had failed outright.
	var invitation *authModels.GuardianInvitation
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		invitation, txErr = rs.InvitationService.Create(ctx, authSvc.GuardianInvitationCreateRequest{
			GuardianProfileID: guardianID,
			CreatedBy:         accountID,
		})
		return txErr
	}); err != nil {
		if errors.Is(err, authSvc.ErrGuardianInvitationPending) {
			common.RenderError(w, r, common.ErrorConflictMessage("Für diese Person ist bereits eine Einladung offen"))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Return invitation details (without token for security).
	//
	// email_status is "queued", never "sent": at this point the mail is a row
	// in platform.email_outbox. The frontend renders "eingeplant" and polls
	// /invitations/{id}/delivery for what actually happened.
	response := map[string]interface{}{
		"id":                  invitation.ID,
		"guardian_profile_id": invitation.GuardianProfileID,
		"expires_at":          invitation.ExpiresAt,
		"email_status":        "queued",
	}
	if shouldExposeSeedInvitationToken(r) {
		response["token"] = invitation.Token
	}

	common.Respond(w, r, http.StatusCreated, response, "Invitation queued")
}

// getInvitationDeliveryStatus reports what actually happened to the invitation
// emails for one invitation: one entry per dispatch attempt, newest first.
// Tenant isolation is RLS — an invitation from another school yields no
// attempts rather than a leak.
func (rs *Resource) getInvitationDeliveryStatus(w http.ResponseWriter, r *http.Request) {
	invitationID, err := common.ParseIDParam(r, "invitationId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid invitation ID")))
		return
	}

	delivery, err := rs.InvitationService.DeliveryStatus(r.Context(), invitationID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, delivery, "Delivery status retrieved successfully")
}

func shouldExposeSeedInvitationToken(r *http.Request) bool {
	return seedtoken.ShouldExposeInvitationToken(r, viper.GetString("app_env"))
}

// listPendingInvitations handles listing all pending guardian invitations
func (rs *Resource) listPendingInvitations(w http.ResponseWriter, r *http.Request) {
	profileIDs, err := parseDeliveryProfileIDs(r.URL.Query().Get("guardian_profile_ids"))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	var invitations []*authModels.GuardianInvitation
	if len(profileIDs) > 0 {
		invitations, err = rs.GuardianService.GetPendingInvitationsForGuardianProfiles(r.Context(), profileIDs)
	} else {
		invitations, err = rs.GuardianService.GetPendingInvitations(r.Context())
	}
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	invitationIDs := make([]int64, 0, len(invitations))
	for _, inv := range invitations {
		invitationIDs = append(invitationIDs, inv.ID)
	}
	deliveryByInvitation, err := rs.InvitationService.DeliverySummaries(r.Context(), invitationIDs)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response format (without tokens)
	responses := make([]map[string]interface{}, 0, len(invitations))
	for _, inv := range invitations {
		responses = append(responses, map[string]interface{}{
			"id":                  inv.ID,
			"guardian_profile_id": inv.GuardianProfileID,
			"created_at":          inv.CreatedAt,
			"expires_at":          inv.ExpiresAt,
			"email_sent_at":       inv.EmailSentAt,
			"email_error":         inv.EmailError,
			"email_retry_count":   inv.EmailRetryCount,
			"delivery":            deliveryByInvitation[inv.ID],
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Pending invitations retrieved successfully")
}

func parseDeliveryProfileIDs(raw string) ([]int64, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) > maxDeliveryProfileIDs {
		return nil, fmt.Errorf("guardian_profile_ids exceeds %d entries", maxDeliveryProfileIDs)
	}
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || id <= 0 {
			return nil, errors.New("invalid guardian_profile_ids")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// getStudentGuardians handles getting all guardians for a student (PUBLIC - everyone can view for emergency)
func (rs *Resource) getStudentGuardians(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := common.ParseIDParam(r, "studentId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}

	// Get guardians with relationships
	guardiansWithRel, err := rs.GuardianService.GetStudentGuardians(r.Context(), studentID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response format
	responses := make([]*GuardianWithRelationship, 0, len(guardiansWithRel))
	for _, gwr := range guardiansWithRel {
		responses = append(responses, &GuardianWithRelationship{
			Guardian:           newGuardianResponse(gwr.Profile),
			RelationshipID:     gwr.Relationship.ID,
			RelationshipType:   gwr.Relationship.RelationshipType,
			GuardianRole:       gwr.Relationship.GuardianRole,
			IsPrimary:          gwr.Relationship.IsPrimary,
			IsEmergencyContact: gwr.Relationship.IsEmergencyContact,
			CanPickup:          gwr.Relationship.CanPickup,
			PickupNotes:        gwr.Relationship.PickupNotes,
			EmergencyPriority:  gwr.Relationship.EmergencyPriority,
			AccountStatus:      guardianAccountStatus(gwr.Profile.HasAccount, gwr.InvitationPending),
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Student guardians retrieved successfully")
}

// getGuardianStudents handles getting all students for a guardian
func (rs *Resource) getGuardianStudents(w http.ResponseWriter, r *http.Request) {
	// Parse guardian ID from URL
	guardianID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidGuardianID)))
		return
	}

	// Get students with relationships
	studentsWithRel, err := rs.GuardianService.GetGuardianStudents(r.Context(), guardianID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response format
	responses := make([]*StudentWithRelationship, 0, len(studentsWithRel))
	for _, swr := range studentsWithRel {
		// Get person data for student
		person, err := rs.PersonService.Get(r.Context(), swr.Student.PersonID)
		if err != nil {
			slog.Default().Error("failed to get person for student",
				slog.Int64("student_id", swr.Student.ID),
				slog.String("error", err.Error()))
			continue
		}

		responses = append(responses, &StudentWithRelationship{
			StudentID:          swr.Student.ID,
			FirstName:          person.FirstName,
			LastName:           person.LastName,
			SchoolClass:        swr.Student.SchoolClass,
			RelationshipID:     swr.Relationship.ID,
			RelationshipType:   swr.Relationship.RelationshipType,
			GuardianRole:       swr.Relationship.GuardianRole,
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
	studentID, err := common.ParseIDParam(r, "studentId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}

	// Check permissions - only supervisors of the student's group can link guardians
	canModify, err := rs.canModifyStudent(r.Context(), studentID)
	if !canModify {
		common.RenderError(w, r, common.ErrorForbidden(err))
		return
	}

	// Parse request
	req := &StudentGuardianLinkRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Convert to service request
	linkReq := guardianSvc.StudentGuardianCreateRequest{
		StudentID:          studentID,
		GuardianProfileID:  req.GuardianProfileID,
		RelationshipType:   req.RelationshipType,
		GuardianRole:       req.GuardianRole,
		IsPrimary:          req.IsPrimary,
		IsEmergencyContact: req.IsEmergencyContact,
		CanPickup:          req.CanPickup,
		PickupNotes:        req.PickupNotes,
		EmergencyPriority:  req.EmergencyPriority,
	}

	// Link guardian to student
	var relationship *users.StudentGuardian
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		relationship, txErr = rs.GuardianService.LinkGuardianToStudent(ctx, linkReq)
		return txErr
	}); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, relationship, "Guardian linked to student successfully")
}

// ToNewStudentGuardians maps the request DTOs onto the service input used by
// GuardianService.AddGuardiansToStudent. Shared with api/students, whose
// student-create flow accepts the same guardian rows.
func ToNewStudentGuardians(inputs []GuardianWithRelationshipInput) []guardianSvc.NewStudentGuardian {
	out := make([]guardianSvc.NewStudentGuardian, 0, len(inputs))
	for i := range inputs {
		in := inputs[i]
		out = append(out, guardianSvc.NewStudentGuardian{
			Profile: guardianSvc.GuardianCreateRequest{
				FirstName:              strings.TrimSpace(in.FirstName),
				LastName:               strings.TrimSpace(in.LastName),
				Email:                  strutil.TrimToNil(in.Email),
				AddressStreet:          strutil.TrimToNil(in.AddressStreet),
				AddressCity:            strutil.TrimToNil(in.AddressCity),
				AddressPostalCode:      strutil.TrimToNil(in.AddressPostalCode),
				PreferredContactMethod: in.PreferredContactMethod,
				LanguagePreference:     in.LanguagePreference,
				Notes:                  strutil.TrimToNil(in.Notes),
			},
			Relationship: guardianSvc.StudentGuardianRelationship{
				RelationshipType:   in.RelationshipType,
				GuardianRole:       in.GuardianRole,
				IsPrimary:          in.IsPrimary,
				IsEmergencyContact: in.IsEmergencyContact,
				CanPickup:          in.CanPickup,
				PickupNotes:        strutil.TrimToNil(in.PickupNotes),
				EmergencyPriority:  in.EmergencyPriority,
			},
			PhoneNumbers:      toPhoneCreateRequests(in.PhoneNumbers),
			ExistingProfileID: in.GuardianProfileID,
		})
	}
	return out
}

// toPhoneCreateRequests maps phone DTOs onto the service phone-number requests.
func toPhoneCreateRequests(phones []GuardianPhoneInput) []guardianSvc.PhoneNumberCreateRequest {
	if len(phones) == 0 {
		return nil
	}
	out := make([]guardianSvc.PhoneNumberCreateRequest, 0, len(phones))
	for i := range phones {
		p := phones[i]
		out = append(out, guardianSvc.PhoneNumberCreateRequest{
			PhoneNumber: strings.TrimSpace(p.PhoneNumber),
			PhoneType:   p.PhoneType,
			Label:       strutil.TrimToNil(p.Label),
			IsPrimary:   p.IsPrimary,
		})
	}
	return out
}

// createStudentGuardians atomically creates (or links) one or more guardians for
// an EXISTING student in a single tenant transaction. It is the server-side
// replacement for the frontend's old create→link→add-phones sequence: any
// failure rolls the whole transaction back, so a partially-created guardian can
// never be orphaned — and there is no client-side compensating delete to
// authorize (which a non-admin supervisor could not perform once the guardian
// had no remaining links). See #819.
//
// Auth mirrors the single-link endpoint (linkGuardianToStudent): only
// supervisors of the student's group, or admins, may attach guardians. That one
// gate is sufficient and grants no extra reach — the endpoint only ever links to
// the {studentId} in the path, and creating the profile + linking it + adding
// its phones are all writes scoped to that student's guardian, exactly what
// "may I modify this student's guardians?" already authorizes.
func (rs *Resource) createStudentGuardians(w http.ResponseWriter, r *http.Request) {
	studentID, err := common.ParseIDParam(r, "studentId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}

	canModify, err := rs.canModifyStudent(r.Context(), studentID)
	if !canModify {
		common.RenderError(w, r, common.ErrorForbidden(err))
		return
	}

	req := &CreateStudentGuardiansRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	guardians := ToNewStudentGuardians(req.Guardians)

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.GuardianService.AddGuardiansToStudent(ctx, studentID, guardians)
	}); err != nil {
		// AddGuardiansToStudent validates before any write, so a ValidationError
		// means nothing was persisted; the explicit tenant tx also rolls back on
		// any other error. Surface bad input as 400, everything else as 500.
		var validationErr *guardianSvc.ValidationError
		if errors.As(err, &validationErr) {
			common.RenderError(w, r, common.ErrorInvalidRequest(validationErr))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, nil, "Guardians added successfully")
}

// updateStudentGuardianRelationship handles updating a student-guardian relationship (SUPERVISOR only)
func (rs *Resource) updateStudentGuardianRelationship(w http.ResponseWriter, r *http.Request) {
	// Parse relationship ID from URL
	relationshipID, err := common.ParseIDParam(r, "relationshipId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid relationship ID")))
		return
	}

	// Get the relationship to find the student ID
	relationship, err := rs.GuardianService.GetStudentGuardianRelationship(r.Context(), relationshipID)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("relationship not found")))
		return
	}

	// Check permissions - only supervisors of the student's group can update relationships
	canModify, err := rs.canModifyStudent(r.Context(), relationship.StudentID)
	if !canModify {
		common.RenderError(w, r, common.ErrorForbidden(err))
		return
	}

	// Parse request
	req := &StudentGuardianUpdateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Convert to service request
	updateReq := guardianSvc.StudentGuardianUpdateRequest{
		RelationshipType:   req.RelationshipType,
		GuardianRole:       req.GuardianRole,
		IsPrimary:          req.IsPrimary,
		IsEmergencyContact: req.IsEmergencyContact,
		CanPickup:          req.CanPickup,
		PickupNotes:        req.PickupNotes,
		EmergencyPriority:  req.EmergencyPriority,
	}

	// Update relationship
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.GuardianService.UpdateStudentGuardianRelationship(ctx, relationshipID, updateReq)
	}); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Relationship updated successfully")
}

// removeGuardianFromStudent handles removing a guardian from a student (SUPERVISOR only)
func (rs *Resource) removeGuardianFromStudent(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := common.ParseIDParam(r, "studentId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}

	// Parse guardian ID from URL
	guardianID, err := common.ParseIDParam(r, "guardianId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidGuardianID)))
		return
	}

	// Check permissions - only supervisors of the student's group can remove guardians
	canModify, err := rs.canModifyStudent(r.Context(), studentID)
	if !canModify {
		common.RenderError(w, r, common.ErrorForbidden(err))
		return
	}

	// Remove guardian from student
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.GuardianService.RemoveGuardianFromStudent(ctx, studentID, guardianID)
	}); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Guardian removed from student successfully")
}

// =============================================================================
// PHONE NUMBER HANDLERS
// =============================================================================

// validatePhoneAccess validates guardian ID, phone ID, permissions, and ownership.
// Returns the validated phone number or renders an error response and returns nil.
func (rs *Resource) validatePhoneAccess(w http.ResponseWriter, r *http.Request) *users.GuardianPhoneNumber {
	guardianID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidGuardianID)))
		return nil
	}

	phoneID, err := common.ParseIDParam(r, "phoneId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidPhoneID)))
		return nil
	}

	// Check permissions
	canModify, err := rs.canModifyGuardian(r.Context(), guardianID)
	if !canModify {
		common.RenderError(w, r, common.ErrorForbidden(err))
		return nil
	}

	// Verify phone belongs to guardian
	phone, err := rs.GuardianService.GetPhoneNumberByID(r.Context(), phoneID)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New(errPhoneNotFound)))
		return nil
	}
	if phone.GuardianProfileID != guardianID {
		common.RenderError(w, r, common.ErrorForbidden(errors.New(errPhoneNotBelongsGuard)))
		return nil
	}

	return phone
}

// listGuardianPhoneNumbers handles getting all phone numbers for a guardian
func (rs *Resource) listGuardianPhoneNumbers(w http.ResponseWriter, r *http.Request) {
	guardianID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidGuardianID)))
		return
	}

	phones, err := rs.GuardianService.GetGuardianPhoneNumbers(r.Context(), guardianID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]*PhoneNumberResponse, 0, len(phones))
	for _, phone := range phones {
		responses = append(responses, newPhoneNumberResponse(phone))
	}

	common.Respond(w, r, http.StatusOK, responses, "Phone numbers retrieved successfully")
}

// addPhoneNumber handles adding a new phone number to a guardian
func (rs *Resource) addPhoneNumber(w http.ResponseWriter, r *http.Request) {
	guardianID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidGuardianID)))
		return
	}

	// Check permissions
	canModify, err := rs.canModifyGuardian(r.Context(), guardianID)
	if !canModify {
		common.RenderError(w, r, common.ErrorForbidden(err))
		return
	}

	req := &PhoneNumberCreateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	createReq := guardianSvc.PhoneNumberCreateRequest{
		PhoneNumber: req.PhoneNumber,
		PhoneType:   req.PhoneType,
		Label:       req.Label,
		IsPrimary:   req.IsPrimary,
	}

	var phone *users.GuardianPhoneNumber
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		phone, txErr = rs.GuardianService.AddPhoneNumber(ctx, guardianID, createReq)
		return txErr
	}); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newPhoneNumberResponse(phone), "Phone number added successfully")
}

// updatePhoneNumber handles updating an existing phone number
func (rs *Resource) updatePhoneNumber(w http.ResponseWriter, r *http.Request) {
	phone := rs.validatePhoneAccess(w, r)
	if phone == nil {
		return // Error already rendered
	}

	req := &PhoneNumberUpdateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	updateReq := guardianSvc.PhoneNumberUpdateRequest{
		PhoneNumber: req.PhoneNumber,
		PhoneType:   req.PhoneType,
		Label:       req.Label,
		IsPrimary:   req.IsPrimary,
		Priority:    req.Priority,
	}

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.GuardianService.UpdatePhoneNumber(ctx, phone.ID, updateReq)
	}); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Get updated phone
	updatedPhone, err := rs.GuardianService.GetPhoneNumberByID(r.Context(), phone.ID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPhoneNumberResponse(updatedPhone), "Phone number updated successfully")
}

// deletePhoneNumber handles removing a phone number
func (rs *Resource) deletePhoneNumber(w http.ResponseWriter, r *http.Request) {
	phone := rs.validatePhoneAccess(w, r)
	if phone == nil {
		return // Error already rendered
	}

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.GuardianService.DeletePhoneNumber(ctx, phone.ID)
	}); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Phone number deleted successfully")
}

// setPrimaryPhone handles setting a phone number as primary
func (rs *Resource) setPrimaryPhone(w http.ResponseWriter, r *http.Request) {
	phone := rs.validatePhoneAccess(w, r)
	if phone == nil {
		return // Error already rendered
	}

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.GuardianService.SetPrimaryPhone(ctx, phone.ID)
	}); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Fetch the updated phone number to return in response
	updatedPhone, err := rs.GuardianService.GetPhoneNumberByID(r.Context(), phone.ID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newPhoneNumberResponse(updatedPhone), "Phone number set as primary successfully")
}
