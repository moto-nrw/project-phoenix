package active

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

// ScheduledCheckout represents a scheduled future checkout for a student
type ScheduledCheckout struct {
	bun.BaseModel `bun:"table:active.scheduled_checkouts"`

	ID           int64      `bun:"id,pk,autoincrement" json:"id"`
	StudentID    int64      `bun:"student_id,notnull" json:"student_id"`
	ScheduledBy  int64      `bun:"scheduled_by,notnull" json:"scheduled_by"`
	ScheduledFor time.Time  `bun:"scheduled_for,notnull" json:"scheduled_for"`
	Reason       string     `bun:"reason" json:"reason,omitempty"`
	Status       string     `bun:"status,notnull" json:"status"` // pending, executed, cancelled
	ExecutedAt   *time.Time `bun:"executed_at" json:"executed_at,omitempty"`
	CancelledAt  *time.Time `bun:"cancelled_at" json:"cancelled_at,omitempty"`
	CancelledBy  *int64     `bun:"cancelled_by" json:"cancelled_by,omitempty"`
	CreatedAt    time.Time  `bun:"created_at,notnull" json:"created_at"`
	UpdatedAt    time.Time  `bun:"updated_at,notnull" json:"updated_at"`
}

// TableName returns the table name for the model
func (sc *ScheduledCheckout) TableName() string {
	return "active.scheduled_checkouts"
}

// BeforeAppendModel is called by BUN before appending the model to the query
// Commented out to prevent triple-quote issues with ORDER BY
// func (sc *ScheduledCheckout) BeforeAppendModel(ctx context.Context, query bun.Query) error {
// 	switch query.(type) {
// 	case *bun.SelectQuery:
// 		// Let repository handle table expression for SELECT
// 	case *bun.InsertQuery:
// 		// Let repository handle table expression for INSERT
// 	case *bun.UpdateQuery:
// 		// Let repository handle table expression for UPDATE
// 	case *bun.DeleteQuery:
// 		// Let repository handle table expression for DELETE
// 	}
// 	return nil
// }

// ScheduledCheckoutStatus constants
const (
	ScheduledCheckoutStatusPending   = "pending"
	ScheduledCheckoutStatusExecuted  = "executed"
	ScheduledCheckoutStatusCancelled = "cancelled"
)

// ScheduledCheckoutRepository defines the interface for scheduled checkout operations
type ScheduledCheckoutRepository interface {
	// Create creates a new scheduled checkout
	Create(ctx context.Context, checkout *ScheduledCheckout) error

	// GetByID retrieves a scheduled checkout by ID
	GetByID(ctx context.Context, id int64) (*ScheduledCheckout, error)

	// GetPendingByStudentID gets the pending scheduled checkout for a student
	GetPendingByStudentID(ctx context.Context, studentID int64) (*ScheduledCheckout, error)

	// GetDueCheckouts retrieves all pending checkouts scheduled for before the given time
	GetDueCheckouts(ctx context.Context, beforeTime time.Time) ([]*ScheduledCheckout, error)

	// Update updates a scheduled checkout
	Update(ctx context.Context, checkout *ScheduledCheckout) error

	// Delete deletes a scheduled checkout
	Delete(ctx context.Context, id int64) error

	// ListByStudentID lists all scheduled checkouts for a student
	ListByStudentID(ctx context.Context, studentID int64) ([]*ScheduledCheckout, error)

	// CancelPendingByStudentID cancels any pending scheduled checkouts for a student
	CancelPendingByStudentID(ctx context.Context, studentID int64, cancelledBy int64) error
}
