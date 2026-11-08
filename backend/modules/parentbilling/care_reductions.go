package parentbilling

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The rule chain of the monthly care contribution (#4685, spec #4611 user
// stories 60 to 62, ADR 0164). It builds on the contribution rules of the
// holiday care: a care rule is a ContributionRule of the service "care" and
// the sibling rule ranks children like ADR 0115 (the child with the highest
// base price is the first). The chain runs in the fixed order of ADR 0088
// section 4 after the base price:
//
//	income level → sibling → single parent → cap
//
// Every rule rounds half up to whole cent and writes its own reduction on
// the invoice. A rule applies at the schools it names and, if it names
// modules, only to their positions. A sibling rule counts the children of
// a family at its counting schools, by default its own schools, never
// silently all schools of the Träger.
//
// Invariants:
//   - Two care rules of one kind never apply to one school in one month;
//     income levels are the exception, a contract has at most one level.
//   - A reduction never makes a position, an invoice or a family pay less
//     than nothing.
//   - An evidence names a contract of the Träger and the kind of proof the
//     Abrechnung checked, never a document or an income. Every read and
//     change of an evidence is recorded per contract.
//   - An issued invoice keeps its own copy of every reduction.

// Kinds of a care rule, in the order of the chain; RuleSibling is shared
// with the holiday care.
const (
	// RuleIncomeLevel is a level of the Träger's statute: a contract with an
	// evidence of the level pays a share, less by an amount or a fixed price.
	RuleIncomeLevel = "income_level"
	// RuleSingleParent reduces the contribution of a contract with an
	// evidence of a single parent.
	RuleSingleParent = "single_parent"
	// RuleCap is the most a family pays per month at the rule's schools.
	RuleCap = "cap"
)

// Kinds of an evidence.
const (
	EvidenceIncomeLevel  = "income_level"
	EvidenceSingleParent = "single_parent"
)

// Limits of a care rule and an evidence.
const (
	MaxCareRuleNameLength = 120
	MaxCareEvidenceLength = 120
	MaxSiblingFromChild   = 9
	MaxCareRuleSchools    = 200
	MaxCareRuleModules    = 500
)

var (
	// ErrCareRuleInUse reports an income level an evidence still names.
	ErrCareRuleInUse = errors.New("an evidence still names this income level")
	// ErrCareEvidenceNotFound reports an evidence that does not exist for
	// the Träger of the session.
	ErrCareEvidenceNotFound = errors.New("care evidence not found")
	// ErrCareEvidenceOverlap reports a second evidence of one kind for a
	// contract in the same time.
	ErrCareEvidenceOverlap = errors.New("the contract has another evidence of this kind in the same time")
)

// CareRuleInput is a care rule as the Abrechnung writes it. ValidFrom is the
// first day of a month, ValidUntil, if set, the last day of one. A rule has
// either a share (DiscountBasisPoints), an amount (DiscountCents) or, for an
// income level or a sibling rule, a fixed end price (PriceCents); a cap has
// CapCents.
type CareRuleInput struct {
	Kind                string         `json:"kind"`
	Name                string         `json:"name"`
	ValidFrom           calendar.Date  `json:"valid_from"`
	ValidUntil          *calendar.Date `json:"valid_until"`
	SchoolIDs           []int64        `json:"school_ids"`
	CareOfferingIDs     []int64        `json:"care_offering_ids"`
	CountSchoolIDs      []int64        `json:"count_school_ids"`
	SiblingFromChild    *int           `json:"sibling_from_child"`
	DiscountBasisPoints *int           `json:"discount_basis_points"`
	DiscountCents       *int64         `json:"discount_cents"`
	PriceCents          *int64         `json:"price_cents"`
	CapCents            *int64         `json:"cap_cents"`
	// CountsFamilyChildren lets a sibling rule count the family's number
	// (#5918); other kinds never do.
	CountsFamilyChildren bool `json:"counts_family_children"`
}

// CareRuleSet is what the rule page shows: the care rules by kind and
// start, the schools of the Träger and the modules its contracts book.
type CareRuleSet struct {
	Rules   []ContributionRule `json:"rules"`
	Schools []CareSchool       `json:"schools"`
	Modules []CareModule       `json:"modules"`
}

// CareEvidence is the income level or the single-parent status of a
// contract from ValidFrom on, with the kind of proof the Abrechnung checked
// and the day it checked it. A nil ValidUntil is open.
type CareEvidence struct {
	ID         int64          `json:"id"`
	ContractID int64          `json:"contract_id"`
	Kind       string         `json:"kind"`
	RuleID     *int64         `json:"rule_id"`
	ValidFrom  calendar.Date  `json:"valid_from"`
	ValidUntil *calendar.Date `json:"valid_until"`
	Evidence   string         `json:"evidence"`
	CheckedOn  calendar.Date  `json:"checked_on"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// CareEvidenceInput is an evidence as the Abrechnung writes it; RuleID
// names the income level of an income evidence.
type CareEvidenceInput struct {
	ContractID int64          `json:"contract_id"`
	Kind       string         `json:"kind"`
	RuleID     *int64         `json:"rule_id"`
	ValidFrom  calendar.Date  `json:"valid_from"`
	ValidUntil *calendar.Date `json:"valid_until"`
	Evidence   string         `json:"evidence"`
	CheckedOn  calendar.Date  `json:"checked_on"`
}

// CareEvidenceContract is a contract of the Träger as the evidence page
// shows it: the child, its school and the evidences of the contract.
type CareEvidenceContract struct {
	ContractID     int64          `json:"contract_id"`
	SchoolID       int64          `json:"school_id"`
	SchoolName     string         `json:"school_name"`
	ChildFirstName string         `json:"child_first_name"`
	ChildLastName  string         `json:"child_last_name"`
	Evidences      []CareEvidence `json:"evidences"`
}

// CareEvidenceSet is what the evidence page shows: the contracts in force
// from three months back to a year ahead with their evidences, and the
// income levels an evidence may name.
type CareEvidenceSet struct {
	Contracts []CareEvidenceContract `json:"contracts"`
	Levels    []ContributionRule     `json:"levels"`
}

// CareReduction is one reduction of an invoice by a rule of the chain, with
// a copy of the rule. A sibling reduction names the child's rank among the
// FamilyChildren its rule counted; the rank follows the base price, the
// highest first, then the age, the oldest first.
type CareReduction struct {
	Kind                string `json:"kind"`
	RuleID              int64  `json:"rule_id"`
	Name                string `json:"name"`
	ChildRank           *int   `json:"child_rank"`
	FamilyChildren      *int   `json:"family_children"`
	DiscountBasisPoints *int   `json:"discount_basis_points"`
	DiscountCents       *int64 `json:"discount_cents"`
	PriceCents          *int64 `json:"price_cents"`
	CapCents            *int64 `json:"cap_cents"`
	// CareOfferingIDs are the modules the rule reduced, none for all; a
	// later run of the month places an earlier reduction by them.
	CareOfferingIDs []int64 `json:"care_offering_ids"`
	AmountCents     int64   `json:"amount_cents"`
}

// CareReductions lets the Abrechnung of a Träger keep the rules of the
// monthly contribution and the evidences of its contracts. Every operation
// names the person of the session and runs in the Träger transaction of its
// Träger, which the caller opens. Every read and change of an evidence is
// recorded per contract.
type CareReductions interface {
	CareRules(ctx context.Context, reviewer HolidayReviewer) (CareRuleSet, error)
	// SaveCareRule creates a rule when ruleID is zero and changes that one
	// otherwise.
	SaveCareRule(ctx context.Context, reviewer HolidayReviewer, ruleID int64, input CareRuleInput) (CareRuleSet, error)
	DeleteCareRule(ctx context.Context, reviewer HolidayReviewer, ruleID int64) (CareRuleSet, error)
	CareEvidences(ctx context.Context, reviewer HolidayReviewer) (CareEvidenceSet, error)
	// SaveCareEvidence creates an evidence when evidenceID is zero and
	// changes that one otherwise.
	SaveCareEvidence(ctx context.Context, reviewer HolidayReviewer, evidenceID int64, input CareEvidenceInput) (CareEvidenceSet, error)
	DeleteCareEvidence(ctx context.Context, reviewer HolidayReviewer, evidenceID int64) (CareEvidenceSet, error)
}
