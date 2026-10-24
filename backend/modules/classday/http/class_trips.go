package classdayhttp

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// Class trips and day outings through "moto schule" (#4799): the Lehrkraft
// of a class reports who is away and when. Every report passes the same four
// gates as the arrival exception — school scope (SchoolMiddleware), the
// class_day:class_trip_write permission, the school's setting
// operations.school_portal_write_scope, and the caller's
// education.class_teachers assignment — before the projection validates the
// days and the children. The writes are the OGS's own: the status
// "Klassenfahrt" or, for one day with a return time, the children's arrival.

// ClassTripRequest is the body of POST /school/class-day/class-trips/{schoolClass}.
type ClassTripRequest struct {
	From       string  `json:"from"`
	To         string  `json:"to"`
	StudentIDs []int64 `json:"student_ids"`
	// ReturnTime is "HH:MM", only for a one-day outing.
	ReturnTime *string `json:"return_time,omitempty"`
	Reason     *string `json:"reason,omitempty"`

	returnTime *time.Time
}

// Bind implements render.Binder.
func (r *ClassTripRequest) Bind(_ *http.Request) error {
	if strings.TrimSpace(r.From) == "" || strings.TrimSpace(r.To) == "" {
		return errors.New("from and to are required")
	}
	if r.ReturnTime != nil {
		returnTime, err := time.Parse("15:04", strings.TrimSpace(*r.ReturnTime))
		if err != nil {
			return errors.New("invalid return_time format, expected HH:MM")
		}
		r.returnTime = &returnTime
	}
	if r.Reason != nil && utf8.RuneCountInString(*r.Reason) > 200 {
		return errors.New("reason cannot exceed 200 characters")
	}
	return nil
}

// classTripRejection carries a refused class trip as a business rejection:
// 409 with its code and details, and the request transaction rolls back the
// children already written.
type classTripRejection struct {
	error
	code    string
	details any
}

func (e classTripRejection) Unwrap() error     { return e.error }
func (e classTripRejection) ErrorCode() string { return e.code }
func (e classTripRejection) ErrorDetails() any { return e.details }

// classTripConflictDetails are the blocking days of a status conflict, the
// shape of the OGS's own conflict answer; nil without a named day.
type classTripConflictDetails struct {
	Conflicts     []classday.ClassTripConflict `json:"conflicts"`
	ConflictCount int                          `json:"conflict_count"`
}

func classTripStatusConflictDetails(err error) any {
	var conflict *classday.ClassTripStatusConflictError
	if !errors.As(err, &conflict) {
		return nil
	}
	return classTripConflictDetails{Conflicts: conflict.Conflicts, ConflictCount: conflict.Total}
}

var classTripErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: classday.ErrClassTripsDisabled, Render: func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, common.CodeClassdaySchoolWriteDisabled)
	}},
	{Target: classday.ErrClassTripPastDate, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeClassdayClassTripPastDate)
	}},
	{Target: classday.ErrClassTripInvalidRange, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeClassdayClassTripInvalidRange)
	}},
	{Target: classday.ErrClassTripReturnTimeMultiDay, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeClassdayClassTripReturnTimeMultiDay)
	}},
	{Target: classday.ErrClassTripReturnTimeWeekend, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeClassdayClassTripReturnTimeWeekend)
	}},
	{Target: classday.ErrClassTripNoStudents, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeClassdayClassTripNoStudents)
	}},
	{Target: classday.ErrClassTripStudentNotInClass, Render: func(err error) render.Renderer {
		return common.ErrorBusinessRejection(classTripRejection{error: err, code: common.CodeClassdayClassTripStudentNotInClass})
	}},
	{Target: classday.ErrClassTripStatusConflict, Render: func(err error) render.Renderer {
		return common.ErrorBusinessRejection(classTripRejection{
			error: err, code: common.CodeClassdayClassTripStatusConflict, details: classTripStatusConflictDetails(err),
		})
	}},
	{Target: classday.ErrClassTripArrivalConflict, Render: func(err error) render.Renderer {
		return common.ErrorBusinessRejection(classTripRejection{error: err, code: common.CodeClassdayClassTripArrivalConflict})
	}},
}, common.ErrorInternalServer)

// canReportClassTrip is the verdict the classes list carries: permission in
// the token AND the setting opened it up. Without the write seams the verdict
// stays false.
func (rs *Resource) canReportClassTrip(r *http.Request) (bool, error) {
	if !common.HasPermission(permissions.ClassDayClassTripWrite, sessiontoken.PermissionsFromCtx(r.Context())) {
		return false, nil
	}
	allowed, err := rs.ClassDay.MayReportClassTrips(r.Context())
	if errors.Is(err, classday.ErrClassTripsNotConfigured) {
		return false, nil
	}
	return allowed, err
}

// postClassTrip handles POST /school/class-day/class-trips/{schoolClass}.
func (rs *Resource) postClassTrip(w http.ResponseWriter, r *http.Request) {
	allowed, err := rs.ClassDay.MayReportClassTrips(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	if !allowed {
		common.RenderError(w, r, common.ErrorForbiddenWithCode(ErrSchoolWriteDisabled, common.CodeClassdaySchoolWriteDisabled))
		return
	}
	class, ok := rs.requireAssignedClass(w, r, chi.URLParam(r, "schoolClass"))
	if !ok {
		return
	}
	req := &ClassTripRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	from, ok := parseArrivalExceptionDate(w, r, req.From)
	if !ok {
		return
	}
	to, ok := parseArrivalExceptionDate(w, r, req.To)
	if !ok {
		return
	}
	staffID, ok := rs.requireCurrentStaffID(w, r)
	if !ok {
		return
	}
	in := classday.ClassTripWrite{
		SchoolClass: class,
		From:        classday.Date(from.String()),
		To:          classday.Date(to.String()),
		StudentIDs:  req.StudentIDs,
		Reason:      req.Reason,
		ReturnTime:  req.returnTime,
		CreatedBy:   staffID,
	}

	result, err := rs.ClassDay.ReportClassTrip(r.Context(), in)
	if err != nil {
		common.RenderError(w, r, classTripErrorRenderer(err))
		return
	}
	if result == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("class trip result was not returned")))
		return
	}
	common.Respond(w, r, http.StatusCreated, result, "Class trip reported successfully")
}
