package enrollmenthttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// CareOfferingResponse is the wire shape for a single offering. IDs
// stringified so the frontend keeps its int64-as-string convention.
type CareOfferingResponse struct {
	ID                  string                        `json:"id"`
	PhaseID             string                        `json:"phase_id"`
	ActivityGroupID     *string                       `json:"activity_group_id,omitempty"`
	Name                string                        `json:"name"`
	Description         *string                       `json:"description,omitempty"`
	DaysOfWeekMode      string                        `json:"days_of_week_mode"`
	AvailableDays       []string                      `json:"available_days"`
	IncludesHolidayCare bool                          `json:"includes_holiday_care"`
	IncludesLunch       bool                          `json:"includes_lunch"`
	Capacity            *int                          `json:"capacity,omitempty"`
	PriceCents          *int                          `json:"price_cents,omitempty"`
	IsActive            bool                          `json:"is_active"`
	IsRequired          bool                          `json:"is_required"`
	CountsAsCare        bool                          `json:"counts_as_care"`
	AutoAddGradeLevels  []int                         `json:"auto_add_grade_levels"`
	AvailabilityRule    *CareOfferingAvailabilityRule `json:"availability_rule,omitempty"`
	AutoAddTriggerIDs   []string                      `json:"auto_add_trigger_offering_ids"`
	SortOrder           int                           `json:"sort_order"`
	SelectionGroup      string                        `json:"selection_group,omitempty"`
	SelectionRule       string                        `json:"selection_rule"`
	PickupTimes         map[string]string             `json:"pickup_times,omitempty"`
	// Translations of Name and Description (#3377). Admin responses carry
	// every stored translation with its German source; parent-facing
	// responses only the ones still matching the German text, without source.
	Translations capability.Translations `json:"translations,omitempty"`
	CreatedAt    time.Time               `json:"created_at"`
	UpdatedAt    time.Time               `json:"updated_at"`
}

var careOfferingWriteErrorRenderer = common.RulesRenderer(
	[]common.ErrorRule{
		{
			Target: capability.ErrCareOfferingTemplatePeriodMismatch,
			Render: func(err error) render.Renderer {
				return common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentCareOfferingTemplatePeriodMismatch)
			},
		},
		{
			Target: capability.ErrCareOfferingDaysRequired,
			Render: func(err error) render.Renderer {
				return common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentCareOfferingDaysRequired)
			},
		},
		{
			Target: capability.ErrCareOfferingPickupTimesRequired,
			Render: func(err error) render.Renderer {
				return common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentCareOfferingPickupTimesRequired)
			},
		},
		{Target: capability.ErrCareOfferingInvalid, Render: common.ErrorInvalidRequest},
		{
			Target: capability.ErrCareOfferingGroupRuleConflict,
			Render: func(err error) render.Renderer {
				return common.ErrorInvalidRequest(capability.InvalidInput(
					common.CodeEnrollmentCareOfferingSelectionInvalid, "selection_rule", err))
			},
		},
	},
	func(err error) render.Renderer {
		return common.ErrorInternalServerWrap("care offering operation failed", err)
	},
)

func toCareOfferingResponse(o *CareOffering) CareOfferingResponse {
	resp := CareOfferingResponse{
		ID:                  strconv.FormatInt(o.ID, 10),
		PhaseID:             strconv.FormatInt(o.PhaseID, 10),
		Name:                o.Name,
		Description:         o.Description,
		DaysOfWeekMode:      o.DaysOfWeekMode,
		AvailableDays:       o.AvailableDays,
		IncludesHolidayCare: o.IncludesHolidayCare,
		IncludesLunch:       o.IncludesLunch,
		Capacity:            o.Capacity,
		PriceCents:          o.PriceCents,
		IsActive:            o.IsActive,
		IsRequired:          o.IsRequired,
		CountsAsCare:        o.CountsAsCare,
		AutoAddGradeLevels:  o.AutoAddGradeLevels,
		AvailabilityRule:    o.AvailabilityRule,
		AutoAddTriggerIDs:   make([]string, 0, len(o.AutoAddTriggerOfferingIDs)),
		SortOrder:           o.SortOrder,
		SelectionGroup:      o.SelectionGroup,
		SelectionRule:       o.SelectionRule,
		PickupTimes:         o.PickupTimes,
		CreatedAt:           o.CreatedAt,
		UpdatedAt:           o.UpdatedAt,
	}
	if o.ActivityGroupID != nil {
		s := strconv.FormatInt(*o.ActivityGroupID, 10)
		resp.ActivityGroupID = &s
	}
	for _, id := range o.AutoAddTriggerOfferingIDs {
		resp.AutoAddTriggerIDs = append(resp.AutoAddTriggerIDs, strconv.FormatInt(id, 10))
	}
	// The document is validated on every write. Should a row still fail to
	// decode, the offering renders untranslated instead of failing the list.
	resp.Translations, _ = careOfferingTranslations(o)
	return resp
}

// toPublicCareOfferingResponse is the parent-facing variant: translations are
// reduced to those still matching the German text, without their source.
func toPublicCareOfferingResponse(o *CareOffering) CareOfferingResponse {
	resp := toCareOfferingResponse(o)
	resp.Translations, _ = careOfferingPublicTranslations(o)
	return resp
}

// Care offerings carry their translations (#3377) as stored JSON: the
// catalog rows belong to Care Plan, which does not interpret Enrollment's
// translation document. The routes decode it for rendering.

// careOfferingTranslations decodes the stored translation document.
func careOfferingTranslations(offering *CareOffering) (capability.Translations, error) {
	if offering == nil || len(offering.Translations) == 0 {
		return nil, nil
	}
	var translations capability.Translations
	if err := json.Unmarshal(offering.Translations, &translations); err != nil {
		return nil, fmt.Errorf("decode care offering translations: %w", err)
	}
	return translations, nil
}

// careOfferingPublicTranslations returns the translations parents may read:
// only those still matching the current German name, description and
// selection group.
func careOfferingPublicTranslations(offering *CareOffering) (capability.Translations, error) {
	translations, err := careOfferingTranslations(offering)
	if err != nil || translations == nil {
		return nil, err
	}
	sources := map[string]string{
		capability.TranslationAttrName:           offering.Name,
		capability.TranslationAttrSelectionGroup: offering.SelectionGroup,
	}
	if offering.Description != nil {
		sources[capability.TranslationAttrDescription] = *offering.Description
	}
	return translations.Fresh(sources), nil
}

// CareOfferingRequest is the wire shape POST + PUT accept.
type CareOfferingRequest struct {
	PhaseID             int64                         `json:"phase_id"`
	ActivityGroupID     *int64                        `json:"activity_group_id,omitempty"`
	Name                string                        `json:"name"`
	Description         *string                       `json:"description,omitempty"`
	DaysOfWeekMode      string                        `json:"days_of_week_mode"`
	AvailableDays       []string                      `json:"available_days"`
	IncludesHolidayCare bool                          `json:"includes_holiday_care"`
	IncludesLunch       bool                          `json:"includes_lunch"`
	Capacity            *int                          `json:"capacity,omitempty"`
	PriceCents          *int                          `json:"price_cents,omitempty"`
	IsActive            bool                          `json:"is_active"`
	IsRequired          bool                          `json:"is_required"`
	CountsAsCare        *bool                         `json:"counts_as_care"`
	AutoAddGradeLevels  []int                         `json:"auto_add_grade_levels"`
	AvailabilityRule    *CareOfferingAvailabilityRule `json:"availability_rule,omitempty"`
	AutoAddTriggerIDs   []string                      `json:"auto_add_trigger_offering_ids"`
	SortOrder           int                           `json:"sort_order"`
	SelectionGroup      string                        `json:"selection_group,omitempty"`
	SelectionRule       string                        `json:"selection_rule,omitempty"`
	// PickupTimes is the booking-derived pickup baseline per weekday. Active
	// offerings that count as care require a value for every selected weekday.
	PickupTimes map[string]string `json:"pickup_times,omitempty"`
	// Translations of Name and Description (#3377). The editor always sends
	// the full document; the service validates it.
	Translations json.RawMessage `json:"translations,omitempty"`
}

// Bind satisfies render.Binder. Field-level validation runs in the
// model's Validate (called inside Repository.Create/Update).
func (req *CareOfferingRequest) Bind(_ *http.Request) error {
	if req.AvailableDays == nil {
		req.AvailableDays = []string{}
	}
	if req.AutoAddGradeLevels == nil {
		req.AutoAddGradeLevels = []int{}
	}
	if req.AutoAddTriggerIDs == nil {
		req.AutoAddTriggerIDs = []string{}
	}
	return nil
}

func (req *CareOfferingRequest) toModel(existingID int64) (*CareOffering, error) {
	countsAsCare := true
	if req.CountsAsCare != nil {
		countsAsCare = *req.CountsAsCare
	}
	triggerIDs, err := parseCareOfferingIDStrings(req.AutoAddTriggerIDs, "auto_add_trigger_offering_ids")
	if err != nil {
		return nil, err
	}
	o := &CareOffering{
		PhaseID:                   req.PhaseID,
		ActivityGroupID:           req.ActivityGroupID,
		Name:                      req.Name,
		Description:               req.Description,
		DaysOfWeekMode:            req.DaysOfWeekMode,
		AvailableDays:             req.AvailableDays,
		IncludesHolidayCare:       req.IncludesHolidayCare,
		IncludesLunch:             req.IncludesLunch,
		Capacity:                  req.Capacity,
		PriceCents:                req.PriceCents,
		IsActive:                  req.IsActive,
		IsRequired:                req.IsRequired,
		CountsAsCare:              countsAsCare,
		AutoAddGradeLevels:        req.AutoAddGradeLevels,
		AvailabilityRule:          req.AvailabilityRule,
		SortOrder:                 req.SortOrder,
		SelectionGroup:            req.SelectionGroup,
		SelectionRule:             req.SelectionRule,
		PickupTimes:               req.PickupTimes,
		Translations:              req.Translations,
		AutoAddTriggerOfferingIDs: triggerIDs,
	}
	o.ID = existingID
	return o, nil
}

func parseCareOfferingIDStrings(values []string, field string) ([]int64, error) {
	out := make([]int64, 0, len(values))
	for _, raw := range values {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("%s contains invalid id %q", field, raw)
		}
		out = append(out, id)
	}
	return out, nil
}

// CloneCareOfferingRequest is the body POST /{id}/clone accepts.
type CloneCareOfferingRequest struct {
	TargetPhaseID int64 `json:"target_phase_id"`
}

func (req *CloneCareOfferingRequest) Bind(_ *http.Request) error { return nil }

func (rs *Resource) listCareOfferings(w http.ResponseWriter, r *http.Request) {
	if rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("care offering service not configured")))
		return
	}

	phaseFilter := r.URL.Query().Get("phase_id")
	var phaseID int64
	if phaseFilter != "" {
		var parseErr error
		phaseID, parseErr = strconv.ParseInt(phaseFilter, 10, 64)
		if parseErr != nil || phaseID <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid phase_id")))
			return
		}
	}
	var (
		offerings []*CareOffering
		err       error
	)
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		var listErr error
		if phaseFilter != "" {
			offerings, listErr = rs.CareOfferingService.ListByPhase(ctx, phaseID)
			return listErr
		}
		offerings, listErr = rs.CareOfferingService.List(ctx)
		return listErr
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("list care offerings failed", err))
		return
	}

	out := make([]CareOfferingResponse, 0, len(offerings))
	for _, o := range offerings {
		out = append(out, toCareOfferingResponse(o))
	}
	common.Respond(w, r, http.StatusOK, out, "Care offerings retrieved")
}

// CareOfferingBookingStatsResponse is the wire shape of one offering's
// booking summary. GradeLevels is keyed by the grade level as a string
// because JSON object keys are always strings.
type CareOfferingBookingStatsResponse struct {
	OfferingID        string         `json:"offering_id"`
	Capacity          *int           `json:"capacity,omitempty"`
	Booked            int            `json:"booked"`
	GradeLevels       map[string]int `json:"grade_levels"`
	UnknownGradeCount int            `json:"unknown_grade_count"`
}

// listCareOfferingBookingStats backs the admin-side capacity display and the
// availability-rule conflict hint (#2186). It is deliberately aggregate-only:
// the client learns how many children hold a slot per grade level, never who
// they are, so a display feature adds no new PII surface.
func (rs *Resource) listCareOfferingBookingStats(w http.ResponseWriter, r *http.Request) {
	if rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("care offering service not configured")))
		return
	}
	phaseID, parseErr := strconv.ParseInt(r.URL.Query().Get("phase_id"), 10, 64)
	if parseErr != nil || phaseID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("phase_id is required")))
		return
	}
	var stats []CareOfferingBookingStat
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		loaded, loadErr := rs.CareOfferingService.ListBookingStats(ctx, phaseID)
		stats = loaded
		return loadErr
	})
	if err != nil {
		if errors.Is(err, capability.ErrCareOfferingInvalid) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("list care offering booking stats failed", err))
		return
	}
	out := make([]CareOfferingBookingStatsResponse, 0, len(stats))
	for _, stat := range stats {
		grades := make(map[string]int, len(stat.GradeLevels))
		for grade, count := range stat.GradeLevels {
			grades[strconv.Itoa(grade)] = count
		}
		out = append(out, CareOfferingBookingStatsResponse{
			OfferingID:        strconv.FormatInt(stat.OfferingID, 10),
			Capacity:          stat.Capacity,
			Booked:            stat.Booked,
			GradeLevels:       grades,
			UnknownGradeCount: stat.UnknownGradeCount,
		})
	}
	common.Respond(w, r, http.StatusOK, out, "Care offering booking stats retrieved")
}

func (rs *Resource) getCareOffering(w http.ResponseWriter, r *http.Request) {
	if rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("care offering service not configured")))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	var offering *CareOffering
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		o, e := rs.CareOfferingService.GetByID(ctx, id)
		offering = o
		return e
	})
	if err != nil {
		if errors.Is(err, capability.ErrCareOfferingNotFound) {
			common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentCareOfferingNotFound))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("load care offering failed", err))
		return
	}
	common.Respond(w, r, http.StatusOK, toCareOfferingResponse(offering), "Care offering retrieved")
}

func (rs *Resource) createCareOffering(w http.ResponseWriter, r *http.Request) {
	if rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("care offering service not configured")))
		return
	}
	req := &CareOfferingRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	var offering *CareOffering
	model, err := req.toModel(0)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		o, e := rs.CareOfferingService.Create(ctx, model)
		offering = o
		return e
	})
	if err != nil {
		common.RenderError(w, r, careOfferingWriteErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, toCareOfferingResponse(offering), "Care offering created")
}

func (rs *Resource) updateCareOffering(w http.ResponseWriter, r *http.Request) {
	req := &CareOfferingRequest{}
	updateWithRefetch(rs, w, r, rs.CareOfferingService == nil, "care offering service not configured",
		func(r *http.Request, id int64) (*CareOffering, error) {
			if err := render.Bind(r, req); err != nil {
				return nil, err
			}
			return req.toModel(id)
		},
		func(ctx context.Context, model *CareOffering) error {
			// Older callers do not send translations. Preserve their document;
			// an explicit {} still reaches normalization and clears it.
			if req.Translations == nil {
				existing, err := rs.CareOfferingService.GetByID(ctx, model.ID)
				if err != nil {
					return err
				}
				if existing == nil {
					return errors.New("care offering service returned no offering")
				}
				model.Translations = existing.Translations
			}
			return rs.CareOfferingService.Update(ctx, model)
		},
		func(ctx context.Context, id int64) (*CareOffering, error) {
			return rs.CareOfferingService.GetByID(ctx, id)
		},
		func(o *CareOffering) any { return toCareOfferingResponse(o) },
		"Care offering updated",
		careOfferingWriteErrorRenderer)
}

func (rs *Resource) deleteCareOffering(w http.ResponseWriter, r *http.Request) {
	if rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("care offering service not configured")))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		return rs.CareOfferingService.Delete(ctx, id)
	})
	if err != nil {
		// FK violation when a selection or care booking already references
		// this offering — admin should soft-delete (is_active=false).
		if common.IsConstraintViolation(err) {
			common.RenderError(w, r, common.ErrorInvalidRequestWithCode(
				//nolint:staticcheck // ST1005: user-facing German message
				errors.New("Das Betreuungsangebot wird bereits verwendet und kann nicht gelöscht werden. Deaktivieren Sie es stattdessen."),
				common.CodeEnrollmentCareOfferingInUse,
			))
			return
		}
		if errors.Is(err, capability.ErrCareOfferingInvalid) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("delete care offering failed", err))
		return
	}
	common.RespondNoContent(w, r)
}

func (rs *Resource) cloneCareOffering(w http.ResponseWriter, r *http.Request) {
	if rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("care offering service not configured")))
		return
	}
	sourceID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	req := &CloneCareOfferingRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	var clone *CareOffering
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		c, e := rs.CareOfferingService.Clone(ctx, sourceID, req.TargetPhaseID)
		clone = c
		return e
	})
	if err != nil {
		common.RenderError(w, r, careOfferingWriteErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, toCareOfferingResponse(clone), "Care offering cloned")
}
