package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// The number of children of a contract's family (#6335): on the one-time
// link of a contract mail, and taken over from the registration when the
// contract is made. carecontract.FamilyNumbers keeps it at the
// Elternabrechnung; the Betreuungsvertrag checks the contract and names its
// family, the contract's billing recipient.

// LinkFamilyChildren reads the number of the family of an open link's
// contract. Like the read of the contract it leaves the link open.
func (s *Signatures) LinkFamilyChildren(ctx context.Context, link string) (carecontract.FamilyChildren, error) {
	return s.linkFamily(ctx, link, func(txCtx context.Context, organizationID int64, family carecontract.FamilyContract) (carecontract.FamilyChildren, error) {
		return s.ports.Families.FamilyChildren(txCtx, organizationID, family)
	})
}

// StateLinkFamilyChildren stores the number the parents state on an open
// link for the family of its contract; the link stays open, so they can
// correct it until they sign.
func (s *Signatures) StateLinkFamilyChildren(ctx context.Context, link string, children int) (carecontract.FamilyChildren, error) {
	return s.linkFamily(ctx, link, func(txCtx context.Context, organizationID int64, family carecontract.FamilyContract) (carecontract.FamilyChildren, error) {
		return s.ports.Families.StateFamilyChildren(txCtx, organizationID, family, children, nil)
	})
}

type familyStep func(txCtx context.Context, organizationID int64, family carecontract.FamilyContract) (carecontract.FamilyChildren, error)

// linkFamily runs step for the family of an open link's contract in the
// link's Träger transaction. Every link openLink refuses is refused here.
func (s *Signatures) linkFamily(ctx context.Context, link string, step familyStep) (carecontract.FamilyChildren, error) {
	organizationID, token, err := parseLink(link)
	if err != nil {
		return carecontract.FamilyChildren{}, err
	}
	var result carecontract.FamilyChildren
	err = s.withinLink(ctx, organizationID, func(txCtx context.Context) error {
		target, err := s.openLink(txCtx, organizationID, token, s.clock.now())
		if err != nil {
			return err
		}
		recipient := billingRecipient(target.record.Parties)
		if recipient.Email == "" {
			return carecontract.ErrRecipientMissing
		}
		result, err = step(txCtx, organizationID, carecontract.FamilyContract{
			ContractID: target.record.Summary.ID, Email: recipient.Email, Channel: carecontract.FamilyChannelLink,
		})
		result.CanState = true
		return err
	})
	if err != nil {
		return carecontract.FamilyChildren{}, err
	}
	return result, nil
}

// adoptFamilyChildren hands the registration's number of a contract just
// made to the Elternabrechnung in the caller's Träger transaction (#6335).
// A contract whose invoice reaches no address has no family and no number.
func adoptFamilyChildren(ctx context.Context, store Store, families carecontract.FamilyNumbers, organizationID, contractID int64, draft NewContract) error {
	if draft.FamilyChildren == nil {
		return nil
	}
	record, err := store.Contract(ctx, organizationID, contractID, false)
	if err != nil {
		return err
	}
	recipient := billingRecipient(record.Parties)
	if recipient.Email == "" {
		return nil
	}
	var account int64
	if draft.Party.AccountID != nil {
		account = *draft.Party.AccountID
	}
	statedAt := draft.FamilyChildren.StatedAt
	_, err = families.StateFamilyChildren(ctx, organizationID, carecontract.FamilyContract{
		ContractID: contractID, Email: recipient.Email, AccountID: account, Channel: carecontract.FamilyChannelEnrollment,
	}, draft.FamilyChildren.Children, &statedAt)
	return err
}
