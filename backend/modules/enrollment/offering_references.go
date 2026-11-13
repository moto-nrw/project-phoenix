package enrollment

import "context"

// OfferingSelectionReferences reports every stored selection that prevents
// deleting a school offering, independently of its request status.
type OfferingSelectionReferences interface {
	ReferencedOfferingIDs(ctx context.Context, ids []int64) ([]int64, error)
}
