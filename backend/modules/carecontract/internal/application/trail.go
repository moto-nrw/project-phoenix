package application

import (
	"context"
	"fmt"
)

type mailingStore interface {
	Mailings(ctx context.Context, organizationID, contractID int64) ([]StoredMailing, error)
}

// deliveryAddress is the address the contract went to last, else fallback:
// a confirmation follows the address the office corrected.
func deliveryAddress(ctx context.Context, store mailingStore, organizationID, contractID int64, fallback string) (string, error) {
	mailings, err := store.Mailings(ctx, organizationID, contractID)
	if err != nil {
		return "", err
	}
	if len(mailings) > 0 {
		return mailings[0].Recipient, nil
	}
	return fallback, nil
}

type eventStore interface {
	InsertEvents(ctx context.Context, organizationID int64, events []Event) error
}

// trail records one operation in the trail of the Träger. accountID 0 is the
// system; contractID and schoolID 0 leave the event without that reference.
func trail(ctx context.Context, store eventStore, organizationID, accountID, contractID, schoolID int64, action string, details map[string]any) error {
	event := Event{AccountID: accountID, Action: action, Details: details}
	if contractID > 0 {
		event.ContractID = &contractID
	}
	if schoolID > 0 {
		event.SchoolID = &schoolID
	}
	if err := store.InsertEvents(ctx, organizationID, []Event{event}); err != nil {
		return fmt.Errorf("care contract: record %s: %w", action, err)
	}
	return nil
}

func trailContract(ctx context.Context, store eventStore, organizationID, accountID int64, record ContractRecord, action string, details map[string]any) error {
	return trail(ctx, store, organizationID, accountID, record.Summary.ID, record.Summary.SchoolID, action, details)
}
