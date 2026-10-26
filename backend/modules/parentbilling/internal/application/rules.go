package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// errTraegerRequired reports a call without the Träger of the session.
var errTraegerRequired = errors.New("parent billing: the Träger is required")

// Store persists the rules and the frozen holiday contributions of a Träger
// in its Träger transaction.
type Store interface {
	ListContributionRules(ctx context.Context, organizationID int64, service string) ([]parentbilling.ContributionRule, error)
	// InsertContributionRule and UpdateContributionRule report an overlap
	// with another rule of the same kind as ErrContributionRuleOverlap;
	// UpdateContributionRule and DeleteContributionRule report a missing
	// rule as ErrContributionRuleNotFound.
	InsertContributionRule(ctx context.Context, organizationID int64, input parentbilling.ContributionRuleInput) (parentbilling.ContributionRule, error)
	UpdateContributionRule(ctx context.Context, organizationID, ruleID int64, input parentbilling.ContributionRuleInput) (parentbilling.ContributionRule, error)
	DeleteContributionRule(ctx context.Context, organizationID, ruleID int64) error
	// HolidayContributions reads the frozen contributions of a registration;
	// found is false when there are none.
	HolidayContributions(ctx context.Context, organizationID, registrationID int64) (contribution parentbilling.HolidayContribution, found bool, err error)
	InsertHolidayContributions(ctx context.Context, organizationID, periodID, registrationID, accountID int64, contribution parentbilling.HolidayContribution) error
}

// Rules serves parentbilling.ContributionRules.
type Rules struct {
	store Store
}

var _ parentbilling.ContributionRules = (*Rules)(nil)

// NewRules builds the rules.
func NewRules(store Store) *Rules {
	if store == nil {
		panic("parent billing: the store is required")
	}
	return &Rules{store: store}
}

// ListContributionRules lists the rules of the Träger by kind and validity.
func (s *Rules) ListContributionRules(ctx context.Context, organizationID int64) ([]parentbilling.ContributionRule, error) {
	if organizationID <= 0 {
		return nil, errTraegerRequired
	}
	rules, err := s.store.ListContributionRules(ctx, organizationID, parentbilling.ServiceHoliday)
	if err != nil {
		return nil, fmt.Errorf("parent billing: list contribution rules: %w", err)
	}
	return rules, nil
}

// CreateContributionRule validates and stores a new rule.
func (s *Rules) CreateContributionRule(ctx context.Context, organizationID int64, input parentbilling.ContributionRuleInput) (parentbilling.ContributionRule, error) {
	if organizationID <= 0 {
		return parentbilling.ContributionRule{}, errTraegerRequired
	}
	input, err := normalizeRule(input)
	if err != nil {
		return parentbilling.ContributionRule{}, err
	}
	return s.store.InsertContributionRule(ctx, organizationID, input)
}

// UpdateContributionRule validates and changes a rule. A frozen contribution
// keeps what it froze.
func (s *Rules) UpdateContributionRule(ctx context.Context, organizationID, ruleID int64, input parentbilling.ContributionRuleInput) (parentbilling.ContributionRule, error) {
	if organizationID <= 0 {
		return parentbilling.ContributionRule{}, errTraegerRequired
	}
	if ruleID <= 0 {
		return parentbilling.ContributionRule{}, parentbilling.ErrContributionRuleNotFound
	}
	input, err := normalizeRule(input)
	if err != nil {
		return parentbilling.ContributionRule{}, err
	}
	return s.store.UpdateContributionRule(ctx, organizationID, ruleID, input)
}

// DeleteContributionRule removes a rule. A frozen contribution keeps what it
// froze.
func (s *Rules) DeleteContributionRule(ctx context.Context, organizationID, ruleID int64) error {
	if organizationID <= 0 {
		return errTraegerRequired
	}
	if ruleID <= 0 {
		return parentbilling.ErrContributionRuleNotFound
	}
	return s.store.DeleteContributionRule(ctx, organizationID, ruleID)
}

// normalizeRule checks a rule as a whole: the parameters its kind needs and
// no others, and a validity that does not end before it starts.
func normalizeRule(input parentbilling.ContributionRuleInput) (parentbilling.ContributionRuleInput, error) {
	input.Service = strings.TrimSpace(input.Service)
	if input.Service == "" {
		input.Service = parentbilling.ServiceHoliday
	}
	input.Kind = strings.TrimSpace(input.Kind)
	input.Note = strings.TrimSpace(input.Note)
	errs := validation.Errors{}
	if input.Service != parentbilling.ServiceHoliday {
		errs["service"] = errors.New("unknown service")
	}
	if input.ValidFrom == "" {
		errs["valid_from"] = errors.New("is required")
	}
	if input.ValidUntil != nil && input.ValidFrom != "" && input.ValidUntil.Before(input.ValidFrom) {
		errs["valid_until"] = errors.New("must not be before valid_from")
	}
	switch input.Kind {
	case parentbilling.RuleSibling:
		validateSiblingRule(&input, errs)
	case parentbilling.RuleBenefitsExemption:
		validateExemptionRule(&input, errs)
	default:
		errs["kind"] = errors.New("unknown rule kind")
	}
	if len(errs) > 0 {
		return input, errs
	}
	return input, nil
}

func validateSiblingRule(input *parentbilling.ContributionRuleInput, errs validation.Errors) {
	input.Note = ""
	if input.SiblingFromChild == nil || *input.SiblingFromChild < 2 || *input.SiblingFromChild > 3 {
		errs["sibling_from_child"] = errors.New("must be 2 or 3")
	}
	switch {
	case input.DiscountBasisPoints != nil && input.DiscountCents != nil:
		errs["discount_cents"] = errors.New("a rule has either a share or an amount")
	case input.DiscountBasisPoints != nil:
		if *input.DiscountBasisPoints < 1 || *input.DiscountBasisPoints > parentbilling.BasisPointsWhole {
			errs["discount_basis_points"] = errors.New("must be between 1 and 10000")
		}
	case input.DiscountCents != nil:
		if *input.DiscountCents < 1 || *input.DiscountCents > parentbilling.MaxDiscountCents {
			errs["discount_cents"] = errors.New("must be a positive amount in cent")
		}
	default:
		errs["discount_basis_points"] = errors.New("a share or an amount is required")
	}
}

func validateExemptionRule(input *parentbilling.ContributionRuleInput, errs validation.Errors) {
	input.SiblingFromChild, input.DiscountBasisPoints, input.DiscountCents = nil, nil, nil
	switch {
	case input.Note == "":
		errs["note"] = errors.New("is required")
	case len([]rune(input.Note)) > parentbilling.MaxRuleNoteLength:
		errs["note"] = errors.New("is too long")
	}
}
