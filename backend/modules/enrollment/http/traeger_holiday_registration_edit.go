package enrollmenthttp

import (
	"encoding/json"
	"net/http"
	"slices"

	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/api/common"
	capability "github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// HolidayRegistrationEditRequest is the body of PUT
// /traeger/holiday-periods/{id}/registrations/{registrationId}/details
// (#5629): the sections the Träger-Büro changes, with the field names of the
// public form. A section that is absent stays as it is. guardian and
// second_guardian and guardian_address travel together, a missing or empty
// second_guardian or guardian_address removes it (#5625);
// emergency_contacts and pickup_persons travel together;
// children name each changed child by its id. Bookings, benefits, consents
// and health information are not part of it.
type HolidayRegistrationEditRequest struct {
	Guardian          *holidayGuardianRequest     `json:"guardian"`
	GuardianAddress   *holidayAddressRequest      `json:"guardian_address"`
	SecondGuardian    *holidayGuardianRequest     `json:"second_guardian"`
	Children          []holidayChildEditRequest   `json:"children"`
	EmergencyContacts *[]holidayContactRequest    `json:"emergency_contacts"`
	PickupPersons     []holidayContactRequest     `json:"pickup_persons"`
	CustomData        *map[string]json.RawMessage `json:"custom_data"`
}

type holidayChildEditRequest struct {
	ID              int64                     `json:"id"`
	FirstName       string                    `json:"first_name"`
	LastName        string                    `json:"last_name"`
	BirthDate       calendar.Date             `json:"birth_date"`
	HomeSchoolID    int64                     `json:"home_school_id"`
	GuestSchoolName string                    `json:"guest_school_name"`
	CustomData      capability.HolidayAnswers `json:"custom_data"`
}

// Bind accepts the body; the Owner validates the details.
func (req *HolidayRegistrationEditRequest) Bind(*http.Request) error { return nil }

func (req *HolidayRegistrationEditRequest) edit() capability.HolidayRegistrationEdit {
	edit := capability.HolidayRegistrationEdit{}
	if req.Guardian != nil {
		guardians := &capability.HolidayGuardiansEdit{Guardian: capability.HolidayGuardianInput(*req.Guardian)}
		if req.SecondGuardian != nil {
			second := capability.HolidayGuardianInput(*req.SecondGuardian)
			guardians.SecondGuardian = &second
		}
		if req.GuardianAddress != nil {
			address := capability.HolidayAddress(*req.GuardianAddress)
			guardians.GuardianAddress = &address
		}
		edit.Guardians = guardians
	}
	if req.EmergencyContacts != nil {
		edit.Contacts = &capability.HolidayContactsEdit{
			EmergencyContacts: holidayContacts(*req.EmergencyContacts), PickupPersons: holidayContacts(req.PickupPersons),
		}
	}
	if req.CustomData != nil {
		answers := capability.HolidayAnswers(*req.CustomData)
		edit.CustomData = &answers
	}
	for _, child := range req.Children {
		edit.Children = append(edit.Children, capability.HolidayChildEdit{
			ChildID: child.ID, FirstName: child.FirstName, LastName: child.LastName, BirthDate: child.BirthDate,
			HomeSchoolID: child.HomeSchoolID, GuestSchoolName: child.GuestSchoolName, CustomData: child.CustomData,
		})
	}
	return edit
}

// editRegistration answers the changed registration. The schools a child
// may attend are the Träger's undeleted schools, read before the Träger
// transaction opened (withSchoolNames).
func (rs *HolidayPeriodResource) editRegistration(w http.ResponseWriter, r *http.Request) {
	reviewer, periodID, registrationID, ok := registrationKey(w, r)
	if !ok {
		return
	}
	req := &HolidayRegistrationEditRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	names, _ := r.Context().Value(holidaySchoolNamesKey{}).(map[int64]string)
	schools := make([]capability.HolidaySchool, 0, len(names))
	for id, name := range names {
		schools = append(schools, capability.HolidaySchool{ID: id, Name: name})
	}
	slices.SortFunc(schools, func(a, b capability.HolidaySchool) int { return int(a.ID - b.ID) })
	detail, err := rs.review.EditHolidayRegistration(r.Context(), reviewer, periodID, registrationID, req.edit(), schools)
	if err != nil {
		renderHolidayReviewError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Holiday registration details changed successfully")
}
