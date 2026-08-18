package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
)

// childGuardianResponse is the parent-facing projection of one guardian of a
// child: profile-level contact data plus the per-child pickup relationship,
// annotated with what the caller may edit. int64 IDs are stringified.
type childGuardianResponse struct {
	GuardianProfileID  string                  `json:"guardian_profile_id"`
	StudentGuardianID  string                  `json:"student_guardian_id"`
	FirstName          string                  `json:"first_name"`
	LastName           string                  `json:"last_name"`
	Email              string                  `json:"email,omitempty"`
	Phones             []guardianPhoneResponse `json:"phones"`
	AddressStreet      string                  `json:"address_street,omitempty"`
	AddressCity        string                  `json:"address_city,omitempty"`
	AddressPostalCode  string                  `json:"address_postal_code,omitempty"`
	RelationshipType   string                  `json:"relationship_type"`
	IsPrimary          bool                    `json:"is_primary"`
	IsEmergencyContact bool                    `json:"is_emergency_contact"`
	CanPickup          bool                    `json:"can_pickup"`
	PickupNotes        string                  `json:"pickup_notes,omitempty"`
	HasAccount         bool                    `json:"has_account"`
	IsSelf             bool                    `json:"is_self"`
	CanEditContact     bool                    `json:"can_edit_contact"`
	CanManagePickup    bool                    `json:"can_manage_pickup"`
	// ContactLockedOwnAccount: this guardian's contact data is read-only to the
	// caller because the guardian manages it through their own portal account.
	ContactLockedOwnAccount bool `json:"contact_locked_own_account"`
	// ContactLockedShared: this contact-only guardian is read-only to the caller
	// because the profile is also linked to a child outside the caller's family.
	ContactLockedShared bool `json:"contact_locked_shared"`
	// ContactLockedSocialWorker: this relationship is a school-managed social
	// worker, so its contact data is redacted and read-only to the caller.
	ContactLockedSocialWorker bool `json:"contact_locked_social_worker"`
	// ContactLockedFullGuardian: this is a full guardian (primary/legal/co)
	// without their own account, so they are read-only to the caller (managed by
	// themselves or the school); contact stays visible.
	ContactLockedFullGuardian bool `json:"contact_locked_full_guardian"`
}

type guardianPhoneResponse struct {
	PhoneNumber string `json:"phone_number"`
	PhoneType   string `json:"phone_type"`
	Label       string `json:"label,omitempty"`
	IsPrimary   bool   `json:"is_primary"`
}

// updateGuardianContactRequest is the wire shape for PUT
// /parent/me/children/{studentId}/guardians/{guardianProfileId}/contact.
// Profile fields and the phone list are replaced wholesale.
type updateGuardianContactRequest struct {
	FirstName         string                 `json:"first_name"`
	LastName          string                 `json:"last_name"`
	Email             *string                `json:"email"`
	AddressStreet     *string                `json:"address_street"`
	AddressCity       *string                `json:"address_city"`
	AddressPostalCode *string                `json:"address_postal_code"`
	Phones            []guardianPhoneRequest `json:"phones"`
}

type guardianPhoneRequest struct {
	PhoneNumber string  `json:"phone_number"`
	PhoneType   string  `json:"phone_type"`
	Label       *string `json:"label"`
	IsPrimary   bool    `json:"is_primary"`
}

// updateGuardianRelationshipRequest is the wire shape for PUT
// /parent/me/children/{studentId}/guardians/{guardianProfileId}/pickup.
// Every field is optional; an omitted field is left unchanged.
type updateGuardianRelationshipRequest struct {
	CanPickup          *bool   `json:"can_pickup"`
	IsEmergencyContact *bool   `json:"is_emergency_contact"`
	PickupNotes        *string `json:"pickup_notes"`
}

// createGuardianContactRequest creates an accountless contact. It never grants
// app access, which remains the responsibility of related-accounts invitations.
type createGuardianContactRequest struct {
	updateGuardianContactRequest
	RelationshipType   string  `json:"relationship_type"`
	CanPickup          bool    `json:"can_pickup"`
	IsEmergencyContact bool    `json:"is_emergency_contact"`
	PickupNotes        *string `json:"pickup_notes"`
}

// listChildGuardians returns the guardians of the parent's child with contact +
// pickup detail. Ownership is verified in the service.
func (rs *Resource) listChildGuardians(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}

	guardians, err := rs.ParentService.ListChildGuardians(r.Context(), accountID, studentID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}

	out := make([]childGuardianResponse, 0, len(guardians))
	for _, g := range guardians {
		out = append(out, guardianResponse(g))
	}
	common.Respond(w, r, http.StatusOK, out, "Guardians")
}

// createGuardianContact adds a contact without creating or inviting an account.
func (rs *Resource) createGuardianContact(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var body createGuardianContactRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}

	created, err := rs.ParentService.CreateGuardianContact(r.Context(), accountID, studentID, parentService.CreateGuardianContactInput{
		Contact:            guardianContactInput(body.updateGuardianContactRequest),
		RelationshipType:   body.RelationshipType,
		CanPickup:          body.CanPickup,
		IsEmergencyContact: body.IsEmergencyContact,
		PickupNotes:        body.PickupNotes,
	})
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, guardianResponse(created), "Guardian contact created")
}

// updateGuardianContact edits a contact-only guardian's contact data.
func (rs *Resource) updateGuardianContact(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	guardianProfileID, ok := parsePathGuardianProfileID(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var body updateGuardianContactRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}

	input := guardianContactInput(body)

	updated, err := rs.ParentService.UpdateGuardianContact(r.Context(), accountID, studentID, guardianProfileID, input)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, guardianResponse(updated), "Guardian contact updated")
}

func guardianContactInput(body updateGuardianContactRequest) parentService.GuardianContactInput {
	input := parentService.GuardianContactInput{
		FirstName:         body.FirstName,
		LastName:          body.LastName,
		Email:             body.Email,
		AddressStreet:     body.AddressStreet,
		AddressCity:       body.AddressCity,
		AddressPostalCode: body.AddressPostalCode,
	}
	for _, p := range body.Phones {
		input.Phones = append(input.Phones, parentService.GuardianPhoneInput{
			PhoneNumber: p.PhoneNumber,
			PhoneType:   p.PhoneType,
			Label:       p.Label,
			IsPrimary:   p.IsPrimary,
		})
	}
	return input
}

// updateGuardianRelationship edits the per-child pickup/relationship fields.
func (rs *Resource) updateGuardianRelationship(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	guardianProfileID, ok := parsePathGuardianProfileID(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 2<<10)
	var body updateGuardianRelationshipRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}

	updated, err := rs.ParentService.UpdateGuardianRelationship(r.Context(), accountID, studentID, guardianProfileID, parentService.GuardianRelationshipInput{
		CanPickup:          body.CanPickup,
		IsEmergencyContact: body.IsEmergencyContact,
		PickupNotes:        body.PickupNotes,
	})
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, guardianResponse(updated), "Guardian pickup updated")
}

func parsePathGuardianProfileID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return common.ParsePositiveInt64IDWithError(w, r, "guardianProfileId", "invalid guardian profile id")
}

func guardianResponse(g *parentService.ChildGuardian) childGuardianResponse {
	phones := make([]guardianPhoneResponse, 0, len(g.Phones))
	for _, p := range g.Phones {
		phones = append(phones, guardianPhoneResponse{
			PhoneNumber: p.PhoneNumber,
			PhoneType:   p.PhoneType,
			Label:       p.Label,
			IsPrimary:   p.IsPrimary,
		})
	}
	return childGuardianResponse{
		GuardianProfileID:         strconv.FormatInt(g.GuardianProfileID, 10),
		StudentGuardianID:         strconv.FormatInt(g.StudentGuardianID, 10),
		FirstName:                 g.FirstName,
		LastName:                  g.LastName,
		Email:                     g.Email,
		Phones:                    phones,
		AddressStreet:             g.AddressStreet,
		AddressCity:               g.AddressCity,
		AddressPostalCode:         g.AddressPostalCode,
		RelationshipType:          g.RelationshipType,
		IsPrimary:                 g.IsPrimary,
		IsEmergencyContact:        g.IsEmergencyContact,
		CanPickup:                 g.CanPickup,
		PickupNotes:               g.PickupNotes,
		HasAccount:                g.HasAccount,
		IsSelf:                    g.IsSelf,
		CanEditContact:            g.CanEditContact,
		CanManagePickup:           g.CanManagePickup,
		ContactLockedOwnAccount:   g.ContactLockedOwnAccount,
		ContactLockedShared:       g.ContactLockedShared,
		ContactLockedSocialWorker: g.ContactLockedSocialWorker,
		ContactLockedFullGuardian: g.ContactLockedFullGuardian,
	}
}
