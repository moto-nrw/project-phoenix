package careplan

import "context"

// ContractOffering is a care offering of a school as the Betreuungsvertrag
// of the Träger names it in a contract and its template (#4678, ADR 0153).
// A contract books it as a module; PickupTimes is its end time per weekday
// ("mon" to "HH:MM").
type ContractOffering struct {
	ID          int64
	PhaseID     int64
	Name        string
	PriceCents  *int
	IsActive    bool
	PickupTimes map[string]string
}

// ContractOfferings reads the offerings of the school that count as care,
// in the tenant transaction the caller holds: a Betreuungsvertrag covers
// care, not an AG or the lunch. A non-empty ids keeps only those.
type ContractOfferings interface {
	ContractOfferings(ctx context.Context, ids []int64) ([]ContractOffering, error)
}
