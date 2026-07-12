package timetable

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

const (
	shiftCoverageInvalidRequestMessage = "invalid shift coverage request"
	shiftCoverageLoadErrorMessage      = "shift coverage could not be checked"
	maxShiftCoverageRequestBytes       = 64 << 10
)

// ShiftCoverageRequest is the read-only recurring/single-occurrence coverage
// probe payload. Dates are explicit candidate weekdays; period + week_pattern
// optionally reduce them to effective series occurrences.
type ShiftCoverageRequest struct {
	Dates                 []string `json:"dates"`
	StartTime             string   `json:"start_time"`
	EndTime               string   `json:"end_time"`
	StaffIDs              []int64  `json:"staff_ids"`
	ExcludeInstanceID     *int64   `json:"exclude_instance_id,omitempty"`
	ConcreteInstanceDate  *string  `json:"concrete_instance_date,omitempty"`
	ReplanActivityGroupID *int64   `json:"replan_activity_group_id,omitempty"`
	CalendarPeriodID      *int64   `json:"calendar_period_id,omitempty"`
	WeekPattern           *int     `json:"week_pattern,omitempty"`
}

// ShiftCoverageResponse deliberately contains no shift rows; callers receive
// only advisory uncovered intervals after passing both permissions.
type ShiftCoverageResponse struct {
	CoverageWarnings     []scheduleSvc.ShiftCoverageWarning `json:"coverage_warnings"`
	CoverageWarningCount int                                `json:"coverage_warning_count"`
}

func (rs *Resource) checkShiftCoverage(w http.ResponseWriter, r *http.Request) {
	if rs.TimetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New(shiftCoverageLoadErrorMessage)))
		return
	}

	var request ShiftCoverageRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxShiftCoverageRequestBytes)
	if err := render.DecodeJSON(r.Body, &request); err != nil {
		renderShiftCoverageBadRequest(w, r)
		return
	}
	dates := make([]timezone.Date, 0, len(request.Dates))
	for _, rawDate := range request.Dates {
		date, err := timezone.ParseDate(rawDate)
		if err != nil {
			renderShiftCoverageBadRequest(w, r)
			return
		}
		dates = append(dates, date)
	}
	start, err := parseClockTime(request.StartTime)
	if err != nil {
		renderShiftCoverageBadRequest(w, r)
		return
	}
	end, err := parseClockTime(request.EndTime)
	if err != nil {
		renderShiftCoverageBadRequest(w, r)
		return
	}
	var concreteInstanceDate *timezone.Date
	if request.ConcreteInstanceDate != nil {
		date, parseErr := timezone.ParseDate(*request.ConcreteInstanceDate)
		if parseErr != nil {
			renderShiftCoverageBadRequest(w, r)
			return
		}
		concreteInstanceDate = &date
	}

	result, err := rs.TimetableData.DetectShiftCoverageWarnings(r.Context(), scheduleSvc.ShiftCoverageQuery{
		Dates:                 dates,
		StartTime:             start,
		EndTime:               end,
		StaffIDs:              request.StaffIDs,
		ExcludeInstanceID:     request.ExcludeInstanceID,
		ConcreteInstanceDate:  concreteInstanceDate,
		ReplanActivityGroupID: request.ReplanActivityGroupID,
		CalendarPeriodID:      request.CalendarPeriodID,
		WeekPattern:           request.WeekPattern,
	})
	if err != nil {
		if errors.Is(err, scheduleSvc.ErrInvalidShiftCoverageQuery) {
			renderShiftCoverageBadRequest(w, r)
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap(shiftCoverageLoadErrorMessage, err))
		return
	}
	if result.Warnings == nil {
		result.Warnings = make([]scheduleSvc.ShiftCoverageWarning, 0)
	}
	common.Respond(w, r, http.StatusOK, ShiftCoverageResponse{
		CoverageWarnings:     result.Warnings,
		CoverageWarningCount: result.TotalWarningCount,
	}, "Shift coverage checked")
}

func renderShiftCoverageBadRequest(w http.ResponseWriter, r *http.Request) {
	common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(shiftCoverageInvalidRequestMessage)))
}
