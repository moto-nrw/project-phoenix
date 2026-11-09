package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// FamilyChildrenStore persists the numbers of the families of a Träger and
// their trail in its Träger transaction (#5918).
type FamilyChildrenStore interface {
	FamilyChildCounts(ctx context.Context, organizationID int64) ([]parentbilling.FamilyChildCount, error)
	FamilyChildCount(ctx context.Context, organizationID int64, family string) (parentbilling.FamilyChildCount, bool, error)
	// StateFamilyChildren and ConfirmFamilyChildren report a contract that
	// is no contract of the Träger as ErrFamilyChildrenNotFound.
	// A statement made before the family's newest one changes nothing:
	// applied is false.
	StateFamilyChildren(ctx context.Context, organizationID, accountID int64, family string, contractID int64, children int,
		statedAt *time.Time) (id int64, applied bool, err error)
	ConfirmFamilyChildren(ctx context.Context, organizationID, accountID int64, family string, input parentbilling.FamilyChildrenInput) (int64, error)
	DeleteFamilyChildren(ctx context.Context, organizationID int64, family string) (parentbilling.FamilyChildCount, error)
	InsertFamilyChildEvents(ctx context.Context, organizationID int64, event parentbilling.FamilyChildEvent, targets []parentbilling.FamilyChildTarget) error
	ListContributionRules(ctx context.Context, organizationID int64, service string) ([]parentbilling.ContributionRule, error)
	// LockCareBilling holds the monthly billing of the Träger until the
	// transaction ends, so a release counts a family before or after a
	// change, never in between.
	LockCareBilling(ctx context.Context, organizationID int64) error
}

// FamilyNumbers serves the Abrechnung's check of the numbers and the
// parents' statement.
type FamilyNumbers struct {
	store     FamilyChildrenStore
	contracts parentbilling.CareContracts
	today     func() calendar.Date
}

var (
	_ parentbilling.FamilyChildrenOffice          = (*FamilyNumbers)(nil)
	_ parentbilling.ParentFamilyChildrenStatement = (*FamilyNumbers)(nil)
	_ parentbilling.ContractFamilyChildren        = (*FamilyNumbers)(nil)
)

// NewFamilyChildren builds the numbers of the families.
func NewFamilyChildren(store FamilyChildrenStore, contracts parentbilling.CareContracts, today func() calendar.Date) *FamilyNumbers {
	if store == nil || contracts == nil || today == nil {
		panic("parent billing: the store, the care contracts and the clock are required")
	}
	return &FamilyNumbers{store: store, contracts: contracts, today: today}
}

// NewParentFamilyChildren builds the parents' statement alone; it reads no
// contract, because the parents portal checked the reader's contract.
func NewParentFamilyChildren(store FamilyChildrenStore) *FamilyNumbers {
	if store == nil {
		panic("parent billing: the store is required")
	}
	return &FamilyNumbers{store: store, today: calendar.TodayDate}
}

// errOfficeNotConfigured reports the parents' build asked for the office.
var errOfficeNotConfigured = errors.New("parent billing: the family children office needs the care contracts")

// familyKey is the key of a family, the same as the rule chain uses (#4689).
func familyKey(email string) string { return parentbilling.FamilyKey(email) }

// FamilyChildren lists every family with a contract in force in the window
// of the evidence page, its children with a contract and its number, and
// records the read of every number it shows.
func (f *FamilyNumbers) FamilyChildren(ctx context.Context, reviewer parentbilling.HolidayReviewer) (parentbilling.FamilyChildrenSet, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.FamilyChildrenSet{}, err
	}
	if f.contracts == nil {
		return parentbilling.FamilyChildrenSet{}, errOfficeNotConfigured
	}
	today := f.today()
	versions, err := f.contracts.VersionsInForce(ctx, reviewer.OrganizationID, today.AddDays(-evidenceLookBack), today.AddDays(evidenceLookAhead))
	if err != nil {
		return parentbilling.FamilyChildrenSet{}, fmt.Errorf("parent billing: read the contract versions: %w", err)
	}
	counts, err := f.store.FamilyChildCounts(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.FamilyChildrenSet{}, err
	}
	// The recipients of the contracts in force and of the contracts a number
	// hangs on: a family that stated its number on a contract not yet in
	// force still needs the check of the Abrechnung.
	contracts := evidenceContracts(versions)
	ids := make([]int64, 0, len(contracts)+len(counts))
	for _, contract := range contracts {
		ids = append(ids, contract.ContractID)
	}
	for _, count := range counts {
		if !slices.Contains(ids, count.ContractID) {
			ids = append(ids, count.ContractID)
		}
	}
	recipients, err := f.contracts.Recipients(ctx, reviewer.OrganizationID, ids)
	if err != nil {
		return parentbilling.FamilyChildrenSet{}, fmt.Errorf("parent billing: read the contract parties: %w", err)
	}
	families := groupFamilies(versions, recipients, counts, today)
	if err := f.recordFamilyReads(ctx, reviewer, families); err != nil {
		return parentbilling.FamilyChildrenSet{}, err
	}
	counting, err := f.rulesCountingFamily(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.FamilyChildrenSet{}, err
	}
	return parentbilling.FamilyChildrenSet{Families: families, Rules: counting}, nil
}

// rulesCountingFamily lists the care sibling rules of the Träger that count
// the family's number.
func (f *FamilyNumbers) rulesCountingFamily(ctx context.Context, organizationID int64) ([]parentbilling.ContributionRule, error) {
	rules, err := f.store.ListContributionRules(ctx, organizationID, parentbilling.ServiceCare)
	if err != nil {
		return nil, fmt.Errorf("parent billing: list care rules: %w", err)
	}
	counting := []parentbilling.ContributionRule{}
	for _, rule := range rules {
		if rule.Kind == parentbilling.RuleSibling && rule.CountsFamilyChildren {
			counting = append(counting, rule)
		}
	}
	return counting, nil
}

// groupFamilies groups the contracts by the address of their invoice's
// recipient. A contract without an address is a family of its own and has
// no number, so it is left out.
func groupFamilies(versions []parentbilling.CareContractVersion, recipients map[int64]parentbilling.CareRecipient,
	counts []parentbilling.FamilyChildCount, today calendar.Date,
) []parentbilling.FamilyChildren {
	byContract, contractOrder := versionsByContract(versions)
	children := childKeys(byContract, contractOrder)
	seen := map[string]bool{}
	byKey := map[string]*parentbilling.FamilyChildren{}
	var order []string
	for _, contract := range evidenceContracts(versions) {
		recipient := recipients[contract.ContractID]
		key := familyKey(recipient.Email)
		if key == "" || seen[key+"|"+children[contract.ContractID]] {
			continue
		}
		seen[key+"|"+children[contract.ContractID]] = true
		family, ok := byKey[key]
		if !ok {
			family = &parentbilling.FamilyChildren{RecipientFirstName: recipient.FirstName, RecipientLastName: recipient.LastName,
				RecipientEmail: strings.TrimSpace(recipient.Email), Children: []parentbilling.FamilyContractChild{}}
			byKey[key] = family
			order = append(order, key)
		}
		family.Children = append(family.Children, parentbilling.FamilyContractChild{ContractID: contract.ContractID,
			SchoolName: contract.SchoolName, ChildFirstName: contract.ChildFirstName, ChildLastName: contract.ChildLastName})
	}
	for i := range counts {
		family, ok := byKey[counts[i].FamilyEmail]
		if !ok {
			// A number without a contract in force, for example stated on a
			// sent contract: the family shows with the recipient of that
			// contract and no child with a contract yet.
			recipient := recipients[counts[i].ContractID]
			family = &parentbilling.FamilyChildren{RecipientFirstName: recipient.FirstName, RecipientLastName: recipient.LastName,
				RecipientEmail: counts[i].FamilyEmail, Children: []parentbilling.FamilyContractChild{}}
			byKey[counts[i].FamilyEmail] = family
			order = append(order, counts[i].FamilyEmail)
		}
		family.Count = &counts[i]
	}
	families := make([]parentbilling.FamilyChildren, 0, len(order))
	for _, key := range order {
		family := byKey[key]
		family.Status = familyChildrenStatus(family.Count, today)
		families = append(families, *family)
	}
	slices.SortFunc(families, func(a, b parentbilling.FamilyChildren) int {
		return firstNonZero(compareOrdered(a.RecipientLastName, b.RecipientLastName),
			compareOrdered(a.RecipientFirstName, b.RecipientFirstName), compareOrdered(a.RecipientEmail, b.RecipientEmail))
	})
	return families
}

// familyChildrenStatus is the state of a number today; the most urgent
// check wins.
func familyChildrenStatus(count *parentbilling.FamilyChildCount, today calendar.Date) string {
	switch {
	case count == nil:
		return parentbilling.FamilyChildrenMissing
	case count.ConfirmedChildren == nil || count.ValidFrom == nil:
		return parentbilling.FamilyChildrenStated
	case count.ValidUntil != nil && count.ValidUntil.Before(today):
		return parentbilling.FamilyChildrenExpired
	case count.StatedChildren != nil && count.StatedAt != nil && count.ConfirmedAt != nil &&
		count.StatedAt.After(*count.ConfirmedAt) && *count.StatedChildren != *count.ConfirmedChildren:
		return parentbilling.FamilyChildrenChanged
	case count.ValidFrom.After(today):
		return laterConfirmationStatus(count, today)
	case count.ValidUntil != nil && !count.ValidUntil.After(today.AddDays(parentbilling.FamilyChildrenExpiryNotice)):
		return parentbilling.FamilyChildrenExpiring
	}
	return parentbilling.FamilyChildrenConfirmed
}

// laterConfirmationStatus is the state of a family whose newest
// confirmation starts after today: the earlier ones in force decide. Has the
// current one ended, the family has a gap now; does a chain of
// confirmations from today leave a first day without one within the notice,
// the family needs a check before it.
func laterConfirmationStatus(count *parentbilling.FamilyChildCount, today calendar.Date) string {
	current, started := count.InForceOn(today)
	switch {
	case !started || current.ValidUntil == nil:
		return parentbilling.FamilyChildrenUpcoming
	case current.ValidUntil.Before(today):
		return parentbilling.FamilyChildrenExpired
	}
	if gap, ok := firstGap(count, today); ok && !gap.After(today.AddDays(parentbilling.FamilyChildrenExpiryNotice+1)) {
		return parentbilling.FamilyChildrenExpiring
	}
	return parentbilling.FamilyChildrenUpcoming
}

// firstGap is the first day from day on without a confirmation in force,
// following one confirmation to the next; none when the chain reaches one
// without an end.
func firstGap(count *parentbilling.FamilyChildCount, day calendar.Date) (calendar.Date, bool) {
	for range len(count.Confirmations) + 2 {
		confirmation, started := count.InForceOn(day)
		if !started || (confirmation.ValidUntil != nil && confirmation.ValidUntil.Before(day)) {
			return day, true
		}
		if confirmation.ValidUntil == nil {
			return "", false
		}
		day = confirmation.ValidUntil.AddDays(1)
	}
	return day, true
}

// recordFamilyReads records the read of every number shown, on the contract
// it hangs on.
func (f *FamilyNumbers) recordFamilyReads(ctx context.Context, reviewer parentbilling.HolidayReviewer, families []parentbilling.FamilyChildren) error {
	var targets []parentbilling.FamilyChildTarget
	for _, family := range families {
		if family.Count != nil {
			targets = append(targets, parentbilling.FamilyChildTarget{ContractID: family.Count.ContractID, CountID: &family.Count.ID})
		}
	}
	event := parentbilling.FamilyChildEvent{Action: parentbilling.FamilyChildEventRead, Actor: parentbilling.FamilyChildActorOffice,
		AccountID: reviewer.AccountID}
	if err := f.store.InsertFamilyChildEvents(ctx, reviewer.OrganizationID, event, targets); err != nil {
		return fmt.Errorf("parent billing: record family child count read: %w", err)
	}
	return nil
}

// ConfirmFamilyChildren confirms or changes the number of the family of the
// input's contract under the lock of the monthly billing.
func (f *FamilyNumbers) ConfirmFamilyChildren(ctx context.Context, reviewer parentbilling.HolidayReviewer, input parentbilling.FamilyChildrenInput) (parentbilling.FamilyChildrenSet, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.FamilyChildrenSet{}, err
	}
	if f.contracts == nil {
		return parentbilling.FamilyChildrenSet{}, errOfficeNotConfigured
	}
	if err := validFamilyChildrenInput(input); err != nil {
		return parentbilling.FamilyChildrenSet{}, err
	}
	family, err := f.familyOfContract(ctx, reviewer.OrganizationID, input.ContractID)
	if err != nil {
		return parentbilling.FamilyChildrenSet{}, err
	}
	if err := f.store.LockCareBilling(ctx, reviewer.OrganizationID); err != nil {
		return parentbilling.FamilyChildrenSet{}, fmt.Errorf("parent billing: hold the monthly billing: %w", err)
	}
	id, err := f.store.ConfirmFamilyChildren(ctx, reviewer.OrganizationID, reviewer.AccountID, family, input)
	if err != nil {
		return parentbilling.FamilyChildrenSet{}, err
	}
	event := parentbilling.FamilyChildEvent{Action: parentbilling.FamilyChildEventConfirm, Actor: parentbilling.FamilyChildActorOffice,
		AccountID: reviewer.AccountID, Children: &input.Children, ValidFrom: &input.ValidFrom, ValidUntil: input.ValidUntil}
	target := parentbilling.FamilyChildTarget{ContractID: input.ContractID, CountID: &id}
	if err := f.store.InsertFamilyChildEvents(ctx, reviewer.OrganizationID, event, []parentbilling.FamilyChildTarget{target}); err != nil {
		return parentbilling.FamilyChildrenSet{}, fmt.Errorf("parent billing: record family child count change: %w", err)
	}
	return f.FamilyChildren(ctx, reviewer)
}

// RemoveFamilyChildren removes the number of the family of a contract; the
// sibling rule counts its children with a contract again.
func (f *FamilyNumbers) RemoveFamilyChildren(ctx context.Context, reviewer parentbilling.HolidayReviewer, contractID int64) (parentbilling.FamilyChildrenSet, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.FamilyChildrenSet{}, err
	}
	if f.contracts == nil {
		return parentbilling.FamilyChildrenSet{}, errOfficeNotConfigured
	}
	family, err := f.familyOfContract(ctx, reviewer.OrganizationID, contractID)
	if err != nil {
		return parentbilling.FamilyChildrenSet{}, err
	}
	if err := f.store.LockCareBilling(ctx, reviewer.OrganizationID); err != nil {
		return parentbilling.FamilyChildrenSet{}, fmt.Errorf("parent billing: hold the monthly billing: %w", err)
	}
	removed, err := f.store.DeleteFamilyChildren(ctx, reviewer.OrganizationID, family)
	if err != nil {
		return parentbilling.FamilyChildrenSet{}, err
	}
	event := parentbilling.FamilyChildEvent{Action: parentbilling.FamilyChildEventDelete, Actor: parentbilling.FamilyChildActorOffice,
		AccountID: reviewer.AccountID}
	target := parentbilling.FamilyChildTarget{ContractID: removed.ContractID}
	if err := f.store.InsertFamilyChildEvents(ctx, reviewer.OrganizationID, event, []parentbilling.FamilyChildTarget{target}); err != nil {
		return parentbilling.FamilyChildrenSet{}, fmt.Errorf("parent billing: record family child count removal: %w", err)
	}
	return f.FamilyChildren(ctx, reviewer)
}

// familyOfContract is the family of a contract of the Träger: the address
// of its invoice's recipient.
func (f *FamilyNumbers) familyOfContract(ctx context.Context, organizationID, contractID int64) (string, error) {
	if contractID <= 0 {
		return "", parentbilling.ErrFamilyChildrenNotFound
	}
	recipients, err := f.contracts.Recipients(ctx, organizationID, []int64{contractID})
	if err != nil {
		return "", fmt.Errorf("parent billing: read the contract parties: %w", err)
	}
	recipient, ok := recipients[contractID]
	if !ok {
		return "", parentbilling.ErrFamilyChildrenNotFound
	}
	family := familyKey(recipient.Email)
	if family == "" {
		return "", parentbilling.ErrFamilyWithoutAddress
	}
	return family, nil
}

func validFamilyChildrenInput(input parentbilling.FamilyChildrenInput) error {
	errs := validation.Errors{}
	if err := validChildren(input.Children); err != nil {
		errs["children"] = err
	}
	if input.ValidFrom == "" {
		errs["valid_from"] = errors.New("is required")
	}
	if input.ValidUntil != nil && input.ValidFrom != "" && input.ValidUntil.Before(input.ValidFrom) {
		errs["valid_until"] = errors.New("must not be before valid_from")
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func validChildren(children int) error {
	if children < 1 || children > parentbilling.MaxFamilyChildren {
		return errors.New("must be a whole number between 1 and 20")
	}
	return nil
}

// ParentFamilyChildren reads the number of the reader's family and records
// the read on the reader's contract.
func (f *FamilyNumbers) ParentFamilyChildren(ctx context.Context, organizationID int64, reader parentbilling.ParentFamilyReader) (parentbilling.ParentFamilyChildren, error) {
	if reader.AccountID <= 0 {
		return parentbilling.ParentFamilyChildren{}, parentbilling.ErrFamilyChildrenNotFound
	}
	return f.ContractFamilyChildren(ctx, organizationID, portalFamily(reader))
}

// StateFamilyChildren stores the number the parents state for their family
// on their contract. It counts once the Abrechnung confirmed it.
func (f *FamilyNumbers) StateFamilyChildren(ctx context.Context, organizationID int64, reader parentbilling.ParentFamilyReader, children int) (parentbilling.ParentFamilyChildren, error) {
	if reader.AccountID <= 0 {
		return parentbilling.ParentFamilyChildren{}, parentbilling.ErrFamilyChildrenNotFound
	}
	return f.StateContractFamilyChildren(ctx, organizationID, portalFamily(reader), children, nil)
}

func portalFamily(reader parentbilling.ParentFamilyReader) parentbilling.ContractFamily {
	return parentbilling.ContractFamily{ContractID: reader.ContractID, Email: reader.Email, AccountID: reader.AccountID,
		Channel: parentbilling.FamilyChannelPortal}
}

// ContractFamilyChildren reads the number of a contract's family and
// records the read with its channel on the contract (#6335).
func (f *FamilyNumbers) ContractFamilyChildren(ctx context.Context, organizationID int64, contract parentbilling.ContractFamily) (parentbilling.ParentFamilyChildren, error) {
	family, err := parentFamily(organizationID, contract)
	if err != nil {
		return parentbilling.ParentFamilyChildren{}, err
	}
	count, found, err := f.store.FamilyChildCount(ctx, organizationID, family)
	if err != nil {
		return parentbilling.ParentFamilyChildren{}, err
	}
	event := parentbilling.FamilyChildEvent{Action: parentbilling.FamilyChildEventRead, Actor: parentbilling.FamilyChildActorParent,
		Channel: contract.Channel, AccountID: contract.AccountID}
	target := parentbilling.FamilyChildTarget{ContractID: contract.ContractID}
	if found {
		target.CountID = &count.ID
	}
	if err := f.store.InsertFamilyChildEvents(ctx, organizationID, event, []parentbilling.FamilyChildTarget{target}); err != nil {
		return parentbilling.ParentFamilyChildren{}, fmt.Errorf("parent billing: record family child count read: %w", err)
	}
	if !found {
		return parentbilling.ParentFamilyChildren{}, nil
	}
	return parentView(count), nil
}

// StateContractFamilyChildren stores the number the parents state for the
// family of a contract. A statement made before the family's newest one
// (the registration's, taken over later) leaves the newer one and records
// nothing (#6335): an older number never silently replaces a newer one.
func (f *FamilyNumbers) StateContractFamilyChildren(ctx context.Context, organizationID int64, contract parentbilling.ContractFamily, children int,
	statedAt *time.Time) (parentbilling.ParentFamilyChildren, error) {
	family, err := parentFamily(organizationID, contract)
	if err != nil {
		return parentbilling.ParentFamilyChildren{}, err
	}
	if err := validChildren(children); err != nil {
		return parentbilling.ParentFamilyChildren{}, validation.Errors{"children": err}
	}
	if err := f.store.LockCareBilling(ctx, organizationID); err != nil {
		return parentbilling.ParentFamilyChildren{}, fmt.Errorf("parent billing: hold the monthly billing: %w", err)
	}
	id, applied, err := f.store.StateFamilyChildren(ctx, organizationID, contract.AccountID, family, contract.ContractID, children, statedAt)
	if err != nil {
		return parentbilling.ParentFamilyChildren{}, err
	}
	if applied {
		event := parentbilling.FamilyChildEvent{Action: parentbilling.FamilyChildEventState, Actor: parentbilling.FamilyChildActorParent,
			Channel: contract.Channel, AccountID: contract.AccountID, Children: &children}
		target := parentbilling.FamilyChildTarget{ContractID: contract.ContractID, CountID: &id}
		if err := f.store.InsertFamilyChildEvents(ctx, organizationID, event, []parentbilling.FamilyChildTarget{target}); err != nil {
			return parentbilling.ParentFamilyChildren{}, fmt.Errorf("parent billing: record family child count statement: %w", err)
		}
	}
	count, _, err := f.store.FamilyChildCount(ctx, organizationID, family)
	if err != nil {
		return parentbilling.ParentFamilyChildren{}, err
	}
	return parentView(count), nil
}

// CountsFamilyChildrenAt tells whether a care sibling rule of the Träger
// counts the family's children for a contract at the school (#6335): it
// counts them, applies to the school and has not ended by day. A rule that
// starts later still asks, since the contract may fall into it.
func (f *FamilyNumbers) CountsFamilyChildrenAt(ctx context.Context, organizationID, schoolID int64, day calendar.Date) (bool, error) {
	if organizationID <= 0 || schoolID <= 0 {
		return false, nil
	}
	rules, err := f.rulesCountingFamily(ctx, organizationID)
	if err != nil {
		return false, err
	}
	return countsFamilyAt(rules, schoolID, day), nil
}

func countsFamilyAt(rules []parentbilling.ContributionRule, schoolID int64, day calendar.Date) bool {
	return slices.ContainsFunc(rules, func(rule parentbilling.ContributionRule) bool {
		return rule.Service == parentbilling.ServiceCare && rule.Kind == parentbilling.RuleSibling && rule.CountsFamilyChildren &&
			(rule.ValidUntil == nil || !rule.ValidUntil.Before(day)) && slices.Contains(rule.SchoolIDs, schoolID)
	})
}

// parentFamily keys the family of a checked contract. The parents portal
// names its account; the link and the registration may have none.
func parentFamily(organizationID int64, contract parentbilling.ContractFamily) (string, error) {
	family := familyKey(contract.Email)
	if organizationID <= 0 || contract.ContractID <= 0 || contract.AccountID < 0 || family == "" {
		return "", parentbilling.ErrFamilyChildrenNotFound
	}
	switch contract.Channel {
	case parentbilling.FamilyChannelPortal:
		if contract.AccountID == 0 {
			return "", parentbilling.ErrFamilyChildrenNotFound
		}
	case parentbilling.FamilyChannelLink, parentbilling.FamilyChannelEnrollment:
	default:
		return "", parentbilling.ErrFamilyChildrenNotFound
	}
	return family, nil
}

func parentView(count parentbilling.FamilyChildCount) parentbilling.ParentFamilyChildren {
	return parentbilling.ParentFamilyChildren{ConfirmedAt: count.ConfirmedAt, StatedChildren: count.StatedChildren, StatedAt: count.StatedAt,
		ConfirmedChildren: count.ConfirmedChildren, ValidFrom: count.ValidFrom, ValidUntil: count.ValidUntil}
}
