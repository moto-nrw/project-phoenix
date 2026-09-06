package test

import (
	"context"
	"encoding/json"
	"testing"

	enrollment "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// insertAuditOfferingSelection preserves audit fixtures, including deliberately
// inconsistent selections that the production commands would reject.
func insertAuditOfferingSelection(t *testing.T, db *bun.DB, ctx context.Context, tenantID int64, row *enrollment.RequestChildOffering) {
	t.Helper()
	days, err := json.Marshal(row.SelectedDays)
	require.NoError(t, err)
	var storedDays any
	if len(row.SelectedDays) > 0 {
		storedDays = string(days)
	}
	row.TenantID = tenantID
	err = db.NewRaw(`INSERT INTO enrollment.request_child_offerings
 (tenant_id, request_child_id, care_offering_id, selected_days, valid_from, valid_until)
 VALUES (?, ?, ?, ?::jsonb, ?, ?) RETURNING id, created_at, updated_at`,
		tenantID, row.RequestChildID, row.CareOfferingID, storedDays, row.ValidFrom, row.ValidUntil).Scan(ctx, row)
	require.NoError(t, err)
}
