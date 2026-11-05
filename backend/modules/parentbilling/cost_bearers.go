package parentbilling

import (
	"errors"
	"time"
)

// The cost takeover of the holiday contribution (#5627, ADR 0115 addendum).
// A cost bearer is a body that pays the contribution of a family in its
// place, for example the youth office of a district for a family receiving
// benefits (§ 90 Abs. 4 SGB VIII). The Abrechnung keeps the list of its
// Träger; the Träger-Büro chooses an active one when it confirms a
// registration. The line cost_takeover then sets what the family would pay
// after the sibling discount to 0 € and keeps the ID and a copy of the name
// of the body, so a rename changes no frozen contribution and no issued
// invoice. A body a frozen contribution or an invoice names can only be
// switched off, not deleted.

// LineCostTakeover is the kind of the line of a cost takeover.
const LineCostTakeover = "cost_takeover"

// Limits of a cost bearer.
const (
	MaxCostBearerNameLength          = 200
	MaxCostBearerAddressLength       = 500
	MaxCostBearerFileReferenceLength = 100
)

var (
	// ErrCostBearerNotFound reports a cost bearer that does not exist for
	// the Träger of the session.
	ErrCostBearerNotFound = errors.New("cost bearer not found")
	// ErrCostBearerInUse reports the deletion of a cost bearer a frozen
	// contribution or an issued invoice names.
	ErrCostBearerInUse = errors.New("a frozen contribution or an invoice names the cost bearer; switch it off instead")
	// ErrCostBearerInactive reports a confirmation with a cost bearer the
	// Abrechnung switched off.
	ErrCostBearerInactive = errors.New("the cost bearer is switched off")
	// ErrCostTakeoverWithExemption reports a confirmation with both the
	// exemption and a cost takeover.
	ErrCostTakeoverWithExemption = errors.New("a registration is either exempt or a cost bearer pays, not both")
)

// CostBearer is a body that pays the contribution of a family in its place.
// Address and FileReference, the reference of the Träger at the body, are
// optional. InUse reports a frozen contribution or an invoice that names
// it: then it can only be switched off. The Abrechnung keeps its cost
// bearers with its contribution rules (ContributionRules).
type CostBearer struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Address       string    `json:"address"`
	FileReference string    `json:"file_reference"`
	Active        bool      `json:"active"`
	InUse         bool      `json:"in_use"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CostBearerInput is a cost bearer as the Abrechnung writes it.
type CostBearerInput struct {
	Name          string
	Address       string
	FileReference string
	Active        bool
}
