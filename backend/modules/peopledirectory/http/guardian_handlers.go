package users

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// PhoneNumberResponse is one guardian phone number on the wire.
type PhoneNumberResponse struct {
	ID          int64   `json:"id"`
	PhoneNumber string  `json:"phone_number"`
	PhoneType   string  `json:"phone_type"`
	Label       *string `json:"label,omitempty"`
	IsPrimary   bool    `json:"is_primary"`
	Priority    int     `json:"priority"`
}

// GuardianResponse is the full guardian profile.
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

// GuardianPickerResponse is the minimal, GDPR-safe projection of the picker
// search (#1513): only a COUNT of linked children, never their names, and
// none of the address, notes or preference fields.
type GuardianPickerResponse struct {
	ID                  int64   `json:"id"`
	FirstName           string  `json:"first_name"`
	LastName            string  `json:"last_name"`
	Email               *string `json:"email,omitempty"`
	LinkedChildrenCount int     `json:"linked_children_count"`
}

// GuardianCreateRequest creates a profile; phone numbers are added through
// the phone-number routes. Names are optional (imports may carry only a
// relationship type).
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

func (req *GuardianCreateRequest) Bind(_ *http.Request) error { return nil }

// GuardianUpdateRequest is a partial update; nil keeps the stored value.
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

func (req *GuardianUpdateRequest) Bind(_ *http.Request) error { return nil }

// StudentGuardianLinkRequest links an existing guardian to a child.
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

// StudentGuardianUpdateRequest changes the relationship flags; every field
// is optional.
type StudentGuardianUpdateRequest struct {
	RelationshipType   *string `json:"relationship_type,omitempty"`
	GuardianRole       *string `json:"guardian_role,omitempty"`
	IsPrimary          *bool   `json:"is_primary,omitempty"`
	IsEmergencyContact *bool   `json:"is_emergency_contact,omitempty"`
	CanPickup          *bool   `json:"can_pickup,omitempty"`
	PickupNotes        *string `json:"pickup_notes,omitempty"`
	EmergencyPriority  *int    `json:"emergency_priority,omitempty"`
}

func (req *StudentGuardianUpdateRequest) Bind(_ *http.Request) error { return nil }

// GuardianLinkResponse is one students_guardians row on the wire. It keeps
// the field names of the persisted row the link endpoint used to echo.
type GuardianLinkResponse struct {
	ID                 int64           `json:"id"`
	TenantID           int64           `json:"tenant_id"`
	StudentID          int64           `json:"student_id"`
	GuardianProfileID  int64           `json:"guardian_profile_id"`
	RelationshipType   string          `json:"relationship_type"`
	GuardianRole       string          `json:"guardian_role"`
	IsPrimary          bool            `json:"is_primary"`
	IsEmergencyContact bool            `json:"is_emergency_contact"`
	CanPickup          bool            `json:"can_pickup"`
	PickupNotes        *string         `json:"pickup_notes,omitempty"`
	EmergencyPriority  int             `json:"emergency_priority"`
	IsPayer            bool            `json:"is_payer"`
	Permissions        map[string]bool `json:"permissions,omitempty"`
}

var validPhoneTypes = map[string]bool{"mobile": true, "home": true, "work": true, "other": true}

// PhoneNumberCreateRequest adds a phone number; the type defaults to mobile.
type PhoneNumberCreateRequest struct {
	PhoneNumber string  `json:"phone_number"`
	PhoneType   string  `json:"phone_type"`
	Label       *string `json:"label,omitempty"`
	IsPrimary   bool    `json:"is_primary"`
}

func (req *PhoneNumberCreateRequest) Bind(_ *http.Request) error {
	if req.PhoneNumber == "" {
		return errors.New("phone_number is required")
	}
	if req.PhoneType != "" && !validPhoneTypes[req.PhoneType] {
		return errors.New("phone_type must be one of: mobile, home, work, other")
	}
	if req.PhoneType == "" {
		req.PhoneType = "mobile"
	}
	return nil
}

// PhoneNumberUpdateRequest changes a phone number; every field is optional.
type PhoneNumberUpdateRequest struct {
	PhoneNumber *string `json:"phone_number,omitempty"`
	PhoneType   *string `json:"phone_type,omitempty"`
	Label       *string `json:"label,omitempty"`
	IsPrimary   *bool   `json:"is_primary,omitempty"`
	Priority    *int    `json:"priority,omitempty"`
}

func (req *PhoneNumberUpdateRequest) Bind(_ *http.Request) error {
	if req.PhoneNumber != nil && *req.PhoneNumber == "" {
		return errors.New("phone_number cannot be empty")
	}
	if req.PhoneType != nil && !validPhoneTypes[*req.PhoneType] {
		return errors.New("phone_type must be one of: mobile, home, work, other")
	}
	return nil
}

// StudentWithRelationship is one child of a guardian.
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

// GuardianWithRelationship is one guardian of a child. IsPayer is only
// shown to callers with the financial permission; AccountStatus is the
// portal-access state for the staff tab: active, active_no_access, pending
// or none.
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
	IsPayer            bool              `json:"is_payer"`
	AccountStatus      string            `json:"account_status"`
}

// guardianAccountStatus derives the staff-facing account status. An open
// invitation outranks "active_no_access": a pending role-upgrade request
// must read as pending, not as an actionable no-access state (#2172).
func guardianAccountStatus(hasAccount, hasPortalAccess, invitationPending bool) string {
	switch {
	case hasAccount && hasPortalAccess:
		return "active"
	case invitationPending:
		return "pending"
	case hasAccount:
		return "active_no_access"
	default:
		return "none"
	}
}

// CreateStudentGuardiansRequest is the body of POST /students/{studentId}/guardians/batch.
type CreateStudentGuardiansRequest struct {
	Guardians []peopledirectory.NewStudentGuardian `json:"guardians"`
}

func (req *CreateStudentGuardiansRequest) Bind(_ *http.Request) error {
	if len(req.Guardians) == 0 {
		return errors.New("at least one guardian is required")
	}
	return nil
}

// GuardianDeletePreview backs GET /guardians/{id}/delete-preview. Link IDs
// travel as strings so JavaScript never rounds them.
type GuardianDeletePreview struct {
	LinkedCount     int      `json:"linked_count"`
	AffectedNames   []string `json:"affected_names"`
	AffectedLinkIDs []string `json:"affected_link_ids"`
	Warning         string   `json:"warning"`
}

func newGuardianResponse(guardian peopledirectory.Guardian) *GuardianResponse {
	response := &GuardianResponse{
		ID: guardian.ID, FirstName: guardian.FirstName, LastName: guardian.LastName, Email: guardian.Email,
		AddressStreet: guardian.AddressStreet, AddressCity: guardian.AddressCity, AddressPostalCode: guardian.AddressPostalCode,
		PreferredContactMethod: guardian.PreferredContactMethod, LanguagePreference: guardian.LanguagePreference,
		Notes: guardian.Notes, HasAccount: guardian.HasAccount, AccountID: guardian.AccountID,
	}
	if len(guardian.PhoneNumbers) > 0 {
		response.PhoneNumbers = make([]*PhoneNumberResponse, 0, len(guardian.PhoneNumbers))
		for _, phone := range guardian.PhoneNumbers {
			response.PhoneNumbers = append(response.PhoneNumbers, newPhoneNumberResponse(phone))
		}
	}
	return response
}

func newGuardianResponses(guardians []peopledirectory.Guardian) []*GuardianResponse {
	responses := make([]*GuardianResponse, 0, len(guardians))
	for _, guardian := range guardians {
		responses = append(responses, newGuardianResponse(guardian))
	}
	return responses
}

func newPhoneNumberResponse(phone peopledirectory.GuardianPhone) *PhoneNumberResponse {
	return &PhoneNumberResponse{
		ID: phone.ID, PhoneNumber: phone.PhoneNumber, PhoneType: phone.PhoneType,
		Label: phone.Label, IsPrimary: phone.IsPrimary, Priority: phone.Priority,
	}
}

func newGuardianLinkResponse(link peopledirectory.GuardianLink) *GuardianLinkResponse {
	response := &GuardianLinkResponse{
		ID: link.ID, TenantID: link.TenantID, StudentID: link.StudentID, GuardianProfileID: link.GuardianProfileID,
		RelationshipType: link.RelationshipType, GuardianRole: link.GuardianRole,
		IsPrimary: link.IsPrimary, IsEmergencyContact: link.IsEmergencyContact, CanPickup: link.CanPickup,
		PickupNotes: link.PickupNotes, EmergencyPriority: link.EmergencyPriority, IsPayer: link.IsPayer,
	}
	if len(link.Permissions) > 0 {
		response.Permissions = make(map[string]bool, len(link.Permissions))
		for _, name := range link.Permissions {
			response.Permissions[name] = true
		}
	}
	return response
}

// --- profile handlers ---

// listGuardians is the admin full-profile list; the picker search lives in
// its own handler with a minimal projection.
func (rs *GuardianResource) listGuardians(w http.ResponseWriter, r *http.Request) {
	page, pageSize := rs.runtime.ParsePagination(r)
	guardians, err := rs.directory.ListGuardians(r.Context(), page, pageSize)
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	responses := newGuardianResponses(guardians)
	// The total is the page length; the legacy list never counted.
	rs.runtime.SuccessPaginated(w, r, http.StatusOK, responses, Pagination{Page: page, PageSize: pageSize, Total: len(responses)}, "Guardians retrieved successfully")
	rs.runtime.ObserveResponse(http.StatusOK, "none")
}

// minGuardianPickerQueryLength is the server-side floor for picker searches
// so a non-admin cannot enumerate the guardian pool one character at a
// time. A shorter query returns an empty page, not a 400.
const minGuardianPickerQueryLength = 3

// maxGuardianPickerResults is the hard ceiling of one picker search: the
// picker is a type-to-narrow lookup, never real pages, so there is no OFFSET
// and the requested page_size is clamped.
const maxGuardianPickerResults = 50

func (rs *GuardianResource) searchGuardiansForPicker(w http.ResponseWriter, r *http.Request) {
	page, pageSize := rs.runtime.ParsePagination(r)
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(search)) < minGuardianPickerQueryLength {
		rs.runtime.SuccessPaginated(w, r, http.StatusOK, []*GuardianPickerResponse{}, Pagination{Page: page, PageSize: pageSize, Total: 0}, "Guardians retrieved successfully")
		rs.runtime.ObserveResponse(http.StatusOK, "none")
		return
	}
	limit := min(pageSize, maxGuardianPickerResults)
	matches, err := rs.directory.SearchGuardians(r.Context(), search, limit)
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	responses := make([]*GuardianPickerResponse, 0, len(matches))
	for _, match := range matches {
		responses = append(responses, &GuardianPickerResponse{
			ID: match.Guardian.ID, FirstName: match.Guardian.FirstName, LastName: match.Guardian.LastName,
			Email: match.Guardian.Email, LinkedChildrenCount: match.LinkedChildrenCount,
		})
	}
	// The envelope reports the clamped limit, the size that was applied.
	rs.runtime.SuccessPaginated(w, r, http.StatusOK, responses, Pagination{Page: page, PageSize: limit, Total: len(responses)}, "Guardians retrieved successfully")
	rs.runtime.ObserveResponse(http.StatusOK, "none")
}

func (rs *GuardianResource) getGuardian(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parseGuardianID(w, r)
	if !ok {
		return
	}
	guardian, err := rs.directory.FindGuardian(r.Context(), id)
	if err != nil {
		rs.guardianLookupFailure(w, r, err)
		return
	}
	rs.succeed(w, r, http.StatusOK, newGuardianResponse(guardian), "Guardian retrieved successfully")
}

// guardianLookupFailure renders a missing guardian as the historical 404
// message and everything else through the module classification.
func (rs *GuardianResource) guardianLookupFailure(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, peopledirectory.ErrGuardianNotFound) {
		rs.failMessage(w, r, FailureNotFound, msgGuardianNotFound)
		return
	}
	rs.moduleFailure(w, r, err)
}

func (rs *GuardianResource) createGuardian(w http.ResponseWriter, r *http.Request) {
	// Admins create freely; everyone else must be verified staff (#2329).
	if !rs.runtime.IsAdmin(r) && !rs.runtime.IsVerifiedStaff(r.Context()) {
		rs.failMessage(w, r, FailureForbidden, "only staff members can create guardian profiles")
		return
	}
	req := &GuardianCreateRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.fail(w, r, FailureInvalidRequest, err)
		return
	}
	guardian, err := rs.directory.CreateGuardian(r.Context(), peopledirectory.GuardianInput{
		FirstName: req.FirstName, LastName: req.LastName, Email: req.Email,
		AddressStreet: req.AddressStreet, AddressCity: req.AddressCity, AddressPostalCode: req.AddressPostalCode,
		PreferredContactMethod: req.PreferredContactMethod, LanguagePreference: req.LanguagePreference, Notes: req.Notes,
	})
	if err != nil {
		// A duplicate e-mail (or other bad input) is a 400 carrying the German
		// message; the tenant transaction has already rolled back.
		rs.moduleFailure(w, r, err)
		return
	}
	rs.succeed(w, r, http.StatusCreated, newGuardianResponse(guardian), "Guardian created successfully")
}

func (rs *GuardianResource) updateGuardian(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parseGuardianID(w, r)
	if !ok {
		return
	}
	if canModify, err := rs.canModifyGuardian(r, id); !canModify {
		rs.fail(w, r, FailureForbidden, err)
		return
	}
	req := &GuardianUpdateRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.fail(w, r, FailureInvalidRequest, err)
		return
	}
	existing, err := rs.directory.FindGuardian(r.Context(), id)
	if err != nil {
		rs.guardianLookupFailure(w, r, err)
		return
	}
	if err := rs.directory.UpdateGuardian(r.Context(), id, mergeGuardianUpdate(existing, req)); err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	updated, err := rs.directory.FindGuardian(r.Context(), id)
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	rs.succeed(w, r, http.StatusOK, newGuardianResponse(updated), "Guardian updated successfully")
}

// mergeGuardianUpdate applies the non-nil fields of the partial update onto
// the stored profile; phone numbers are managed separately.
func mergeGuardianUpdate(existing peopledirectory.Guardian, req *GuardianUpdateRequest) peopledirectory.GuardianInput {
	input := peopledirectory.GuardianInput{
		FirstName: existing.FirstName, LastName: existing.LastName, Email: existing.Email,
		AddressStreet: existing.AddressStreet, AddressCity: existing.AddressCity, AddressPostalCode: existing.AddressPostalCode,
		PreferredContactMethod: existing.PreferredContactMethod, LanguagePreference: existing.LanguagePreference, Notes: existing.Notes,
	}
	if req.FirstName != nil {
		input.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		input.LastName = *req.LastName
	}
	if req.Email != nil {
		input.Email = req.Email
	}
	if req.AddressStreet != nil {
		input.AddressStreet = req.AddressStreet
	}
	if req.AddressCity != nil {
		input.AddressCity = req.AddressCity
	}
	if req.AddressPostalCode != nil {
		input.AddressPostalCode = req.AddressPostalCode
	}
	if req.PreferredContactMethod != nil {
		input.PreferredContactMethod = *req.PreferredContactMethod
	}
	if req.LanguagePreference != nil {
		input.LanguagePreference = *req.LanguagePreference
	}
	if req.Notes != nil {
		input.Notes = req.Notes
	}
	return input
}

// guardianFullDeleteWarning describes what a full delete WOULD do, so it
// reads correctly both as the preview hint and as the admin 409 body.
func guardianFullDeleteWarning(names []string) string {
	switch len(names) {
	case 0:
		return "Die Person ist mit keinem Kind verknüpft und wird mit dem Profil vollständig gelöscht."
	case 1:
		return "Die Person ist nur mit diesem Kind verknüpft und wird mit dem Profil vollständig gelöscht."
	default:
		return fmt.Sprintf("Die Person ist mit %d Kindern verknüpft und wird bei allen entfernt: %s.", len(names), strings.Join(names, ", "))
	}
}

func stringifyLinkIDs(ids []int64) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = strconv.FormatInt(id, 10)
	}
	return out
}

// guardianDeletePreview is read-only and admin-only: a full delete reaches
// across siblings the caller may not supervise (#819).
func (rs *GuardianResource) guardianDeletePreview(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parseGuardianID(w, r)
	if !ok {
		return
	}
	if !rs.runtime.IsAdmin(r) {
		rs.failMessage(w, r, FailureForbidden, "only administrators can preview a full guardian delete")
		return
	}
	if _, err := rs.directory.FindGuardian(r.Context(), id); err != nil {
		rs.guardianLookupFailure(w, r, err)
		return
	}
	impact, err := rs.directory.GuardianDeleteImpact(r.Context(), id)
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	rs.succeed(w, r, http.StatusOK, &GuardianDeletePreview{
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

// deleteGuardian removes a guardian. "force" requests the deliberate full
// delete (guardian plus every link); without it a linked guardian is refused
// with a 409 listing the affected children. The single-student unlink lives
// at DELETE /students/{id}/guardians/{gid}.
func (rs *GuardianResource) deleteGuardian(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parseGuardianID(w, r)
	if !ok {
		return
	}
	if canModify, err := rs.canModifyGuardian(r, id); !canModify {
		rs.fail(w, r, FailureForbidden, err)
		return
	}
	force := r.URL.Query().Get("force") == "true"
	isAdmin := rs.runtime.IsAdmin(r)
	hasLinks, err := rs.directory.EvaluateGuardianDelete(r.Context(), id, force, isAdmin)
	if err != nil {
		rs.renderGuardianDeleteError(w, r, err, isAdmin)
		return
	}
	expectedLinkIDs, err := parseExpectedGuardianLinkIDs(r)
	if err != nil {
		rs.fail(w, r, FailureInvalidRequest, err)
		return
	}
	accountID, ok := rs.actingAccountID(w, r)
	if !ok {
		return
	}
	if err := rs.directory.DeleteGuardian(r.Context(), peopledirectory.GuardianDelete{
		GuardianID: id, ActorAccountID: accountID, WithLinks: hasLinks, ExpectedLinkIDs: expectedLinkIDs,
	}); err != nil {
		rs.renderGuardianDeleteError(w, r, err, isAdmin)
		return
	}
	rs.succeed(w, r, http.StatusOK, nil, "Guardian deleted successfully")
}

// renderGuardianDeleteError keeps the two distinct German conflict messages:
// the admin warning lists the affected children, the non-admin one does not.
func (rs *GuardianResource) renderGuardianDeleteError(w http.ResponseWriter, r *http.Request, err error, isAdmin bool) {
	var stillLinked *peopledirectory.GuardianStillLinkedError
	switch {
	case errors.As(err, &stillLinked):
		message := msgGuardianStillLinked
		if isAdmin {
			message = guardianFullDeleteWarning(stillLinked.StudentNames)
		}
		rs.failMessage(w, r, FailureConflict, message)
	case errors.Is(err, peopledirectory.ErrGuardianForceDeleteRequiresAdmin):
		rs.fail(w, r, FailureForbidden, err)
	case errors.Is(err, peopledirectory.ErrGuardianDeletePreviewChanged):
		rs.runtime.MarkRollback(r.Context())
		rs.failMessage(w, r, FailureConflict, peopledirectory.ErrGuardianDeletePreviewChanged.Error())
	case errors.Is(err, peopledirectory.ErrGuardianLinkConflict):
		// A link added between the check and the delete trips the RESTRICT
		// constraint; surface it as the same 409 rather than a 500.
		rs.runtime.MarkRollback(r.Context())
		rs.failMessage(w, r, FailureConflict, msgGuardianStillLinked)
	case errors.Is(err, peopledirectory.ErrGuardianNotFound):
		rs.fail(w, r, FailureNotFound, err)
	default:
		rs.fail(w, r, FailureInternal, err)
	}
}

func (rs *GuardianResource) listGuardiansWithoutAccount(w http.ResponseWriter, r *http.Request) {
	guardians, err := rs.directory.ListGuardiansWithoutAccount(r.Context())
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	rs.succeed(w, r, http.StatusOK, newGuardianResponses(guardians), "Guardians without accounts retrieved successfully")
}

func (rs *GuardianResource) listInvitableGuardians(w http.ResponseWriter, r *http.Request) {
	guardians, err := rs.directory.ListInvitableGuardians(r.Context())
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	rs.succeed(w, r, http.StatusOK, newGuardianResponses(guardians), "Invitable guardians retrieved successfully")
}

// sendInvitation is the staff-initiated per-guardian invitation. The raw
// token is exposed to the local seeder only.
func (rs *GuardianResource) sendInvitation(w http.ResponseWriter, r *http.Request) {
	guardianID, ok := rs.parseGuardianID(w, r)
	if !ok {
		return
	}
	accountID, ok := rs.actingAccountID(w, r)
	if !ok {
		return
	}
	invitation, err := rs.runtime.SendInvitation(r.Context(), guardianID, accountID)
	if err != nil {
		rs.fail(w, r, FailureInternal, err)
		return
	}
	response := map[string]any{
		"id":                  invitation.ID,
		"guardian_profile_id": invitation.GuardianProfileID,
		"expires_at":          invitation.ExpiresAt,
		"email_sent":          invitation.EmailSent,
	}
	if rs.runtime.ExposeInvitationToken(r) {
		response["token"] = invitation.Token
	}
	rs.succeed(w, r, http.StatusCreated, response, "Invitation sent successfully")
}

func (rs *GuardianResource) listPendingInvitations(w http.ResponseWriter, r *http.Request) {
	invitations, err := rs.runtime.ListPendingInvitations(r.Context())
	if err != nil {
		rs.fail(w, r, FailureInternal, err)
		return
	}
	responses := make([]map[string]any, 0, len(invitations))
	for _, invitation := range invitations {
		responses = append(responses, map[string]any{
			"id":                  invitation.ID,
			"guardian_profile_id": invitation.GuardianProfileID,
			"created_at":          invitation.CreatedAt,
			"expires_at":          invitation.ExpiresAt,
			"email_sent_at":       invitation.EmailSentAt,
			"email_error":         invitation.EmailError,
			"email_retry_count":   invitation.EmailRetryCount,
		})
	}
	rs.succeed(w, r, http.StatusOK, responses, "Pending invitations retrieved successfully")
}

// --- relationship handlers ---

// getStudentGuardians lists a child's guardians; everyone with users:read
// may view them for emergencies. The payer mark needs the financial
// permission.
func (rs *GuardianResource) getStudentGuardians(w http.ResponseWriter, r *http.Request) {
	canSeePayment := rs.runtime.HasPermission(r, permissions.GuardiansFinancial)
	studentID, ok := rs.parseStudentID(w, r)
	if !ok {
		return
	}
	if _, active := rs.activeStudent(r.Context(), studentID); !active {
		rs.failMessage(w, r, FailureNotFound, msgStudentNotFound)
		return
	}
	rows, err := rs.directory.ListStudentGuardians(r.Context(), studentID)
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	responses := make([]*GuardianWithRelationship, 0, len(rows))
	for _, row := range rows {
		responses = append(responses, &GuardianWithRelationship{
			Guardian:           newGuardianResponse(row.Guardian),
			RelationshipID:     row.Link.ID,
			RelationshipType:   row.Link.RelationshipType,
			GuardianRole:       row.Link.GuardianRole,
			IsPrimary:          row.Link.IsPrimary,
			IsEmergencyContact: row.Link.IsEmergencyContact,
			CanPickup:          row.Link.CanPickup,
			PickupNotes:        row.Link.PickupNotes,
			EmergencyPriority:  row.Link.EmergencyPriority,
			IsPayer:            canSeePayment && row.Link.IsPayer,
			AccountStatus: guardianAccountStatus(
				row.Guardian.HasAccount,
				row.Link.HasPermission(peopledirectory.GuardianPermissionPortalAccess),
				row.InvitationPending,
			),
		})
	}
	rs.succeed(w, r, http.StatusOK, responses, "Student guardians retrieved successfully")
}

func (rs *GuardianResource) getGuardianStudents(w http.ResponseWriter, r *http.Request) {
	guardianID, ok := rs.parseGuardianID(w, r)
	if !ok {
		return
	}
	rows, err := rs.directory.ListGuardianStudents(r.Context(), guardianID)
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	personIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		personIDs = append(personIDs, row.Student.PersonID)
	}
	persons, err := rs.directory.ListPersonsByID(r.Context(), personIDs)
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	byID := make(map[int64]peopledirectory.Person, len(persons))
	for _, person := range persons {
		byID[person.ID] = person
	}
	responses := make([]*StudentWithRelationship, 0, len(rows))
	for _, row := range rows {
		person, found := byID[row.Student.PersonID]
		if !found {
			rs.runtime.Log.Error("failed to get person for student",
				"student_id", row.Student.ID,
				"person_id", row.Student.PersonID)
			continue
		}
		responses = append(responses, &StudentWithRelationship{
			StudentID:          row.Student.ID,
			FirstName:          person.FirstName,
			LastName:           person.LastName,
			SchoolClass:        row.Student.SchoolClass,
			RelationshipID:     row.Link.ID,
			RelationshipType:   row.Link.RelationshipType,
			GuardianRole:       row.Link.GuardianRole,
			IsPrimary:          row.Link.IsPrimary,
			IsEmergencyContact: row.Link.IsEmergencyContact,
			CanPickup:          row.Link.CanPickup,
			PickupNotes:        row.Link.PickupNotes,
			EmergencyPriority:  row.Link.EmergencyPriority,
		})
	}
	rs.succeed(w, r, http.StatusOK, responses, "Guardian students retrieved successfully")
}

func (rs *GuardianResource) linkGuardianToStudent(w http.ResponseWriter, r *http.Request) {
	studentID, ok := rs.parseStudentID(w, r)
	if !ok {
		return
	}
	if canModify, err := rs.canModifyStudent(r, studentID); !canModify {
		rs.fail(w, r, FailureForbidden, err)
		return
	}
	req := &StudentGuardianLinkRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.fail(w, r, FailureInvalidRequest, err)
		return
	}
	link, err := rs.directory.LinkGuardianToStudent(r.Context(), peopledirectory.LinkGuardian{
		StudentID: studentID, GuardianProfileID: req.GuardianProfileID,
		RelationshipType: req.RelationshipType, GuardianRole: req.GuardianRole,
		IsPrimary: req.IsPrimary, IsEmergencyContact: req.IsEmergencyContact, CanPickup: req.CanPickup,
		PickupNotes: req.PickupNotes, EmergencyPriority: req.EmergencyPriority,
	})
	if err != nil {
		// The legacy handler answered every link failure as a 500; a missing
		// guardian or student keeps that status so the screens stay unchanged.
		rs.fail(w, r, FailureInternal, err)
		return
	}
	rs.succeed(w, r, http.StatusCreated, newGuardianLinkResponse(link), "Guardian linked to student successfully")
}

// createStudentGuardians atomically creates or links one or more guardians
// for an existing student (#819). Existing-profile links need users:update;
// a batch that creates a profile additionally needs users:create.
func (rs *GuardianResource) createStudentGuardians(w http.ResponseWriter, r *http.Request) {
	studentID, ok := rs.parseStudentID(w, r)
	if !ok {
		return
	}
	if canModify, err := rs.canModifyStudent(r, studentID); !canModify {
		rs.fail(w, r, FailureForbidden, err)
		return
	}
	req := &CreateStudentGuardiansRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.fail(w, r, FailureInvalidRequest, err)
		return
	}
	if batchCreatesGuardian(req.Guardians) && !rs.runtime.HasPermission(r, permissions.UsersCreate) {
		rs.failMessage(w, r, FailureForbidden, "forbidden")
		return
	}
	if err := rs.directory.AddGuardiansToStudent(r.Context(), studentID, req.Guardians); err != nil {
		// Validation runs before any write, so bad input means nothing was
		// persisted; everything else rolled the transaction back.
		if errors.Is(err, peopledirectory.ErrInvalidGuardian) {
			rs.fail(w, r, FailureInvalidRequest, err)
			return
		}
		rs.fail(w, r, FailureInternal, err)
		return
	}
	rs.succeed(w, r, http.StatusCreated, nil, "Guardians added successfully")
}

func batchCreatesGuardian(guardians []peopledirectory.NewStudentGuardian) bool {
	for _, guardian := range guardians {
		if guardian.CreatesProfile() {
			return true
		}
	}
	return false
}

func (rs *GuardianResource) updateStudentGuardianRelationship(w http.ResponseWriter, r *http.Request) {
	relationshipID, ok := rs.parseIDParam(w, r, "relationshipId", msgInvalidRelationshipID)
	if !ok {
		return
	}
	link, err := rs.directory.FindGuardianLink(r.Context(), relationshipID)
	if err != nil {
		rs.failMessage(w, r, FailureNotFound, msgRelationshipNotFound)
		return
	}
	if canModify, err := rs.canModifyStudent(r, link.StudentID); !canModify {
		rs.fail(w, r, FailureForbidden, err)
		return
	}
	req := &StudentGuardianUpdateRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.fail(w, r, FailureInvalidRequest, err)
		return
	}
	if err := rs.directory.UpdateGuardianLink(r.Context(), relationshipID, peopledirectory.GuardianLinkUpdate{
		RelationshipType: req.RelationshipType, GuardianRole: req.GuardianRole,
		IsPrimary: req.IsPrimary, IsEmergencyContact: req.IsEmergencyContact, CanPickup: req.CanPickup,
		PickupNotes: req.PickupNotes, EmergencyPriority: req.EmergencyPriority,
	}); err != nil {
		rs.fail(w, r, FailureInternal, err)
		return
	}
	rs.succeed(w, r, http.StatusOK, nil, "Relationship updated successfully")
}

func (rs *GuardianResource) removeGuardianFromStudent(w http.ResponseWriter, r *http.Request) {
	studentID, ok := rs.parseStudentID(w, r)
	if !ok {
		return
	}
	guardianID, ok := rs.parseIDParam(w, r, "guardianId", msgInvalidGuardianID)
	if !ok {
		return
	}
	if canModify, err := rs.canModifyStudent(r, studentID); !canModify {
		rs.fail(w, r, FailureForbidden, err)
		return
	}
	accountID, ok := rs.actingAccountID(w, r)
	if !ok {
		return
	}
	// Unlinking the child's payer clears the payer mark, which belongs to
	// guardians:financial (#2608); the owner refuses it for everyone else.
	if err := rs.directory.RemoveGuardianFromStudent(r.Context(), peopledirectory.RemoveGuardian{
		StudentID: studentID, GuardianProfileID: guardianID, ActorAccountID: accountID,
		MayClearPayer: rs.runtime.HasPermission(r, permissions.GuardiansFinancial),
	}); err != nil {
		if errors.Is(err, peopledirectory.ErrPayerRemovalRequiresFinancial) {
			rs.fail(w, r, FailureForbidden, err)
			return
		}
		rs.fail(w, r, FailureInternal, err)
		return
	}
	rs.succeed(w, r, http.StatusOK, nil, "Guardian removed from student successfully")
}

// --- phone number handlers ---

// validatePhoneAccess parses both IDs, runs the guardian write gate and
// checks that the phone belongs to the guardian in the URL.
func (rs *GuardianResource) validatePhoneAccess(w http.ResponseWriter, r *http.Request) (peopledirectory.GuardianPhone, bool) {
	guardianID, ok := rs.parseGuardianID(w, r)
	if !ok {
		return peopledirectory.GuardianPhone{}, false
	}
	phoneID, ok := rs.parseIDParam(w, r, "phoneId", msgInvalidPhoneID)
	if !ok {
		return peopledirectory.GuardianPhone{}, false
	}
	if canModify, err := rs.canModifyGuardian(r, guardianID); !canModify {
		rs.fail(w, r, FailureForbidden, err)
		return peopledirectory.GuardianPhone{}, false
	}
	phone, err := rs.directory.FindGuardianPhone(r.Context(), phoneID)
	if err != nil {
		rs.failMessage(w, r, FailureNotFound, msgPhoneNotFound)
		return peopledirectory.GuardianPhone{}, false
	}
	if phone.GuardianProfileID != guardianID {
		rs.failMessage(w, r, FailureForbidden, msgPhoneNotBelongsGuard)
		return peopledirectory.GuardianPhone{}, false
	}
	return phone, true
}

func (rs *GuardianResource) listGuardianPhoneNumbers(w http.ResponseWriter, r *http.Request) {
	guardianID, ok := rs.parseGuardianID(w, r)
	if !ok {
		return
	}
	phones, err := rs.directory.ListGuardianPhones(r.Context(), guardianID)
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	responses := make([]*PhoneNumberResponse, 0, len(phones))
	for _, phone := range phones {
		responses = append(responses, newPhoneNumberResponse(phone))
	}
	rs.succeed(w, r, http.StatusOK, responses, "Phone numbers retrieved successfully")
}

func (rs *GuardianResource) addPhoneNumber(w http.ResponseWriter, r *http.Request) {
	guardianID, ok := rs.parseGuardianID(w, r)
	if !ok {
		return
	}
	if canModify, err := rs.canModifyGuardian(r, guardianID); !canModify {
		rs.fail(w, r, FailureForbidden, err)
		return
	}
	req := &PhoneNumberCreateRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.fail(w, r, FailureInvalidRequest, err)
		return
	}
	phone, err := rs.directory.AddGuardianPhone(r.Context(), guardianID, peopledirectory.GuardianPhoneInput{
		PhoneNumber: req.PhoneNumber, PhoneType: req.PhoneType, Label: req.Label, IsPrimary: req.IsPrimary,
	})
	if err != nil {
		rs.fail(w, r, FailureInternal, err)
		return
	}
	rs.succeed(w, r, http.StatusCreated, newPhoneNumberResponse(phone), "Phone number added successfully")
}

func (rs *GuardianResource) updatePhoneNumber(w http.ResponseWriter, r *http.Request) {
	phone, ok := rs.validatePhoneAccess(w, r)
	if !ok {
		return
	}
	req := &PhoneNumberUpdateRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.fail(w, r, FailureInvalidRequest, err)
		return
	}
	if err := rs.directory.UpdateGuardianPhone(r.Context(), phone.ID, peopledirectory.GuardianPhoneUpdate{
		PhoneNumber: req.PhoneNumber, PhoneType: req.PhoneType, Label: req.Label, IsPrimary: req.IsPrimary, Priority: req.Priority,
	}); err != nil {
		rs.fail(w, r, FailureInternal, err)
		return
	}
	rs.respondPhone(w, r, phone.ID, "Phone number updated successfully")
}

func (rs *GuardianResource) deletePhoneNumber(w http.ResponseWriter, r *http.Request) {
	phone, ok := rs.validatePhoneAccess(w, r)
	if !ok {
		return
	}
	if err := rs.directory.DeleteGuardianPhone(r.Context(), phone.ID); err != nil {
		rs.fail(w, r, FailureInternal, err)
		return
	}
	rs.succeed(w, r, http.StatusOK, nil, "Phone number deleted successfully")
}

func (rs *GuardianResource) setPrimaryPhone(w http.ResponseWriter, r *http.Request) {
	phone, ok := rs.validatePhoneAccess(w, r)
	if !ok {
		return
	}
	if err := rs.directory.SetPrimaryGuardianPhone(r.Context(), phone.ID); err != nil {
		rs.fail(w, r, FailureInternal, err)
		return
	}
	rs.respondPhone(w, r, phone.ID, "Phone number set as primary successfully")
}

// respondPhone re-reads the phone after a write so the response carries the
// stored state.
func (rs *GuardianResource) respondPhone(w http.ResponseWriter, r *http.Request, phoneID int64, message string) {
	updated, err := rs.directory.FindGuardianPhone(r.Context(), phoneID)
	if err != nil {
		rs.fail(w, r, FailureInternal, err)
		return
	}
	rs.succeed(w, r, http.StatusOK, newPhoneNumberResponse(updated), message)
}
