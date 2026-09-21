package application

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// MarkDone closes an expired pickup request without applying a plan. The
// caller's tenant transaction contains the status, ledger, and durable notice.
func (s *RequestDecisions) MarkDone(ctx context.Context, id int64, version, reason string, reviewerID int64) error {
	if id <= 0 {
		return careplan.ErrCareScheduleRequestNotFound
	}
	request, err := s.loadAuthorized(ctx, id, version)
	if err != nil {
		return err
	}
	end := requestScopeEnd(request)
	if end.IsZero() || !end.Before(s.Today()) {
		return careplan.ErrParentRequestNotPast
	}
	trimmed := strings.TrimSpace(reason)
	if utf8.RuneCountInString(trimmed) > maxDecisionReasonRunes {
		return carerequests.ErrRejectReasonTooLong
	}
	var reasonPtr *string
	if trimmed != "" {
		reasonPtr = &trimmed
	}
	if err := s.Records.Decide(ctx, id, "done", reasonPtr, reviewerID, false); err != nil {
		return err
	}
	if err := s.Effects.RecordMarkedDone(ctx, request, reviewerID, trimmed); err != nil {
		return err
	}
	return s.Effects.NotifyMarkedDone(ctx, request, reviewerID, trimmed)
}

func requestScopeEnd(request *carerequests.Request) calendar.Date {
	if request.RequestKind != "pickup_change" {
		return ""
	}
	var payload struct {
		Date string `json:"date"`
	}
	if json.Unmarshal(request.Payload, &payload) != nil {
		return ""
	}
	date, err := calendar.ParseDate(payload.Date)
	if err != nil {
		return ""
	}
	return date
}
