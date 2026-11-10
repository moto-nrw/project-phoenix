package application

import (
	"context"
	"log/slog"
	"net"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

// The moto team records and withdraws the erasure order of a deactivated
// Träger (#4657). The order and its operator audit entry commit together;
// without the audit nothing is recorded.

// RecordTraegerErasureOrder records the written confirmation and the end of
// the grace period of a deactivated Träger.
func (p *Provisioning) RecordTraegerErasureOrder(ctx context.Context, input organizationtenancy.RecordTraegerErasureOrder) (organizationtenancy.TraegerErasureOrder, error) {
	order, err := adminValue(ctx, p, func(adminCtx context.Context) (organizationtenancy.TraegerErasureOrder, error) {
		order, err := p.organizations.CreateTraegerErasureOrder(adminCtx, organizationtenancy.NewTraegerErasureOrder{
			OrganizationID: input.OrganizationID, OrderReference: input.OrderReference,
			ConfirmedOn: input.ConfirmedOn, EraseNotBefore: input.EraseNotBefore, RecordedBy: input.OperatorID,
			Records: input.Records,
		})
		if err != nil {
			return organizationtenancy.TraegerErasureOrder{}, mapOrganizationError(err, input.OrganizationID)
		}
		return order, p.recordAction(adminCtx, input.OperatorID, domain.AuditActionRecordErasureOrder, domain.AuditResourceOrganization, &input.OrganizationID, input.ClientIP, map[string]any{
			"order_id":         order.ID,
			"order_reference":  order.OrderReference,
			"confirmed_on":     order.ConfirmedOn.String(),
			"erase_not_before": order.EraseNotBefore.String(),
			// The hint on records subject to retention and the customer's
			// choice (#4720): a day and a checksum, no personal data.
			"records_notice_acknowledged": order.RecordsNoticeShownAt != nil,
			"records_choice":              string(order.RecordsChoice),
			"records_handed_over_on":      recordsHandedOverOn(order),
			"records_export_sha256":       order.RecordsExportSHA256,
		})
	})
	if err != nil {
		return organizationtenancy.TraegerErasureOrder{}, err
	}
	p.logger.Info("traeger_erasure_order_recorded",
		slog.Int64("organization_id", input.OrganizationID),
		slog.Int64("order_id", order.ID),
		slog.Int64("operator_id", input.OperatorID),
		slog.String("records_choice", string(order.RecordsChoice)))
	return order, nil
}

func recordsHandedOverOn(order organizationtenancy.TraegerErasureOrder) string {
	if order.RecordsHandedOverOn == nil {
		return ""
	}
	return order.RecordsHandedOverOn.String()
}

var _ organizationtenancy.TraegerErasureOrderRecordsProvisioning = (*Provisioning)(nil)

// RecordTraegerErasureOrderRecords records the hint and the customer's
// choice on an open order recorded without them, with the operator audit.
func (p *Provisioning) RecordTraegerErasureOrderRecords(ctx context.Context, input organizationtenancy.RecordTraegerErasureOrderRecords) (organizationtenancy.TraegerErasureOrder, error) {
	organizationID := input.OrganizationID
	order, err := adminValue(ctx, p, func(adminCtx context.Context) (organizationtenancy.TraegerErasureOrder, error) {
		order, err := p.erasureRecords.RecordTraegerErasureOrderRecords(adminCtx, organizationID, input.Records)
		if err != nil {
			return organizationtenancy.TraegerErasureOrder{}, mapOrganizationError(err, organizationID)
		}
		return order, p.recordAction(adminCtx, input.OperatorID, domain.AuditActionRecordErasureOrderRecords, domain.AuditResourceOrganization, &organizationID, input.ClientIP, map[string]any{
			"order_id":                    order.ID,
			"order_reference":             order.OrderReference,
			"started":                     order.StartedAt != nil,
			"records_notice_acknowledged": order.RecordsNoticeShownAt != nil,
			"records_choice":              string(order.RecordsChoice),
			"records_handed_over_on":      recordsHandedOverOn(order),
			"records_export_sha256":       order.RecordsExportSHA256,
		})
	})
	if err != nil {
		return organizationtenancy.TraegerErasureOrder{}, err
	}
	p.logger.Info("traeger_erasure_order_records_recorded",
		slog.Int64("organization_id", organizationID),
		slog.Int64("order_id", order.ID),
		slog.Int64("operator_id", input.OperatorID),
		slog.String("records_choice", string(order.RecordsChoice)))
	return order, nil
}

// WithdrawTraegerErasureOrder withdraws the open order during the grace
// period. The Träger stays deactivated.
func (p *Provisioning) WithdrawTraegerErasureOrder(ctx context.Context, organizationID, operatorID int64, clientIP net.IP) error {
	_, err := adminValue(ctx, p, func(adminCtx context.Context) (organizationtenancy.TraegerErasureOrder, error) {
		order, err := p.organizations.WithdrawTraegerErasureOrder(adminCtx, organizationID, operatorID)
		if err != nil {
			return organizationtenancy.TraegerErasureOrder{}, mapOrganizationError(err, organizationID)
		}
		return order, p.recordAction(adminCtx, operatorID, domain.AuditActionWithdrawErasureOrder, domain.AuditResourceOrganization, &organizationID, clientIP, map[string]any{
			"order_id":        order.ID,
			"order_reference": order.OrderReference,
		})
	})
	if err != nil {
		return err
	}
	p.logger.Info("traeger_erasure_order_withdrawn",
		slog.Int64("organization_id", organizationID),
		slog.Int64("operator_id", operatorID))
	return nil
}
