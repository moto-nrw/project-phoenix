//go:build testhooks

package application

import (
	"context"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
)

// SendLinkForTests stores the link a send of the contract makes, with what
// was sent, for a test that wrote the contract and its PDF directly.
func SendLinkForTests(ctx context.Context, store ContractStore, links carecontract.SigningLinks, organizationID, contractID int64) error {
	record, err := store.Contract(ctx, organizationID, contractID, true)
	if err != nil {
		return err
	}
	sent, err := sentState(ctx, store, links.Fingerprint, organizationID, record)
	if err != nil {
		return err
	}
	document, err := store.CurrentDocument(ctx, organizationID, contractID, record.Version.ID)
	if err != nil {
		return err
	}
	_, err = createSigningLink(ctx, store, links, Clock{Now: time.Now}, organizationID, contractID, document.ID, sent)
	return err
}
