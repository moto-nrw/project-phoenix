package enrollmenthttp

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/moto-ogs/project-phoenix/api/common"
	capability "github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// HolidayRegistrationStatusWeek is a booked week of a child on the status
// page (#6676).
type HolidayRegistrationStatusWeek struct {
	FirstDay       calendar.Date `json:"first_day"`
	LastDay        calendar.Date `json:"last_day"`
	TimeModelName  string        `json:"time_model_name"`
	StartsAt       string        `json:"starts_at"`
	EndsAt         string        `json:"ends_at"`
	HostSchoolName string        `json:"host_school_name"`
	Placement      string        `json:"placement"`
	Lunch          bool          `json:"lunch"`
}

// HolidayRegistrationStatusChild is a child of the registration by its
// first name.
type HolidayRegistrationStatusChild struct {
	FirstName string                          `json:"first_name"`
	Weeks     []HolidayRegistrationStatusWeek `json:"weeks"`
}

// HolidayRegistrationStatusResponse is the body of GET
// /traeger/holiday-registrations/status/{statusToken}: the state of one
// registration, without health information, address, contacts, last names
// or dates of birth. first_day and last_day are null while the period has no
// weeks.
type HolidayRegistrationStatusResponse struct {
	TraegerName          string                           `json:"traeger_name"`
	PeriodName           string                           `json:"period_name"`
	FirstDay             *calendar.Date                   `json:"first_day"`
	LastDay              *calendar.Date                   `json:"last_day"`
	RegistrationClosesOn calendar.Date                    `json:"registration_closes_on"`
	Binding              bool                             `json:"binding"`
	AllocationMode       string                           `json:"allocation_mode"`
	SubmittedAt          time.Time                        `json:"submitted_at"`
	Status               string                           `json:"status"`
	Children             []HolidayRegistrationStatusChild `json:"children"`
}

// status answers the status link of a registration: the token is resolved
// in the administrative transaction, the registration read in the Träger
// transaction of its Träger.
func (rs *HolidayRegistrationResource) status(w http.ResponseWriter, r *http.Request) {
	var link capability.HolidayRegistrationStatusLink
	err := withinAdmin(r.Context(), func(adminCtx context.Context) error {
		resolved, resolveErr := rs.registrations.ResolveHolidayRegistrationStatus(adminCtx, chi.URLParam(r, "statusToken"))
		link = resolved
		return resolveErr
	})
	if err != nil {
		renderHolidayRegistrationError(w, r, err)
		return
	}
	var status capability.HolidayRegistrationStatus
	err = withinTraeger(r.Context(), link.Reference.OrganizationID, func(txCtx context.Context) error {
		loaded, loadErr := rs.registrations.HolidayRegistrationStatus(txCtx, link)
		status = loaded
		return loadErr
	})
	if err != nil {
		renderHolidayRegistrationError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, holidayRegistrationStatusResponse(status), "Holiday registration status retrieved")
}

func holidayRegistrationStatusResponse(status capability.HolidayRegistrationStatus) HolidayRegistrationStatusResponse {
	response := HolidayRegistrationStatusResponse{
		TraegerName: status.TraegerName, PeriodName: status.PeriodName, RegistrationClosesOn: status.RegistrationClosesOn,
		Binding: status.Binding, AllocationMode: status.AllocationMode, SubmittedAt: status.SubmittedAt, Status: status.Status,
		Children: make([]HolidayRegistrationStatusChild, 0, len(status.Children)),
	}
	if !status.FirstDay.IsZero() && !status.LastDay.IsZero() {
		response.FirstDay, response.LastDay = &status.FirstDay, &status.LastDay
	}
	for _, child := range status.Children {
		weeks := make([]HolidayRegistrationStatusWeek, 0, len(child.Weeks))
		for _, week := range child.Weeks {
			weeks = append(weeks, HolidayRegistrationStatusWeek(week))
		}
		response.Children = append(response.Children, HolidayRegistrationStatusChild{FirstName: child.FirstName, Weeks: weeks})
	}
	return response
}
