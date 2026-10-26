// Package parentbilling is the owner Elternabrechnung (parent-billing, ADR
// 0088): it computes the contributions of the parents of a Träger from a
// fixed chain of rules and keeps what it computed. Its tables belong to the
// Träger and are read and written in the Träger transaction (ADR 0086
// section 5, ADR 0105), which the caller opens. Money is whole cent in
// int64, shares are basis points, calendar days are calendar.Date.
//
// v1 (#4664, ADR 0115) computes the holiday contribution: the base price of
// every booked week from its time model, the sibling rule and the exemption
// for families receiving benefits. Every rule is a row of the Träger; no
// price and no discount stands in the code.
package parentbilling

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Services a contribution rule applies to. v1 knows the holiday care.
const ServiceHoliday = "holiday"

// Kinds of a contribution rule, in the fixed order of the chain after the
// base price (ADR 0088 section 4). A new kind is code.
const (
	// RuleSibling lowers the contribution of the second or third child of
	// a family: by a share in basis points or by a fixed amount in cent.
	RuleSibling = "sibling"
	// RuleBenefitsExemption frees a family receiving benefits from the
	// contribution, with the note the contribution shows.
	RuleBenefitsExemption = "benefits_exemption"
)

// Limits of a rule.
const (
	// BasisPointsWhole is 100 %.
	BasisPointsWhole = 10000
	// MaxDiscountCents caps a fixed discount at one million euro, far above
	// any contribution, so a typo cannot overflow a sum.
	MaxDiscountCents = 100_000_000
	// MaxRuleNoteLength is the longest note of an exemption.
	MaxRuleNoteLength = 300
)

var (
	// ErrContributionRuleNotFound reports a rule that does not exist for the
	// Träger of the session.
	ErrContributionRuleNotFound = errors.New("contribution rule not found")
	// ErrContributionRuleOverlap reports a rule whose validity overlaps
	// another rule of the same kind and service.
	ErrContributionRuleOverlap = errors.New("another contribution rule of this kind is valid in the same time")
)

// ContributionRule is a rule of a Träger with its parameters and validity.
// A sibling rule carries SiblingFromChild and either DiscountBasisPoints or
// DiscountCents; an exemption carries its Note. A nil ValidUntil is open.
type ContributionRule struct {
	ID                  int64          `json:"id"`
	Service             string         `json:"service"`
	Kind                string         `json:"kind"`
	ValidFrom           calendar.Date  `json:"valid_from"`
	ValidUntil          *calendar.Date `json:"valid_until"`
	SiblingFromChild    *int           `json:"sibling_from_child"`
	DiscountBasisPoints *int           `json:"discount_basis_points"`
	DiscountCents       *int64         `json:"discount_cents"`
	Note                string         `json:"note"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

// ContributionRuleInput is a rule as the Abrechnung writes it.
type ContributionRuleInput struct {
	Service             string
	Kind                string
	ValidFrom           calendar.Date
	ValidUntil          *calendar.Date
	SiblingFromChild    *int
	DiscountBasisPoints *int
	DiscountCents       *int64
	Note                string
}

// ContributionRules lets the Abrechnung of a Träger keep its rules. Every
// operation names the Träger of the session and runs in its Träger
// transaction, which the caller opens.
type ContributionRules interface {
	ListContributionRules(ctx context.Context, organizationID int64) ([]ContributionRule, error)
	CreateContributionRule(ctx context.Context, organizationID int64, input ContributionRuleInput) (ContributionRule, error)
	UpdateContributionRule(ctx context.Context, organizationID, ruleID int64, input ContributionRuleInput) (ContributionRule, error)
	DeleteContributionRule(ctx context.Context, organizationID, ruleID int64) error
}
