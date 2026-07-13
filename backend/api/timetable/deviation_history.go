// Package timetable — Änderungsprotokoll read endpoint (#1886).
//
//	GET /api/timetable/deviations/history?date=YYYY-MM-DD&date_to=YYYY-MM-DD
//	    [&activity_group_id=N&start_time=HH:MM]
//
// Returns the append-only deviation protocol for the requested window, newest
// first. Display names for staff and the acting account are resolved here at
// read time — the audit rows store IDs only (GDPR). A missing staff/account
// (offboarded, deleted) yields a nil name; the frontend renders a fallback.
//
// Permission: SchedulesRead.
package timetable

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
)

// DeviationHistoryEvent is one protocol row in the response.
type DeviationHistoryEvent struct {
	ID               int64           `json:"id"`
	ActivityGroupID  *int64          `json:"activity_group_id,omitempty"`
	OccurrenceDate   string          `json:"occurrence_date"`
	StartTime        string          `json:"start_time"`
	InstanceID       *int64          `json:"instance_id,omitempty"`
	EventType        string          `json:"event_type"`
	SubjectStaffID   *int64          `json:"subject_staff_id,omitempty"`
	SubjectStaffName *string         `json:"subject_staff_name,omitempty"`
	RelatedStaffID   *int64          `json:"related_staff_id,omitempty"`
	RelatedStaffName *string         `json:"related_staff_name,omitempty"`
	ActorAccountID   *int64          `json:"actor_account_id,omitempty"`
	ActorName        *string         `json:"actor_name,omitempty"`
	OldValue         json.RawMessage `json:"old_value,omitempty"`
	NewValue         json.RawMessage `json:"new_value,omitempty"`
	Reason           *string         `json:"reason,omitempty"`
	OccurredAt       string          `json:"occurred_at"` // RFC3339
}

// DeviationHistoryResponse is the 200 body.
type DeviationHistoryResponse struct {
	Events []DeviationHistoryEvent `json:"events"`
}

// deviationHistoryQuery is the parsed filter set of GET /deviations/history.
type deviationHistoryQuery struct {
	from, to        timezone.Date
	activityGroupID *int64
	startTime       *string
}

// parseDeviationHistoryQuery validates the query params; a non-nil renderer
// is the 400 to send.
func parseDeviationHistoryQuery(q url.Values) (deviationHistoryQuery, render.Renderer) {
	var out deviationHistoryQuery
	from, err := berlinDate(q.Get("date"))
	if err != nil {
		return out, common.ErrorInvalidRequest(errors.New("invalid date format, expected YYYY-MM-DD"))
	}
	to := from
	if toStr := q.Get("date_to"); toStr != "" {
		if to, err = berlinDate(toStr); err != nil {
			return out, common.ErrorInvalidRequest(errors.New("invalid date_to format, expected YYYY-MM-DD"))
		}
	}
	if to.Before(from) {
		return out, common.ErrorInvalidRequest(errors.New("'date' must be before or equal to 'date_to'"))
	}
	if inclusiveDayCount(from, to) > maxInstanceListRangeDays {
		return out, common.ErrorInvalidRequest(errors.New("date range is too large"))
	}
	out.from, out.to = from, to

	if raw := q.Get("activity_group_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			return out, common.ErrorInvalidRequest(errors.New("activity_group_id must be a positive id"))
		}
		out.activityGroupID = &id
	}
	if raw := q.Get("start_time"); raw != "" {
		if len(raw) != 5 || raw[2] != ':' {
			return out, common.ErrorInvalidRequest(errors.New("start_time must be HH:MM"))
		}
		out.startTime = &raw
	}
	return out, nil
}

// getDeviationHistory handles GET /api/timetable/deviations/history.
func (rs *Resource) getDeviationHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query, rndr := parseDeviationHistoryQuery(r.URL.Query())
	if rndr != nil {
		common.RenderError(w, r, rndr)
		return
	}

	events, err := rs.TimetableData.ListDeviationEvents(ctx, query.from, query.to, query.activityGroupID, query.startTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load deviation history failed", err))
		return
	}

	staffNames, actorNames := rs.resolveDeviationNames(ctx, events)

	out := make([]DeviationHistoryEvent, 0, len(events))
	for _, ev := range events {
		out = append(out, DeviationHistoryEvent{
			ID:               ev.ID,
			ActivityGroupID:  ev.ActivityGroupID,
			OccurrenceDate:   ev.OccurrenceDate.String(),
			StartTime:        ev.StartTime.Format("15:04"),
			InstanceID:       ev.InstanceID,
			EventType:        ev.EventType,
			SubjectStaffID:   ev.SubjectStaffID,
			SubjectStaffName: lookupName(staffNames, ev.SubjectStaffID),
			RelatedStaffID:   ev.RelatedStaffID,
			RelatedStaffName: lookupName(staffNames, ev.RelatedStaffID),
			ActorAccountID:   ev.ActorAccountID,
			ActorName:        lookupName(actorNames, ev.ActorAccountID),
			OldValue:         ev.OldValue,
			NewValue:         ev.NewValue,
			Reason:           ev.Reason,
			OccurredAt:       ev.OccurredAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	common.Respond(w, r, http.StatusOK, DeviationHistoryResponse{Events: out}, "Deviation history retrieved")
}

// resolveDeviationNames batch-resolves display names for every distinct staff
// and actor account referenced by the events. Best effort: a missing person
// simply stays unnamed (frontend renders a fallback); only per-ID lookups run,
// bounded by the distinct IDs in the page.
func (rs *Resource) resolveDeviationNames(ctx context.Context, events []*auditModel.DeviationEvent) (staffNames, actorNames map[int64]string) {
	staffIDs := make(map[int64]bool)
	actorIDs := make(map[int64]bool)
	for _, ev := range events {
		if ev.SubjectStaffID != nil {
			staffIDs[*ev.SubjectStaffID] = true
		}
		if ev.RelatedStaffID != nil {
			staffIDs[*ev.RelatedStaffID] = true
		}
		if ev.ActorAccountID != nil {
			actorIDs[*ev.ActorAccountID] = true
		}
	}

	return rs.resolveStaffDisplayNames(ctx, staffIDs), rs.resolveActorDisplayNames(ctx, actorIDs)
}

// resolveStaffDisplayNames maps staff IDs to person display names; unknown
// staff are simply absent from the map.
func (rs *Resource) resolveStaffDisplayNames(ctx context.Context, staffIDs map[int64]bool) map[int64]string {
	names := make(map[int64]string, len(staffIDs))
	personIDByStaff := make(map[int64]int64, len(staffIDs))
	personIDs := make([]int64, 0, len(staffIDs))
	for staffID := range staffIDs {
		staff, err := rs.PersonService.GetStaffByID(ctx, staffID)
		if err != nil || staff == nil || staff.ID == 0 {
			continue
		}
		personIDByStaff[staffID] = staff.PersonID
		personIDs = append(personIDs, staff.PersonID)
	}
	persons, err := rs.PersonService.GetByIDs(ctx, personIDs)
	if err != nil {
		return names
	}
	for staffID, personID := range personIDByStaff {
		if p := persons[personID]; p != nil {
			names[staffID] = p.FirstName + " " + p.LastName
		}
	}
	return names
}

// resolveActorDisplayNames maps account IDs to person display names; deleted
// or person-less accounts are simply absent from the map.
func (rs *Resource) resolveActorDisplayNames(ctx context.Context, actorIDs map[int64]bool) map[int64]string {
	names := make(map[int64]string, len(actorIDs))
	for accountID := range actorIDs {
		person, err := rs.PersonService.FindByAccountID(ctx, accountID)
		if err != nil || person == nil {
			continue
		}
		names[accountID] = person.FirstName + " " + person.LastName
	}
	return names
}

// lookupName returns a pointer into the resolved-name map, nil when unknown.
func lookupName(names map[int64]string, id *int64) *string {
	if id == nil {
		return nil
	}
	if name, ok := names[*id]; ok {
		return &name
	}
	return nil
}
