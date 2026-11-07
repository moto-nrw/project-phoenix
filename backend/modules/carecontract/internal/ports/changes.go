package ports

import "context"

// ChangeContract is what a decision of the parents' change needs from a
// contract (#5979): its school, its child and its status.
type ChangeContract struct {
	ID             int64
	SchoolID       int64
	StudentID      int64
	RequestChildID int64
	Status         string
}

// ChangeStore reads the contract of a change in its Träger transaction.
type ChangeStore interface {
	ChangeContract(ctx context.Context, organizationID, contractID int64) (ChangeContract, error)
}
