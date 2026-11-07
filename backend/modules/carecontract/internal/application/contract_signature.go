package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// The signature on the contracts of the Träger-Büro (#4679): every send
// carries a new one-time link, and the detail shows the signature with the
// check of its checksums and the reminders.

// signingLink is the link of one mail.
type signingLink struct {
	url       string
	expiresOn string
}

// signingLinkFor makes the one-time link of a mail of record. Only a
// contract that waits for the signature gets one (#4679); a contract
// concluded at the acceptance (Weg b, #4680) or an active one sent again
// needs none.
func (c *Contracts) signingLinkFor(ctx context.Context, organizationID int64, record ContractRecord, documentID int64) (signingLink, error) {
	if record.Concluded != nil || record.Summary.Status == carecontract.StatusActive {
		return signingLink{}, nil
	}
	return c.newSigningLink(ctx, organizationID, record.Summary.ID, documentID)
}

// newSigningLink stores a new one-time link for the contract's frozen PDF
// and answers its URL. Only the SHA-256 of the token is stored.
func (c *Contracts) newSigningLink(ctx context.Context, organizationID, contractID, documentID int64) (signingLink, error) {
	return createSigningLink(ctx, c.store, c.links, c.clock, organizationID, contractID, documentID)
}

func createSigningLink(ctx context.Context, store SignatureStore, links carecontract.SigningLinks, clock Clock,
	organizationID, contractID, documentID int64) (signingLink, error) {
	raw, fingerprint, err := links.Tokens.NewToken()
	if err != nil {
		return signingLink{}, fmt.Errorf("care contract: signing token: %w", err)
	}
	expires := clock.now().Add(carecontract.SigningLinkLifetime)
	if err := store.InsertSigningLink(ctx, organizationID, contractID, documentID, fingerprint, expires); err != nil {
		return signingLink{}, err
	}
	base := strings.TrimRight(links.ParentsURL, "/")
	return signingLink{
		url:       fmt.Sprintf("%s/vertrag-unterschreiben/%d.%s", base, organizationID, raw),
		expiresOn: berlinDay(expires),
	}, nil
}

// addSignature adds the signature of the current version and the reminders
// to a detail. The signature is intact only when the ledger entry is, and
// when the PDF and the version still have the checksums it was given on.
func (c *Contracts) addSignature(ctx context.Context, organizationID int64, record ContractRecord, detail *carecontract.ContractDetail) error {
	reminders, err := c.store.Reminders(ctx, organizationID, record.Summary.ID)
	if err != nil {
		return err
	}
	detail.Reminders = reminders
	stored, err := c.store.Signature(ctx, organizationID, record.Summary.ID, record.Version.ID)
	if err != nil || stored == nil {
		return err
	}
	signature, err := c.signatureOf(ctx, organizationID, record, stored)
	if err != nil {
		return err
	}
	detail.Signature = &signature
	return nil
}

func (c *Contracts) signatureOf(ctx context.Context, organizationID int64, record ContractRecord, stored *StoredSignature) (carecontract.ContractSignature, error) {
	checksum, err := versionChecksum(c.links.Fingerprint, record.Version)
	if err != nil {
		return carecontract.ContractSignature{}, err
	}
	document, err := c.store.CurrentDocument(ctx, organizationID, record.Summary.ID, record.Version.ID)
	if err != nil {
		return carecontract.ContractSignature{}, err
	}
	signature := carecontract.ContractSignature{
		Method: stored.Method, Channel: stored.Channel, SignerName: stored.SignerName, SignedAt: stored.SignedAt,
		SignedOn: stored.SignedOn, VersionSHA256: stored.VersionSHA256, DocumentSHA256: stored.DocumentSHA256,
		WithdrawalNoticeAt: stored.WithdrawalNoticeAt, EarlyStartRequested: stored.EarlyStartRequested,
		PaperFilename: stored.PaperFilename,
		Intact: checksum == stored.VersionSHA256 && document.SHA256 == stored.DocumentSHA256 &&
			stored.DocumentID != nil && *stored.DocumentID == document.ID,
	}
	if stored.Method == carecontract.SignaturePaper {
		signature.Intact = signature.Intact && stored.PaperIntact
		return signature, nil
	}
	ends := withdrawalEnd(stored.SignedAt)
	signature.WithdrawalEndsOn = &ends
	if stored.AuditSignatureID == nil {
		signature.Intact = false
		return signature, nil
	}
	proofs, err := c.ledger.ContractSignatureProofs(ctx, []int64{*stored.AuditSignatureID})
	if err != nil {
		return carecontract.ContractSignature{}, fmt.Errorf("care contract: read signature: %w", err)
	}
	proof, ok := proofs[*stored.AuditSignatureID]
	signature.Intact = signature.Intact && ok && proof.Intact && proof.VersionID == record.Version.ID &&
		proof.DocumentSHA256 == stored.DocumentSHA256 && proof.Method == stored.Method &&
		proof.SignerName == stored.SignerName && proof.SignedAt.Equal(stored.SignedAt)
	signature.TypedName, signature.ImagePNG = proof.TypedName, proof.PNG
	return signature, nil
}
