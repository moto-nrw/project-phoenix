package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The cost bearer shares of the contracts (#4687, ADR 0169). The share page
// lists the contracts of the evidence page (ADR 0164 point 7) with their
// shares and records every read and change per contract in
// parent_billing.care_cost_share_events. A change runs under the lock of the
// monthly billing, so a release computes with the shares before or after it.

// CareCostShareStore persists the shares and their trail in the Träger
// transaction.
type CareCostShareStore interface {
	// CareCostShares reads the shares of the contracts with the name of
	// their cost bearer, all when contractIDs is nil.
	CareCostShares(ctx context.Context, organizationID int64, contractIDs []int64) ([]parentbilling.CareCostShare, error)
	// InsertCareCostShare and UpdateCareCostShare report a contract or a cost
	// bearer of another Träger as a validation error and an overlap as
	// ErrCareCostShareOverlap; UpdateCareCostShare and DeleteCareCostShare a
	// missing share as ErrCareCostShareNotFound. They answer the contract.
	InsertCareCostShare(ctx context.Context, organizationID, accountID int64, input parentbilling.CareCostShareInput) (int64, error)
	UpdateCareCostShare(ctx context.Context, organizationID, accountID, shareID int64, input parentbilling.CareCostShareInput) (int64, error)
	DeleteCareCostShare(ctx context.Context, organizationID, shareID int64) (int64, error)
	// InsertCareCostShareEvents records one action of the account per
	// contract.
	InsertCareCostShareEvents(ctx context.Context, organizationID, accountID int64, action string, shareID *int64, contractIDs []int64) error
	ListCostBearers(ctx context.Context, organizationID int64) ([]parentbilling.CostBearer, error)
	// LockCostBearer reads a cost bearer of the Träger and holds its row
	// until the transaction ends; ErrCostBearerNotFound for another Träger's.
	LockCostBearer(ctx context.Context, organizationID, costBearerID int64, exclusive bool) (parentbilling.CostBearer, error)
}

// CareCostShares lists the contracts in force with their shares and the
// cost bearers of the Träger, and records the read for every contract it
// shows.
func (c *CareReductions) CareCostShares(ctx context.Context, reviewer parentbilling.HolidayReviewer) (parentbilling.CareCostShareSet, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareCostShareSet{}, err
	}
	today := c.today()
	versions, err := c.versions(ctx, reviewer.OrganizationID, today.AddDays(-evidenceLookBack), today.AddDays(evidenceLookAhead))
	if err != nil {
		return parentbilling.CareCostShareSet{}, err
	}
	evidenceContracts := evidenceContracts(versions)
	contracts := make([]parentbilling.CareCostShareContract, len(evidenceContracts))
	ids := make([]int64, len(evidenceContracts))
	for i, contract := range evidenceContracts {
		ids[i] = contract.ContractID
		contracts[i] = parentbilling.CareCostShareContract{
			ContractID: contract.ContractID, SchoolID: contract.SchoolID, SchoolName: contract.SchoolName,
			ChildFirstName: contract.ChildFirstName, ChildLastName: contract.ChildLastName, Shares: []parentbilling.CareCostShare{},
		}
	}
	shares, err := c.store.CareCostShares(ctx, reviewer.OrganizationID, ids)
	if err != nil {
		return parentbilling.CareCostShareSet{}, err
	}
	bearers, err := c.store.ListCostBearers(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.CareCostShareSet{}, fmt.Errorf("parent billing: list cost bearers: %w", err)
	}
	set := parentbilling.CareCostShareSet{Contracts: contracts, CostBearers: bearers, WarningDays: parentbilling.CostShareWarningDays}
	byContract := map[int64][]parentbilling.CareCostShare{}
	for _, share := range shares {
		share.Status = costShareStatus(share, shares, today)
		if share.Status == parentbilling.CostShareExpiring {
			set.Expiring++
		}
		byContract[share.ContractID] = append(byContract[share.ContractID], share)
	}
	for i := range set.Contracts {
		if list, ok := byContract[set.Contracts[i].ContractID]; ok {
			set.Contracts[i].Shares = list
		}
	}
	if len(ids) > 0 {
		if err := c.store.InsertCareCostShareEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, evidenceEventRead, nil, ids); err != nil {
			return parentbilling.CareCostShareSet{}, fmt.Errorf("parent billing: record cost share read: %w", err)
		}
	}
	return set, nil
}

// costShareStatus says where a share stands today. A share that ends within
// CostShareWarningDays warns unless a share of its kind for the contract
// covers the next billing month (the first share starting in a month covers
// that whole month, as in careCostShareInMonth).
func costShareStatus(share parentbilling.CareCostShare, all []parentbilling.CareCostShare, today calendar.Date) string {
	switch {
	case share.ValidFrom.After(today):
		return parentbilling.CostShareUpcoming
	case share.ValidUntil.Before(today):
		return parentbilling.CostShareExpired
	case share.ValidUntil.After(today.AddDays(parentbilling.CostShareWarningDays)):
		return parentbilling.CostShareActive
	}
	nextMonth := monthEnd(share.ValidUntil).AddDays(1)
	nextMonthEnd := monthEnd(nextMonth)
	if slices.ContainsFunc(all, func(next parentbilling.CareCostShare) bool {
		return next.ContractID == share.ContractID && next.Kind == share.Kind &&
			next.ValidFrom.After(share.ValidUntil) && !next.ValidFrom.After(nextMonthEnd) && !next.ValidUntil.Before(nextMonth)
	}) {
		return parentbilling.CostShareActive
	}
	return parentbilling.CostShareExpiring
}

// SaveCareCostShare validates and stores a share under the lock of the
// monthly billing and records the change.
func (c *CareReductions) SaveCareCostShare(ctx context.Context, reviewer parentbilling.HolidayReviewer, shareID int64, input parentbilling.CareCostShareInput) (parentbilling.CareCostShareSet, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareCostShareSet{}, err
	}
	if shareID < 0 {
		return parentbilling.CareCostShareSet{}, parentbilling.ErrCareCostShareNotFound
	}
	if err := c.store.LockCareBilling(ctx, reviewer.OrganizationID); err != nil {
		return parentbilling.CareCostShareSet{}, fmt.Errorf("parent billing: hold the monthly billing: %w", err)
	}
	input, err := normalizeCareCostShare(input, c.today())
	if err != nil {
		return parentbilling.CareCostShareSet{}, err
	}
	if err := c.bearerOffered(ctx, reviewer.OrganizationID, shareID, input); err != nil {
		return parentbilling.CareCostShareSet{}, err
	}
	action := evidenceEventCreate
	var contractID int64
	if shareID == 0 {
		shareID, err = c.store.InsertCareCostShare(ctx, reviewer.OrganizationID, reviewer.AccountID, input)
		contractID = input.ContractID
	} else {
		action = evidenceEventUpdate
		contractID, err = c.store.UpdateCareCostShare(ctx, reviewer.OrganizationID, reviewer.AccountID, shareID, input)
	}
	if err != nil {
		return parentbilling.CareCostShareSet{}, err
	}
	if err := c.store.InsertCareCostShareEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, action, &shareID, []int64{contractID}); err != nil {
		return parentbilling.CareCostShareSet{}, fmt.Errorf("parent billing: record cost share change: %w", err)
	}
	return c.CareCostShares(ctx, reviewer)
}

// bearerOffered holds the cost bearer of the Träger the share names under a
// shared row lock until the transaction ends, so a deletion of the cost
// bearer waits and then sees the share (#5627: the share names it without a
// foreign key). It refuses a cost bearer of another Träger and one the
// Träger switched off, unless the share already names it: a switched-off
// cost bearer stays on the shares that have it but is offered for no new one.
func (c *CareReductions) bearerOffered(ctx context.Context, organizationID, shareID int64, input parentbilling.CareCostShareInput) error {
	bearer, err := c.store.LockCostBearer(ctx, organizationID, input.CostBearerID, false)
	if errors.Is(err, parentbilling.ErrCostBearerNotFound) {
		return validation.Errors{"cost_bearer_id": errors.New("must name a cost bearer of the Träger")}
	}
	if err != nil {
		return fmt.Errorf("parent billing: hold the cost bearer: %w", err)
	}
	switchedOff := validation.Errors{"cost_bearer_id": errors.New("the cost bearer is switched off")}
	if bearer.Active {
		return nil
	}
	if shareID == 0 {
		return switchedOff
	}
	shares, err := c.store.CareCostShares(ctx, organizationID, []int64{input.ContractID})
	if err != nil {
		return err
	}
	if slices.ContainsFunc(shares, func(share parentbilling.CareCostShare) bool {
		return share.ID == shareID && share.CostBearerID == input.CostBearerID
	}) {
		return nil
	}
	return switchedOff
}

// DeleteCareCostShare removes a share and records it. An issued invoice
// keeps the share it copied.
func (c *CareReductions) DeleteCareCostShare(ctx context.Context, reviewer parentbilling.HolidayReviewer, shareID int64) (parentbilling.CareCostShareSet, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareCostShareSet{}, err
	}
	if shareID <= 0 {
		return parentbilling.CareCostShareSet{}, parentbilling.ErrCareCostShareNotFound
	}
	if err := c.store.LockCareBilling(ctx, reviewer.OrganizationID); err != nil {
		return parentbilling.CareCostShareSet{}, fmt.Errorf("parent billing: hold the monthly billing: %w", err)
	}
	contractID, err := c.store.DeleteCareCostShare(ctx, reviewer.OrganizationID, shareID)
	if err != nil {
		return parentbilling.CareCostShareSet{}, err
	}
	if err := c.store.InsertCareCostShareEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, evidenceEventDelete, &shareID, []int64{contractID}); err != nil {
		return parentbilling.CareCostShareSet{}, fmt.Errorf("parent billing: record cost share removal: %w", err)
	}
	return c.CareCostShares(ctx, reviewer)
}

// normalizeCareCostShare checks a share: a contract, a kind, a cost bearer,
// exactly one of a share or an amount, what it covers (the care when not
// given, #4686), a validity with an end, the reference
// to the proof and the day it was checked, not in the future.
func normalizeCareCostShare(input parentbilling.CareCostShareInput, today calendar.Date) (parentbilling.CareCostShareInput, error) {
	input.Kind = strings.TrimSpace(input.Kind)
	input.Reference = strings.TrimSpace(input.Reference)
	errs := validation.Errors{}
	if input.ContractID <= 0 {
		errs["contract_id"] = errors.New("is required")
	}
	if !slices.Contains(parentbilling.CostShareKinds, input.Kind) {
		errs["kind"] = errors.New("unknown cost share kind")
	}
	if input.CostBearerID <= 0 {
		errs["cost_bearer_id"] = errors.New("is required")
	}
	checkCostShareAmount(input, errs)
	if input.Scope = strings.TrimSpace(input.Scope); input.Scope == "" {
		input.Scope = parentbilling.CostShareScopeCare
	}
	if !slices.Contains(parentbilling.CostShareScopes, input.Scope) {
		errs["scope"] = errors.New("unknown cost share scope")
	}
	if input.ValidFrom == "" {
		errs["valid_from"] = errors.New("is required")
	}
	if input.ValidUntil == "" || (input.ValidFrom != "" && input.ValidUntil.Before(input.ValidFrom)) {
		errs["valid_until"] = errors.New("is required and not before valid_from")
	}
	switch {
	case input.Reference == "":
		errs["reference"] = errors.New("is required")
	case len([]rune(input.Reference)) > parentbilling.MaxCostShareReferenceLength:
		errs["reference"] = errors.New("is too long")
	}
	if input.CheckedOn == "" || input.CheckedOn.After(today) {
		errs["checked_on"] = errors.New("is required and not in the future")
	}
	if len(errs) > 0 {
		return input, errs
	}
	return input, nil
}

// checkCostShareAmount wants exactly one of a share in basis points up to
// the whole contribution or a monthly amount in cent.
func checkCostShareAmount(input parentbilling.CareCostShareInput, errs validation.Errors) {
	switch {
	case (input.ShareBasisPoints == nil) == (input.ShareCents == nil):
		errs["share_basis_points"] = errors.New("a share has exactly one of a share or an amount")
	case input.ShareBasisPoints != nil && (*input.ShareBasisPoints < 1 || *input.ShareBasisPoints > parentbilling.BasisPointsWhole):
		errs["share_basis_points"] = errors.New("must be between 1 and 10000 basis points")
	case input.ShareCents != nil && (*input.ShareCents < 1 || *input.ShareCents > parentbilling.MaxDiscountCents):
		errs["share_cents"] = errors.New("must be an amount in cent")
	}
}
