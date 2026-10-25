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

// RecordTraegerErasureOrder is the operator's request to record an order.
type RecordTraegerErasureOrder struct {
	OrganizationID int64
	OrderReference string
	ConfirmedOn    calendar.Date
	EraseNotBefore calendar.Date
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
