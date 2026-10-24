package compose

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/sftp"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/sftp/sftptest"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type inspectingPolicy struct{ beforeConnect func() }

func (p inspectingPolicy) Allow(addr netip.Addr) bool {
	p.beforeConnect()
	return addr.IsLoopback()
}

func TestTransferCommitsItsEvidenceBeforeTouchingTheCounterpart(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	dir := t.TempDir()
	server := sftptest.Start(t)
	connections := 0
	request := auditedTransferRequest(t, db)
	finalPath := filepath.Join(dir, request.Filename)
	require.NoError(t, os.WriteFile(finalPath, []byte("previous export"), 0600))
	module, err := newModule(Dependencies{DB: db, Settings: staticSettings(server, dir), Keys: testSettingKeys()},
		sftp.WithAddressPolicy(inspectingPolicy{beforeConnect: func() {
			connections++
			previous, err := os.ReadFile(finalPath)
			require.NoError(t, err)
			assert.Equal(t, []byte("previous export"), previous)
			// A separate tenant transaction can only see committed evidence.
			assert.Equal(t, 1, accessAuditCount(t, ctx, db), "access audit must commit before connecting")
			rows := readTransferRows(t, ctx, db)
			require.Len(t, rows, 1, "no remote operation may precede the durable intent and audit")
			assert.Equal(t, "failed", rows[0].Status, "an unfinished attempt must never claim success")
		}}))
	require.NoError(t, err)
	outcome, err := transferInTenantTx(t, ctx, module, request)
	require.NoError(t, err)
	require.True(t, outcome.Transferred)
	assert.Equal(t, 1, connections)
	require.Len(t, readTransferRows(t, ctx, db), 1)
}

func dispatchState(t *testing.T, ctx context.Context, db *bun.DB) (int64, string) {
	t.Helper()
	var row struct {
		ID            int64
		DispatchState string
	}
	require.NoError(t, db.NewRaw(`SELECT id,dispatch_state FROM audit.export_transfers WHERE tenant_id=?`, testpkg.Tenant(t)).Scan(ctx, &row))
	return row.ID, row.DispatchState
}

func TestFailedTransferIsTerminalAndNeverReplacesThePreviousFile(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	dir := t.TempDir()
	server := sftptest.Start(t)
	server.Fingerprint = "SHA256:47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU"
	module := buildModule(t, db, server, dir)
	request := transferRequest()
	finalPath := filepath.Join(dir, request.Filename)
	require.NoError(t, os.WriteFile(finalPath, []byte("previous export"), 0600))
	outcome, err := transferInTenantTx(t, ctx, module, request)
	require.NoError(t, err)
	assert.False(t, outcome.Transferred)
	assert.Equal(t, "host_key_mismatch", outcome.Reason)
	id, state := dispatchState(t, ctx, db)
	assert.Equal(t, "terminal", state)
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		var token string
		tx, _, err := retainedDatabase()(txCtx)
		if err != nil {
			return err
		}
		if err := tx.NewRaw(`SELECT audit.claim_export_transfer(?,?)`, testpkg.Tenant(t), id).Scan(txCtx, &token); err != nil {
			return err
		}
		assert.Empty(t, token, "a terminal failure must never be claimed again")
		return nil
	}))
	settled, err := NewDispatchRecovery()(ctx, 25)
	require.NoError(t, err)
	assert.Zero(t, settled)
	previous, err := os.ReadFile(finalPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("previous export"), previous)
	rows := readTransferRows(t, ctx, db)
	require.Len(t, rows, 1)
	assert.Equal(t, "failed", rows[0].Status)
	assert.Equal(t, "host_key_mismatch", rows[0].FailureReason)
}

func TestCrashAfterCommitSettlesUnknownWithoutReupload(t *testing.T) {
	t.Parallel()
	for _, where := range []string{"before_claim", "before_upload"} {
		t.Run(where, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupIsolatedTestDB(t)
			ctx := testpkg.Ctx(t)
			dir := t.TempDir()
			server := sftptest.Start(t)
			request := transferRequest()
			finalPath := filepath.Join(dir, request.Filename)
			require.NoError(t, os.WriteFile(finalPath, []byte("previous export"), 0600))
			connects := 0
			module, err := newModule(Dependencies{DB: db, Settings: staticSettings(server, dir), Keys: testSettingKeys()},
				sftp.WithAddressPolicy(inspectingPolicy{beforeConnect: func() { connects++; panic("crash before upload") }}))
			require.NoError(t, err)
			commandCtx, _ := tenant.WithAfterCommitHooksForTest(ctx)
			if where == "before_claim" {
				commandCtx = tenant.WithUnitOfWorkObserver(commandCtx, func(event tenant.UnitOfWorkEvent) {
					if event.Kind == tenant.UnitOfWorkTransaction && event.Result == tenant.UnitOfWorkCommitted {
						panic("crash after intent commit")
					}
				})
			}
			require.Panics(t, func() { _, _ = module.Transfer(commandCtx, request) })
			id, state := dispatchState(t, ctx, db)
			expected := "dispatching"
			if where == "before_claim" {
				expected = "pending"
			}
			assert.Equal(t, expected, state)
			// Recovery must not steal an unexpired foreground dispatch.
			n, err := NewDispatchRecovery()(ctx, 25)
			require.NoError(t, err)
			assert.Zero(t, n)
			_, err = db.NewRaw(`UPDATE audit.export_transfers SET dispatch_expires_at=clock_timestamp()-INTERVAL '1 second' WHERE id=?`, id).Exec(ctx)
			require.NoError(t, err)
			n, err = NewDispatchRecovery()(ctx, 25)
			require.NoError(t, err)
			assert.Equal(t, 1, n)
			_, state = dispatchState(t, ctx, db)
			assert.Equal(t, "unknown", state)
			n, err = NewDispatchRecovery()(ctx, 25)
			require.NoError(t, err)
			assert.Zero(t, n)
			expectedConnects := 1
			if where == "before_claim" {
				expectedConnects = 0
			}
			assert.Equal(t, expectedConnects, connects, "recovery must never dial the counterpart")
			previous, err := os.ReadFile(finalPath)
			require.NoError(t, err)
			assert.Equal(t, []byte("previous export"), previous)
			rows := readTransferRows(t, ctx, db)
			require.Len(t, rows, 1)
			assert.Equal(t, "failed", rows[0].Status)
			assert.Equal(t, "internal_error", rows[0].FailureReason)
		})
	}
}

// The command's accepted upload and its committed evidence cannot be erased
// by later response middleware failure. This replaces compensation by an
// explicit, independently committed command boundary.
func TestAcceptedTransferSurvivesOuterResponseTransactionRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	dir := t.TempDir()
	server := sftptest.Start(t)
	module := buildModule(t, db, server, dir)
	request := auditedTransferRequest(t, db)
	laterFailure := errors.New("response transaction failed after accepted command")
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		outcome, err := module.Transfer(txCtx, request)
		require.NoError(t, err)
		require.True(t, outcome.Transferred)
		return laterFailure
	})
	require.ErrorIs(t, err, laterFailure)
	contents, err := os.ReadFile(filepath.Join(dir, request.Filename))
	require.NoError(t, err)
	assert.Equal(t, request.Data, contents)
	rows := readTransferRows(t, ctx, db)
	require.Len(t, rows, 1)
	assert.Equal(t, "success", rows[0].Status)
	assert.Equal(t, 1, accessAuditCount(t, ctx, db))
}

func auditedTransferRequest(t *testing.T, db *bun.DB) exporttransfer.Request {
	t.Helper()
	account := testpkg.CreateTestAccount(t, db, "transfer@example.test")
	request := transferRequest()
	request.ActorAccountID = account.ID
	request.PrepareFile = func(ctx context.Context) (string, []byte, error) {
		tx, tenantID, err := retainedDatabase()(ctx)
		if err != nil {
			return "", nil, err
		}
		_, err = tx.NewRaw(`INSERT INTO audit.data_access_log
			(tenant_id,actor_account_id,actor_role,resource_type,range_start,range_end)
			VALUES (?,?,'staff','time_tracking_export',now(),now())`, tenantID, account.ID).Exec(ctx)
		return request.Filename, request.Data, err
	}
	return request
}

func accessAuditCount(t *testing.T, ctx context.Context, db *bun.DB) int {
	t.Helper()
	n, err := db.NewSelect().TableExpr("audit.data_access_log").Where("tenant_id = ?", testpkg.Tenant(t)).Count(ctx)
	require.NoError(t, err)
	return int(n)
}

func TestIntentRollbackAlsoRollsBackAccessAudit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	dir := t.TempDir()
	module := buildModule(t, db, sftptest.Start(t), dir)
	rollbackErr := errors.New("intent commit failed")
	_, err := module.Transfer(rollbackCommandContext(t, ctx, db, rollbackErr), auditedTransferRequest(t, db))
	require.ErrorIs(t, err, rollbackErr)
	assert.Zero(t, accessAuditCount(t, ctx, db))
	assert.Empty(t, readTransferRows(t, ctx, db))
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestPreparedUnconfiguredTransferPreservesOriginalResult(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	settings := staticSettings(sftptest.Server{Host: "counterpart.example", Port: 22}, "/exports")
	module, err := newModule(Dependencies{DB: db, Settings: settings, Keys: testSettingKeys()})
	require.NoError(t, err)
	request := auditedTransferRequest(t, db)
	expectedFilename := request.Filename
	// Match the HTTP producer-only request, rather than an already-built file.
	request.Filename = ""
	request.Data = nil
	outcome, err := transferInTenantTx(t, ctx, module, request)
	require.NoError(t, err)
	assert.Equal(t, "not_configured", outcome.Reason)
	assert.False(t, outcome.Transferred)
	assert.Zero(t, outcome.ByteSize)
	assert.Empty(t, outcome.TargetHost)
	assert.Empty(t, outcome.TargetDirectory)
	assert.Equal(t, expectedFilename, outcome.Filename)
	assert.Equal(t, 1, accessAuditCount(t, ctx, db))
	assert.Empty(t, readTransferRows(t, ctx, db))
}

func TestPreparedTransferPreservesProducerErrorPrecedence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	settings := staticSettings(sftptest.Server{Host: "counterpart.example", Port: 22}, "/exports")
	settingsErr := errors.New("settings unavailable")
	settings.Enabled = func(context.Context) (bool, error) { return false, settingsErr }
	module, err := newModule(Dependencies{DB: db, Settings: settings, Keys: testSettingKeys()})
	require.NoError(t, err)
	producerErr := errors.New("export selection rejected")
	request := transferRequest()
	request.Filename = ""
	request.Data = nil
	request.PrepareFile = func(context.Context) (string, []byte, error) { return "", nil, producerErr }
	_, err = transferInTenantTx(t, ctx, module, request)
	require.ErrorIs(t, err, producerErr)
	assert.NotErrorIs(t, err, settingsErr)
	assert.Empty(t, readTransferRows(t, ctx, db))
	assert.Zero(t, accessAuditCount(t, ctx, db))
}
