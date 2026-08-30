package students

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// OfferingRequestResponse is the staff-facing projection of one parent
// offering-change request in the review queue, with the live
// "current → requested" diff (#1665).
type OfferingRequestResponse struct {
	ID          string `json:"id"`
	StudentID   string `json:"student_id"`
	StudentName string `json:"student_name"`
	Status      string `json:"status"`
	// EffectiveFrom is the date the switch would take effect (YYYY-MM-DD).
	EffectiveFrom string `json:"effective_from"`
	// EarliestEffectiveFrom / LatestEffectiveFrom bound the date staff may
	// confirm the switch for (#2484), so the review card cannot offer a date the
	// approval refuses. Omitted when the care period could not be resolved.
	EarliestEffectiveFrom string `json:"earliest_effective_from,omitempty"`
	LatestEffectiveFrom   string `json:"latest_effective_from,omitempty"`
	// RequestedEffectiveFrom is the date the family asked for, sent only when it
	// is not the date the queue offers — a request whose date passed while it
	// waited applies at the earliest date left instead (#2484).
	RequestedEffectiveFrom string                        `json:"requested_effective_from,omitempty"`
	Note                   string                        `json:"note,omitempty"`
	Diff                   []OfferingRequestDiffResponse `json:"diff"`
	// Unchanged lists the bookings the request leaves as they are, so the review
	// card shows the child's complete picture, not only the changed lines (#2434).
	Unchanged []OfferingRequestUnchangedResponse `json:"unchanged,omitempty"`
	// FullWithdrawal marks a Komplett-Abmeldung: approving would leave the child
	// without any offering at all (#2434).
	FullWithdrawal bool       `json:"full_withdrawal,omitempty"`
	Reason         *string    `json:"reason,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitempty"`
}

// OfferingRequestDiffResponse is one German-localized diff line for the
// German-only staff portal.
type OfferingRequestDiffResponse struct {
	OfferingID string `json:"offering_id"`
	Label      string `json:"label"`
	Old        string `json:"old"`
	New        string `json:"new"`
	// Automatic marks a line whose NEW side contains days a Mitbuchungs-Regel
	// (or the required lunch) added rather than the parents (#2365).
	Automatic bool `json:"automatic,omitempty"`
	// AutomaticDays is the German day list of that automatic share ("Do, Fr").
	AutomaticDays string `json:"automatic_days,omitempty"`
	// RuleDays is the part attributed to TriggerNames. Required-lunch days are
	// excluded so the explanation does not ascribe them to a Mitbuchungs-Regel.
	RuleDays string `json:"rule_days,omitempty"`
	// NewWhenExcluded is the materialized NEW side after this line's
	// Mitbuchungs-Regel is suppressed. Manual and required-lunch days remain.
	NewWhenExcluded string `json:"new_when_excluded,omitempty"`
	// TriggerIDs / TriggerNames identify the selected offerings whose rule
	// produced the automatic share. TriggerIDs lets the review card grey out
	// dependent lines while staff untick an override (#2370).
	TriggerIDs   []string `json:"trigger_ids,omitempty"`
	TriggerNames []string `json:"trigger_names,omitempty"`
	// Optoutable marks a rule-triggered line staff may exclude per request.
	Optoutable bool `json:"optoutable,omitempty"`
}

// OfferingRequestUnchangedResponse is one booking the request does not touch.
type OfferingRequestUnchangedResponse struct {
	OfferingID string `json:"offering_id"`
	Label      string `json:"label"`
	Days       string `json:"days"`
}

// offeringRequestDiffLines renders the review diff, including the bookkeeping a
// Mitbuchungs-Regel line carries so the card can explain and override it.
func offeringRequestDiffLines(
	entries []enrollmentService.OfferingChangeDiffEntry,
) []OfferingRequestDiffResponse {
	diff := make([]OfferingRequestDiffResponse, 0, len(entries))
	for _, entry := range entries {
		line := OfferingRequestDiffResponse{
			OfferingID: strconv.FormatInt(entry.OfferingID, 10),
			Label:      entry.Label,
			Old:        germanOfferingDiffLabel(entry.OldState, entry.OldDays),
			New:        germanOfferingDiffLabel(entry.NewState, entry.NewDays),
		}
		if len(entry.NewAutomaticDays) > 0 {
			line.Automatic = true
			line.AutomaticDays = germanOfferingDiffLabel("booked", entry.NewAutomaticDays)
			if len(entry.NewRuleDays) > 0 {
				line.RuleDays = germanOfferingDiffLabel("booked", entry.NewRuleDays)
			}
			line.Optoutable = len(entry.AutoTriggerIDs) > 0
			if len(entry.NewDaysWithoutRules) > 0 {
				line.NewWhenExcluded = germanOfferingDiffLabel("booked", entry.NewDaysWithoutRules)
			}
			for _, triggerID := range entry.AutoTriggerIDs {
				line.TriggerIDs = append(line.TriggerIDs, strconv.FormatInt(triggerID, 10))
			}
			line.TriggerNames = entry.AutoTriggerNames
		}
		diff = append(diff, line)
	}
	return diff
}

func toOfferingRequestResponse(item *enrollmentService.OfferingChangeView) OfferingRequestResponse {
	row := item.Request
	diff := offeringRequestDiffLines(item.Diff)
	unchanged := make([]OfferingRequestUnchangedResponse, 0, len(item.Unchanged))
	for _, entry := range item.Unchanged {
		unchanged = append(unchanged, OfferingRequestUnchangedResponse{
			OfferingID: strconv.FormatInt(entry.OfferingID, 10),
			Label:      entry.Label,
			Days:       germanOfferingDiffLabel(entry.NewState, entry.NewDays),
		})
	}
	resp := OfferingRequestResponse{
		ID:             strconv.FormatInt(row.ID, 10),
		StudentID:      strconv.FormatInt(row.StudentID, 10),
		StudentName:    item.StudentName,
		Status:         row.Status,
		EffectiveFrom:  row.EffectiveFrom.String(),
		Diff:           diff,
		Reason:         row.DecisionReason,
		FullWithdrawal: item.FullWithdrawal,
		CreatedAt:      row.CreatedAt,
		ReviewedAt:     row.ReviewedAt,
	}
	if !item.EarliestEffectiveFrom.IsZero() {
		resp.EarliestEffectiveFrom = item.EarliestEffectiveFrom.String()
	}
	if !item.LatestEffectiveFrom.IsZero() {
		resp.LatestEffectiveFrom = item.LatestEffectiveFrom.String()
	}
	if !item.RequestedEffectiveFrom.IsZero() && item.RequestedEffectiveFrom != row.EffectiveFrom {
		resp.RequestedEffectiveFrom = item.RequestedEffectiveFrom.String()
	}
	if len(unchanged) > 0 {
		resp.Unchanged = unchanged
	}
	if row.ParentNote != nil {
		resp.Note = *row.ParentNote
	}
	return resp
}

func germanOfferingDiffLabel(state string, days []string) string {
	switch state {
	case "not_booked":
		return "nicht gebucht"
	case "removed":
		return "abgemeldet"
	}
	if len(days) == 0 {
		return "alle Betreuungstage"
	}
	labels := map[string]string{
		"mon": "Mo", "tue": "Di", "wed": "Mi", "thu": "Do",
		"fri": "Fr", "sat": "Sa", "sun": "So",
	}
	parts := make([]string, 0, len(days))
	for _, day := range days {
		if label, ok := labels[day]; ok {
			parts = append(parts, label)
		}
	}
	return strings.Join(parts, ", ")
}

// DecideOfferingRequestBody is the body of POST
// .../offering-change-requests/{requestId}/decide.
type DecideOfferingRequestBody struct {
	Approve *bool  `json:"approve"`
	Reason  string `json:"reason"`
	// EffectiveFrom is the date staff confirmed the switch takes effect on
	// (YYYY-MM-DD), required on an approval (#2484).
	EffectiveFrom string `json:"effective_from,omitempty"`
	// ExcludedOfferingIDs are the rule-added offerings staff unticked for this
	// one approval (#2370); the Mitbuchungs-Regel itself stays active.
	ExcludedOfferingIDs         []string `json:"excluded_offering_ids,omitempty"`
	CompleteWithdrawalConfirmed bool     `json:"complete_withdrawal_confirmed,omitempty"`
	// ExpectedVersion is the expected_version the list emitted for this row.
	// Empty is accepted (old clients) and skips the check.
	ExpectedVersion string `json:"expected_version,omitempty"`
}

type PreviewOfferingRequestBody struct {
	ExcludedOfferingIDs []string `json:"excluded_offering_ids"`
	// EffectiveFrom previews the decision for the date currently chosen in the
	// card; empty previews the date the parents requested.
	EffectiveFrom string `json:"effective_from,omitempty"`
}

// parseConfirmedEffectiveFrom reads the staff-confirmed effective date. An
// empty value is no date at all, which the caller decides how to treat.
func parseConfirmedEffectiveFrom(raw string) (*timezone.Date, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := timezone.ParseDate(trimmed)
	if err != nil {
		return nil, errors.New("effective_from must be a date (YYYY-MM-DD)")
	}
	return &parsed, nil
}

type OfferingRequestPreviewSelectionResponse struct {
	OfferingID string `json:"offering_id"`
	New        string `json:"new"`
	Removed    bool   `json:"removed,omitempty"`
}

type OfferingRequestPreviewResponse struct {
	Selections                        []OfferingRequestPreviewSelectionResponse `json:"selections"`
	ManualPlanningConflicts           []ManualPlanningConflictResponse          `json:"manual_planning_conflicts"`
	ArrivalExpectationsFollowBookings bool                                      `json:"arrival_expectations_follow_bookings"`
}

type ManualPlanningConflictResponse struct {
	ActivityGroupID   string   `json:"activity_group_id"`
	ActivityGroupName string   `json:"activity_group_name"`
	Days              []string `json:"days"`
	FirstDate         string   `json:"first_date"`
	OccurrenceCount   int      `json:"occurrence_count"`
}

func parseExcludedOfferingIDs(rawIDs []string) ([]int64, error) {
	excluded := make([]int64, 0, len(rawIDs))
	for _, raw := range rawIDs {
		id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil || id <= 0 {
			return nil, errors.New("excluded_offering_ids must contain numeric ids")
		}
		excluded = append(excluded, id)
	}
	return excluded, nil
}

// previewOfferingChangeRequest returns the exact materialized selection for
// the review card's current per-request Mitbuchungs-Regel overrides.
func (rs *Resource) previewOfferingChangeRequest(w http.ResponseWriter, r *http.Request) {
	if rs.OfferingChangeService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("offering change service not configured")))
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "requestId", "invalid request id")
	if !ok {
		return
	}
	var body PreviewOfferingRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	excluded, err := parseExcludedOfferingIDs(body.ExcludedOfferingIDs)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	effectiveFrom, err := parseConfirmedEffectiveFrom(body.EffectiveFrom)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	preview, err := rs.OfferingChangeService.PreviewDecision(r.Context(), requestID, excluded, effectiveFrom)
	if err != nil {
		renderOfferingDecisionError(w, r, err)
		return
	}
	if preview == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("offering change preview is empty")))
		return
	}
	selections := make([]OfferingRequestPreviewSelectionResponse, 0, len(preview.Selections))
	for _, selection := range preview.Selections {
		selections = append(selections, OfferingRequestPreviewSelectionResponse{
			OfferingID: strconv.FormatInt(selection.OfferingID, 10),
			New:        germanOfferingDiffLabel(selection.State, selection.Days),
			Removed:    selection.State == "removed",
		})
	}
	conflicts := make([]ManualPlanningConflictResponse, 0, len(preview.ManualPlanningConflicts))
	for _, conflict := range preview.ManualPlanningConflicts {
		conflicts = append(conflicts, ManualPlanningConflictResponse{
			ActivityGroupID:   strconv.FormatInt(conflict.ActivityGroupID, 10),
			ActivityGroupName: conflict.ActivityGroupName,
			Days:              conflict.Days,
			FirstDate:         conflict.FirstDate.String(),
			OccurrenceCount:   conflict.OccurrenceCount,
		})
	}
	common.Respond(w, r, http.StatusOK, OfferingRequestPreviewResponse{
		Selections:                        selections,
		ManualPlanningConflicts:           conflicts,
		ArrivalExpectationsFollowBookings: preview.ArrivalExpectationsFollowBookings,
	}, "Preview materialized")
}

// decideOfferingChangeRequest approves (and applies the dated switch) or
// rejects one pending request.
func (rs *Resource) decideOfferingChangeRequest(w http.ResponseWriter, r *http.Request) {
	if rs.OfferingChangeService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("offering change service not configured")))
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "requestId", "invalid request id")
	if !ok {
		return
	}
	var body DecideOfferingRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if body.Approve == nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("approve is required")))
		return
	}

	excluded, err := parseExcludedOfferingIDs(body.ExcludedOfferingIDs)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	effectiveFrom, err := parseConfirmedEffectiveFrom(body.EffectiveFrom)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	// An approval always applies on a date, and the office confirms which one:
	// without it there is nothing to check the switch against.
	if *body.Approve && effectiveFrom == nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("effective_from is required to approve")))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	actorRole := strings.Join(claims.Roles, ",")
	if actorRole == "" {
		actorRole = "unknown"
	}
	if err := rs.OfferingChangeService.Decide(r.Context(), enrollmentService.DecideOfferingChangeInput{
		RequestID:                   requestID,
		Approve:                     *body.Approve,
		Reason:                      body.Reason,
		ReviewedBy:                  int64(claims.ID),
		ActorRole:                   actorRole,
		ExcludedAutoOfferingIDs:     excluded,
		EffectiveFrom:               effectiveFrom,
		CompleteWithdrawalConfirmed: body.CompleteWithdrawalConfirmed,
		ExpectedVersion:             body.ExpectedVersion,
		ReasonRequired:              rs.staffReasonRequired(r),
	}); err != nil {
		renderOfferingDecisionError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]string{"status": "ok"}, "Decision applied")
}

// renderOfferingDecisionError maps the decision failures a reviewer can act on.
// Capacity and validation problems are the interesting ones: they mean the
// request is no longer applicable, and the row deliberately stays pending so
// the office can talk to the family instead of finding a "done" request that
// never applied.
func renderOfferingDecisionError(w http.ResponseWriter, r *http.Request, err error) {
	renderError(w, r, offeringDecisionErrorRenderer(err))
}

var offeringDecisionErrorRenderer = common.RulesRenderer(parentRequestRules(
	common.ErrorRule{Target: enrollmentModels.ErrOfferingChangeNotFound, Render: common.ErrorNotFound},
	common.ErrorRule{Target: enrollmentModels.ErrOfferingChangeNotPending, Render: conflictWithCode("change_request_not_pending")},
	common.ErrorRule{Target: enrollmentService.ErrOfferingChangeForbidden, Render: common.ErrorForbidden},
	common.ErrorRule{Target: enrollmentService.ErrCareOfferingsDisabled, Render: func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, "care_offerings_disabled")
	}},
	common.ErrorRule{Target: enrollmentService.ErrOfferingChangeCapacityFull, Render: conflictWithCode("offering_change_capacity_full")},
	common.ErrorRule{Target: enrollmentService.ErrOfferingChangeNoEnrollment, Render: conflictWithCode("offering_changes_no_enrollment")},
	common.ErrorRule{Target: enrollmentService.ErrOfferingChangeDateOutOfRange, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, "offering_change_date_out_of_range")
	}},
	common.ErrorRule{
		Target: enrollmentService.ErrCompleteWithdrawalConfirmationRequired,
		Render: conflictWithCode("enrollment.complete_withdrawal_confirmation_required"),
	},
	common.ErrorRule{Target: enrollmentService.ErrOfferingChangeInvalid, Render: common.ErrorInvalidRequest},
	common.ErrorRule{Target: enrollmentService.ErrOfferingAdjustmentInvalid, Render: common.ErrorInvalidRequest},
), common.ErrorInternalServer)
