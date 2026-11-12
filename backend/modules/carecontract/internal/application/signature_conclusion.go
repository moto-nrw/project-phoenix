package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
)

// The conclusion of a signed contract (#4679). A signature, online or on
// paper, concludes the sent contract through the one transition of #4680
// (concludeContract): it becomes aktiv with the method and the instant of
// the signature, its version wirksam, under the row lock the caller holds.

// concludeSigned concludes the sent contract of record with the signature
// made at at: einfach_elektronisch for a drawn or typed signature, papier
// for a paper contract. A contract the transition refuses is no contract a
// signature can conclude.
func concludeSigned(ctx context.Context, store Store, organizationID, accountID int64, record ContractRecord,
	signature string, at time.Time) (string, error) {
	method := carecontract.MethodElectronic
	if signature == carecontract.SignaturePaper {
		method = carecontract.MethodPaper
	}
	err := concludeContract(ctx, store, organizationID, accountID, record, carecontract.Concluded{Method: method, At: at})
	if errors.Is(err, carecontract.ErrContractNotConcludable) {
		return "", fmt.Errorf("%w: %w", carecontract.ErrContractNotSignable, err)
	}
	if err != nil {
		return "", err
	}
	return carecontract.StatusActive, nil
}

// versionChecksum is the SHA-256 of the structured version the parents
// sign: child, place of care, services, price, terms and dates. What the
// timeline derives is left out: the state changes when the version takes
// effect, and ValidUntil when a later version starts (#4681).
func versionChecksum(fingerprint func([]byte) string, version carecontract.Version) (string, error) {
	version.Status, version.ValidUntil = "", nil
	encoded, err := json.Marshal(version)
	if err != nil {
		return "", fmt.Errorf("care contract: encode version: %w", err)
	}
	return fingerprint(encoded), nil
}
