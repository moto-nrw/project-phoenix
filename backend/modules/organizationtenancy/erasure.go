package organizationtenancy

import (
	"context"
	"encoding/json"
	"time"
)

// Erasure is the internal, non-HTTP tenant lifecycle capability (ADR 0092).
// Completed evidence covers database and local files, not external systems.
type Erasure interface {
	Run(context.Context, ErasureRequest) ([]json.RawMessage, error)
}

type ErasureRequest struct {
	ReceiptID, SchoolID, OrganizationID, OperatorID int64
	OrderReference, Confirmation                    string
	DryRun, PrerequisitesChecked, WritersStopped    bool
	LockTimeout, StatementTimeout                   time.Duration
}

// ErasureFiles uses deterministic tenant namespaces, never stored filenames.
// Implementations reject unsafe paths and do not return object names in errors.
type ErasureFiles interface {
	TenantObjects(context.Context, string, string, int64, bool) (int64, error)
}

type ErasureProof struct {
	ID          int64      `json:"id"`
	SchoolID    int64      `json:"school_id"`
	Status      string     `json:"status"`
	DBDoneAt    *time.Time `json:"db_done_at"`
	InventoryAt *time.Time `json:"files_inventory_at"`
}

// ErasureLedger survives database restores. Check never creates a missing list.
// The root binds the independently protected host store to the erasure command.
type ErasureLedger interface {
	Check() error
	Record(json.RawMessage) error
}
