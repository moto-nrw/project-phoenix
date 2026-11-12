package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// A fan-out may commit these ID-only entries after demo erasure (#6385).
// Exercise the same administrative retention command the delivery worker runs,
// independently of a live school, active account or pending erasure order.
func TestRetentionDeletesOrphanedReleaseAccountPushes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	org := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Release retention", Slug: fmt.Sprintf("release-retention-%d", testpkg.Tenant(t)),
	}).ID
	testpkg.OwnTraegerDocuments(t, db, org)
	school := testpkg.CreateTestSchoolInOrganization(t, db, org, "Deleted demo school")
	sibling := testpkg.CreateTestSchoolInOrganization(t, db, org, "Remaining school")
	anonymized := testpkg.CreateTestAccount(t, db, "erased@example.org").ID
	remaining := testpkg.CreateTestAccount(t, db, "remaining@example.org").ID
	testpkg.EnsureAccountTenant(t, db, remaining, sibling)

	var document int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.traeger_documents
		(organization_id, title, file_name, content_type, size_bytes, content)
		VALUES (?, 'Retention', 'test.pdf', 'application/pdf', 8, '%PDF-1.4'::bytea) RETURNING id`, org).Scan(ctx, &document))
	var releases []int64
	for _, scope := range []any{school, nil} {
		var release int64
		require.NoError(t, db.NewRaw(`INSERT INTO platform.traeger_document_releases
			(organization_id, document_id, audience, school_id)
			VALUES (?, ?, 'parents', ?) RETURNING id`, org, document, scope).Scan(ctx, &release))
		releases = append(releases, release)
	}
	testpkg.SoftDeleteTestSchool(t, db, school)
	_, err := db.ExecContext(ctx, `UPDATE auth.accounts
		SET email = 'demo-besuch-' || id || '@example.invalid', username = NULL,
		active = FALSE, password_hash = NULL WHERE id = ?`, anonymized)
	require.NoError(t, err)

	now := time.Date(2026, time.November, 9, 12, 0, 0, 0, time.UTC)
	require.Equal(t, 30*24*time.Hour, domain.RetentionWindow, "the documented retention is fixed at 30 days")
	before := now.Add(-domain.RetentionWindow)
	var expired, retained []int64
	for _, account := range []int64{anonymized, remaining} {
		for _, release := range releases {
			for _, state := range []struct {
				status string
				ended  time.Time
				remove bool
			}{
				{"sent", before.Add(-time.Second), true},
				{"dead_letter", before.Add(-time.Second), true},
				{"cancelled", before.Add(-time.Second), true},
				{"sent", before, false},
				{"cancelled", before.Add(time.Second), false},
				{"pending", before.Add(-time.Hour), false},
			} {
				var id int64
				require.NoError(t, db.NewRaw(`INSERT INTO platform.push_outbox
					(tenant_id, organization_id, kind, related_entity_type, related_entity_id,
					recipient, payload, status, sent_at, dead_letter_at, cancelled_at, updated_at)
					VALUES (NULL, ?, 'traeger_document_release_account', 'traeger_document_release', ?,
					'{}'::jsonb, jsonb_build_object('organization_id', ?::bigint, 'release_id', ?::bigint, 'account_id', ?::bigint),
					?, CASE WHEN ? = 'sent' THEN ?::timestamptz END,
					CASE WHEN ? = 'dead_letter' THEN ?::timestamptz END,
					CASE WHEN ? = 'cancelled' THEN ?::timestamptz END, ?) RETURNING id`,
					org, release, org, release, account, state.status,
					state.status, state.ended, state.status, state.ended, state.status, state.ended,
					before.Add(-time.Hour)).Scan(ctx, &id))
				if state.remove {
					expired = append(expired, id)
				} else {
					retained = append(retained, id)
				}
			}
		}
	}

	var deleted int
	require.NoError(t, tenant.WithAdminTx(ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		deleted, err = store.DeleteExpiredSystemEmails(txCtx, before, 25)
		return err
	}))
	assert.Equal(t, len(expired), deleted, "all expired orphan entries are removed in the worker's batch")
	var actual []int64
	require.NoError(t, db.NewRaw(`SELECT id FROM platform.push_outbox WHERE organization_id = ?`, org).Scan(context.Background(), &actual))
	assert.ElementsMatch(t, retained, actual, "terminal timestamps, not account state, school state or updated_at, decide retention")
}
