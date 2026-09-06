package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// CoursesResponse is the parents-portal Kurse section (#3075). It is empty and
// carries a reason whenever the school has no course requests, so the portal
// can explain instead of showing a list nobody can act on.
type CoursesResponse struct {
	Enabled bool `json:"enabled"`
	// DisabledReason is a stable identifier (school_disabled, no_enrollment,
	// no_courses, no_permission) the portal turns into German copy.
	DisabledReason string `json:"disabled_reason,omitempty"`
	PhaseName      string `json:"phase_name,omitempty"`
	// EffectiveFrom is the date a new request would take effect on.
	EffectiveFrom string `json:"effective_from,omitempty"`
	// PendingRequestID is the child's open course request, absent when none.
	PendingRequestID       string `json:"pending_request_id,omitempty"`
	PendingSubmittedBySelf bool   `json:"pending_submitted_by_self"`
	// OtherRequestPending marks an open request about care offerings, which
	// blocks a course request until it is decided.
	OtherRequestPending bool                 `json:"other_request_pending"`
	Items               []CourseItemResponse `json:"items"`
}

// CourseItemResponse is one course.
type CourseItemResponse struct {
	ID              string   `json:"id"`
	ActivityGroupID string   `json:"activity_group_id"`
	Name            string   `json:"name"`
	Description     string   `json:"description,omitempty"`
	AvailableDays   []string `json:"available_days"`
	// Capacity is the effective participant limit; absent means unlimited.
	Capacity  *int `json:"capacity,omitempty"`
	FreeSlots *int `json:"free_slots,omitempty"`
	Booked    bool `json:"booked"`
	Requested bool `json:"requested"`
	// Waitlisted marks a requested course that was full when it was asked for.
	Waitlisted       bool `json:"waitlisted"`
	WaitlistPosition int  `json:"waitlist_position,omitempty"`
}

// CourseRequestBody is the wire shape for POST .../courses/requests.
type CourseRequestBody struct {
	CourseID string `json:"course_id"`
	Note     string `json:"note"`
}

func toCoursesResponse(catalog *enrollmentService.CourseCatalog) CoursesResponse {
	resp := CoursesResponse{
		Enabled:                catalog.Enabled,
		DisabledReason:         catalog.DisabledReason,
		PhaseName:              catalog.PhaseName,
		PendingSubmittedBySelf: catalog.PendingSubmittedBySelf,
		OtherRequestPending:    catalog.OtherRequestPending,
		Items:                  make([]CourseItemResponse, 0, len(catalog.Items)),
	}
	if !catalog.EffectiveFrom.IsZero() {
		resp.EffectiveFrom = catalog.EffectiveFrom.String()
	}
	if catalog.PendingRequestID > 0 {
		resp.PendingRequestID = strconv.FormatInt(catalog.PendingRequestID, 10)
	}
	for _, item := range catalog.Items {
		resp.Items = append(resp.Items, CourseItemResponse{
			ID:               strconv.FormatInt(item.OfferingID, 10),
			ActivityGroupID:  strconv.FormatInt(item.ActivityGroupID, 10),
			Name:             item.Name,
			Description:      item.Description,
			AvailableDays:    stringsOrEmpty(item.AvailableDays),
			Capacity:         item.Capacity,
			FreeSlots:        item.FreeSlots,
			Booked:           item.Booked,
			Requested:        item.Requested,
			Waitlisted:       item.Waitlisted,
			WaitlistPosition: item.WaitlistPosition,
		})
	}
	return resp
}

// getChildCourses returns the courses the family can see for this child.
func (rs *Resource) getChildCourses(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	catalog, err := rs.ParentService.GetChildCourses(r.Context(), accountID, studentID)
	if err != nil {
		renderCourseError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toCoursesResponse(catalog), "Courses retrieved")
}

// createCourseRequest asks the OGS for one course.
func (rs *Resource) createCourseRequest(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	var body CourseRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	courseID, convErr := strconv.ParseInt(strings.TrimSpace(body.CourseID), 10, 64)
	if convErr != nil || courseID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(
			errors.New("course_id must be a numeric id"), "course_request_invalid"))
		return
	}
	catalog, err := rs.ParentService.RequestChildCourse(r.Context(), accountID, studentID, courseID, body.Note)
	if err != nil {
		renderCourseError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toCoursesResponse(catalog), "Course requested")
}

// withdrawCourseRequest takes back the caller's own open course request.
func (rs *Resource) withdrawCourseRequest(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "requestId", "invalid request ID")
	if !ok {
		return
	}
	catalog, err := rs.ParentService.WithdrawChildCourseRequest(r.Context(), accountID, studentID, requestID)
	if err != nil {
		renderCourseError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toCoursesResponse(catalog), "Course request withdrawn")
}

// renderCourseError maps the course-specific outcomes and hands everything
// else to the shared parent write renderer.
func renderCourseError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, enrollmentService.ErrCourseRequestsDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "course_requests_disabled"))
	case errors.Is(err, enrollmentService.ErrCourseNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, enrollmentService.ErrCourseAlreadyBooked):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "course_already_booked"))
	case errors.Is(err, enrollmentService.ErrCourseRequestNotOwn):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "course_request_not_own"))
	default:
		renderParentWriteError(w, r, err)
	}
}
