package organizationtenancy

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Träger lifecycle (#4657, spec #4611 user story 11): the moto team
// deactivates a Träger, which ends every Träger session, and after the end of
// the contract records the written confirmation of the erasure order. The
// order opens a grace period; only after it may the internal erasure CLI
// erase the schools and the Träger root (ADR 0092, #3837). During the grace
// period the order can be withdrawn. Every step is in the operator audit.

// ErrTraegerStillActive reports an erasure order for a Träger that is still
// active: the moto team deactivates it first.
var ErrTraegerStillActive = errors.New("organization must be deactivated before its erasure order")

// ErrTraegerErasureOrderOpen reports an open erasure order where none may be:
// a second order, or reactivating the Träger before the order is withdrawn.
var ErrTraegerErasureOrderOpen = errors.New("organization has an open erasure order")

// ErrTraegerErasureStarted reports a withdrawal after the erasure CLI began:
// a run that failed halfway is resumed with the same order.
var ErrTraegerErasureStarted = errors.New("organization erasure has started")

// ErrTraegerErasureRecordsRecorded reports a second record of the hint and
// the customer's choice: once recorded, the choice is evidence (#4720).
var ErrTraegerErasureRecordsRecorded = errors.New("organization erasure order already records the customer's choice")

// ErrTraegerErasureOrderNotFound reports a withdrawal without an open order.
var ErrTraegerErasureOrderNotFound = errors.New("organization has no open erasure order")

// TraegerErasureOrder is the open erasure order of a Träger. It carries no
// personal data: the order reference is a case number, the operators are IDs.
type TraegerErasureOrder struct {
	ID             int64         `json:"id"`
	OrganizationID int64         `json:"organization_id"`
	OrderReference string        `json:"order_reference"`
	ConfirmedOn    calendar.Date `json:"confirmed_on"`
	EraseNotBefore calendar.Date `json:"erase_not_before"`
	RecordedBy     int64         `json:"recorded_by"`
	RecordedAt     time.Time     `json:"recorded_at"`
	// StartedAt is set once the erasure CLI erased the first school; the
	// order can no longer be withdrawn.
	StartedAt *time.Time `json:"started_at,omitempty"`
	// RecordsNoticeShownAt is when the operator confirmed the hint on the
	// records subject to retention (AVV Ziff. 8.4, #4720); RecordsChoice is
	// what the customer chose. Both are empty on orders recorded before.
	RecordsNoticeShownAt *time.Time     `json:"records_notice_shown_at,omitempty"`
	RecordsChoice        RecordsChoice  `json:"records_choice,omitempty"`
	RecordsHandedOverOn  *calendar.Date `json:"records_handed_over_on,omitempty"`
	RecordsExportSHA256  string         `json:"records_export_sha256,omitempty"`
}

// RecordsChoice is the customer's choice after the hint on the records
// subject to retention (AVV Ziff. 8.2/8.4, #4720): the Rückgabe was handed
// over, or the customer chose lawful erasure. Either lets the order proceed;
// the hint never blocks the erasure indefinitely.
type RecordsChoice string

const (
	RecordsHandedOver    RecordsChoice = "handed_over"
	RecordsErasureChosen RecordsChoice = "erasure_chosen"
)

// RecordsHandover is the hint and the customer's choice an operator records
// with the erasure order. HandedOverOn is the day of the handover, only for
// RecordsHandedOver; ExportSHA256 optionally names the COMPLETE checksum of
// the handed-over Rückgabe export.
type RecordsHandover struct {
	NoticeAcknowledged bool
	Choice             RecordsChoice
	HandedOverOn       *calendar.Date
	ExportSHA256       string
}

// NewTraegerErasureOrder is an order the operator records: the case number,
// the day the written confirmation was received and the first day on which
// the erasure may run, the end of the grace period.
type NewTraegerErasureOrder struct {
	OrganizationID int64
	OrderReference string
	ConfirmedOn    calendar.Date
	EraseNotBefore calendar.Date
	RecordedBy     int64
	Records        RecordsHandover
}

// TraegerErasureOrderQuery reads the erasure order of a Träger.
type TraegerErasureOrderQuery interface {
	// FindOpenTraegerErasureOrder returns the open order of the Träger, if
	// any; an unknown Träger is ErrOrganizationNotFound.
	FindOpenTraegerErasureOrder(ctx context.Context, organizationID int64) (TraegerErasureOrder, bool, error)
}

// TraegerErasureOrderCommand writes the erasure order of a Träger on the
// caller's administrative transaction.
type TraegerErasureOrderCommand interface {
	// CreateTraegerErasureOrder records the order of a deactivated Träger.
	CreateTraegerErasureOrder(ctx context.Context, order NewTraegerErasureOrder) (TraegerErasureOrder, error)
	// WithdrawTraegerErasureOrder withdraws the open order during its grace
	// period.
	WithdrawTraegerErasureOrder(ctx context.Context, organizationID, operatorID int64) (TraegerErasureOrder, error)
}

// TraegerErasureOrderRecords records the hint and the customer's choice on
// an open order that has none, started or not (#4720), on the caller's
// administrative transaction. It is its own capability beside the owner's
// facade; the owner's service serves it.
type TraegerErasureOrderRecords interface {
	RecordTraegerErasureOrderRecords(ctx context.Context, organizationID int64, records RecordsHandover) (TraegerErasureOrder, error)
}

// RecordTraegerErasureOrder is the operator's request to record an order.
type RecordTraegerErasureOrder struct {
	OrganizationID int64
	OrderReference string
	ConfirmedOn    calendar.Date
	EraseNotBefore calendar.Date
	Records        RecordsHandover
	OperatorID     int64
	ClientIP       net.IP
}

// RecordTraegerErasureOrderRecords is the operator's request to record the
// hint and the customer's choice on an open order recorded without them.
type RecordTraegerErasureOrderRecords struct {
	OrganizationID int64
	Records        RecordsHandover
	OperatorID     int64
	ClientIP       net.IP
}

// TraegerLifecycleProvisioning lets the operator record and withdraw the
// erasure order of a Träger. Deactivation and reactivation go through
// OrganizationProvisioning.UpdateOrganization.
type TraegerLifecycleProvisioning interface {
	TraegerErasureOrderQuery
	RecordTraegerErasureOrder(ctx context.Context, input RecordTraegerErasureOrder) (TraegerErasureOrder, error)
	WithdrawTraegerErasureOrder(ctx context.Context, organizationID, operatorID int64, clientIP net.IP) error
}

// OperatorProvisioning is the operator provisioning and, beside its facade
// (ADR 0178), the records command of an older erasure order (#4720), both
// served by one composition.
type OperatorProvisioning struct {
	Provisioning
	ErasureOrderRecords TraegerErasureOrderRecordsProvisioning
}

// TraegerErasureOrderRecordsProvisioning lets the operator record the hint
// and the customer's choice on an older open order, with the operator audit
// (#4720). The operator provisioning serves it beside its facade.
type TraegerErasureOrderRecordsProvisioning interface {
	RecordTraegerErasureOrderRecords(ctx context.Context, input RecordTraegerErasureOrderRecords) (TraegerErasureOrder, error)
}
