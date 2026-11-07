package carecontract

import "context"

// ContractBilling is what the parents' billing reads of the contracts
// (#4681, #4684): the versions in force and the party each monthly invoice
// goes to. It runs in the Träger transaction the caller holds.
type ContractBilling interface {
	ContractVersions
	// BillingRecipients names per contract of the Träger the party its
	// invoice goes to: the first party with an e-mail address, else the
	// first party. A contract of another Träger or without a party has
	// none.
	BillingRecipients(ctx context.Context, organizationID int64, contractIDs []int64) (map[int64]Party, error)
}
