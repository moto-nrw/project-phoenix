package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Actions of the evidence trail.
const (
	evidenceEventRead   = "read"
	evidenceEventCreate = "create"
	evidenceEventUpdate = "update"
	evidenceEventDelete = "delete"
)

// The evidence page shows the contracts in force from evidenceLookBack days
// back to evidenceLookAhead days ahead: one year, the span the contract
// owner answers at once.
const (
	evidenceLookBack  = 92
	evidenceLookAhead = 273
)

// CareReductionStore persists the care rules, the evidences and their trail
// of a Träger in its Träger transaction.
type CareReductionStore interface {
	ListContributionRules(ctx context.Context, organizationID int64, service string) ([]parentbilling.ContributionRule, error)
	// InsertCareRule and UpdateCareRule store a validated care rule;
	// UpdateCareRule and DeleteCareRule report a missing one as
	// ErrContributionRuleNotFound, DeleteCareRule an income level an
	// evidence names as ErrCareRuleInUse.
	InsertCareRule(ctx context.Context, organizationID int64, input parentbilling.CareRuleInput) error
	UpdateCareRule(ctx context.Context, organizationID, ruleID int64, input parentbilling.CareRuleInput) error
	DeleteCareRule(ctx context.Context, organizationID, ruleID int64) error
	// LockCareBilling holds the monthly billing of the Träger until the
	// transaction ends.
	LockCareBilling(ctx context.Context, organizationID int64) error
	// CareEvidences reads the evidences of the contracts, all when
	// contractIDs is nil.
	CareEvidences(ctx context.Context, organizationID int64, contractIDs []int64) ([]parentbilling.CareEvidence, error)
	// InsertCareEvidence and UpdateCareEvidence report a contract that is
	// no contract of the Träger as a validation error and an overlap as
	// ErrCareEvidenceOverlap; UpdateCareEvidence and DeleteCareEvidence a
	// missing evidence as ErrCareEvidenceNotFound. They answer the contract.
	InsertCareEvidence(ctx context.Context, organizationID, accountID int64, input parentbilling.CareEvidenceInput) (int64, error)
	UpdateCareEvidence(ctx context.Context, organizationID, accountID, evidenceID int64, input parentbilling.CareEvidenceInput) (int64, error)
	DeleteCareEvidence(ctx context.Context, organizationID, evidenceID int64) (int64, error)
	// InsertCareEvidenceEvents records one action of the account per
	// contract.
	InsertCareEvidenceEvents(ctx context.Context, organizationID, accountID int64, action string, evidenceID *int64, contractIDs []int64) error
	CareTariffs(ctx context.Context, organizationID int64) ([]parentbilling.CareTariff, error)
}

// CareReductions serves parentbilling.CareReductions.
type CareReductions struct {
	store     CareReductionStore
	contracts parentbilling.CareContracts
	schools   parentbilling.CareSchools
	today     func() calendar.Date
}

var _ parentbilling.CareReductions = (*CareReductions)(nil)

// NewCareReductions builds the rules and evidences of the monthly
// contribution.
func NewCareReductions(store CareReductionStore, contracts parentbilling.CareContracts, schools parentbilling.CareSchools,
	today func() calendar.Date,
) *CareReductions {
	if store == nil || contracts == nil || schools == nil || today == nil {
		panic("parent billing: the store, the care contracts, the schools and the clock are required")
	}
	return &CareReductions{store: store, contracts: contracts, schools: schools, today: today}
}

// CareRules lists the care rules with the schools of the Träger and the
// modules its contracts book. It names no person.
func (c *CareReductions) CareRules(ctx context.Context, reviewer parentbilling.HolidayReviewer) (parentbilling.CareRuleSet, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareRuleSet{}, err
	}
	rules, err := c.store.ListContributionRules(ctx, reviewer.OrganizationID, parentbilling.ServiceCare)
	if err != nil {
		return parentbilling.CareRuleSet{}, fmt.Errorf("parent billing: list care rules: %w", err)
	}
	schools, err := c.schools.TraegerSchools(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.CareRuleSet{}, fmt.Errorf("parent billing: read the schools: %w", err)
	}
	versions, err := c.bookedVersions(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.CareRuleSet{}, err
	}
	tariffs, err := c.store.CareTariffs(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.CareRuleSet{}, err
	}
	return parentbilling.CareRuleSet{Rules: rules, Schools: schools, Modules: careModules(versions, tariffs)}, nil
}

// SaveCareRule validates a care rule as a whole and stores it under the
// lock of the monthly billing, so a release computes with the rules before
// or after the change, never with half of it.
func (c *CareReductions) SaveCareRule(ctx context.Context, reviewer parentbilling.HolidayReviewer, ruleID int64, input parentbilling.CareRuleInput) (parentbilling.CareRuleSet, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareRuleSet{}, err
	}
	if ruleID < 0 {
		return parentbilling.CareRuleSet{}, parentbilling.ErrContributionRuleNotFound
	}
	schools, err := c.schools.TraegerSchools(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.CareRuleSet{}, fmt.Errorf("parent billing: read the schools: %w", err)
	}
	input, err = normalizeCareRule(input, schools)
	if err != nil {
		return parentbilling.CareRuleSet{}, err
	}
	if err := c.store.LockCareBilling(ctx, reviewer.OrganizationID); err != nil {
		return parentbilling.CareRuleSet{}, fmt.Errorf("parent billing: hold the monthly billing: %w", err)
	}
	rules, err := c.store.ListContributionRules(ctx, reviewer.OrganizationID, parentbilling.ServiceCare)
	if err != nil {
		return parentbilling.CareRuleSet{}, fmt.Errorf("parent billing: list care rules: %w", err)
	}
	if ruleID > 0 && !slices.ContainsFunc(rules, func(rule parentbilling.ContributionRule) bool { return rule.ID == ruleID }) {
		return parentbilling.CareRuleSet{}, parentbilling.ErrContributionRuleNotFound
	}
	if overlapsCareRule(rules, ruleID, input) {
		return parentbilling.CareRuleSet{}, parentbilling.ErrContributionRuleOverlap
	}
	if ruleID == 0 {
		err = c.store.InsertCareRule(ctx, reviewer.OrganizationID, input)
	} else {
		err = c.store.UpdateCareRule(ctx, reviewer.OrganizationID, ruleID, input)
	}
	if err != nil {
		return parentbilling.CareRuleSet{}, err
	}
	return c.CareRules(ctx, reviewer)
}

// DeleteCareRule removes a care rule; an income level an evidence names
// stays. An issued invoice keeps its copy of the rule.
func (c *CareReductions) DeleteCareRule(ctx context.Context, reviewer parentbilling.HolidayReviewer, ruleID int64) (parentbilling.CareRuleSet, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareRuleSet{}, err
	}
	if ruleID <= 0 {
		return parentbilling.CareRuleSet{}, parentbilling.ErrContributionRuleNotFound
	}
	if err := c.store.LockCareBilling(ctx, reviewer.OrganizationID); err != nil {
		return parentbilling.CareRuleSet{}, fmt.Errorf("parent billing: hold the monthly billing: %w", err)
	}
	if err := c.store.DeleteCareRule(ctx, reviewer.OrganizationID, ruleID); err != nil {
		return parentbilling.CareRuleSet{}, err
	}
	return c.CareRules(ctx, reviewer)
}

// overlapsCareRule reports another rule of the same kind that applies to
// one of the schools in a common month. Income levels may overlap: a
// contract names one of them.
func overlapsCareRule(rules []parentbilling.ContributionRule, ruleID int64, input parentbilling.CareRuleInput) bool {
	if input.Kind == parentbilling.RuleIncomeLevel {
		return false
	}
	for _, rule := range rules {
		if rule.ID == ruleID || rule.Kind != input.Kind || !validityOverlaps(rule.ValidFrom, rule.ValidUntil, input.ValidFrom, input.ValidUntil) {
			continue
		}
		if slices.ContainsFunc(rule.SchoolIDs, func(id int64) bool { return slices.Contains(input.SchoolIDs, id) }) {
			return true
		}
	}
	return false
}

func validityOverlaps(fromA calendar.Date, untilA *calendar.Date, fromB calendar.Date, untilB *calendar.Date) bool {
	return (untilA == nil || !untilA.Before(fromB)) && (untilB == nil || !untilB.Before(fromA))
}

// versions reads the contract versions in force between from and to.
func (c *CareReductions) versions(ctx context.Context, organizationID int64, from, to calendar.Date) ([]parentbilling.CareContractVersion, error) {
	versions, err := c.contracts.VersionsInForce(ctx, organizationID, from, to)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read the contract versions: %w", err)
	}
	return versions, nil
}

// bookedVersions reads the versions of the year before and the year after
// the current month, as the price list does, so the rule page offers the
// modules the price list shows.
func (c *CareReductions) bookedVersions(ctx context.Context, organizationID int64) ([]parentbilling.CareContractVersion, error) {
	current := firstOfMonth(c.today())
	earlier, err := c.versions(ctx, organizationID, calendar.NewDate(current.Year()-1, current.Month(), 1), current.AddDays(-1))
	if err != nil {
		return nil, err
	}
	later, err := c.versions(ctx, organizationID, current, current.AddDays(careModuleHorizon))
	if err != nil {
		return nil, err
	}
	return append(earlier, later...), nil
}

// CareEvidences lists the contracts in force with their evidences and the
// income levels, and records the read for every contract it shows.
func (c *CareReductions) CareEvidences(ctx context.Context, reviewer parentbilling.HolidayReviewer) (parentbilling.CareEvidenceSet, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareEvidenceSet{}, err
	}
	today := c.today()
	versions, err := c.versions(ctx, reviewer.OrganizationID, today.AddDays(-evidenceLookBack), today.AddDays(evidenceLookAhead))
	if err != nil {
		return parentbilling.CareEvidenceSet{}, err
	}
	contracts := evidenceContracts(versions)
	ids := make([]int64, len(contracts))
	for i, contract := range contracts {
		ids[i] = contract.ContractID
	}
	evidences, err := c.store.CareEvidences(ctx, reviewer.OrganizationID, ids)
	if err != nil {
		return parentbilling.CareEvidenceSet{}, err
	}
	byContract := map[int64][]parentbilling.CareEvidence{}
	for _, evidence := range evidences {
		byContract[evidence.ContractID] = append(byContract[evidence.ContractID], evidence)
	}
	for i := range contracts {
		if list, ok := byContract[contracts[i].ContractID]; ok {
			contracts[i].Evidences = list
		}
	}
	rules, err := c.store.ListContributionRules(ctx, reviewer.OrganizationID, parentbilling.ServiceCare)
	if err != nil {
		return parentbilling.CareEvidenceSet{}, fmt.Errorf("parent billing: list care rules: %w", err)
	}
	levels := []parentbilling.ContributionRule{}
	for _, rule := range rules {
		if rule.Kind == parentbilling.RuleIncomeLevel {
			levels = append(levels, rule)
		}
	}
	if len(ids) > 0 {
		if err := c.store.InsertCareEvidenceEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, evidenceEventRead, nil, ids); err != nil {
			return parentbilling.CareEvidenceSet{}, fmt.Errorf("parent billing: record evidence read: %w", err)
		}
	}
	return parentbilling.CareEvidenceSet{Contracts: contracts, Levels: levels}, nil
}

// evidenceContracts is one entry per contract with the child and school of
// its latest version, by school and child.
func evidenceContracts(versions []parentbilling.CareContractVersion) []parentbilling.CareEvidenceContract {
	latest := map[int64]parentbilling.CareContractVersion{}
	for _, version := range versions {
		if current, ok := latest[version.ContractID]; !ok || current.From.Before(version.From) {
			latest[version.ContractID] = version
		}
	}
	contracts := make([]parentbilling.CareEvidenceContract, 0, len(latest))
	for _, version := range latest {
		contracts = append(contracts, parentbilling.CareEvidenceContract{
			ContractID: version.ContractID, SchoolID: version.SchoolID, SchoolName: version.SchoolName,
			ChildFirstName: version.ChildFirstName, ChildLastName: version.ChildLastName, Evidences: []parentbilling.CareEvidence{},
		})
	}
	slices.SortFunc(contracts, func(a, b parentbilling.CareEvidenceContract) int {
		return firstNonZero(compareOrdered(a.SchoolName, b.SchoolName), compareOrdered(a.ChildLastName, b.ChildLastName),
			compareOrdered(a.ChildFirstName, b.ChildFirstName), compareOrdered(a.ContractID, b.ContractID))
	})
	return contracts
}

// SaveCareEvidence validates and stores an evidence and records the change.
func (c *CareReductions) SaveCareEvidence(ctx context.Context, reviewer parentbilling.HolidayReviewer, evidenceID int64, input parentbilling.CareEvidenceInput) (parentbilling.CareEvidenceSet, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareEvidenceSet{}, err
	}
	if evidenceID < 0 {
		return parentbilling.CareEvidenceSet{}, parentbilling.ErrCareEvidenceNotFound
	}
	rules, err := c.store.ListContributionRules(ctx, reviewer.OrganizationID, parentbilling.ServiceCare)
	if err != nil {
		return parentbilling.CareEvidenceSet{}, fmt.Errorf("parent billing: list care rules: %w", err)
	}
	input, err = normalizeCareEvidence(input, rules, c.today())
	if err != nil {
		return parentbilling.CareEvidenceSet{}, err
	}
	if err := c.store.LockCareBilling(ctx, reviewer.OrganizationID); err != nil {
		return parentbilling.CareEvidenceSet{}, fmt.Errorf("parent billing: hold the monthly billing: %w", err)
	}
	action := evidenceEventCreate
	var contractID int64
	if evidenceID == 0 {
		evidenceID, err = c.store.InsertCareEvidence(ctx, reviewer.OrganizationID, reviewer.AccountID, input)
		contractID = input.ContractID
	} else {
		action = evidenceEventUpdate
		contractID, err = c.store.UpdateCareEvidence(ctx, reviewer.OrganizationID, reviewer.AccountID, evidenceID, input)
	}
	if err != nil {
		return parentbilling.CareEvidenceSet{}, err
	}
	if err := c.store.InsertCareEvidenceEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, action, &evidenceID, []int64{contractID}); err != nil {
		return parentbilling.CareEvidenceSet{}, fmt.Errorf("parent billing: record evidence change: %w", err)
	}
	return c.CareEvidences(ctx, reviewer)
}

// DeleteCareEvidence removes an evidence and records it. An issued invoice
// keeps the reduction it copied.
func (c *CareReductions) DeleteCareEvidence(ctx context.Context, reviewer parentbilling.HolidayReviewer, evidenceID int64) (parentbilling.CareEvidenceSet, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareEvidenceSet{}, err
	}
	if evidenceID <= 0 {
		return parentbilling.CareEvidenceSet{}, parentbilling.ErrCareEvidenceNotFound
	}
	if err := c.store.LockCareBilling(ctx, reviewer.OrganizationID); err != nil {
		return parentbilling.CareEvidenceSet{}, fmt.Errorf("parent billing: hold the monthly billing: %w", err)
	}
	contractID, err := c.store.DeleteCareEvidence(ctx, reviewer.OrganizationID, evidenceID)
	if err != nil {
		return parentbilling.CareEvidenceSet{}, err
	}
	if err := c.store.InsertCareEvidenceEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, evidenceEventDelete, &evidenceID, []int64{contractID}); err != nil {
		return parentbilling.CareEvidenceSet{}, fmt.Errorf("parent billing: record evidence removal: %w", err)
	}
	return c.CareEvidences(ctx, reviewer)
}
