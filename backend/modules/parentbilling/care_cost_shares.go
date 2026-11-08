package parentbilling

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The cost bearer shares of the monthly care contribution (#4687, spec
// #4611 user stories 66 to 69, ADR 0169). A share says that a cost bearer
// of the Träger (CostBearer, #5627) pays a part of the monthly contribution
// of a contract: Bildung und Teilhabe, the Jugendamt or another subsidy.
// The rule chain of ADR 0164 runs first; the share splits what the family
// would pay after the cap:
//
//	income level → sibling → single parent → cap → cost bearer shares
//
// Invariants:
//   - A share names a contract and a cost bearer of the Träger, its kind,
//     either a share in basis points or a monthly amount in cent, its
//     validity with an end, the day the Abrechnung checked the proof and a
//     reference to the proof. It holds no document and nothing of the
//     decision itself (data minimisation).
//   - Two shares of one kind never hold for one contract on the same day.
//   - The shares of an invoice never take more than its contribution; the
//     family pays the rest, never less than nothing.
//   - Every read and change of a share is recorded per contract.
//   - An issued invoice keeps its own copy of every share; a later change of
//     the share or the cost bearer never reaches it.

// Kinds of a cost bearer share, in the order the chain applies them.
const (
	CostShareEducationParticipation = "education_participation"
	CostShareYouthOffice            = "youth_office"
	CostShareOtherSubsidy           = "other_subsidy"
)

// CostShareKinds are the kinds of a share in the order of the chain.
var CostShareKinds = []string{CostShareEducationParticipation, CostShareYouthOffice, CostShareOtherSubsidy}

// Status of a share as the share page shows it on the day it is read.
const (
	CostShareUpcoming = "upcoming"
	CostShareActive   = "active"
	CostShareExpiring = "expiring"
	CostShareExpired  = "expired"
)

// CostShareWarningDays is how many days before its end a share without a
// follow-up warns.
const CostShareWarningDays = 30

// MaxCostShareReferenceLength limits the reference to the proof.
const MaxCostShareReferenceLength = 120

var (
	// ErrCareCostShareNotFound reports a share that does not exist for the
	// Träger of the session.
	ErrCareCostShareNotFound = errors.New("care cost share not found")
	// ErrCareCostShareOverlap reports a second share of one kind for a
	// contract in the same time.
	ErrCareCostShareOverlap = errors.New("the contract has another cost bearer share of this kind in the same time")
)

// CareCostShare is the part a cost bearer pays of the monthly contribution
// of a contract from ValidFrom to ValidUntil: ShareBasisPoints of it or
// ShareCents a month, never more than the contribution. Status says whether
// it is ahead, in force, ends within CostShareWarningDays without a
// follow-up of its kind, or over.
type CareCostShare struct {
	ID               int64         `json:"id"`
	ContractID       int64         `json:"contract_id"`
	Kind             string        `json:"kind"`
	CostBearerID     int64         `json:"cost_bearer_id"`
	CostBearerName   string        `json:"cost_bearer_name"`
	ShareBasisPoints *int          `json:"share_basis_points"`
	ShareCents       *int64        `json:"share_cents"`
	ValidFrom        calendar.Date `json:"valid_from"`
	ValidUntil       calendar.Date `json:"valid_until"`
	Reference        string        `json:"reference"`
	CheckedOn        calendar.Date `json:"checked_on"`
	Status           string        `json:"status"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

// CareCostShareInput is a share as the Abrechnung writes it.
type CareCostShareInput struct {
	ContractID       int64         `json:"contract_id"`
	Kind             string        `json:"kind"`
	CostBearerID     int64         `json:"cost_bearer_id"`
	ShareBasisPoints *int          `json:"share_basis_points"`
	ShareCents       *int64        `json:"share_cents"`
	ValidFrom        calendar.Date `json:"valid_from"`
	ValidUntil       calendar.Date `json:"valid_until"`
	Reference        string        `json:"reference"`
	CheckedOn        calendar.Date `json:"checked_on"`
}

// CareCostShareContract is a contract of the Träger as the share page shows
// it: the child, its school and the shares of the contract.
type CareCostShareContract struct {
	ContractID     int64           `json:"contract_id"`
	SchoolID       int64           `json:"school_id"`
	SchoolName     string          `json:"school_name"`
	ChildFirstName string          `json:"child_first_name"`
	ChildLastName  string          `json:"child_last_name"`
	Shares         []CareCostShare `json:"shares"`
}

// CareCostShareSet is what the share page shows: the contracts in force
// from three months back to nine months ahead with their shares, the cost
// bearers of the Träger and how many shares warn.
type CareCostShareSet struct {
	Contracts   []CareCostShareContract `json:"contracts"`
	CostBearers []CostBearer            `json:"cost_bearers"`
	Expiring    int                     `json:"expiring"`
	WarningDays int                     `json:"warning_days"`
}

// CareInvoiceCostShare is the part one cost bearer pays of an invoice, with
// a copy of the share and the cost bearer's name at the time. AmountCents is
// what the cost bearer pays, more than zero.
type CareInvoiceCostShare struct {
	Kind             string `json:"kind"`
	CostBearerID     int64  `json:"cost_bearer_id"`
	CostBearerName   string `json:"cost_bearer_name"`
	ShareBasisPoints *int   `json:"share_basis_points"`
	ShareCents       *int64 `json:"share_cents"`
	AmountCents      int64  `json:"amount_cents"`
}

// ContributionCents is the contribution of the invoice before the cost
// bearer shares: what the family pays plus what the cost bearers pay.
func (i CareInvoice) ContributionCents() int64 {
	total := i.TotalCents
	for _, share := range i.CostShares {
		total += share.AmountCents
	}
	return total
}

// CareCostBearerTotal is what one cost bearer pays of the invoices of a run:
// how many invoices and how much. A calculated run sums its preview without
// the excluded invoices, a released run its issued invoices that no Storno
// took back.
type CareCostBearerTotal struct {
	CostBearerID   int64  `json:"cost_bearer_id"`
	CostBearerName string `json:"cost_bearer_name"`
	InvoiceCount   int    `json:"invoice_count"`
	AmountCents    int64  `json:"amount_cents"`
}

// CareCostShares lets the Abrechnung of a Träger keep the cost bearer
// shares of its contracts. Every operation names the person of the session
// and runs in the Träger transaction of its Träger, which the caller opens.
// Every read and change of a share is recorded per contract.
type CareCostShares interface {
	CareCostShares(ctx context.Context, reviewer HolidayReviewer) (CareCostShareSet, error)
	// SaveCareCostShare creates a share when shareID is zero and changes
	// that one otherwise.
	SaveCareCostShare(ctx context.Context, reviewer HolidayReviewer, shareID int64, input CareCostShareInput) (CareCostShareSet, error)
	DeleteCareCostShare(ctx context.Context, reviewer HolidayReviewer, shareID int64) (CareCostShareSet, error)
}
