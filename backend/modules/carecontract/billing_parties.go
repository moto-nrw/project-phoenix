package carecontract

import "context"

// ContractParties reads the parties of contracts for the parents' billing
// (#4684): the monthly invoice of a contract goes to one of them. It runs in
// the Träger transaction the caller holds, which records the read in its
// own trail.
type ContractParties interface {
	// PartiesOf names the parties of every given contract of the Träger in
	// the order they were added; a contract of another Träger has none.
	PartiesOf(ctx context.Context, organizationID int64, contractIDs []int64) (map[int64][]Party, error)
}
