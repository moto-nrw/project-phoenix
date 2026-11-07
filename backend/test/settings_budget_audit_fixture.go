//go:build testhooks

package test

import (
	"context"
	"encoding/json"

	"github.com/uptrace/bun"
)

// OperatorSettingsReadAuditSnapshot contains the audit fields asserted after
// an operator reads the school settings schema.
type OperatorSettingsReadAuditSnapshot struct {
	OperatorID int64
	Changes    json.RawMessage
	RequestIP  string
	ResourceID int64
}

// ReadOperatorSettingsReadAuditSnapshots reads the exact operator and school
// fixture's view entries in the administrative transaction supplied by the test.
func ReadOperatorSettingsReadAuditSnapshots(ctx context.Context, db bun.IDB, operatorID, schoolID int64) ([]OperatorSettingsReadAuditSnapshot, error) {
	var rows []OperatorSettingsReadAuditSnapshot
	err := db.NewRaw(`SELECT operator_id, changes, host(request_ip) AS request_ip, resource_id
 FROM platform.operator_audit_log
 WHERE operator_id = ? AND resource_id = ? AND action = 'view' AND resource_type = 'school'
 ORDER BY id`, operatorID, schoolID).Scan(ctx, &rows)
	return rows, err
}
