package application

import (
	"errors"
	"slices"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// normalizeCareRule checks a care rule as a whole: a name, whole months,
// schools of the Träger, the parameters its kind needs and no others.
func normalizeCareRule(input parentbilling.CareRuleInput, schools []parentbilling.CareSchool) (parentbilling.CareRuleInput, error) {
	input.Kind = strings.TrimSpace(input.Kind)
	input.Name = strings.TrimSpace(input.Name)
	errs := validation.Errors{}
	switch {
	case input.Name == "":
		errs["name"] = errors.New("is required")
	case len([]rune(input.Name)) > parentbilling.MaxCareRuleNameLength:
		errs["name"] = errors.New("is too long")
	}
	if !isMonthStart(input.ValidFrom) {
		errs["valid_from"] = errors.New("must be the first day of a month")
	}
	if input.ValidUntil != nil && (input.ValidUntil.AddDays(1).Day() != 1 || (input.ValidFrom != "" && input.ValidUntil.Before(input.ValidFrom))) {
		errs["valid_until"] = errors.New("must be the last day of a month from valid_from on")
	}
	own := make([]int64, len(schools))
	for i, school := range schools {
		own[i] = school.ID
	}
	input.SchoolIDs = uniqueIDs(input.SchoolIDs)
	input.CountSchoolIDs = uniqueIDs(input.CountSchoolIDs)
	input.CareOfferingIDs = uniqueIDs(input.CareOfferingIDs)
	switch {
	case len(input.SchoolIDs) == 0:
		errs["school_ids"] = errors.New("needs at least one school")
	case len(input.SchoolIDs) > parentbilling.MaxCareRuleSchools || !allIn(input.SchoolIDs, own):
		errs["school_ids"] = errors.New("must name schools of the Träger")
	}
	if len(input.CountSchoolIDs) > parentbilling.MaxCareRuleSchools || !allIn(input.CountSchoolIDs, own) {
		errs["count_school_ids"] = errors.New("must name schools of the Träger")
	}
	if len(input.CareOfferingIDs) > parentbilling.MaxCareRuleModules || slices.ContainsFunc(input.CareOfferingIDs, func(id int64) bool { return id <= 0 }) {
		errs["care_offering_ids"] = errors.New("must name modules")
	}
	validateCareRuleKind(&input, errs)
	if len(errs) > 0 {
		return input, errs
	}
	return input, nil
}

// validateCareRuleKind checks the parameters of the rule's kind.
func validateCareRuleKind(input *parentbilling.CareRuleInput, errs validation.Errors) {
	if input.Kind != parentbilling.RuleSibling {
		input.SiblingFromChild = nil
		input.TariffSiblingPrice = false
		if len(input.CountSchoolIDs) > 0 {
			errs["count_school_ids"] = errors.New("only a sibling rule counts other schools")
		}
	}
	switch input.Kind {
	case parentbilling.RuleIncomeLevel:
		input.CapCents = nil
		validateCareDiscount(input, errs, true)
	case parentbilling.RuleSibling:
		input.CapCents = nil
		if input.SiblingFromChild == nil || *input.SiblingFromChild < 2 || *input.SiblingFromChild > parentbilling.MaxSiblingFromChild {
			errs["sibling_from_child"] = errors.New("must be between 2 and 9")
		}
		validateCareDiscount(input, errs, true)
	case parentbilling.RuleSingleParent:
		input.CapCents = nil
		validateCareDiscount(input, errs, false)
	case parentbilling.RuleCap:
		input.DiscountBasisPoints, input.DiscountCents, input.PriceCents = nil, nil, nil
		if input.CapCents == nil || *input.CapCents < 0 || *input.CapCents > parentbilling.MaxDiscountCents {
			errs["cap_cents"] = errors.New("must be an amount in cent")
		}
	default:
		errs["kind"] = errors.New("unknown rule kind")
	}
}

// validateCareDiscount wants exactly one of a share, an amount and, where
// allowed, a fixed end price.
func validateCareDiscount(input *parentbilling.CareRuleInput, errs validation.Errors, priceAllowed bool) {
	set := 0
	if input.TariffSiblingPrice {
		// The sibling prices of the price list (#6196); only a sibling rule
		// keeps the flag, validateCareRuleKind clears it on the others.
		set++
	}
	if input.DiscountBasisPoints != nil {
		set++
		if *input.DiscountBasisPoints < 1 || *input.DiscountBasisPoints > parentbilling.BasisPointsWhole {
			errs["discount_basis_points"] = errors.New("must be between 1 and 10000")
		}
	}
	if input.DiscountCents != nil {
		set++
		if *input.DiscountCents < 1 || *input.DiscountCents > parentbilling.MaxDiscountCents {
			errs["discount_cents"] = errors.New("must be a positive amount in cent")
		}
	}
	if input.PriceCents != nil {
		set++
		switch {
		case !priceAllowed:
			errs["price_cents"] = errors.New("this rule has no fixed price")
		case *input.PriceCents < 0 || *input.PriceCents > parentbilling.MaxDiscountCents:
			errs["price_cents"] = errors.New("must be an amount in cent")
		}
	}
	if set != 1 {
		errs["discount_basis_points"] = errors.New("a rule has exactly one of a share, an amount, a price or the sibling prices of the price list")
	}
}

// normalizeCareEvidence checks an evidence: a contract, a kind with an
// income level of the Träger where it needs one, a validity, the kind of
// proof and the day it was checked, not in the future.
func normalizeCareEvidence(input parentbilling.CareEvidenceInput, rules []parentbilling.ContributionRule, today calendar.Date) (parentbilling.CareEvidenceInput, error) {
	input.Kind = strings.TrimSpace(input.Kind)
	input.Evidence = strings.TrimSpace(input.Evidence)
	errs := validation.Errors{}
	if input.ContractID <= 0 {
		errs["contract_id"] = errors.New("is required")
	}
	switch input.Kind {
	case parentbilling.EvidenceIncomeLevel:
		if input.RuleID == nil || !slices.ContainsFunc(rules, func(rule parentbilling.ContributionRule) bool {
			return rule.ID == *input.RuleID && rule.Kind == parentbilling.RuleIncomeLevel
		}) {
			errs["rule_id"] = errors.New("must name an income level of the Träger")
		}
	case parentbilling.EvidenceSingleParent:
		input.RuleID = nil
	default:
		errs["kind"] = errors.New("unknown evidence kind")
	}
	if input.ValidFrom == "" {
		errs["valid_from"] = errors.New("is required")
	}
	if input.ValidUntil != nil && input.ValidFrom != "" && input.ValidUntil.Before(input.ValidFrom) {
		errs["valid_until"] = errors.New("must not be before valid_from")
	}
	switch {
	case input.Evidence == "":
		errs["evidence"] = errors.New("is required")
	case len([]rune(input.Evidence)) > parentbilling.MaxCareEvidenceLength:
		errs["evidence"] = errors.New("is too long")
	}
	if input.CheckedOn == "" || input.CheckedOn.After(today) {
		errs["checked_on"] = errors.New("is required and not in the future")
	}
	if len(errs) > 0 {
		return input, errs
	}
	return input, nil
}

func uniqueIDs(ids []int64) []int64 {
	unique := []int64{}
	for _, id := range ids {
		if !slices.Contains(unique, id) {
			unique = append(unique, id)
		}
	}
	slices.Sort(unique)
	return unique
}

func allIn(ids, of []int64) bool {
	for _, id := range ids {
		if !slices.Contains(of, id) {
			return false
		}
	}
	return true
}
