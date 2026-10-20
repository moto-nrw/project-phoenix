package compose

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

// End-to-end check of the manual export transfer (#3050): the wired capability,
// a real SSH/SFTP counterpart, and a real audit row in PostgreSQL.
//
// The unit tests above cover the transport and the workflow with fakes. What
// only this test can show is that the pieces fit: that the journal row lands in
// audit.export_transfers on the caller's tenant transaction, under RLS, with
// the tenant it belongs to and without credentials.

// transferRow is what the journal actually stored.
type transferRow struct {
	bun.BaseModel   `bun:"table:export_transfers,alias:export_transfer"`
	TenantID        int64  `bun:"tenant_id"`
	ActorName       string `bun:"actor_name"`
	ExportKind      string `bun:"export_kind"`
	ExportFormat    string `bun:"export_format"`
	Filename        string `bun:"filename"`
	ByteSize        int64  `bun:"byte_size"`
	TargetHost      string `bun:"target_host"`
	TargetPort      int    `bun:"target_port"`
	TargetDirectory string `bun:"target_directory"`
	Status          string `bun:"status"`
	FailureReason   string `bun:"failure_reason"`
}

// buildModule wires the capability exactly as production does, with the one
// exception the test cannot avoid: the counterpart is on loopback, which the
// production address policy refuses on purpose.
func buildModule(t *testing.T, db *bun.DB, server sftptest.Server, dir string) exporttransfer.Capability {
	t.Helper()
	settings := staticSettings(server, dir)
	module, err := newModule(Dependencies{
		DB:       db,
		Settings: settings,
		Keys:     testSettingKeys(),
	}, sftp.WithAddressPolicy(sftptest.AllowLoopbackPolicy{}))
	require.NoError(t, err)
	return module
}

func testSettingKeys() SettingKeys {
	return SettingKeys{
		Host:               "sftp.host",
		Port:               "sftp.port",
		Username:           "sftp.username",
		Password:           "sftp.password",
		RemoteDirectory:    "sftp.remote_directory",
		HostKeyFingerprint: "sftp.host_key_fingerprint",
	}
}

// staticSettings stands in for the tenant settings. The settings system itself
// is covered by its own tests; what matters here is the wiring behind it.
func staticSettings(server sftptest.Server, dir string) Settings {
	str := func(value string) func(context.Context) (string, error) {
		return func(context.Context) (string, error) { return value, nil }
	}
	return Settings{
		Enabled:            func(context.Context) (bool, error) { return true, nil },
		Host:               str(server.Host),
		Port:               func(context.Context) (int, error) { return server.Port, nil },
		Username:           str(sftptest.User),
		Password:           str(sftptest.Password),
		RemoteDirectory:    str(dir),
		HostKeyFingerprint: str(server.Fingerprint),
	}
}

func transferRequest() exporttransfer.Request {
	return exporttransfer.Request{
		Kind:      exporttransfer.KindStaffTimeTracking,
		Format:    "csv",
		Filename:  "zeitkonten-2026-08.csv",
		Data:      []byte("Personalnummer;Lohnart;Stunden\r\n4711;100;38,50\r\n"),
		ActorName: "A. Beispiel",
	}
}

func readTransferRows(t *testing.T, ctx context.Context, db *bun.DB) []transferRow {
	t.Helper()
	var rows []transferRow
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		scoped, _, err := retainedDatabase()(txCtx)
		if err != nil {
			return err
		}
		// Read through the tenant role: the assertion is only meaningful if
		// RLS lets this tenant see its own rows and nobody else's.
		return scoped.NewSelect().
			Model(&rows).
			ModelTableExpr(`audit.export_transfers AS "export_transfer"`).
			OrderExpr(`"export_transfer".id DESC`).
			Scan(txCtx)
	}))
	return rows
}

func TestTransferDeliversTheFileAndRecordsIt(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	dir := t.TempDir()
	server := sftptest.Start(t)
	module := buildModule(t, db, server, dir)

	request := transferRequest()
	outcome, err := transferInTenantTx(t, ctx, module, request)
	require.NoError(t, err)
	require.True(t, outcome.Transferred, "reason: %s", outcome.Reason)

	// Die Datei liegt unter ihrem endgültigen Namen und ist bytegleich.
	written, err := os.ReadFile(filepath.Join(dir, request.Filename))
	require.NoError(t, err)
	assert.Equal(t, request.Data, written)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "no temporary file may remain")

	// Und die Übertragung steht im Protokoll — ohne Zugangsdaten.
	rows := readTransferRows(t, ctx, db)
	require.Len(t, rows, 1)
	row := rows[0]
	assert.Equal(t, "success", row.Status)
	assert.Empty(t, row.FailureReason)
	assert.Equal(t, exporttransfer.KindStaffTimeTracking, row.ExportKind)
	assert.Equal(t, "csv", row.ExportFormat)
	assert.Equal(t, request.Filename, row.Filename)
	assert.Equal(t, int64(len(request.Data)), row.ByteSize)
	assert.Equal(t, server.Host, row.TargetHost)
	assert.Equal(t, server.Port, row.TargetPort)
	assert.Equal(t, dir, row.TargetDirectory)
	assert.Equal(t, "A. Beispiel", row.ActorName)
}

func TestCommandTransactionRollbackNeverReplacesThePreviousRemoteFile(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	dir := t.TempDir()
	server := sftptest.Start(t)
	module := buildModule(t, db, server, dir)
	request := transferRequest()
	finalPath := filepath.Join(dir, request.Filename)
	require.NoError(t, os.WriteFile(finalPath, []byte("bisherige fassung"), 0o600))

	rollbackErr := errors.New("force command transaction rollback")
	outcome, err := module.Transfer(rollbackCommandContext(t, ctx, db, rollbackErr), request)
	require.ErrorIs(t, err, rollbackErr)
	assert.False(t, outcome.Transferred)

	written, err := os.ReadFile(finalPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("bisherige fassung"), written)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "the command rollback must leave no remote artifacts")
	assert.Empty(t, readTransferRows(t, ctx, db), "a rolled-back attempt must leave no audit row")
}

func TestCommandTransactionRollbackNeverCreatesANewRemoteFile(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	dir := t.TempDir()
	server := sftptest.Start(t)
	module := buildModule(t, db, server, dir)
	request := transferRequest()

	rollbackErr := errors.New("force command transaction rollback")
	outcome, err := module.Transfer(rollbackCommandContext(t, ctx, db, rollbackErr), request)
	require.ErrorIs(t, err, rollbackErr)
	assert.False(t, outcome.Transferred)

	_, err = os.Stat(filepath.Join(dir, request.Filename))
	assert.ErrorIs(t, err, os.ErrNotExist)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "rolling back an intent must never create remote artifacts")
	assert.Empty(t, readTransferRows(t, ctx, db), "a rolled-back attempt must leave no audit row")
}

func TestTransferWithoutATransactionDoesNotTouchTheRemote(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	dir := t.TempDir()
	server := sftptest.Start(t)
	module := buildModule(t, db, server, dir)

	outcome, err := module.Transfer(ctx, transferRequest())
	require.Error(t, err)
	assert.False(t, outcome.Transferred)
	entries, readErr := os.ReadDir(dir)
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

// A wrong host key is the failure that must never be smoothed over: nothing is
// written on the far side, and the attempt is recorded as a failure.
func TestTransferWithWrongHostKeyWritesNothingAndRecordsTheFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	dir := t.TempDir()
	server := sftptest.Start(t)
	server.Fingerprint = "SHA256:47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU"
	module := buildModule(t, db, server, dir)

	outcome, err := transferInTenantTx(t, ctx, module, transferRequest())
	require.NoError(t, err, "a refused counterpart is a normal answer, not a server error")
	assert.False(t, outcome.Transferred)
	assert.Equal(t, exporttransfer.ReasonHostKey, outcome.Reason)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "nothing may reach an unverified counterpart")

	rows := readTransferRows(t, ctx, db)
	require.Len(t, rows, 1)
	assert.Equal(t, "failed", rows[0].Status)
	assert.Equal(t, exporttransfer.ReasonHostKey, rows[0].FailureReason)
}

// Without a complete configuration nothing is attempted — and nothing is
// recorded, because no file went anywhere.
func TestTransferWithoutConfigurationAttemptsNothing(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	settings := staticSettings(sftptest.Server{Host: "dateien.beispiel.de", Port: 22}, "/upload")
	settings.HostKeyFingerprint = func(context.Context) (string, error) { return "", nil }
	module, err := newModule(Dependencies{DB: db, Settings: settings, Keys: testSettingKeys()})
	require.NoError(t, err)

	outcome, err := transferInTenantTx(t, ctx, module, transferRequest())
	require.NoError(t, err)
	assert.False(t, outcome.Transferred)
	assert.Equal(t, exporttransfer.ReasonNotConfigured, outcome.Reason)
	assert.Empty(t, readTransferRows(t, ctx, db))
}

// The production wiring keeps the public-only policy: a loopback counterpart is
// refused even though something is listening there.
func TestProductionWiringRefusesALoopbackCounterpart(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	dir := t.TempDir()
	server := sftptest.Start(t)
	module, err := New(Dependencies{
		DB:       db,
		Settings: staticSettings(server, dir),
		Keys:     testSettingKeys(),
	})
	require.NoError(t, err)

	outcome, err := transferInTenantTx(t, ctx, module, transferRequest())
	require.NoError(t, err)
	assert.False(t, outcome.Transferred)
	assert.Equal(t, exporttransfer.ReasonAddressDenied, outcome.Reason)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// transferInTenantTx runs the transfer inside a tenant transaction, the way
// TenantTxMiddleware wraps the HTTP request in production. Transfer owns its
// independent command transaction so transport acceptance is known on return.
func transferInTenantTx(
	t *testing.T,
	ctx context.Context,
	module exporttransfer.Capability,
	request exporttransfer.Request,
) (exporttransfer.Outcome, error) {
	t.Helper()
	var outcome exporttransfer.Outcome
	var transferErr error
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		outcome, transferErr = module.Transfer(txCtx, request)
		// The command has already committed its own intent and outcome.
		return nil
	})
	require.NoError(t, err)
	return outcome, transferErr
}

// Inject a failure at the command's real PostgreSQL commit boundary, after the
// intent INSERT succeeded. No callback or mock database simulates rollback.
func rollbackCommandContext(t *testing.T, ctx context.Context, db *bun.DB, rollbackErr error) context.Context {
	t.Helper()
	runtime, err := tenant.NewUnitOfWork(
		func(ctx context.Context, id int64, fn func(context.Context, any) error) error {
			return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
				if _, err := tx.NewRaw(`SET LOCAL ROLE phoenix_tenant`).Exec(ctx); err != nil {
					return err
				}
				if _, err := tx.NewRaw(`SELECT set_config('app.current_tenant_id',?,true)`, fmt.Sprint(id)).Exec(ctx); err != nil {
					return err
				}
				if err := fn(ctx, tx); err != nil {
					return err
				}
				return rollbackErr
			})
		}, func(context.Context, func(context.Context, any) error) error {
			return errors.New("unexpected admin transaction")
		},
		func(context.Context, tenant.SavepointAction) error { return nil }, func(error) bool { return false })
	require.NoError(t, err)
	ctx = tenant.WithUnitOfWork(ctx, runtime)
	ctx, _ = tenant.WithAfterCommitHooksForTest(ctx)
	return ctx
}
