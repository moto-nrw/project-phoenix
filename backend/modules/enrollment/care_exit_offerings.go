package enrollment

import (
	"context"
	"encoding/json"
)

// CareOfferingLink is one care-offering selection of a request child with its
// half-open validity interval. It carries no child or guardian names and no
// Care Plan facts: consumers join it with their own offering projection.
type CareOfferingLink struct {
	ID             int64    `json:"id"`
	TenantID       int64    `json:"tenant_id"`
	RequestChildID int64    `json:"request_child_id"`
	CareOfferingID int64    `json:"care_offering_id"`
	SelectedDays   []string `json:"selected_days"`
	ValidFrom      *Date    `json:"valid_from"`
	ValidUntil     *Date    `json:"valid_until"`
}

// CareExitOfferingSnapshot is the verbatim copy of one offering link a care
// exit ends or deletes. Snapshot is the full row so the restore can put a
// deleted link back under its original id.
type CareExitOfferingSnapshot struct {
	TenantID       int64           `json:"tenant_id"`
	StudentID      int64           `json:"student_id"`
	RequestChildID int64           `json:"request_child_id"`
	SourceRowID    int64           `json:"source_row_id"`
	WasDeleted     bool            `json:"was_deleted"`
	Snapshot       json.RawMessage `json:"snapshot"`
}

// CareExitOfferingSnapshotRestore names one ledger entry the restore replays.
type CareExitOfferingSnapshotRestore struct {
	SourceRowID int64           `json:"source_row_id"`
	WasDeleted  bool            `json:"was_deleted"`
	Snapshot    json.RawMessage `json:"snapshot"`
}

// ApprovedBookingOfferingLinks returns the offering links of every approved
// request child of the caller's school for booking consistency audits.
func (m *Module) ApprovedBookingOfferingLinks(ctx context.Context) ([]CareOfferingLink, error) {
	var links []CareOfferingLink
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		links, err = m.engine.ApprovedBookingOfferingLinks(txCtx)
		return err
	})
	return links, err
}

// CareExitOfferingLinks returns the offering links of the request children that
// reference the selected students as created or matched student. An empty
// selection supplies the school's links for the scheduled expiry evaluation.
func (m *Module) CareExitOfferingLinks(ctx context.Context, studentIDs []int64) ([]CareOfferingLink, error) {
	var links []CareOfferingLink
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		links, err = m.engine.CareExitOfferingLinks(txCtx, studentIDs)
		return err
	})
	return links, err
}

// LockCareExitOfferingLinks locks the offering links of the given request
// children that are still open on or after the day a care exit ends them.
// It requires the caller's transaction, which holds the lock until it ends.
func (m *Module) LockCareExitOfferingLinks(ctx context.Context, requestChildIDs []int64, validUntil Date) error {
	if len(requestChildIDs) == 0 {
		return nil
	}
	return m.engine.LockCareExitOfferingLinks(ctx, requestChildIDs, validUntil)
}

// CareExitOfferingSnapshots copies every offering link of the students' source
// applications that a care exit at validUntil (exclusive) would cap or delete.
// sourceRequestChildID narrows the copy to one application.
func (m *Module) CareExitOfferingSnapshots(ctx context.Context, studentIDs []int64, validUntil Date, sourceRequestChildID *int64) ([]CareExitOfferingSnapshot, error) {
	if len(studentIDs) == 0 {
		return []CareExitOfferingSnapshot{}, nil
	}
	var snapshots []CareExitOfferingSnapshot
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		snapshots, err = m.engine.CareExitOfferingSnapshots(txCtx, studentIDs, validUntil, sourceRequestChildID)
		return err
	})
	return snapshots, err
}

// EndCareExitOfferingLinks deletes the links of the given request children
// that start on or after validUntil and caps the running ones at validUntil
// (exclusive). It returns the number of changed rows.
func (m *Module) EndCareExitOfferingLinks(ctx context.Context, requestChildIDs []int64, sourceRequestChildID *int64, validUntil Date) (int64, error) {
	if len(requestChildIDs) == 0 {
		return 0, nil
	}
	var changed int64
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		changed, err = m.engine.EndCareExitOfferingLinks(txCtx, requestChildIDs, sourceRequestChildID, validUntil)
		return err
	})
	return changed, err
}

// RestoreCareExitOfferingLinks replays the ledger of an ended care exit:
// capped links recover their original exclusive end and deleted links are
// recreated under their original id. A link that already exists again is left
// alone. It returns the number of recreated rows.
func (m *Module) RestoreCareExitOfferingLinks(ctx context.Context, snapshots []CareExitOfferingSnapshotRestore) (int64, error) {
	if len(snapshots) == 0 {
		return 0, nil
	}
	var restored int64
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		restored, err = m.engine.RestoreCareExitOfferingLinks(txCtx, snapshots)
		return err
	})
	return restored, err
}
