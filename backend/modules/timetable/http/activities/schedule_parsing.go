package httpadapter

import (
	"errors"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
)

// SchedulesResource defines the schedules API resource
func (rs *SchedulesResource) parseDateframeDates(w http.ResponseWriter, r *http.Request, startStr, endStr string) (time.Time, time.Time, bool) {
	startDate, err := time.Parse(scheduleDateLayout, startStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidStartDate)))
		return time.Time{}, time.Time{}, false
	}

	endDate, err := time.Parse(scheduleDateLayout, endStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidEndDate)))
		return time.Time{}, time.Time{}, false
	}

	return startDate, endDate, true
}

// parseTimeframeTimes parses and validates start time and optional end time, handling errors internally
func (rs *SchedulesResource) parseTimeframeTimes(w http.ResponseWriter, r *http.Request, startStr string, endStr *string) (time.Time, *time.Time, bool) {
	startTime, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidStartTime)))
		return time.Time{}, nil, false
	}

	if endStr != nil {
		endTime, err := time.Parse(time.RFC3339, *endStr)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidEndTime)))
			return time.Time{}, nil, false
		}
		return startTime, &endTime, true
	}

	return startTime, nil, true
}

// parseOptionalEndDate parses and validates an optional end date, handling errors internally
func (rs *SchedulesResource) parseOptionalEndDate(w http.ResponseWriter, r *http.Request, endDateStr *string) (*time.Time, bool) {
	if endDateStr == nil {
		return nil, true
	}

	endDate, err := time.Parse(scheduleDateLayout, *endDateStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidEndDate)))
		return nil, false
	}

	return &endDate, true
}

func newDateframeResponse(dateframe schoolcalendar.Dateframe) DateframeResponse {
	return DateframeResponse{
		ID:          dateframe.ID,
		StartDate:   common.Time(dateframe.StartDate),
		EndDate:     common.Time(dateframe.EndDate),
		Name:        dateframe.Name,
		Description: dateframe.Description,
		CreatedAt:   common.Time(dateframe.CreatedAt),
		UpdatedAt:   common.Time(dateframe.UpdatedAt),
	}
}

func newTimeframeResponse(timeframe timeframeSlot) TimeframeResponse {
	resp := TimeframeResponse{
		ID:          timeframe.ID,
		StartTime:   common.Time(timeframe.StartTime),
		IsActive:    timeframe.IsActive,
		Description: timeframe.Description,
		CreatedAt:   common.Time(timeframe.CreatedAt),
		UpdatedAt:   common.Time(timeframe.UpdatedAt),
	}

	if timeframe.EndTime != nil {
		endTime := common.Time(*timeframe.EndTime)
		resp.EndTime = &endTime
	}

	return resp
}

func newRecurrenceRuleResponse(rule timetableModule.RecurrenceRule) RecurrenceRuleResponse {
	resp := RecurrenceRuleResponse{
		ID:            rule.ID,
		Frequency:     rule.Frequency,
		IntervalCount: rule.IntervalCount,
		Weekdays:      rule.Weekdays,
		MonthDays:     rule.MonthDays,
		Count:         rule.Count,
		CreatedAt:     common.Time(rule.CreatedAt),
		UpdatedAt:     common.Time(rule.UpdatedAt),
	}

	if rule.EndDate != nil {
		endDate := common.Time(*rule.EndDate)
		resp.EndDate = &endDate
	}

	return resp
}

// Dateframe endpoints
