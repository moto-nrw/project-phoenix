package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
)

// The number of children of a contract's family (#6335): on the one-time
// link of a contract mail, and taken over from the registration when the
// contract is made. carecontract.FamilyNumbers keeps it at the
// Elternabrechnung; the Betreuungsvertrag checks the contract and names its
// family, the contract's billing recipient.

// LinkFamilyChildren reads the number of the family of an open link's
// contract. Like the read of the contract it leaves the link open.
func (s *Signatures) LinkFamilyChildren(ctx context.Context, link string) (carecontract.FamilyChildren, error) {
	organizationID, token, err := parseLink(link)
	if err != nil {
		return carecontract.FamilyChildren{}, err
	}
	return s.linkFamily(ctx, organizationID, token, nil)
}

// StateLinkFamilyChildren stores the number the parents state on an open
// link for the family of its contract; the link stays open, so they can
// correct it until they sign.
func (s *Signatures) StateLinkFamilyChildren(ctx context.Context, link string, children int) (carecontract.FamilyChildren, error) {
	organizationID, token, err := parseLink(link)
	if err != nil {
		return carecontract.FamilyChildren{}, err
	}
	return s.linkFamily(ctx, organizationID, token, &children)
}

// linkFamily reads, or with children stores, the number of the family of an
// open link's contract in the link's Träger transaction. Every link
// openLink refuses is refused here. The contract's row is not locked: the
// number changes no contract, and the Elternabrechnung takes its own lock of
// the monthly billing. Holding the row while waiting for that lock would
// deadlock with the Abrechnung, which holds the lock and needs the row for
// the key of the number (40P01, #6335).
func (s *Signatures) linkFamily(ctx context.Context, organizationID int64, token string, children *int) (carecontract.FamilyChildren, error) {
	var result carecontract.FamilyChildren
	err := s.withinLink(ctx, organizationID, func(txCtx context.Context) error {
		target, err := s.openLinkContract(txCtx, organizationID, token, s.clock.now(), false)
		if err != nil {
			return err
		}
		recipient := billingRecipient(target.record.Parties)
		if recipient.Email == "" {
			return carecontract.ErrRecipientMissing
		}
		family := carecontract.FamilyContract{ContractID: target.record.Summary.ID, Email: recipient.Email, Channel: carecontract.FamilyChannelLink}
		if children == nil {
			result, err = s.ports.Families.FamilyChildren(txCtx, organizationID, family)
		} else {
			result, err = s.ports.Families.StateFamilyChildren(txCtx, organizationID, family, *children, nil)
		}
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
