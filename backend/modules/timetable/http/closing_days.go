package timetablehttp

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ClosingDayRequest represents a create/update request for a closing day
// range (#1418 3b).
type ClosingDayRequest struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Reason    string `json:"reason"`
}

// Bind validates the request
func (req *ClosingDayRequest) Bind(_ *http.Request) error {
	req.Reason = strings.TrimSpace(req.Reason)
	invalid := common.CodeTimetableClosingDayInvalid
	if req.Reason == "" {
		return invalidField(invalid, "reason", "reason is required")
	}
	if utf8.RuneCountInString(req.Reason) > schoolcalendar.ClosingDayReasonMaxLength {
		return invalidField(invalid, "reason", "reason cannot exceed 255 characters")
	}
	if req.StartDate == "" {
		return invalidField(invalid, "start_date", "start_date is required")
	}
	if req.EndDate == "" {
		return invalidField(invalid, "end_date", "end_date is required")
	}
	return nil
}

// ClosingDayResponse represents a closing day in API responses
type ClosingDayResponse struct {
	ID        int64  `json:"id"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Reason    string `json:"reason"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func mapClosingDayToResponse(d schoolcalendar.ClosingDay) ClosingDayResponse {
	return ClosingDayResponse{
		ID:        d.ID,
		StartDate: d.StartDate,
		EndDate:   d.EndDate,
		Reason:    d.Reason,
		CreatedAt: d.CreatedAt.Format(time.RFC3339),
		UpdatedAt: d.UpdatedAt.Format(time.RFC3339),
	}
}

// parseClosingDayDates extracts start_date and end_date from a request.
// Returns parsed calendar dates and true on success, or renders an error and
// returns false. A single closed day is a range with start_date = end_date,
// hence Before (not !After) in the order check.
func parseClosingDayDates(w http.ResponseWriter, r *http.Request, req *ClosingDayRequest) (startDate, endDate calendar.Date, ok bool) {
	var err error
	startDate, err = calendar.ParseDate(req.StartDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid start_date format, expected YYYY-MM-DD")))
		return calendar.Date(""), calendar.Date(""), false
	}

	endDate, err = calendar.ParseDate(req.EndDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid end_date format, expected YYYY-MM-DD")))
		return calendar.Date(""), calendar.Date(""), false
	}

	if endDate.Before(startDate) {
		common.RenderError(w, r, invalidOnField(common.CodeTimetableClosingDayEndBeforeStart, "end_date", "end_date must not be before start_date"))
		return calendar.Date(""), calendar.Date(""), false
	}

	return startDate, endDate, true
}

func (rs *Resource) listClosingDays(w http.ResponseWriter, r *http.Request) {
	days, err := rs.ClosingDays.ListClosingDays(r.Context(), schoolcalendar.ClosingDayFilter{})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("Schließtage konnten nicht geladen werden", err))
		return
	}

	responses := make([]ClosingDayResponse, len(days))
	for i, d := range days {
		responses[i] = mapClosingDayToResponse(d)
	}

	common.Respond(w, r, http.StatusOK, responses, "Closing days retrieved successfully")
}

func (rs *Resource) createClosingDay(w http.ResponseWriter, r *http.Request) {
	req := &ClosingDayRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, bindErrorRenderer(err))
		return
	}

	startDate, endDate, ok := parseClosingDayDates(w, r, req)
	if !ok {
		return
	}

	day, err := rs.ClosingDays.CreateClosingDay(r.Context(), schoolcalendar.CreateClosingDay{ClosingDayFields: schoolcalendar.ClosingDayFields{
		StartDate: startDate.String(),
		EndDate:   endDate.String(),
		Reason:    req.Reason,
	}})
	if err != nil {
		common.RenderError(w, r, closingDayWriteErrorRenderer(err, "Schließtag konnte nicht angelegt werden"))
		return
	}

	common.Respond(w, r, http.StatusCreated, mapClosingDayToResponse(day), "Closing day created successfully")
}

func (rs *Resource) updateClosingDay(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid closing day ID")))
		return
	}

	req := &ClosingDayRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, bindErrorRenderer(err))
		return
	}

	startDate, endDate, ok := parseClosingDayDates(w, r, req)
	if !ok {
		return
	}

	day, err := rs.ClosingDays.FindClosingDay(r.Context(), id)
	if err != nil {
		if errors.Is(err, schoolcalendar.ErrClosingDayNotFound) {
			common.RenderError(w, r, closingDayNotFound())
		} else {
			common.RenderError(w, r, common.ErrorInternalServerWrap("Schließtag konnte nicht geladen werden", err))
		}
		return
	}

	updated, err := rs.ClosingDays.UpdateClosingDay(r.Context(), schoolcalendar.UpdateClosingDay{ID: day.ID, ClosingDayFields: schoolcalendar.ClosingDayFields{
		StartDate: startDate.String(),
		EndDate:   endDate.String(),
		Reason:    req.Reason,
	}})
	if err != nil {
		common.RenderError(w, r, closingDayWriteErrorRenderer(err, "Schließtag konnte nicht aktualisiert werden"))
		return
	}

	common.Respond(w, r, http.StatusOK, mapClosingDayToResponse(updated), "Closing day updated successfully")
}

func (rs *Resource) deleteClosingDay(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid closing day ID")))
		return
	}

	if _, err := rs.ClosingDays.FindClosingDay(r.Context(), id); err != nil {
		if errors.Is(err, schoolcalendar.ErrClosingDayNotFound) {
			common.RenderError(w, r, closingDayNotFound())
		} else {
			common.RenderError(w, r, common.ErrorInternalServerWrap("Schließtag konnte nicht geladen werden", err))
		}
		return
	}

	if err := rs.ClosingDays.DeleteClosingDay(r.Context(), id); err != nil {
		common.RenderError(w, r, closingDayWriteErrorRenderer(err, "Schließtag konnte nicht gelöscht werden"))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Closing day deleted successfully")
}

// closingDayNotFound answers a closing day that is gone (#2516).
func closingDayNotFound() render.Renderer {
	return common.ErrorNotFoundWithCode(schoolcalendar.ErrClosingDayNotFound, common.CodeTimetableClosingDayNotFound)
}

// closingDayWriteErrorRenderer classifies a refused closing-day write
// (#2516): an invalid range is the caller's input and a vanished row a stale
// page, neither a server error.
func closingDayWriteErrorRenderer(err error, serverMsg string) render.Renderer {
	switch {
	case errors.Is(err, schoolcalendar.ErrInvalidClosingDay):
		return common.ErrorInvalidRequestWithCode(err, common.CodeTimetableClosingDayInvalid)
	case errors.Is(err, schoolcalendar.ErrClosingDayNotFound):
		return closingDayNotFound()
	default:
		return common.ErrorInternalServerWrap(serverMsg, err)
	}
}
