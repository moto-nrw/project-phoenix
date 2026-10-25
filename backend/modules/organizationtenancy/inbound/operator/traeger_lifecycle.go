package operator

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The erasure order of a deactivated Träger (#4657): the operator records the
// written confirmation with the end of the grace period and may withdraw the
// order until the erasure CLI completes it.

type recordTraegerErasureOrderRequest struct {
	OrderReference string `json:"order_reference"`
	ConfirmedOn    string `json:"confirmed_on"`
	EraseNotBefore string `json:"erase_not_before"`

	confirmedOn    calendar.Date
	eraseNotBefore calendar.Date
}

func (req *recordTraegerErasureOrderRequest) Bind(_ *http.Request) error {
	req.OrderReference = strings.TrimSpace(req.OrderReference)
	var err error
	if req.confirmedOn, err = calendar.ParseDate(strings.TrimSpace(req.ConfirmedOn)); err != nil {
		return errors.New("confirmed_on must be a date (YYYY-MM-DD)")
	}
	if req.eraseNotBefore, err = calendar.ParseDate(strings.TrimSpace(req.EraseNotBefore)); err != nil {
		return errors.New("erase_not_before must be a date (YYYY-MM-DD)")
	}
	return nil
}

// TraegerErasureOrder answers GET /operator/organizations/{id}/erasure-order:
// the open order, or null without one.
func (rs *ProvisioningResource) TraegerErasureOrder(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return
	}
	order, found, err := rs.service.FindOpenTraegerErasureOrder(r.Context(), organizationID)
	if errors.Is(err, organizationtenancy.ErrOrganizationNotFound) {
		err = &organizationtenancy.OrganizationNotFoundError{OrganizationID: organizationID}
	}
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	if !found {
		common.Respond(w, r, http.StatusOK, nil, "No open erasure order")
		return
	}
	common.Respond(w, r, http.StatusOK, order, "Erasure order retrieved successfully")
}

// RecordTraegerErasureOrder answers POST /operator/organizations/{id}/erasure-order.
func (rs *ProvisioningResource) RecordTraegerErasureOrder(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return
	}
	req := &recordTraegerErasureOrderRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	order, err := rs.service.RecordTraegerErasureOrder(r.Context(), organizationtenancy.RecordTraegerErasureOrder{
		OrganizationID: organizationID, OrderReference: req.OrderReference,
		ConfirmedOn: req.confirmedOn, EraseNotBefore: req.eraseNotBefore,
		OperatorID: int64(sessiontoken.ClaimsFromCtx(r.Context()).ID), ClientIP: common.ParseClientIP(r),
	})
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, order, "Erasure order recorded successfully")
}

// WithdrawTraegerErasureOrder answers DELETE /operator/organizations/{id}/erasure-order.
func (rs *ProvisioningResource) WithdrawTraegerErasureOrder(w http.ResponseWriter, r *http.Request) {
	common.OperatorAuditedIDAction(w, r, int64(sessiontoken.ClaimsFromCtx(r.Context()).ID), "id", "invalid organization ID",
		rs.service.WithdrawTraegerErasureOrder, ProvisioningErrorRenderer, "Erasure order withdrawn successfully")
}

// traegerLifecycleErrorRenderer maps the refusals of the Träger lifecycle.
func traegerLifecycleErrorRenderer(err error) (render.Renderer, bool) {
	switch {
	case errors.Is(err, organizationtenancy.ErrTraegerStillActive):
		return common.OperatorRejection(http.StatusConflict, common.CodeProvisioningTraegerActive, "Organization is still active"), true
	case errors.Is(err, organizationtenancy.ErrTraegerErasureOrderOpen):
		return common.OperatorRejection(http.StatusConflict, common.CodeProvisioningTraegerErasureOrderOpen, "Organization has an open erasure order"), true
	case errors.Is(err, organizationtenancy.ErrTraegerErasureStarted):
		return common.OperatorRejection(http.StatusConflict, common.CodeProvisioningTraegerErasureStarted, "Organization erasure has started"), true
	case errors.Is(err, organizationtenancy.ErrTraegerErasureOrderNotFound):
		return common.OperatorRejection(http.StatusNotFound, common.CodeProvisioningTraegerErasureOrderNotFound, "Organization has no open erasure order"), true
	}
	return nil, false
}
