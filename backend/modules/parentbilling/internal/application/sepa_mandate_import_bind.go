package application

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

// What binds an import to the stored mandates and to the preview the office
// confirmed (#6198): the account of a stored mandate the file names again is
// opened in memory to tell a duplicate from a mandate with another account,
// and the import takes over only the rows the confirmed preview showed.

// importAccounts opens the IBAN and account holder of the stored mandates a
// row of the file names by its reference. The values stay in memory for
// the comparison and leave neither in an answer nor in a log; the trail of
// each opened mandate records the read with the source import.
func (m *SEPAMandates) importAccounts(ctx context.Context, reviewer parentbilling.HolidayReviewer, table importTable,
	columns importColumnIndex, existing []parentbilling.SEPAMandate,
) (map[int64]importAccount, error) {
	named := map[string]bool{}
	if position, ok := columns.fields[parentbilling.ImportFieldReference]; ok {
		for _, cells := range table.rows {
			if position < len(cells) {
				named[cells[position]] = true
			}
		}
	}
	ids := []int64{}
	for _, mandate := range existing {
		if named[mandate.Reference] {
			ids = append(ids, mandate.ID)
		}
	}
	accounts := map[int64]importAccount{}
	if len(ids) == 0 {
		return accounts, nil
	}
	sealed, err := m.store.SealedSEPAMandates(ctx, reviewer.OrganizationID, ids)
	if err != nil {
		return nil, err
	}
	opened := make([]parentbilling.SEPAMandate, len(sealed))
	for i, entry := range sealed {
		account, err := m.openAccount(reviewer.OrganizationID, entry)
		if err != nil {
			return nil, err
		}
		accounts[entry.Mandate.ID], opened[i] = account, entry.Mandate
	}
	if err := m.store.InsertSEPAMandateEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, mandateRead,
		parentbilling.MandateFromImport, nil, opened); err != nil {
		return nil, fmt.Errorf("parent billing: record the comparison of the SEPA mandates: %w", err)
	}
	return accounts, nil
}

// openAccount opens the sealed IBAN and account holder of one mandate with
// the contexts the owner sealed them with.
func (m *SEPAMandates) openAccount(organizationID int64, entry parentbilling.SealedSEPAMandate) (importAccount, error) {
	iban, err := m.cipher.Open(entry.IBAN, ibanContext(organizationID, entry.Mandate.Reference))
	if err != nil {
		return importAccount{}, fmt.Errorf("parent billing: open the IBAN of mandate %d: %w", entry.Mandate.ID, err)
	}
	holder, err := m.cipher.Open(entry.Holder, holderContext(organizationID, entry.Mandate.Reference))
	if err != nil {
		return importAccount{}, fmt.Errorf("parent billing: open the account holder of mandate %d: %w", entry.Mandate.ID, err)
	}
	return importAccount{iban: string(iban), holder: string(holder)}, nil
}

// importFingerprint names the rows of a look at a file and what the import
// does with each: its line, status, contract, mandate, sequence, problems
// and hints, with the mapping and the creditor identifier they were read
// with. The import recomputes it and takes over only when it matches the
// preview the office confirmed. A change detector, not a secret: a 128-bit
// FNV hash of the canonical JSON tells two states apart, like the review of
// a monthly run (#5916). It covers no bank data beyond the masked IBAN.
func importFingerprint(mapping parentbilling.SEPAMandateImportMapping, creditorID string, candidates []importCandidate) string {
	rows := make([]parentbilling.SEPAMandateImportRow, len(candidates))
	// Where each number goes, the family or the child (#7007): a change of
	// the Träger's scope since the preview changes the confirmation.
	destinations := make([]string, len(candidates))
	for i, candidate := range candidates {
		rows[i] = candidate.row
		destinations[i] = "family\x1e" + candidate.family
		if candidate.perChild {
			destinations[i] = "child\x1e" + importChildKey(candidate)
		}
	}
	state, err := json.Marshal(struct {
		Mapping      parentbilling.SEPAMandateImportMapping `json:"mapping"`
		Creditor     string                                 `json:"creditor"`
		Rows         []parentbilling.SEPAMandateImportRow   `json:"rows"`
		Destinations []string                               `json:"destinations"`
	}{mapping, creditorID, rows, destinations})
	if err != nil {
		// Every value above marshals; a failure leaves no fingerprint an
		// import could match.
		return ""
	}
	hash := fnv.New128a()
	_, _ = hash.Write(state)
	return hex.EncodeToString(hash.Sum(nil))
}
