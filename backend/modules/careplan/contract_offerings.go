package careplan

import "context"

// ContractOffering is a care offering of a school as the Betreuungsvertrag
// of the Träger names it in a contract and its template (#4678, ADR 0153).
type ContractOffering struct {
	ID         int64
	PhaseID    int64
	Name       string
	PriceCents *int
	IsActive   bool
}

// ContractOfferings reads the offerings of the school whose tenant
// transaction the caller holds. A non-empty ids keeps only those.
type ContractOfferings interface {
	ContractOfferings(ctx context.Context, ids []int64) ([]ContractOffering, error)
}
