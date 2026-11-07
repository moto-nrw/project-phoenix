package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// enqueueDocumentNotice puts the push „Neues Dokument vom Träger“ of a send
// on the outbox (#5941), in the Träger transaction of the send, before the
// mailing is recorded. Only the first send to an address does: the stored
// mailings are the durable proof of earlier sends, so a send again, a
// correction back to an earlier address or a retry adds no push, also once
// the outbox has forgotten the earlier one. A corrected address is a new
// recipient and gets its own push; the worker still checks its account and
// relationship. A failed enqueue fails the send, which rolls back.
func (c *Contracts) enqueueDocumentNotice(ctx context.Context, organizationID, contractID int64, address string) error {
	if c.notices == nil {
		return nil
	}
	recipient := strings.TrimSpace(address)
	stored, err := c.store.Mailings(ctx, organizationID, contractID)
	if err != nil {
		return err
	}
	for _, mailing := range stored {
		if strings.EqualFold(strings.TrimSpace(mailing.Recipient), recipient) {
			return nil
		}
	}
	err = c.notices.EnqueueContractDocumentNotice(ctx, carecontract.ContractDocumentNotice{
		OrganizationID: organizationID, ContractID: contractID, RecipientEmail: recipient,
	}, contractDocumentNoticeKey(contractID, recipient))
	if err != nil {
		return fmt.Errorf("care contract: enqueue document push: %w", err)
	}
	return nil
}

// contractDocumentNoticeKey names the contract and the address without
// spelling the address out: the key travels into Delivery's logs.
func contractDocumentNoticeKey(contractID int64, address string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(address)))
	return fmt.Sprintf("care-contract-document:%d:%s", contractID, hex.EncodeToString(sum[:8]))
}
