package students

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type pickupAdjustmentSelectionRequest struct {
	OfferingID   string   `json:"offering_id"`
	SelectedDays []string `json:"selected_days"`
}

type pickupAdjustmentRequest struct {
	Schedules               []PickupScheduleRequest             `json:"schedules"`
	ArrivalSchedules        *[]ArrivalScheduleRequestItem       `json:"arrival_schedules,omitempty"`
	CareDays                []int                               `json:"care_days"`
	EffectiveFrom           *timezone.Date                      `json:"effective_from,omitempty"`
	Selections              *[]pickupAdjustmentSelectionRequest `json:"selections,omitempty"`
	ExcludedAutoOfferingIDs []string                            `json:"excluded_auto_offering_ids,omitempty"`
	PreviewToken            string                              `json:"preview_token,omitempty"`
	Resolution              string                              `json:"resolution,omitempty"`
	Reason                  string                              `json:"reason,omitempty"`
}

func (r *pickupAdjustmentRequest) Bind(_ *http.Request) error {
	if err := validatePickupScheduleItems(r.Schedules); err != nil {
		return err
	}
	if len(r.Schedules) > 0 && len(r.CareDays) == 0 {
		return errors.New("care_days must not be empty when schedules are set")
	}
	if r.ArrivalSchedules != nil {
		if err := validateArrivalScheduleItems(*r.ArrivalSchedules); err != nil {
			return err
		}
	}
	if utf8.RuneCountInString(r.Reason) > 255 {
		return errors.New("reason cannot exceed 255 characters")
	}
	return nil
}

type pickupAdjustmentMatchResponse struct {
	OfferingID   string                             `json:"offering_id"`
	Name         string                             `json:"name"`
	SelectedDays []string                           `json:"selected_days"`
	Selections   []pickupAdjustmentSelectionRequest `json:"selections"`
}

type pickupAdjustmentCatalogItemResponse struct {
	OfferingID      string            `json:"offering_id"`
	Name            string            `json:"name"`
	Description     string            `json:"description,omitempty"`
	DaysOfWeekMode  string            `json:"days_of_week_mode"`
	AvailableDays   []string          `json:"available_days"`
	SelectionGroup  string            `json:"selection_group,omitempty"`
	SelectionRule   string            `json:"selection_rule"`
	IsRequired      bool              `json:"is_required"`
	PriceCents      *int              `json:"price_cents,omitempty"`
	IncludesLunch   bool              `json:"includes_lunch"`
	IncludesHoliday bool              `json:"includes_holiday_care"`
	Selected        bool              `json:"selected"`
	SelectedDays    []string          `json:"selected_days"`
	Automatic       bool              `json:"automatic"`
	IsActive        bool              `json:"is_active"`
	Capacity        *int              `json:"capacity,omitempty"`
	FreeSlots       *int              `json:"free_slots,omitempty"`
	PickupTimes     map[string]string `json:"pickup_times"`
	CountsAsCare    bool              `json:"counts_as_care"`
}

type pickupAdjustmentCatalogResponse struct {
	PhaseID               string                                `json:"phase_id"`
	PhaseName             string                                `json:"phase_name"`
	SelectionMode         string                                `json:"selection_mode"`
	EarliestEffectiveFrom string                                `json:"earliest_effective_from"`
	LatestEffectiveFrom   string                                `json:"latest_effective_from"`
	Items                 []pickupAdjustmentCatalogItemResponse `json:"items"`
}

type pickupAdjustmentConsequenceSelectionResponse struct {
	OfferingID string   `json:"offering_id"`
	State      string   `json:"state"`
	Days       []string `json:"days"`
}

type pickupAdjustmentConflictResponse struct {
	ActivityGroupID   string   `json:"activity_group_id"`
	ActivityGroupName string   `json:"activity_group_name"`
	Days              []string `json:"days"`
	FirstDate         string   `json:"first_date"`
	OccurrenceCount   int      `json:"occurrence_count"`
}

type pickupAdjustmentConsequencesResponse struct {
	Selections                        []pickupAdjustmentConsequenceSelectionResponse `json:"selections"`
	ManualPlanningConflicts           []pickupAdjustmentConflictResponse             `json:"manual_planning_conflicts"`
	ArrivalExpectationsFollowBookings bool                                           `json:"arrival_expectations_follow_bookings"`
}

type pickupAdjustmentPreviewResponse struct {
	PreviewToken         string                                `json:"preview_token"`
	EffectiveFrom        string                                `json:"effective_from"`
	CurrentPlan          string                                `json:"current_plan"`
	ProposedPlan         string                                `json:"proposed_plan"`
	DeviatesFromOffering bool                                  `json:"deviates_from_offering"`
	ResolutionRequired   bool                                  `json:"resolution_required"`
	MatchingOfferings    []pickupAdjustmentMatchResponse       `json:"matching_offerings"`
	OfferingCatalog      *pickupAdjustmentCatalogResponse      `json:"offering_catalog,omitempty"`
	OfferingConsequences *pickupAdjustmentConsequencesResponse `json:"offering_consequences,omitempty"`
	RemovedManualNotes   []pickupAdjustmentRemovedNoteResponse `json:"removed_manual_notes,omitempty"`
}

type pickupAdjustmentRemovedNoteResponse struct {
	Weekday int    `json:"weekday"`
	Note    string `json:"note"`
}

func (rs *Resource) previewStudentPickupAdjustment(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupWriteAccess(w, r, "preview pickup schedule adjustment")
	if student == nil {
		return
	}
	if rs.PickupAdjustmentService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("pickup adjustment service not configured")))
		return
	}
	var body pickupAdjustmentRequest
	if err := render.Bind(r, &body); err != nil {
		renderError(w, r, common.ErrorInvalidRequestWithCode(err, "pickup.invalid"))
		return
	}
	input, err := pickupAdjustmentPreviewInput(student.ID, body)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequestWithCode(err, "pickup.invalid"))
		return
	}
	preview, err := rs.PickupAdjustmentService.Preview(r.Context(), input)
	if err != nil {
		renderPickupAdjustmentError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toPickupAdjustmentPreviewResponse(preview), "Pickup adjustment previewed")
}

func (rs *Resource) applyStudentPickupAdjustment(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupWriteAccess(w, r, "apply pickup schedule adjustment")
	if student == nil {
		return
	}
	if rs.PickupAdjustmentService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("pickup adjustment service not configured")))
		return
	}
	var body pickupAdjustmentRequest
	if err := render.Bind(r, &body); err != nil {
		renderError(w, r, common.ErrorInvalidRequestWithCode(err, "pickup.invalid"))
		return
	}
	input, err := pickupAdjustmentPreviewInput(student.ID, body)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequestWithCode(err, "pickup.invalid"))
		return
	}
	staffID, err := rs.getStaffIDFromJWT(r)
	if err != nil {
		renderError(w, r, common.ErrorForbidden(err))
		return
	}
	applyInput := rs.pickupAdjustmentApplyInput(r, input, body, staffID)
	result, err := rs.PickupAdjustmentService.Apply(r.Context(), applyInput)
	if err != nil {
		renderPickupAdjustmentError(w, r, err)
		return
	}
	tenantID := tenant.FromContext(r.Context())
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.wakeChildGuardians(tenantID, student.ID)
		rs.broadcastPickupScheduleChanged(tenantID, student.ID)
		if body.ArrivalSchedules != nil {
			rs.broadcastArrivalScheduleChanged(student.ID)
		}
	})
	common.Respond(w, r, http.StatusOK, result, "Pickup adjustment applied")
}

func (rs *Resource) pickupAdjustmentApplyInput(
	r *http.Request,
	input enrollmentService.PickupAdjustmentPreviewInput,
	body pickupAdjustmentRequest,
	staffID int64,
) enrollmentService.PickupAdjustmentApplyInput {
	claims := jwt.ClaimsFromCtx(r.Context())
	permissions := jwt.PermissionsFromCtx(r.Context())
	return enrollmentService.PickupAdjustmentApplyInput{
		PickupAdjustmentPreviewInput: input,
		PreviewToken:                 body.PreviewToken, Resolution: body.Resolution, Reason: body.Reason,
		ActorAccountID: int64(claims.ID), ActorRole: strings.Join(claims.Roles, ","),
		CreatedByStaffID: staffID,
		Authorize: func(ctx context.Context, fresh *users.Student) (bool, error) {
			return canUpdateStudent(ctx, permissions, fresh, rs.UserContextService)
		},
	}
}

func pickupAdjustmentPreviewInput(
	studentID int64,
	body pickupAdjustmentRequest,
) (enrollmentService.PickupAdjustmentPreviewInput, error) {
	effectiveFrom := timezone.TodayDate()
	if body.EffectiveFrom != nil {
		effectiveFrom = *body.EffectiveFrom
	}
	schedules := make([]enrollmentService.PickupAdjustmentSchedule, 0, len(body.Schedules))
	for _, row := range body.Schedules {
		schedules = append(schedules, enrollmentService.PickupAdjustmentSchedule{
			Weekday: row.Weekday, PickupTime: row.PickupTime, Notes: row.Notes,
		})
	}
	var selections []enrollmentService.OfferingChangeSelection
	if body.Selections != nil {
		selections = make([]enrollmentService.OfferingChangeSelection, 0, len(*body.Selections))
		for _, row := range *body.Selections {
			id, err := strconv.ParseInt(strings.TrimSpace(row.OfferingID), 10, 64)
			if err != nil || id <= 0 {
				return enrollmentService.PickupAdjustmentPreviewInput{}, errors.New("offering_id must be a positive number")
			}
			selections = append(selections, enrollmentService.OfferingChangeSelection{
				OfferingID: id, SelectedDays: row.SelectedDays,
			})
		}
	}
	excluded, err := parseExcludedOfferingIDs(body.ExcludedAutoOfferingIDs)
	if err != nil {
		return enrollmentService.PickupAdjustmentPreviewInput{}, err
	}
	var arrivalSchedules *[]enrollmentService.PickupAdjustmentArrivalSchedule
	if body.ArrivalSchedules != nil {
		rows := make([]enrollmentService.PickupAdjustmentArrivalSchedule, 0, len(*body.ArrivalSchedules))
		for _, row := range *body.ArrivalSchedules {
			rows = append(rows, enrollmentService.PickupAdjustmentArrivalSchedule{
				Weekday: row.Weekday, ExpectedArrival: row.ExpectedArrival, Notes: row.Notes,
			})
		}
		arrivalSchedules = &rows
	}
	return enrollmentService.PickupAdjustmentPreviewInput{
		StudentID:               studentID,
		Schedules:               schedules,
		ArrivalSchedules:        arrivalSchedules,
		CareDays:                body.CareDays,
		EffectiveFrom:           effectiveFrom,
		Selections:              selections,
		ExcludedAutoOfferingIDs: excluded,
	}, nil
}

func toPickupAdjustmentPreviewResponse(preview *enrollmentService.PickupAdjustmentPreview) pickupAdjustmentPreviewResponse {
	response := pickupAdjustmentPreviewResponse{
		PreviewToken:         preview.PreviewToken,
		EffectiveFrom:        preview.EffectiveFrom.String(),
		CurrentPlan:          preview.CurrentPlan,
		ProposedPlan:         preview.ProposedPlan,
		DeviatesFromOffering: preview.DeviatesFromOffering,
		ResolutionRequired:   preview.ResolutionRequired,
		MatchingOfferings:    make([]pickupAdjustmentMatchResponse, 0, len(preview.MatchingOfferings)),
	}
	for _, match := range preview.MatchingOfferings {
		item := pickupAdjustmentMatchResponse{
			OfferingID: strconv.FormatInt(match.OfferingID, 10), Name: match.Name,
			SelectedDays: match.SelectedDays,
			Selections:   make([]pickupAdjustmentSelectionRequest, 0, len(match.Selections)),
		}
		for _, selection := range match.Selections {
			item.Selections = append(item.Selections, pickupAdjustmentSelectionRequest{
				OfferingID: strconv.FormatInt(selection.OfferingID, 10), SelectedDays: selection.SelectedDays,
			})
		}
		response.MatchingOfferings = append(response.MatchingOfferings, item)
	}
	response.OfferingCatalog = pickupAdjustmentCatalogResponseFrom(preview.OfferingCatalog)
	response.OfferingConsequences = pickupAdjustmentConsequencesResponseFrom(preview.OfferingConsequences)
	for _, note := range preview.RemovedManualNotes {
		response.RemovedManualNotes = append(response.RemovedManualNotes, pickupAdjustmentRemovedNoteResponse{
			Weekday: note.Weekday,
			Note:    note.Note,
		})
	}
	return response
}

func pickupAdjustmentCatalogResponseFrom(catalog *enrollmentService.OfferingChangeCatalog) *pickupAdjustmentCatalogResponse {
	if catalog == nil {
		return nil
	}
	response := &pickupAdjustmentCatalogResponse{
		PhaseID: strconv.FormatInt(catalog.PhaseID, 10), PhaseName: catalog.PhaseName,
		SelectionMode: catalog.SelectionMode, EarliestEffectiveFrom: catalog.EarliestEffectiveFrom.String(),
		LatestEffectiveFrom: catalog.LatestEffectiveFrom.String(),
		Items:               make([]pickupAdjustmentCatalogItemResponse, 0, len(catalog.Items)),
	}
	for _, item := range catalog.Items {
		response.Items = append(response.Items, pickupAdjustmentCatalogItemResponse{
			OfferingID: strconv.FormatInt(item.OfferingID, 10), Name: item.Name, Description: item.Description,
			DaysOfWeekMode: item.DaysOfWeekMode, AvailableDays: item.AvailableDays,
			SelectionGroup: item.SelectionGroup, SelectionRule: item.SelectionRule, IsRequired: item.IsRequired,
			PriceCents: item.PriceCents, IncludesLunch: item.IncludesLunch, IncludesHoliday: item.IncludesHoliday,
			Selected: item.Selected, SelectedDays: item.SelectedDays, Automatic: item.Automatic, IsActive: item.IsActive,
			Capacity: item.Capacity, FreeSlots: item.FreeSlots, PickupTimes: item.PickupTimes,
			CountsAsCare: item.CountsAsCare,
		})
	}
	return response
}

func pickupAdjustmentConsequencesResponseFrom(preview *enrollmentService.OfferingChangePreview) *pickupAdjustmentConsequencesResponse {
	if preview == nil {
		return nil
	}
	response := &pickupAdjustmentConsequencesResponse{
		Selections:                        make([]pickupAdjustmentConsequenceSelectionResponse, 0, len(preview.Selections)),
		ManualPlanningConflicts:           make([]pickupAdjustmentConflictResponse, 0, len(preview.ManualPlanningConflicts)),
		ArrivalExpectationsFollowBookings: preview.ArrivalExpectationsFollowBookings,
	}
	for _, selection := range preview.Selections {
		response.Selections = append(response.Selections, pickupAdjustmentConsequenceSelectionResponse{
			OfferingID: strconv.FormatInt(selection.OfferingID, 10), State: selection.State, Days: selection.Days,
		})
	}
	for _, conflict := range preview.ManualPlanningConflicts {
		response.ManualPlanningConflicts = append(response.ManualPlanningConflicts, pickupAdjustmentConflictResponse{
			ActivityGroupID: strconv.FormatInt(conflict.ActivityGroupID, 10), ActivityGroupName: conflict.ActivityGroupName,
			Days: conflict.Days, FirstDate: conflict.FirstDate.String(), OccurrenceCount: conflict.OccurrenceCount,
		})
	}
	return response
}

func renderPickupAdjustmentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, enrollmentService.ErrPickupAdjustmentResolutionRequired):
		renderError(w, r, common.ErrorInvalidRequestWithCode(err, "pickup.resolution_required"))
	case errors.Is(err, enrollmentService.ErrPickupAdjustmentStale):
		renderError(w, r, common.ErrorConflictWithCode(err, "pickup.preview_stale"))
	case errors.Is(err, enrollmentService.ErrPickupAdjustmentFutureManualReset):
		renderError(w, r, common.ErrorConflictWithCode(err, "pickup.future_manual_reset"))
	case errors.Is(err, enrollmentService.ErrOfferingChangeCapacityFull):
		renderError(w, r, common.ErrorConflictWithCode(err, "pickup.offering_capacity_full"))
	case errors.Is(err, enrollmentService.ErrOfferingChangeDateOutOfRange),
		errors.Is(err, enrollmentService.ErrOfferingChangeInvalid),
		errors.Is(err, enrollmentService.ErrPickupAdjustmentInvalid):
		renderError(w, r, common.ErrorInvalidRequestWithCode(err, "pickup.invalid"))
	case errors.Is(err, enrollmentService.ErrPickupAdjustmentUnauthorized):
		renderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, enrollmentService.ErrPickupAdjustmentStudentNotFound):
		renderError(w, r, common.ErrorNotFound(err))
	default:
		renderError(w, r, common.ErrorInternalServer(err))
	}
}
