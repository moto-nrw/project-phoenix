package enrollmenthttp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type AdminUpdateOfferingsRequest struct {
	Offerings                   []AdminUpdateOfferingSelection `json:"offerings"`
	Reason                      string                         `json:"reason"`
	EffectiveFrom               string                         `json:"effective_from,omitempty"`
	CompleteWithdrawalConfirmed bool                           `json:"complete_withdrawal_confirmed"`
}

type AdminUpdateOfferingSelection struct {
	OfferingID   string   `json:"offering_id"`
	SelectedDays []string `json:"selected_days,omitempty"`
}

func (req *AdminUpdateOfferingsRequest) Bind(_ *http.Request) error { return nil }

// renderRequestNotFound answers a missing request or child with 404 and a
// code, so the page can say the enrollment is gone instead of "check input".
func renderRequestNotFound(err error) render.Renderer {
	return common.ErrorNotFoundWithCode(err, common.CodeEnrollmentRequestNotFound)
}

var updateAdminOfferingsErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: capability.ErrDecisionChildNotFound, Render: renderRequestNotFound},
	{Target: capability.ErrDecisionRequestNotFound, Render: renderRequestNotFound},
	{Target: capability.ErrOfferingAdjustmentInvalid, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentOfferingAdjustmentInvalid)
	}},
	{Target: capability.ErrCareOfferingClosed, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentCareOfferingClosed)
	}},
	{Target: capability.ErrRequiredCareOfferingMissing, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentRequiredCareOfferingMissing)
	}},
	{Target: capability.ErrCareOfferingMissing, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentCareOfferingMissing)
	}},
	{Target: capability.ErrCareOfferingExactlyOneRequired, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentCareOfferingExactlyOne)
	}},
	{Target: capability.ErrCareOfferingsDisabled, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentCareOfferingsDisabled)
	}},
	{Target: capability.ErrCompleteWithdrawalConfirmationRequired, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeEnrollmentCompleteWithdrawalConfirmationRequired)
	}},
}, common.ErrorInternalServer)

func (rs *Resource) updateAdminChildOfferings(w http.ResponseWriter, r *http.Request) {
	if rs.DecisionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("decision service not configured")))
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	childID, ok := common.ParsePositiveInt64IDWithError(w, r, "childId", "invalid childId")
	if !ok {
		return
	}
	body := &AdminUpdateOfferingsRequest{}
	if err := render.Bind(r, body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	updated, err := rs.applyAdminOfferingUpdate(r, requestID, childID, body)
	if err != nil {
		common.RenderError(w, r, updateAdminOfferingsErrorRenderer(err))
		return
	}
	out := newAdminRequestChild(updated)
	out.CustomData = updated.CustomData
	rs.enrichUpdatedAdminOfferings(r, requestID, childID, &out)
	common.Respond(w, r, http.StatusOK, out, "Child offerings updated")
}

func (rs *Resource) applyAdminOfferingUpdate(
	r *http.Request, requestID, childID int64, body *AdminUpdateOfferingsRequest,
) (*RequestChild, error) {
	selections, effectiveFrom, err := parseAdminOfferingUpdate(body)
	if err != nil {
		return nil, err
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	var updated *RequestChild
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		child, updateErr := rs.DecisionService.UpdateChildOfferings(ctx, UpdateChildOfferingsInput{
			RequestID: requestID, ChildID: childID, Offerings: selections, Reason: body.Reason,
			ActorAccountID: int64(claims.ID), ActorRole: actorRoleFromClaims(claims.Roles),
			EffectiveFrom: effectiveFrom, CompleteWithdrawalConfirmed: body.CompleteWithdrawalConfirmed,
		})
		updated = child
		return updateErr
	})
	return updated, err
}

func parseAdminOfferingUpdate(
	body *AdminUpdateOfferingsRequest,
) ([]OfferingAdjustmentSelection, *calendar.Date, error) {
	selections := make([]OfferingAdjustmentSelection, 0, len(body.Offerings))
	for _, row := range body.Offerings {
		offeringID, parseErr := strconv.ParseInt(row.OfferingID, 10, 64)
		if parseErr != nil || offeringID <= 0 {
			return nil, nil, fmt.Errorf("%w: invalid offering_id", capability.ErrOfferingAdjustmentInvalid)
		}
		selections = append(selections, OfferingAdjustmentSelection{
			OfferingID:   offeringID,
			SelectedDays: row.SelectedDays,
		})
	}
	var effectiveFrom *calendar.Date
	if strings.TrimSpace(body.EffectiveFrom) != "" {
		parsed, parseErr := calendar.ParseDate(body.EffectiveFrom)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("%w: das Datum muss im Format JJJJ-MM-TT angegeben werden", capability.ErrOfferingAdjustmentInvalid)
		}
		effectiveFrom = &parsed
	}
	return selections, effectiveFrom, nil
}

func (rs *Resource) enrichUpdatedAdminOfferings(
	r *http.Request, requestID, childID int64, out *AdminRequestChild,
) {
	// The correction is already committed. This re-read only enriches the
	// response, so a transient failure here must not report the save as
	// failed — the admin would retry and apply the replacement twice, with
	// a second audit entry blaming them for it. Flag it instead, so the
	// client refuses to save on top of an unknown selection.
	if err := rs.runInTenantTx(r, func(ctx context.Context) error {
		rowsByChild, listErr := rs.DecisionService.ListChildOfferings(ctx, requestID)
		if listErr != nil {
			return listErr
		}
		set := rowsByChild[childID]
		out.Offerings = toAdminChildOfferings(set.Current)
		out.UpcomingOfferings = toAdminChildOfferings(set.Upcoming)
		return nil
	}); err != nil {
		out.OfferingsUnavailable = true
		slog.WarnContext(r.Context(), "offering adjustment saved but re-read failed",
			slog.String("error", err.Error()),
			slog.Int64("request_id", requestID),
			slog.Int64("child_id", childID),
		)
	}
}

func optionalInt64String(value *int64) string {
	if value == nil || *value <= 0 {
		return ""
	}
	return strconv.FormatInt(*value, 10)
}

// optionalDateString renders a nullable calendar date as ISO
// "YYYY-MM-DD", or "" so the field drops out of the JSON payload.
func optionalDateString(value *calendar.Date) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.String()
}

// optionalInclusiveEndDateString renders a stored EXCLUSIVE interval end
// as the inclusive last covered day, the shape the parent endpoint has
// always used (modules/careplan/inbound/parent/care_offerings_handlers.go).
func optionalInclusiveEndDateString(value *calendar.Date) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.AddDays(-1).String()
}

func (rs *Resource) listAdminChildOfferingAdjustments(w http.ResponseWriter, r *http.Request) {
	if rs.DecisionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("decision service not configured")))
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	childID, ok := common.ParsePositiveInt64IDWithError(w, r, "childId", "invalid childId")
	if !ok {
		return
	}
	var rows []*OfferingAdjustment
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		list, listErr := rs.DecisionService.ListOfferingAdjustments(ctx, requestID, childID)
		rows = list
		return listErr
	})
	if err != nil {
		if errors.Is(err, capability.ErrDecisionChildNotFound) {
			common.RenderError(w, r, renderRequestNotFound(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	out := make([]AdminOfferingAdjustment, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAdminOfferingAdjustment(row))
	}
	common.Respond(w, r, http.StatusOK, out, "Offering adjustments retrieved")
}

func toAdminChildOffering(row ChildOfferingRow) AdminRequestChildOffering {
	return AdminRequestChildOffering{
		OfferingID:            strconv.FormatInt(row.OfferingID, 10),
		OfferingName:          row.OfferingName,
		DaysOfWeekMode:        row.DaysOfWeekMode,
		SelectedDays:          row.SelectedDays,
		ManualSelectedDays:    row.ManualSelectedDays,
		AutomaticSelectedDays: row.AutomaticSelectedDays,
		AvailableDays:         row.AvailableDays,
		IncludesLunch:         row.IncludesLunch,
		IncludesHolidayCare:   row.IncludesHolidayCare,
		PriceCents:            row.PriceCents,
		ValidFrom:             optionalDateString(row.ValidFrom),
		ValidUntil:            optionalInclusiveEndDateString(row.ValidUntil),
		StartsLater:           row.StartsLater,
	}
}

func toAdminOfferingAdjustment(row *OfferingAdjustment) AdminOfferingAdjustment {
	return AdminOfferingAdjustment{
		ID:                 strconv.FormatInt(row.ID, 10),
		RequestID:          strconv.FormatInt(row.RequestID, 10),
		RequestChildID:     strconv.FormatInt(row.RequestChildID, 10),
		StudentID:          strconv.FormatInt(row.StudentID, 10),
		ActorAccountID:     strconv.FormatInt(row.ActorAccountID, 10),
		ActorRole:          row.ActorRole,
		ActorNameSnapshot:  row.ActorNameSnapshot,
		ActorEmailSnapshot: row.ActorEmailSnapshot,
		Reason:             row.Reason,
		Before:             row.Before,
		After:              row.After,
		ChangedAt:          row.ChangedAt,
	}
}
