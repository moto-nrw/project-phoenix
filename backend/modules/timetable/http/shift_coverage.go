package timetablehttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	CoverageWarnings     []timetable.ShiftCoverageWarning `json:"coverage_warnings"`
	CoverageWarningCount int                              `json:"coverage_warning_count"`
}

func (rs *Resource) checkShiftCoverage(w http.ResponseWriter, r *http.Request) {
	if rs.ConflictDetection == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New(shiftCoverageLoadErrorMessage)))
		return
	}

	var request ShiftCoverageRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxShiftCoverageRequestBytes)
	if err := render.DecodeJSON(r.Body, &request); err != nil {
		renderShiftCoverageBadRequest(w, r)
		return
	}
	probe, err := parseShiftCoverageProbe(request)
	if err != nil {
		renderShiftCoverageBadRequest(w, r)
		return
	}

	result, err := rs.ConflictDetection.DetectShiftCoverage(r.Context(), probe)
	if err != nil {
		if errors.Is(err, timetable.ErrInvalidShiftCoverageQuery) {
			renderShiftCoverageBadRequest(w, r)
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap(shiftCoverageLoadErrorMessage, err))
		return
	}
	if result.Warnings == nil {
		result.Warnings = make([]timetable.ShiftCoverageWarning, 0)
	}
	common.Respond(w, r, http.StatusOK, ShiftCoverageResponse{
		CoverageWarnings:     result.Warnings,
		CoverageWarningCount: result.TotalWarningCount,
	}, "Shift coverage checked")
}

// parseShiftCoverageProbe reads the dates and the clock window of the probe;
// any malformed value is an error.
func parseShiftCoverageProbe(request ShiftCoverageRequest) (timetable.ShiftCoverageProbe, error) {
	dates := make([]calendar.Date, 0, len(request.Dates))
	for _, rawDate := range request.Dates {
		date, err := calendar.ParseDate(rawDate)
		if err != nil {
			return timetable.ShiftCoverageProbe{}, err
		}
		dates = append(dates, date)
	}
	start, err := parseClockTime(request.StartTime)
	if err != nil {
		return timetable.ShiftCoverageProbe{}, err
	}
	end, err := parseClockTime(request.EndTime)
	if err != nil {
		return timetable.ShiftCoverageProbe{}, err
	}
	var concreteInstanceDate *calendar.Date
	if request.ConcreteInstanceDate != nil {
		date, parseErr := calendar.ParseDate(*request.ConcreteInstanceDate)
		if parseErr != nil {
			return timetable.ShiftCoverageProbe{}, parseErr
		}
		concreteInstanceDate = &date
	}
	return timetable.ShiftCoverageProbe{
		Dates:                 dates,
		StartTime:             start,
		EndTime:               end,
		StaffIDs:              request.StaffIDs,
		ExcludeInstanceID:     request.ExcludeInstanceID,
		ConcreteInstanceDate:  concreteInstanceDate,
		ReplanActivityGroupID: request.ReplanActivityGroupID,
		CalendarPeriodID:      request.CalendarPeriodID,
		WeekPattern:           request.WeekPattern,
	}, nil
}

func renderShiftCoverageBadRequest(w http.ResponseWriter, r *http.Request) {
	common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(shiftCoverageInvalidRequestMessage)))
}
