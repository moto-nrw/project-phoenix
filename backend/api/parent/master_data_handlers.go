package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
)

// MasterDataResponse is the parent-facing Stammdaten projection: stringified
// ids, dates as YYYY-MM-DD, the calling guardian's own contact data, and the
// child's pending Track B change requests.
type MasterDataResponse struct {
	StudentID string `json:"student_id"`

	// Identity — Track B editable, read-only here.
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Birthday  *string `json:"birthday,omitempty"`

	// School-controlled — read-only.
	SchoolClass   string  `json:"school_class"`
	Status        string  `json:"status"`
	EnrolledFrom  *string `json:"enrolled_from,omitempty"`
	EnrolledUntil *string `json:"enrolled_until,omitempty"`

	// Health — Track A editable.
	HealthInfo *string `json:"health_info,omitempty"`

	// Calling guardian's own contact data — Track A editable.
	GuardianProfileID      string  `json:"guardian_profile_id"`
	Email                  *string `json:"email,omitempty"`
	AddressStreet          *string `json:"address_street,omitempty"`
	AddressCity            *string `json:"address_city,omitempty"`
	AddressPostalCode      *string `json:"address_postal_code,omitempty"`
	PreferredContactMethod string  `json:"preferred_contact_method"`
	LanguagePreference     string  `json:"language_preference"`
	PrimaryPhone           *string `json:"primary_phone,omitempty"`

	// Permanent weekday departure config — Track B editable, read-only here.
	DepartureDays         usersModels.DepartureDays         `json:"departure_days,omitempty"`
	AllowedDepartureModes usersModels.AllowedDepartureModes `json:"allowed_departure_modes,omitempty"`

	PendingChanges []MasterDataChangeResponse `json:"pending_changes"`
}

// MasterDataChangeResponse is one change-request row in the parent view.
type MasterDataChangeResponse struct {
	ID        string          `json:"id"`
	Target    string          `json:"target"`
	FieldKey  string          `json:"field_key"`
	OldValue  json.RawMessage `json:"old_value,omitempty"`
	NewValue  json.RawMessage `json:"new_value"`
	Status    string          `json:"status"`
	CreatedAt string          `json:"created_at"`
	IsSelf    bool            `json:"is_self"`
}

// UpdateMasterDataRequest is the wire shape for the Track A PATCH. The value is
// arbitrary JSON (a string for every Track A field today).
type UpdateMasterDataRequest struct {
	Value json.RawMessage `json:"value"`
}

func dateStrPtr(d *timezone.Date) *string {
	if d == nil {
		return nil
	}
	s := d.String()
	return &s
}

func toMasterDataResponse(d *parentService.ChildMasterData, accountID int64) MasterDataResponse {
	resp := MasterDataResponse{
		StudentID:              strconv.FormatInt(d.StudentID, 10),
		FirstName:              d.FirstName,
		LastName:               d.LastName,
		Birthday:               dateStrPtr(d.Birthday),
		SchoolClass:            d.SchoolClass,
		Status:                 d.Status,
		EnrolledFrom:           dateStrPtr(d.EnrolledFrom),
		EnrolledUntil:          dateStrPtr(d.EnrolledUntil),
		HealthInfo:             d.HealthInfo,
		GuardianProfileID:      strconv.FormatInt(d.GuardianProfileID, 10),
		Email:                  d.Email,
		AddressStreet:          d.AddressStreet,
		AddressCity:            d.AddressCity,
		AddressPostalCode:      d.AddressPostalCode,
		PreferredContactMethod: d.PreferredContactMethod,
		LanguagePreference:     d.LanguagePreference,
		PrimaryPhone:           d.PrimaryPhone,
		DepartureDays:          d.DepartureDays,
		AllowedDepartureModes:  d.AllowedDepartureModes,
		PendingChanges:         make([]MasterDataChangeResponse, 0, len(d.PendingChanges)),
	}
	for _, c := range d.PendingChanges {
		resp.PendingChanges = append(resp.PendingChanges, MasterDataChangeResponse{
			ID:        strconv.FormatInt(c.ID, 10),
			Target:    c.Target,
			FieldKey:  c.FieldKey,
			OldValue:  c.OldValue,
			NewValue:  c.NewValue,
			Status:    c.Status,
			CreatedAt: c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			IsSelf:    c.SubmittedBy == accountID,
		})
	}
	return resp
}

// getMasterData returns the structured Stammdaten view for the child.
func (rs *Resource) getMasterData(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}

	data, err := rs.ParentService.GetChildMasterData(r.Context(), accountID, studentID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toMasterDataResponse(data, accountID), "Master data retrieved")
}

// updateMasterDataField applies a Track A direct edit to a single field.
func (rs *Resource) updateMasterDataField(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	target := chi.URLParam(r, "target")
	field := chi.URLParam(r, "field")
	if target == "" || field == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("target and field are required")))
		return
	}

	var req UpdateMasterDataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}

	data, err := rs.ParentService.UpdateMasterDataField(r.Context(), accountID, studentID, target, field, req.Value)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toMasterDataResponse(data, accountID), "Master data updated")
}

// MasterDataChangeRequestBody is the wire shape for POST .../master-data/requests.
type MasterDataChangeRequestBody struct {
	Changes []MasterDataChangeInput `json:"changes"`
	// RecipientGuardianProfileIDs travel with the creation so the share is
	// written in the same transaction (#2267); empty shares with nobody.
	RecipientGuardianProfileIDs []string `json:"recipient_guardian_profile_ids"`
}

// MasterDataChangeInput is one proposed Track B field change.
type MasterDataChangeInput struct {
	Target   string          `json:"target"`
	FieldKey string          `json:"field_key"`
	Value    json.RawMessage `json:"value"`
}

func toMasterDataChangeResponses(rows []*usersModels.StudentDataChangeRequest, accountID int64) []MasterDataChangeResponse {
	out := make([]MasterDataChangeResponse, 0, len(rows))
	for _, c := range rows {
		out = append(out, MasterDataChangeResponse{
			ID:        strconv.FormatInt(c.ID, 10),
			Target:    c.Target,
			FieldKey:  c.FieldKey,
			OldValue:  c.OldValue,
			NewValue:  c.NewValue,
			Status:    c.Status,
			CreatedAt: c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			IsSelf:    c.SubmittedBy == accountID,
		})
	}
	return out
}

// submitMasterDataRequest records pending Track B change requests for approval.
func (rs *Resource) submitMasterDataRequest(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}

	var body MasterDataChangeRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	changes := make([]parentService.MasterDataFieldChange, 0, len(body.Changes))
	for _, c := range body.Changes {
		changes = append(changes, parentService.MasterDataFieldChange{
			Target:   c.Target,
			FieldKey: c.FieldKey,
			Value:    c.Value,
		})
	}

	recipients, ok := parseCreateRecipients(w, r, body.RecipientGuardianProfileIDs)
	if !ok {
		return
	}
	rows, err := rs.ParentService.SubmitMasterDataChangeRequest(r.Context(), accountID, studentID, changes, recipients)
	if err != nil {
		renderParentRequestError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toMasterDataChangeResponses(rows, accountID), "Change request submitted")
}

// listMasterDataRequests returns the child's change requests (any status).
func (rs *Resource) listMasterDataRequests(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	rows, err := rs.ParentService.ListMyMasterDataRequests(r.Context(), accountID, studentID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toMasterDataChangeResponses(rows, accountID), "Change requests retrieved")
}
