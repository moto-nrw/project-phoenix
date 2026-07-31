package staffshifts

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// occurrenceShiftID accepts legacy JSON numbers as well as the lossless
// decimal-string representation used by the frontend. JavaScript numbers
// cannot faithfully represent every valid PostgreSQL bigint ID.
type occurrenceShiftID int64

func (id *occurrenceShiftID) UnmarshalJSON(data []byte) error {
	var encoded string
	if err := json.Unmarshal(data, &encoded); err == nil {
		parsed, err := strconv.ParseInt(encoded, 10, 64)
		if err != nil {
			return errors.New("occurrence_shift_id must be an int64")
		}
		*id = occurrenceShiftID(parsed)
		return nil
	}

	var parsed int64
	if err := json.Unmarshal(data, &parsed); err != nil {
		return errors.New("occurrence_shift_id must be an int64")
	}
	*id = occurrenceShiftID(parsed)
	return nil
}

// SeriesRequest is the create payload for a recurring shift series (#1889).
// Weekdays are ISO (1=Monday … 7=Sunday); week_pattern is 0 = every week,
// 1 = week A, 2 = week B. valid_from/valid_until bound the series within its
// calendar period (valid_until exclusive; omitted = until period end). The
// split payload reuses this shape plus effective_date; staff_id, the period,
// and the validity bounds stay with the series lineage there.
type SeriesRequest struct {
	StaffID          int64      `json:"staff_id"`
	Weekdays         []int      `json:"weekdays"` // omitted on split = keep predecessor weekdays
	StartTime        string     `json:"start_time"`
	EndTime          string     `json:"end_time"`
	BreakMinutes     int        `json:"break_minutes"`
	ShiftTypeID      optionalID `json:"shift_type_id"`
	Notes            *string    `json:"notes"`
	CalendarPeriodID int64      `json:"calendar_period_id"`
	WeekPattern      *int       `json:"week_pattern"` // omitted: create = every week, split = keep predecessor
	ValidFrom        string     `json:"valid_from"`
	// ValidUntil is presence-aware on split (#2028): an omitted key keeps the
	// predecessor's end, an explicit null lets the series run to the period end.
	ValidUntil    optionalString `json:"valid_until"`
	EffectiveDate string         `json:"effective_date"`
	// OccurrenceShiftID is the concrete row opened by the planner. For a
	// permanent edit effective today it is updated in place before the series
	// is re-planned from tomorrow.
	OccurrenceShiftID occurrenceShiftID `json:"occurrence_shift_id"`
}

// SeriesResponse is the wire format for series create/split/end results.
// Skipped dates are days the series left out because an existing shift of
// the same person would overlap there.
type SeriesResponse struct {
	SeriesID     int64    `json:"series_id"`
	OldSeriesID  int64    `json:"old_series_id,omitempty"`
	Created      int      `json:"created"`
	Deleted      int64    `json:"deleted"`
	SkippedDates []string `json:"skipped_dates"`
}

func toSeriesResponse(result *scheduleSvc.SeriesResult) SeriesResponse {
	skipped := make([]string, 0, len(result.SkippedDates))
	for _, d := range result.SkippedDates {
		skipped = append(skipped, d.String())
	}
	resp := SeriesResponse{
		OldSeriesID:  result.OldSeriesID,
		Created:      result.Created,
		Deleted:      result.Deleted,
		SkippedDates: skipped,
	}
	if result.Series != nil {
		resp.SeriesID = result.Series.ID
	}
	return resp
}

func toWeekdays(values []int) ([]int16, error) {
	if values == nil {
		return nil, nil // split: keep predecessor weekdays
	}
	out := make([]int16, 0, len(values))
	for _, v := range values {
		// Bounds check BEFORE the int16 conversion: without it an
		// out-of-range value like 65538 silently wraps to a valid weekday.
		if v < 1 || v > 7 {
			return nil, errors.New("weekdays must be between 1 (Monday) and 7 (Sunday)")
		}
		out = append(out, int16(v))
	}
	return out, nil
}

// SeriesDetailResponse is the stored rule behind a shift: what the series
// editor loads before writing the change back through the split (#2028). Times are "HH:MM"
// wall-clock, dates "YYYY-MM-DD"; valid_until is exclusive, as everywhere in
// the series API.
type SeriesDetailResponse struct {
	ID               int64   `json:"id"`
	StaffID          int64   `json:"staff_id"`
	Weekdays         []int   `json:"weekdays"`
	StartTime        string  `json:"start_time"`
	EndTime          string  `json:"end_time"`
	BreakMinutes     int     `json:"break_minutes"`
	ShiftTypeID      *int64  `json:"shift_type_id"`
	Notes            string  `json:"notes,omitempty"`
	CalendarPeriodID int64   `json:"calendar_period_id"`
	WeekPattern      int     `json:"week_pattern"`
	ValidFrom        string  `json:"valid_from"`
	ValidUntil       *string `json:"valid_until"`
}

func toSeriesDetailResponse(series *scheduleModels.StaffShiftSeries) SeriesDetailResponse {
	weekdays := make([]int, 0, len(series.Weekdays))
	for _, wd := range series.Weekdays {
		weekdays = append(weekdays, int(wd))
	}
	resp := SeriesDetailResponse{
		ID:               series.ID,
		StaffID:          series.StaffID,
		Weekdays:         weekdays,
		StartTime:        timezone.WallClock(series.StartTime).Format("15:04"),
		EndTime:          timezone.WallClock(series.EndTime).Format("15:04"),
		BreakMinutes:     series.BreakMinutes,
		ShiftTypeID:      series.ShiftTypeID,
		Notes:            series.Notes,
		CalendarPeriodID: series.CalendarPeriodID,
		WeekPattern:      series.WeekPattern,
		ValidFrom:        series.ValidFrom.String(),
	}
	if series.ValidUntil != nil {
		until := series.ValidUntil.String()
		resp.ValidUntil = &until
	}
	return resp
}

// parseOptionalValidUntil turns the presence-aware payload field into a date.
// An absent key and an explicit null both mean "no end date" here; the split
// handler additionally reads Present to tell "clear the end" from "keep the
// predecessor's end".
func parseOptionalValidUntil(raw optionalString) (*timezone.Date, error) {
	if raw.Value == nil || *raw.Value == "" {
		return nil, nil
	}
	parsed, err := timezone.ParseDate(*raw.Value)
	if err != nil {
		return nil, errors.New("valid_until must be YYYY-MM-DD")
	}
	return &parsed, nil
}

func renderSeriesServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, scheduleSvc.ErrSeriesNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, scheduleSvc.ErrSeriesInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		renderServiceError(w, r, err)
	}
}

func (rs *Resource) buildSeries(req SeriesRequest) (*scheduleModels.StaffShiftSeries, error) {
	validFrom, err := timezone.ParseDate(req.ValidFrom)
	if err != nil {
		return nil, errors.New("valid_from must be YYYY-MM-DD")
	}
	validUntil, err := parseOptionalValidUntil(req.ValidUntil)
	if err != nil {
		return nil, err
	}
	start, end, err := ParseShiftTimes(req.StartTime, req.EndTime)
	if err != nil {
		return nil, err
	}
	notes := ""
	if req.Notes != nil {
		notes = *req.Notes
	}
	weekPattern := scheduleModels.WeekPatternEvery
	if req.WeekPattern != nil {
		weekPattern = *req.WeekPattern
	}
	weekdays, err := toWeekdays(req.Weekdays)
	if err != nil {
		return nil, err
	}
	return &scheduleModels.StaffShiftSeries{
		StaffID:          req.StaffID,
		Weekdays:         weekdays,
		StartTime:        start,
		EndTime:          end,
		BreakMinutes:     req.BreakMinutes,
		ShiftTypeID:      req.ShiftTypeID.Value,
		Notes:            notes,
		CalendarPeriodID: req.CalendarPeriodID,
		WeekPattern:      weekPattern,
		ValidFrom:        validFrom,
		ValidUntil:       validUntil,
	}, nil
}

func (rs *Resource) createSeries(w http.ResponseWriter, r *http.Request) {
	var req SeriesRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	series, err := rs.buildSeries(req)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	editorID, err := rs.editorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	series.CreatedBy = editorID

	result, err := rs.SeriesService.CreateSeries(r.Context(), series)
	if err != nil {
		renderSeriesServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toSeriesResponse(result), "Staff shift series created")
}

func (rs *Resource) splitSeries(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	var req SeriesRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	effective, err := timezone.ParseDate(req.EffectiveDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("effective_date must be YYYY-MM-DD")))
		return
	}
	start, end, err := ParseShiftTimes(req.StartTime, req.EndTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	weekdays, err := toWeekdays(req.Weekdays)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	validUntil, err := parseOptionalValidUntil(req.ValidUntil)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	editorID, err := rs.editorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	result, err := rs.SeriesService.SplitSeries(r.Context(), scheduleSvc.SplitSeriesInput{
		SeriesID:          id,
		EffectiveDate:     effective,
		OccurrenceShiftID: int64(req.OccurrenceShiftID),
		Weekdays:          weekdays,
		StartTime:         start,
		EndTime:           end,
		BreakMinutes:      req.BreakMinutes,
		ShiftTypeID:       req.ShiftTypeID.Value,
		ShiftTypeIDSet:    req.ShiftTypeID.Present,
		Notes:             req.Notes,
		ValidUntil:        validUntil,
		ValidUntilSet:     req.ValidUntil.Present,
		WeekPattern:       req.WeekPattern,
		ActorStaffID:      editorID,
	})
	if err != nil {
		renderSeriesServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toSeriesResponse(result), "Staff shift series split")
}

// getSeries returns the rule behind a shift so the planner can edit the whole
// series (weekdays, rhythm, validity), not just one occurrence (#2028).
func (rs *Resource) getSeries(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	series, err := rs.SeriesService.GetSeries(r.Context(), id)
	if err != nil {
		renderSeriesServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toSeriesDetailResponse(series), "Staff shift series retrieved")
}

func (rs *Resource) endSeries(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	fromStr := r.URL.Query().Get("from")
	if fromStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("from query parameter is required")))
		return
	}
	from, err := timezone.ParseDate(fromStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid from date format, expected YYYY-MM-DD")))
		return
	}
	result, err := rs.SeriesService.EndSeries(r.Context(), id, from)
	if err != nil {
		renderSeriesServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toSeriesResponse(result), "Staff shift series ended")
}
