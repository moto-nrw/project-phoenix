package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestTraegerPushesStayWithTheirTraeger pins ADR 0126 on the push outbox:
// under the request role a Träger adds and reads only its own pushes, a
// foreign Träger and a school transaction see none, a push has exactly one
// scope, and an idempotency key is one push per Träger.
func TestTraegerPushesStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newTraeger := func(name string) (int64, int64) {
		slug := fmt.Sprintf("traeger-push-%d", testpkg.UniqueTestTenantID(t))
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug}).ID
		return organization, testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS")
	}
	own, ownSchool := newTraeger("Träger Push Eigen")
	foreign, _ := newTraeger("Träger Push Fremd")

	asRequestRole := func(setting, value string, fn func(context.Context, bun.Tx) error) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config(?, ?, true)`, setting, value); err != nil {
				return err
			}
			return fn(ctx, tx)
		})
	}
	insertPush := func(organizationID int64, key string) func(context.Context, bun.Tx) error {
		return func(ctx context.Context, tx bun.Tx) error {
			_, err := tx.ExecContext(ctx, `INSERT INTO platform.push_outbox (organization_id, kind, idempotency_key, recipient, payload)
				VALUES (?, 'parent_billing_document', ?, '{"deferred":true}', '{}')`, organizationID, key)
			return err
		}
	}
	countPushes := func(count *int) func(context.Context, bun.Tx) error {
		return func(ctx context.Context, tx bun.Tx) error {
			return tx.NewRaw(`SELECT COUNT(*) FROM platform.push_outbox WHERE organization_id IN (?, ?)`, own, foreign).Scan(ctx, count)
		}
	}
	ownOrg := fmt.Sprint(own)

	require.NoError(t, asRequestRole("app.current_org_id", ownOrg, insertPush(own, "notice:1")),
		"the request role adds a push of its Träger")
	assert.ErrorContains(t, asRequestRole("app.current_org_id", ownOrg, insertPush(own, "notice:1")),
		"uq_push_outbox_tenant_idempotency", "an idempotency key is one push of the Träger")
	assert.ErrorContains(t, asRequestRole("app.current_org_id", fmt.Sprint(foreign), insertPush(own, "notice:2")),
		"row-level security", "a foreign Träger adds no push of this Träger")

	var seen int
	require.NoError(t, asRequestRole("app.current_org_id", ownOrg, countPushes(&seen)))
	assert.Equal(t, 1, seen, "the Träger reads its own push")
	require.NoError(t, asRequestRole("app.current_org_id", fmt.Sprint(foreign), countPushes(&seen)))
	assert.Zero(t, seen, "a foreign Träger reads no push of this Träger")
	require.NoError(t, asRequestRole("app.current_tenant_id", fmt.Sprint(ownSchool), countPushes(&seen)))
	assert.Zero(t, seen, "a school transaction reads no push of its Träger")

	_, err := db.ExecContext(ctx, `INSERT INTO platform.push_outbox (kind, recipient, payload) VALUES ('test', '{}', '{}')`)
	assert.ErrorContains(t, err, "chk_push_outbox_one_scope", "a push names a school or a Träger")
	_, err = db.ExecContext(ctx, `INSERT INTO platform.push_outbox (tenant_id, organization_id, kind, recipient, payload)
		VALUES (?, ?, 'test', '{}', '{}')`, ownSchool, own)
	assert.ErrorContains(t, err, "chk_push_outbox_one_scope", "a push never names both")
}
