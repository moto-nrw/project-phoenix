package care

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Children of the family in the parents portal (#5918, ADR 0088 section 4
// addendum 2026-11-08). On a care contract of their inbox the parents read
// how many children their family has for the sibling rule of the Träger and
// state the number, siblings without a contract included. Three facts must
// hold, none alone is enough:
//
//   - The payer: the contract was sent to the account's own address. The
//     family is that address, so an account only ever reaches its own
//     family's number.
//   - The relationship to the contract's child: parent_portal.enrollments.view
//     reads; parent_portal.enrollment.submit states, the right that binds the
//     family with an enrollment or a contract.
//   - The contract is sent or active: an ended contract shows the number
//     but takes no new one.
//
// The Elternabrechnung stores the number, records every read and statement
// and counts it once its Abrechnung confirmed it.

// ErrFamilyChildrenInvalid reports a number that is no whole number from 1
// to 20.
var ErrFamilyChildrenInvalid = errors.New("parent: the number of children is invalid")

// FamilyChildrenReader is the account that reads or states the number on a
// contract the flow checked.
type FamilyChildrenReader struct {
	AccountID  int64
	Email      string
	ContractID int64
}

// FamilyChildren is the number of the family as the parents see it, and
// whether they may state one on this contract.
type FamilyChildren struct {
	StatedChildren    *int           `json:"stated_children"`
	StatedAt          *time.Time     `json:"stated_at"`
	ConfirmedChildren *int           `json:"confirmed_children"`
	ValidFrom         *calendar.Date `json:"valid_from"`
	ValidUntil        *calendar.Date `json:"valid_until"`
	CanState          bool           `json:"can_state"`
}

// FamilyChildrenStatements is the port to the Elternabrechnung's numbers in
// the Träger transaction the flow opens.
type FamilyChildrenStatements interface {
	FamilyChildren(ctx context.Context, organizationID int64, reader FamilyChildrenReader) (FamilyChildren, error)
	StateFamilyChildren(ctx context.Context, organizationID int64, reader FamilyChildrenReader, children int) (FamilyChildren, error)
}

// FamilyChildren reads the number of the account's family on a contract of
// its inbox.
func (s *Service) FamilyChildren(ctx context.Context, accountID, organizationID, contractID int64) (FamilyChildren, error) {
	return s.familyChildren(ctx, accountID, organizationID, contractID, GuardianPermissionEnrollmentsView,
		func(txCtx context.Context, reader FamilyChildrenReader) (FamilyChildren, error) {
			return s.FamilyChildrenStatements.FamilyChildren(txCtx, organizationID, reader)
		})
}

// StateFamilyChildren stores the number the account states for its family
// on a sent or active contract of its inbox.
func (s *Service) StateFamilyChildren(ctx context.Context, accountID, organizationID, contractID int64, children int) (FamilyChildren, error) {
	return s.familyChildren(ctx, accountID, organizationID, contractID, GuardianPermissionEnrollmentSubmit,
		func(txCtx context.Context, reader FamilyChildrenReader) (FamilyChildren, error) {
			return s.FamilyChildrenStatements.StateFamilyChildren(txCtx, organizationID, reader, children)
		})
}

type familyChildrenStep func(txCtx context.Context, reader FamilyChildrenReader) (FamilyChildren, error)

// familyChildren resolves the children the account holds permission for at
// the Träger, checks in that Träger's transaction that the contract was
// sent to the account for one of them and runs step. An account without
// such a child at any Träger gets ErrGuardianPermissionDenied; a contract
// outside its inbox, ErrBillingDocumentNotFound.
func (s *Service) familyChildren(ctx context.Context, accountID, organizationID, contractID int64, permission string, step familyChildrenStep) (FamilyChildren, error) {
	if s.FamilyChildrenStatements == nil || s.ContractDocuments == nil || s.TraegerRuntime == nil || s.SchoolTraeger == nil || s.ParentAccounts == nil {
		return FamilyChildren{}, errors.New("parent: family children are not configured")
	}
	var access documentAccess
	var email string
	err := s.accountDocumentAccess(ctx, accountID, func(adminCtx context.Context, children []*ChildSummary) error {
		var err error
		if access, err = s.documentAccessFor(adminCtx, children, permission); err != nil || len(access.traeger) == 0 {
			return err
		}
		email, err = s.ParentAccounts.AccountEmail(adminCtx, accountID)
		return err
	})
	if err != nil {
		return FamilyChildren{}, fmt.Errorf("parent: resolve family children: %w", err)
	}
	if len(access.traeger) == 0 {
		return FamilyChildren{}, ErrGuardianPermissionDenied
	}
	if !slices.ContainsFunc(access.traeger, func(t Traeger) bool { return t.ID == organizationID }) || contractID <= 0 {
		return FamilyChildren{}, ErrBillingDocumentNotFound
	}
	inbox := BillingDocumentReader{AccountID: accountID, Email: email, Children: access.permittedChildren(),
		SigningChildren: access.signingChildren()}
	var result FamilyChildren
	err = s.TraegerRuntime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		contracts, err := s.ContractDocuments.ListContractDocuments(txCtx, organizationID, inbox)
		if err != nil {
			return err
		}
		index := slices.IndexFunc(contracts, func(c BillingDocument) bool { return c.ID == contractID })
		if index < 0 {
			return ErrBillingDocumentNotFound
		}
		// A sent contract the account may sign or an active one it may end:
		// both need parent_portal.enrollment.submit on the contract's child.
		canState := contracts[index].CanSign || contracts[index].CanEndContract
		if permission == GuardianPermissionEnrollmentSubmit && !canState {
			return ErrGuardianPermissionDenied
		}
		result, err = step(txCtx, FamilyChildrenReader{AccountID: accountID, Email: email, ContractID: contractID})
		result.CanState = canState
		return err
	})
	if err != nil {
		return FamilyChildren{}, err
	}
	return result, nil
}
