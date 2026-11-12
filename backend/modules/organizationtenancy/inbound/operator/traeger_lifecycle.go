package operator

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The erasure order of a deactivated Träger (#4657): the operator records the
// written confirmation with the end of the grace period and may withdraw the
// order until the erasure CLI completes it.

type recordTraegerErasureOrderRequest struct {
	OrderReference string `json:"order_reference"`
	ConfirmedOn    string `json:"confirmed_on"`
	EraseNotBefore string `json:"erase_not_before"`
	// The hint on records subject to retention and the customer's choice
	// (#4720, AVV Ziff. 8.4).
	RecordsNoticeAcknowledged bool   `json:"records_notice_acknowledged"`
	RecordsChoice             string `json:"records_choice"`
	RecordsHandedOverOn       string `json:"records_handed_over_on"`
	RecordsExportSHA256       string `json:"records_export_sha256"`

	confirmedOn    calendar.Date
	eraseNotBefore calendar.Date
	handedOverOn   *calendar.Date
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
	if handedOverOn := strings.TrimSpace(req.RecordsHandedOverOn); handedOverOn != "" {
		day, err := calendar.ParseDate(handedOverOn)
		if err != nil {
			return errors.New("records_handed_over_on must be a date (YYYY-MM-DD)")
		}
		req.handedOverOn = &day
	}
	req.RecordsExportSHA256 = strings.ToLower(strings.TrimSpace(req.RecordsExportSHA256))
	return nil
}

func (req *recordTraegerErasureOrderRequest) records() organizationtenancy.RecordsHandover {
	return organizationtenancy.RecordsHandover{
		NoticeAcknowledged: req.RecordsNoticeAcknowledged, Choice: organizationtenancy.RecordsChoice(req.RecordsChoice),
		HandedOverOn: req.handedOverOn, ExportSHA256: req.RecordsExportSHA256,
	}
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
		ConfirmedOn: req.confirmedOn, EraseNotBefore: req.eraseNotBefore, Records: req.records(),
		OperatorID: int64(sessiontoken.ClaimsFromCtx(r.Context()).ID), ClientIP: common.ParseClientIP(r),
	})
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, order, "Erasure order recorded successfully")
}

// recordTraegerErasureOrderRecordsRequest is the hint and the customer's
// choice recorded on an open order afterwards (#4720).
type recordTraegerErasureOrderRecordsRequest struct {
	RecordsNoticeAcknowledged bool   `json:"records_notice_acknowledged"`
	RecordsChoice             string `json:"records_choice"`
	RecordsHandedOverOn       string `json:"records_handed_over_on"`
	RecordsExportSHA256       string `json:"records_export_sha256"`

	handedOverOn *calendar.Date
}

func (req *recordTraegerErasureOrderRecordsRequest) Bind(_ *http.Request) error {
	if handedOverOn := strings.TrimSpace(req.RecordsHandedOverOn); handedOverOn != "" {
		day, err := calendar.ParseDate(handedOverOn)
		if err != nil {
			return errors.New("records_handed_over_on must be a date (YYYY-MM-DD)")
		}
		req.handedOverOn = &day
	}
	req.RecordsExportSHA256 = strings.ToLower(strings.TrimSpace(req.RecordsExportSHA256))
	return nil
}

// TraegerErasureOrderRecordsResource serves the hint and the customer's
// choice recorded on an older open order (#4720). It depends on that
// capability alone, beside the provisioning facade (ADR 0178).
type TraegerErasureOrderRecordsResource struct {
	records organizationtenancy.TraegerErasureOrderRecordsProvisioning
}

// NewTraegerErasureOrderRecordsResource builds the records route; without
// the capability it is a construction error (ADR 0068 point 5).
func NewTraegerErasureOrderRecordsResource(records organizationtenancy.TraegerErasureOrderRecordsProvisioning) (*TraegerErasureOrderRecordsResource, error) {
	if records == nil {
		return nil, errors.New("operator provisioning: the erasure order records capability is required")
	}
	return &TraegerErasureOrderRecordsResource{records: records}, nil
}

// RecordTraegerErasureOrderRecords answers PUT
// /operator/organizations/{id}/erasure-order/records.
func (rs *TraegerErasureOrderRecordsResource) RecordTraegerErasureOrderRecords(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return
	}
	req := &recordTraegerErasureOrderRecordsRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	order, err := rs.records.RecordTraegerErasureOrderRecords(r.Context(), organizationtenancy.RecordTraegerErasureOrderRecords{
		OrganizationID: organizationID, OperatorID: int64(sessiontoken.ClaimsFromCtx(r.Context()).ID), ClientIP: common.ParseClientIP(r),
		Records: organizationtenancy.RecordsHandover{NoticeAcknowledged: req.RecordsNoticeAcknowledged,
			Choice: organizationtenancy.RecordsChoice(req.RecordsChoice), HandedOverOn: req.handedOverOn, ExportSHA256: req.RecordsExportSHA256},
	})
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, order, "Erasure order records recorded successfully")
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
	case errors.Is(err, organizationtenancy.ErrTraegerErasureRecordsRecorded):
		return common.OperatorRejection(http.StatusConflict, common.CodeProvisioningTraegerErasureRecordsRecorded, "Erasure order already records the customer's choice"), true
	case errors.Is(err, organizationtenancy.ErrTraegerErasureOrderNotFound):
		return common.OperatorRejection(http.StatusNotFound, common.CodeProvisioningTraegerErasureOrderNotFound, "Organization has no open erasure order"), true
	}
	return nil, false
}
