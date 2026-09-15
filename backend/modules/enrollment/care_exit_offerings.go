package enrollment

import (
	"encoding/json"
)

// CareOfferingLink is the retained projection contract for a request child's
// effective booking. Care Plan owns its half-open validity and effective days;
// consumers join it with Enrollment identity and their offering catalog.
type CareOfferingLink struct {
	ID             int64    `json:"id"`
	TenantID       int64    `json:"tenant_id"`
	RequestChildID int64    `json:"request_child_id"`
	CareOfferingID int64    `json:"care_offering_id"`
	SelectedDays   []string `json:"selected_days"`
	ValidFrom      *Date    `json:"valid_from"`
	ValidUntil     *Date    `json:"valid_until"`
}

// CareOfferingLinkRecordColumns is the jsonb_to_recordset column list that
// matches the JSON encoding of CareOfferingLink. Consumers that join the
// projection inside their own SQL use it as `AS link(` + columns + `)`.
const CareOfferingLinkRecordColumns = `id bigint, tenant_id bigint, request_child_id bigint, care_offering_id bigint,
 selected_days jsonb, valid_from date, valid_until date`

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
