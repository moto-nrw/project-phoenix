package timetable

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
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
	if req.Reason == "" {
		return errors.New("reason is required")
	}
	if utf8.RuneCountInString(req.Reason) > schedule.ClosingDayReasonMaxLength {
		return errors.New("reason cannot exceed 255 characters")
	}
	if req.StartDate == "" {
		return errors.New("start_date is required")
	}
	if req.EndDate == "" {
		return errors.New("end_date is required")
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

func mapClosingDayToResponse(d *schedule.ClosingDay) ClosingDayResponse {
	return ClosingDayResponse{
		ID:        d.ID,
		StartDate: d.StartDate.String(),
		EndDate:   d.EndDate.String(),
		Reason:    d.Reason,
		CreatedAt: d.CreatedAt.Format(time.RFC3339),
		UpdatedAt: d.UpdatedAt.Format(time.RFC3339),
	}
}

// parseClosingDayDates extracts start_date and end_date from a request.
// Returns parsed calendar dates and true on success, or renders an error and
// returns false. A single closed day is a range with start_date = end_date,
// hence Before (not !After) in the order check.
func parseClosingDayDates(w http.ResponseWriter, r *http.Request, req *ClosingDayRequest) (startDate, endDate timezone.Date, ok bool) {
	var err error
	startDate, err = timezone.ParseDate(req.StartDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid start_date format, expected YYYY-MM-DD")))
		return timezone.Date(""), timezone.Date(""), false
	}

	endDate, err = timezone.ParseDate(req.EndDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid end_date format, expected YYYY-MM-DD")))
		return timezone.Date(""), timezone.Date(""), false
	}

	if endDate.Before(startDate) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("end_date must not be before start_date")))
		return timezone.Date(""), timezone.Date(""), false
	}

	return startDate, endDate, true
}

func (rs *Resource) listClosingDays(w http.ResponseWriter, r *http.Request) {
	days, err := rs.ClosingDayService.GetAll(r.Context())
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
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	startDate, endDate, ok := parseClosingDayDates(w, r, req)
	if !ok {
		return
	}

	day := &schedule.ClosingDay{
		StartDate: startDate,
		EndDate:   endDate,
		Reason:    req.Reason,
	}

	if err := rs.ClosingDayService.Create(r.Context(), day); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("Schließtag konnte nicht angelegt werden", err))
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
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	startDate, endDate, ok := parseClosingDayDates(w, r, req)
	if !ok {
		return
	}

	day, err := rs.ClosingDayService.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("closing day not found")))
		} else {
			common.RenderError(w, r, common.ErrorInternalServerWrap("Schließtag konnte nicht geladen werden", err))
		}
		return
	}

	day.StartDate = startDate
	day.EndDate = endDate
	day.Reason = req.Reason

	if err := rs.ClosingDayService.Update(r.Context(), day); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("Schließtag konnte nicht aktualisiert werden", err))
		return
	}

	common.Respond(w, r, http.StatusOK, mapClosingDayToResponse(day), "Closing day updated successfully")
}

func (rs *Resource) deleteClosingDay(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid closing day ID")))
		return
	}

	if _, err := rs.ClosingDayService.GetByID(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("closing day not found")))
		} else {
			common.RenderError(w, r, common.ErrorInternalServerWrap("Schließtag konnte nicht geladen werden", err))
		}
		return
	}

	if err := rs.ClosingDayService.Delete(r.Context(), id); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("Schließtag konnte nicht gelöscht werden", err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Closing day deleted successfully")
}
