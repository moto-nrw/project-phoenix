package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
)

// concludeContract is the one transition of a contract to "concluded"
// (#4680, ADR 0088 sections 2 and 3). The conclusion at acceptance (Weg b,
// Conclusions) and the signature of the parents (Weg a, #4679) both end
// here, so a concluded contract is the same whichever way it took.
//
// Invariants:
//   - The caller holds the row lock of the contract (Contract with
//     forUpdate) in the Träger transaction of organizationID, and record was
//     read under that lock.
//   - Only a sent contract is concluded: its PDF is frozen and the parents
//     received it. A draft, a concluded or an ended contract is
//     ErrContractNotConcludable; concluding twice is refused, not repeated.
//   - Its current version is still beantragt and becomes wirksam with the
//     contract becoming aktiv, in one statement pair under the same lock.
//   - The method is one of ADR 0088 section 3; the trail records it with the
//     version and the document, under the acting account (none for the
//     automatic run).
func concludeContract(ctx context.Context, store Store, organizationID, accountID int64, record ContractRecord,
	conclusion carecontract.Concluded) error {
	if !slices.Contains([]string{carecontract.MethodWithApplication, carecontract.MethodElectronic, carecontract.MethodPaper},
		conclusion.Method) || conclusion.At.IsZero() {
		return fmt.Errorf("care contract: unknown conclusion %q", conclusion.Method)
	}
	if record.Summary.Status != carecontract.StatusSent || record.DocumentID == nil ||
		record.Version.Status != carecontract.VersionRequested {
		return carecontract.ErrContractNotConcludable
	}
	if err := store.MarkConcluded(ctx, organizationID, record.Summary.ID, record.Version.ID, conclusion); err != nil {
		return err
	}
	return trailContract(ctx, store, organizationID, accountID, record, ActionConclude,
		map[string]any{"method": conclusion.Method, "version_id": record.Version.ID, "document_id": *record.DocumentID})
}
