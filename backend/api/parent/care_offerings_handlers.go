package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
)

// CareOfferingsResponse is the parent-facing view of a child's booked care
// offerings and activity groups (#1665).
type CareOfferingsResponse struct {
	// Period* describe the care period the offerings belong to. Empty when the
	// child has no approved enrollment behind it.
	PeriodName  string `json:"period_name,omitempty"`
	PeriodStart string `json:"period_start,omitempty"`
	PeriodEnd   string `json:"period_end,omitempty"`

	Offerings []CareOfferingItemResponse `json:"offerings"`
	Groups    []CareGroupItemResponse    `json:"groups"`

	CanRequest bool `json:"can_request"`
	// PendingRequest is the child's open change request, absent when none.
	PendingRequest *PendingOfferingChangeResponse `json:"pending_request,omitempty"`
	// EarliestEffectiveFrom bounds the date picker (YYYY-MM-DD).
	EarliestEffectiveFrom string `json:"earliest_effective_from,omitempty"`
	// LastDecision is the most recent decided request inside the recency window,
	// absent when there is none.
	LastDecision *OfferingDecisionResponse `json:"last_decision,omitempty"`
	// ChangesDisabledReason is a stable identifier (no_enrollment,
	// no_permission, school_disabled, period_over) the UI turns into German
	// copy. Empty when CanRequest is true.
	ChangesDisabledReason string `json:"changes_disabled_reason,omitempty"`
}

// CareOfferingItemResponse is one booked care offering.
type CareOfferingItemResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Weekdays are ISO weekdays (1=Mon..7=Sun); empty when the offering has no
	// per-day choice.
	Weekdays        []int `json:"weekdays"`
	PriceCents      *int  `json:"price_cents,omitempty"`
	IncludesLunch   bool  `json:"includes_lunch"`
	IncludesHoliday bool  `json:"includes_holiday_care"`
	// ValidFrom is present for a scheduled offering. ValidUntil is the
	// inclusive last day, converted from the exclusive database interval.
	ValidFrom   string `json:"valid_from,omitempty"`
	ValidUntil  string `json:"valid_until,omitempty"`
	StartsLater bool   `json:"starts_later"`
}

// CareGroupItemResponse is one activity-group membership.
type CareGroupItemResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Weekdays []int  `json:"weekdays"`
	// ValidFrom is the first day (YYYY-MM-DD); ValidUntil is the LAST day, not
	// the exclusive end the column stores, because a guardian reading "bis
	// 31.07." must not be shown 01.08.
	ValidFrom    string `json:"valid_from"`
	ValidUntil   string `json:"valid_until,omitempty"`
	FromOffering bool   `json:"from_offering"`
	StartsLater  bool   `json:"starts_later"`
}

func toCareOfferingsResponse(v *parentService.ChildCareOfferings) CareOfferingsResponse {
	resp := CareOfferingsResponse{
		PeriodName:            v.PeriodName,
		Offerings:             make([]CareOfferingItemResponse, 0, len(v.Offerings)),
		Groups:                make([]CareGroupItemResponse, 0, len(v.Groups)),
		CanRequest:            v.CanRequest,
		ChangesDisabledReason: v.ChangesDisabledReason,
	}
	if !v.PeriodStart.IsZero() {
		resp.PeriodStart = v.PeriodStart.String()
	}
	if !v.PeriodEnd.IsZero() {
		resp.PeriodEnd = v.PeriodEnd.String()
	}
	if !v.EarliestEffectiveFrom.IsZero() {
		resp.EarliestEffectiveFrom = v.EarliestEffectiveFrom.String()
	}
	if v.LastDecision != nil {
		resp.LastDecision = offeringDecisionResponse(v.LastDecision)
	}
	if v.PendingRequest != nil {
		pending := &PendingOfferingChangeResponse{
			ID:              strconv.FormatInt(v.PendingRequest.ID, 10),
			CreatedAt:       v.PendingRequest.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			EffectiveFrom:   v.PendingRequest.EffectiveFrom.String(),
			Note:            v.PendingRequest.Note,
			Diff:            make([]OfferingDiffResponse, 0, len(v.PendingRequest.Diff)),
			SubmittedBySelf: v.PendingRequest.SubmittedBySelf,
		}
		for _, entry := range v.PendingRequest.Diff {
			pending.Diff = append(pending.Diff, offeringDiffResponse(entry))
		}
		resp.PendingRequest = pending
	}
	for _, offering := range v.Offerings {
		resp.Offerings = append(resp.Offerings, careOfferingItemResponse(offering))
	}
	for _, group := range v.Groups {
		item := CareGroupItemResponse{
			ID:           strconv.FormatInt(group.GroupID, 10),
			Name:         group.Name,
			Weekdays:     weekdaysOrEmpty(group.Weekdays),
			ValidFrom:    group.ValidFrom.String(),
			FromOffering: group.FromOffering,
			StartsLater:  group.StartsLater,
		}
		if group.ValidUntil != nil {
			// Stored end is exclusive; report the inclusive last day.
			item.ValidUntil = group.ValidUntil.AddDays(-1).String()
		}
		resp.Groups = append(resp.Groups, item)
	}
	return resp
}

func offeringDecisionResponse(decision *enrollmentService.OfferingChangeDecision) *OfferingDecisionResponse {
	resp := &OfferingDecisionResponse{
		ID:            strconv.FormatInt(decision.ID, 10),
		Status:        decision.Status,
		DecidedAt:     decision.DecidedAt.Format("2006-01-02T15:04:05Z07:00"),
		EffectiveFrom: decision.EffectiveFrom.String(),
		Reason:        decision.Reason,
		Requested:     make([]OfferingRequestedItemResponse, 0, len(decision.Requested)),
	}
	for _, item := range decision.Requested {
		resp.Requested = append(resp.Requested, OfferingRequestedItemResponse{
			ID:       strconv.FormatInt(item.OfferingID, 10),
			Name:     item.Name,
			Weekdays: weekdaysFromDayKeys(item.Days),
		})
	}
	for _, entry := range decision.AppliedDiff {
		resp.Applied = append(resp.Applied, offeringDiffResponse(entry))
	}
	for _, offering := range decision.OverriddenOfferings {
		resp.OverriddenNames = append(resp.OverriddenNames, offering.Name)
	}
	return resp
}

func careOfferingItemResponse(offering parentService.CareOfferingSelection) CareOfferingItemResponse {
	item := CareOfferingItemResponse{
		ID:              strconv.FormatInt(offering.OfferingID, 10),
		Name:            offering.Name,
		Description:     offering.Description,
		Weekdays:        weekdaysOrEmpty(offering.Weekdays),
		PriceCents:      offering.PriceCents,
		IncludesLunch:   offering.IncludesLunch,
		IncludesHoliday: offering.IncludesHoliday,
		StartsLater:     offering.StartsLater,
	}
	if offering.ValidFrom != nil {
		item.ValidFrom = offering.ValidFrom.String()
	}
	if offering.ValidUntil != nil {
		// Stored end is exclusive; report the inclusive last day.
		item.ValidUntil = offering.ValidUntil.AddDays(-1).String()
	}
	return item
}

func offeringDiffResponse(entry enrollmentService.OfferingChangeDiffEntry) OfferingDiffResponse {
	resp := OfferingDiffResponse{
		Label:    entry.Label,
		OldState: entry.OldState,
		OldDays:  append([]string{}, entry.OldDays...),
		NewState: entry.NewState,
		NewDays:  append([]string{}, entry.NewDays...),
	}
	if len(entry.NewAutomaticDays) > 0 {
		resp.NewAutomaticDays = append([]string{}, entry.NewAutomaticDays...)
		resp.AutoTriggerNames = append([]string{}, entry.AutoTriggerNames...)
	}
	return resp
}

// weekdaysFromDayKeys maps stored day abbreviations to ISO weekdays so the
// frontend renders them with the same labels as everywhere else.
func weekdaysFromDayKeys(days []string) []int {
	weekdays := make([]int, 0, len(days))
	for _, day := range days {
		if weekday, ok := enrollmentModels.CanonicalDayToISOWeekday(day); ok {
			weekdays = append(weekdays, weekday)
		}
	}
	sort.Ints(weekdays)
	return weekdays
}

// weekdaysOrEmpty keeps the JSON array non-null so the frontend can map over it
// without a nil check.
func weekdaysOrEmpty(weekdays []int) []int {
	if weekdays == nil {
		return []int{}
	}
	return weekdays
}

// PendingOfferingChangeResponse is the child's open change request.
type PendingOfferingChangeResponse struct {
	ID            string `json:"id"`
	CreatedAt     string `json:"created_at"`
	EffectiveFrom string `json:"effective_from"`
	Note          string `json:"note,omitempty"`
	// Diff lists the "current → requested" lines with stable states and canonical
	// day keys. The parents portal renders them in the guardian's language.
	Diff            []OfferingDiffResponse `json:"diff"`
	SubmittedBySelf bool                   `json:"submitted_by_self"`
}

// OfferingDecisionResponse is a decided request: what the office decided, when,
// from which date it would have applied, and why it was rejected.
type OfferingDecisionResponse struct {
	ID string `json:"id"`
	// Status is "approved" or "rejected".
	Status        string `json:"status"`
	DecidedAt     string `json:"decided_at"`
	EffectiveFrom string `json:"effective_from"`
	Reason        string `json:"reason,omitempty"`
	// Requested recaps what the family asked for, so a decision stays readable
	// on its own.
	Requested []OfferingRequestedItemResponse `json:"requested"`
	// Applied is the frozen diff the decision was made on (including what a
	// Mitbuchungs-Regel added). Absent for rows decided before the snapshot
	// existed (#2365).
	Applied []OfferingDiffResponse `json:"applied,omitempty"`
	// OverriddenNames lists rule-added offerings the school excluded for this
	// one request at approval time (#2370).
	OverriddenNames []string `json:"overridden_names,omitempty"`
}

// OfferingRequestedItemResponse is one offering of a decided request.
type OfferingRequestedItemResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Weekdays are ISO weekdays (1=Mon..7=Sun); empty means every care day.
	Weekdays []int `json:"weekdays"`
}

// OfferingDiffResponse is one diff line.
type OfferingDiffResponse struct {
	Label    string   `json:"label"`
	OldState string   `json:"old_state"`
	OldDays  []string `json:"old_days"` // Empty for not_booked.
	NewState string   `json:"new_state"`
	NewDays  []string `json:"new_days"` // Empty for removed.
	// NewAutomaticDays is the share of NewDays a Mitbuchungs-Regel added rather
	// than the guardian's own selection (#2365); empty for a manual line.
	NewAutomaticDays []string `json:"new_automatic_days,omitempty"`
	// AutoTriggerNames names the selected offerings that triggered the
	// automatic share, so the portal can say WHY a line appeared.
	AutoTriggerNames []string `json:"auto_trigger_names,omitempty"`
}

// OfferingCatalogResponse is the selectable catalog for the request modal.
type OfferingCatalogResponse struct {
	PhaseName             string                        `json:"phase_name"`
	SelectionMode         string                        `json:"selection_mode"`
	EarliestEffectiveFrom string                        `json:"earliest_effective_from"`
	LatestEffectiveFrom   string                        `json:"latest_effective_from"`
	Items                 []OfferingCatalogItemResponse `json:"items"`
}

// OfferingCatalogItemResponse is one selectable offering.
type OfferingCatalogItemResponse struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description,omitempty"`
	DaysOfWeekMode  string   `json:"days_of_week_mode"`
	AvailableDays   []string `json:"available_days"`
	SelectionGroup  string   `json:"selection_group,omitempty"`
	SelectionRule   string   `json:"selection_rule"`
	IsRequired      bool     `json:"is_required"`
	PriceCents      *int     `json:"price_cents,omitempty"`
	IncludesLunch   bool     `json:"includes_lunch"`
	IncludesHoliday bool     `json:"includes_holiday_care"`
	Selected        bool     `json:"selected"`
	SelectedDays    []string `json:"selected_days"`
	Automatic       bool     `json:"automatic"`
	IsActive        bool     `json:"is_active"`
	Capacity        *int     `json:"capacity,omitempty"`
	FreeSlots       *int     `json:"free_slots,omitempty"`
}

// OfferingChangeRequestBody is the wire shape for POST .../care-offerings/requests.
type OfferingChangeRequestBody struct {
	Offerings []struct {
		OfferingID   string   `json:"offering_id"`
		SelectedDays []string `json:"selected_days"`
	} `json:"offerings"`
	EffectiveFrom string `json:"effective_from"`
	Note          string `json:"note"`
}

// getChildCareOfferings returns what the child is booked into.
func (rs *Resource) getChildCareOfferings(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	view, err := rs.ParentService.GetChildCareOfferings(r.Context(), accountID, studentID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toCareOfferingsResponse(view), "Care offerings retrieved")
}

// getChildOfferingCatalog returns the offerings a guardian may pick from.
func (rs *Resource) getChildOfferingCatalog(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	var effectiveFrom timezone.Date
	if raw := strings.TrimSpace(r.URL.Query().Get("effective_from")); raw != "" {
		parsed, parseErr := timezone.ParseDate(raw)
		if parseErr != nil {
			common.RenderError(w, r, common.ErrorInvalidRequestWithCode(
				errors.New("effective_from must be a date in YYYY-MM-DD form"), "offering_change_invalid"))
			return
		}
		effectiveFrom = parsed
	}
	catalog, err := rs.ParentService.GetChildOfferingCatalogAt(r.Context(), accountID, studentID, effectiveFrom)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toOfferingCatalogResponse(catalog), "Care offering catalog retrieved")
}

// createOfferingChangeRequest stores a pending post-enrollment change request.
func (rs *Resource) createOfferingChangeRequest(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	var body OfferingChangeRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	effectiveFrom, err := timezone.ParseDate(strings.TrimSpace(body.EffectiveFrom))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(
			errors.New("effective_from must be a date in YYYY-MM-DD form"), "offering_change_invalid"))
		return
	}
	selections := make([]enrollmentService.OfferingChangeSelection, 0, len(body.Offerings))
	for _, entry := range body.Offerings {
		offeringID, convErr := strconv.ParseInt(strings.TrimSpace(entry.OfferingID), 10, 64)
		if convErr != nil || offeringID <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequestWithCode(
				errors.New("offering_id must be a numeric id"), "offering_change_invalid"))
			return
		}
		selections = append(selections, enrollmentService.OfferingChangeSelection{
			OfferingID:   offeringID,
			SelectedDays: entry.SelectedDays,
		})
	}
	view, err := rs.ParentService.CreateOfferingChangeRequest(
		r.Context(), accountID, studentID, selections, effectiveFrom, body.Note,
	)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toCareOfferingsResponse(view), "Care offering change requested")
}

// withdrawOfferingChangeRequest flips the caller's own pending request to
// withdrawn.
func (rs *Resource) withdrawOfferingChangeRequest(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "requestId", "invalid request ID")
	if !ok {
		return
	}
	view, err := rs.ParentService.WithdrawOfferingChangeRequest(r.Context(), accountID, studentID, requestID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toCareOfferingsResponse(view), "Care offering change request withdrawn")
}

func toOfferingCatalogResponse(catalog *enrollmentService.OfferingChangeCatalog) OfferingCatalogResponse {
	resp := OfferingCatalogResponse{
		PhaseName:     catalog.PhaseName,
		SelectionMode: catalog.SelectionMode,
		Items:         make([]OfferingCatalogItemResponse, 0, len(catalog.Items)),
	}
	if !catalog.EarliestEffectiveFrom.IsZero() {
		resp.EarliestEffectiveFrom = catalog.EarliestEffectiveFrom.String()
	}
	if !catalog.LatestEffectiveFrom.IsZero() {
		resp.LatestEffectiveFrom = catalog.LatestEffectiveFrom.String()
	}
	for _, item := range catalog.Items {
		resp.Items = append(resp.Items, OfferingCatalogItemResponse{
			ID:              strconv.FormatInt(item.OfferingID, 10),
			Name:            item.Name,
			Description:     item.Description,
			DaysOfWeekMode:  item.DaysOfWeekMode,
			AvailableDays:   stringsOrEmpty(item.AvailableDays),
			SelectionGroup:  item.SelectionGroup,
			SelectionRule:   item.SelectionRule,
			IsRequired:      item.IsRequired,
			PriceCents:      item.PriceCents,
			IncludesLunch:   item.IncludesLunch,
			IncludesHoliday: item.IncludesHoliday,
			Selected:        item.Selected,
			SelectedDays:    stringsOrEmpty(item.SelectedDays),
			Automatic:       item.Automatic,
			IsActive:        item.IsActive,
			Capacity:        item.Capacity,
			FreeSlots:       item.FreeSlots,
		})
	}
	return resp
}

func stringsOrEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
