package students

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
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
	EffectiveFrom string                        `json:"effective_from"`
	Note          string                        `json:"note,omitempty"`
	Diff          []OfferingRequestDiffResponse `json:"diff"`
	Reason        *string                       `json:"reason,omitempty"`
	CreatedAt     time.Time                     `json:"created_at"`
	ReviewedAt    *time.Time                    `json:"reviewed_at,omitempty"`
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

func toOfferingRequestResponse(item *enrollmentService.OfferingChangeView) OfferingRequestResponse {
	row := item.Request
	diff := make([]OfferingRequestDiffResponse, 0, len(item.Diff))
	for _, entry := range item.Diff {
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
	resp := OfferingRequestResponse{
		ID:            strconv.FormatInt(row.ID, 10),
		StudentID:     strconv.FormatInt(row.StudentID, 10),
		StudentName:   item.StudentName,
		Status:        row.Status,
		EffectiveFrom: row.EffectiveFrom.String(),
		Diff:          diff,
		Reason:        row.DecisionReason,
		CreatedAt:     row.CreatedAt,
		ReviewedAt:    row.ReviewedAt,
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

// listOfferingChangeRequests returns the tenant's pending offering-change
// requests, soonest effective date first.
func (rs *Resource) listOfferingChangeRequests(w http.ResponseWriter, r *http.Request) {
	if rs.OfferingChangeService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("offering change service not configured")))
		return
	}
	items, err := rs.OfferingChangeService.ListPending(r.Context())
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	out := make([]OfferingRequestResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toOfferingRequestResponse(item))
	}
	common.Respond(w, r, http.StatusOK, out, "Offering change requests retrieved")
}

// DecideOfferingRequestBody is the body of POST
// .../offering-change-requests/{requestId}/decide.
type DecideOfferingRequestBody struct {
	Approve *bool  `json:"approve"`
	Reason  string `json:"reason"`
	// ExcludedOfferingIDs are the rule-added offerings staff unticked for this
	// one approval (#2370); the Mitbuchungs-Regel itself stays active.
	ExcludedOfferingIDs []string `json:"excluded_offering_ids,omitempty"`
}

type PreviewOfferingRequestBody struct {
	ExcludedOfferingIDs []string `json:"excluded_offering_ids"`
}

type OfferingRequestPreviewSelectionResponse struct {
	OfferingID string `json:"offering_id"`
	New        string `json:"new"`
	Removed    bool   `json:"removed,omitempty"`
}

type OfferingRequestPreviewResponse struct {
	Selections []OfferingRequestPreviewSelectionResponse `json:"selections"`
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
	preview, err := rs.OfferingChangeService.PreviewDecision(r.Context(), requestID, excluded)
	if err != nil {
		renderOfferingDecisionError(w, r, err)
		return
	}
	selections := make([]OfferingRequestPreviewSelectionResponse, 0, len(preview))
	for _, selection := range preview {
		selections = append(selections, OfferingRequestPreviewSelectionResponse{
			OfferingID: strconv.FormatInt(selection.OfferingID, 10),
			New:        germanOfferingDiffLabel(selection.State, selection.Days),
			Removed:    selection.State == "removed",
		})
	}
	common.Respond(w, r, http.StatusOK, OfferingRequestPreviewResponse{Selections: selections}, "Preview materialized")
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

	claims := jwt.ClaimsFromCtx(r.Context())
	actorRole := strings.Join(claims.Roles, ",")
	if actorRole == "" {
		actorRole = "unknown"
	}
	if err := rs.OfferingChangeService.Decide(r.Context(), enrollmentService.DecideOfferingChangeInput{
		RequestID:               requestID,
		Approve:                 *body.Approve,
		Reason:                  body.Reason,
		ReviewedBy:              int64(claims.ID),
		ActorRole:               actorRole,
		ExcludedAutoOfferingIDs: excluded,
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
	switch {
	case errors.Is(err, enrollmentModels.ErrOfferingChangeNotFound):
		renderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, enrollmentModels.ErrOfferingChangeNotPending):
		renderError(w, r, common.ErrorConflictWithCode(err, "change_request_not_pending"))
	case errors.Is(err, enrollmentService.ErrOfferingChangeForbidden):
		renderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, enrollmentService.ErrCareOfferingsDisabled):
		renderError(w, r, common.ErrorForbiddenWithCode(err, "care_offerings_disabled"))
	case errors.Is(err, enrollmentService.ErrOfferingChangeCapacityFull):
		renderError(w, r, common.ErrorConflictWithCode(err, "offering_change_capacity_full"))
	case errors.Is(err, enrollmentService.ErrOfferingChangeNoEnrollment):
		renderError(w, r, common.ErrorConflictWithCode(err, "offering_changes_no_enrollment"))
	case errors.Is(err, enrollmentService.ErrOfferingChangeInvalid),
		errors.Is(err, enrollmentService.ErrOfferingAdjustmentInvalid):
		renderError(w, r, common.ErrorInvalidRequest(err))
	default:
		renderError(w, r, common.ErrorInternalServer(err))
	}
}
