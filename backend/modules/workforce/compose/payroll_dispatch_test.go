package compose

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type payrollTestSettings struct {
	config workforce.PayrollDispatchConfig
}

func (s payrollTestSettings) PayrollDispatchConfig(context.Context) (workforce.PayrollDispatchConfig, error) {
	return s.config, nil
}

type payrollTestExport struct{}

func (payrollTestExport) ExportStaffTime(_ context.Context, request workforce.TimeExportRequest, _ int64, _ string) (*workforce.ExportFile, error) {
	return &workforce.ExportFile{Data: []byte("Zeiten"), Filename: "zeiterfassung_alle_2026-09." + request.Format, ContentType: "text/csv"}, nil
}

func (payrollTestExport) DatevReport(context.Context, workforce.TimeExportRequest) (*workforce.DatevExportReport, error) {
	return &workforce.DatevExportReport{}, nil
}

type payrollTestTransfer struct {
	mu   sync.Mutex
	sent []string
}

func (t *payrollTestTransfer) Transfer(ctx context.Context, request workforce.PayrollTransferRequest) (workforce.PayrollTransferOutcome, error) {
	name, _, err := request.Prepare(ctx)
	if err != nil {
		return workforce.PayrollTransferOutcome{}, err
	}
	t.mu.Lock()
	t.sent = append(t.sent, name)
	t.mu.Unlock()
	return workforce.PayrollTransferOutcome{Transferred: true}, nil
}

func (*payrollTestTransfer) RecordEmail(context.Context, workforce.PayrollEmailResult) error {
	return nil
}

type payrollTestMail struct {
	mu       sync.Mutex
	links    []workforce.PayrollMailMessage
	problems []workforce.PayrollMailMessage
}

func (m *payrollTestMail) Enqueue(_ context.Context, message workforce.PayrollMailMessage) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if message.Type == "payroll_dispatch_problem" {
		m.problems = append(m.problems, message)
	} else {
		m.links = append(m.links, message)
	}
	return int64(len(m.links) + len(m.problems)), nil
}

func (*payrollTestMail) State(context.Context, int64) (string, error) {
	return workforce.PayrollMailSent, nil
}

type payrollTestSchools struct{}

func (payrollTestSchools) School(context.Context, int64) (workforce.PayrollSchool, error) {
	return workforce.PayrollSchool{Name: "OGS", Subdomain: "ogs"}, nil
}

type payrollTestManagers struct{}

func (payrollTestManagers) TimeTrackingManagers(context.Context) ([]workforce.PayrollContact, error) {
	return []workforce.PayrollContact{{StaffID: 31, FirstName: "Lea", LastName: "Leitung", Email: "lea@ogs.example"}}, nil
}

type payrollTestTokens struct{}

func (payrollTestTokens) NewToken() (string, string, error) {
	return "secret-link", "fp:secret-link", nil
}
func (payrollTestTokens) Fingerprint(raw string) string { return "fp:" + raw }

func newPayrollTestDispatch(t *testing.T, config workforce.PayrollDispatchConfig, clock ...func() time.Time) (workforce.PayrollDispatch, *payrollTestTransfer, *payrollTestMail) {
	t.Helper()
	var now func() time.Time
	if len(clock) > 0 {
		now = clock[0]
	}
	transfer, mail := &payrollTestTransfer{}, &payrollTestMail{}
	dispatch, err := NewPayrollDispatch(testpkg.SetupTestDB(t), workforce.PayrollDispatchPorts{
		Settings: payrollTestSettings{config: config}, Export: payrollTestExport{},
		Transfer: transfer, Mail: mail, Schools: payrollTestSchools{}, Managers: payrollTestManagers{},
		Tokens: payrollTestTokens{}, FrontendURL: "https://moto.example", Now: now,
	})
	require.NoError(t, err)
	return dispatch, transfer, mail
}

// overviewIn reads the overview in the request transaction it runs in.
func overviewIn(t *testing.T, ctx context.Context, dispatch workforce.PayrollDispatch) workforce.PayrollDispatchOverview {
	t.Helper()
	var overview workforce.PayrollDispatchOverview
	require.NoError(t, testpkg.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		var err error
		overview, err = dispatch.Overview(txCtx)
		return err
	}))
	return overview
}

func closeSeptember(t *testing.T, ctx context.Context, dispatch workforce.PayrollDispatch) {
	t.Helper()
	require.NoError(t, testpkg.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		return dispatch.QueueAfterMonthClose(txCtx, workforce.PayrollDispatchTrigger{
			Year: 2026, Month: 9, ClosedStaff: 4, ActorAccountID: 31, ActorName: "Lea Leitung",
		})
	}))
}

// TestPayrollDispatchStaysInsideItsSchool pins the tenant boundary of #4805:
// another school's worker pass neither sees nor sends the dispatch, and a
// download link only opens in the school it was issued for.
func TestPayrollDispatchStaysInsideItsSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	otherID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherID)
	other := testpkg.TenantContext(otherID)

	sftp, transfer, _ := newPayrollTestDispatch(t, workforce.PayrollDispatchConfig{Auto: true, Channel: workforce.PayrollChannelSFTP, Format: "csv"})
	closeSeptember(t, ctx, sftp)

	finished, err := sftp.RunDue(other)
	require.NoError(t, err)
	assert.Zero(t, finished, "another school's pass finds nothing")
	assert.Empty(t, transfer.sent)
	overview := overviewIn(t, other, sftp)
	assert.Nil(t, overview.Latest)

	finished, err = sftp.RunDue(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, finished)
	assert.Equal(t, []string{"zeiterfassung_alle_2026-09.csv"}, transfer.sent)
	overview = overviewIn(t, ctx, sftp)
	require.NotNil(t, overview.Latest)
	assert.Equal(t, workforce.PayrollDispatchSent, overview.Latest.State)
	assert.False(t, overview.Latest.Correction)

	finished, err = sftp.RunDue(ctx)
	require.NoError(t, err)
	assert.Zero(t, finished, "a sent dispatch is not sent twice")
	assert.Len(t, transfer.sent, 1)
}

// TestPayrollDispatchLinkOpensOnceInItsSchool runs the e-mail way against
// the database: the file waits behind the link, the link fails in another
// school, opens once in its own and is spent afterwards.
func TestPayrollDispatchLinkOpensOnceInItsSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.TenantIDFromContext(ctx)
	otherID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherID)

	// The public route carries the runtime but no school: the link names it.
	public := testpkg.WithTestTenantRuntime(t, context.Background())
	later := time.Duration(0)
	dispatch, _, mail := newPayrollTestDispatch(t, workforce.PayrollDispatchConfig{
		Auto: true, Channel: workforce.PayrollChannelEmail, Format: "xlsx", Email: "lohn@steuerbuero-beispiel.de",
	}, func() time.Time { return time.Now().Add(later) })
	closeSeptember(t, ctx, dispatch)
	_, err := dispatch.RunDue(ctx)
	require.NoError(t, err)
	require.Len(t, mail.links, 1)
	assert.Equal(t, tenantID, mail.links[0].TenantID)

	var stored int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM active.payroll_dispatches
		WHERE tenant_id = ? AND download_token_hash = 'fp:secret-link' AND file_data IS NOT NULL`, tenantID).Scan(ctx, &stored))
	assert.Equal(t, 1, stored, "only the fingerprint of the link is stored")

	_, err = dispatch.Download(public, otherID, "secret-link")
	require.ErrorIs(t, err, workforce.ErrPayrollDownloadUnavailable, "the link does not open in another school")

	file, err := dispatch.Download(public, tenantID, "secret-link")
	require.NoError(t, err)
	assert.Equal(t, "zeiterfassung_alle_2026-09.xlsx", file.Filename)
	assert.Equal(t, []byte("Zeiten"), file.Data)

	_, err = dispatch.Download(public, tenantID, "secret-link")
	require.ErrorIs(t, err, workforce.ErrPayrollDownloadUnavailable, "a link opens once")
	var cleared int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM active.payroll_dispatches
		WHERE tenant_id = ? AND downloaded_at IS NOT NULL AND file_data IS NULL`, tenantID).Scan(ctx, &cleared))
	assert.Equal(t, 1, cleared, "the download is recorded and the file is gone")

	later = 2 * time.Minute // the mail's result is read on the next pass
	_, err = dispatch.RunDue(ctx)
	require.NoError(t, err)
	overview := overviewIn(t, ctx, dispatch)
	require.NotNil(t, overview.Latest)
	assert.Equal(t, workforce.PayrollDispatchSent, overview.Latest.State)
	assert.NotNil(t, overview.Latest.DownloadedAt)
}

// TestPayrollDispatchNumbersCorrections pins that a second close of the same
// month queues a correction, and that a rolled-back close queues nothing.
func TestPayrollDispatchNumbersCorrections(t *testing.T) {
	t.Parallel()
	ctx := testpkg.Ctx(t)
	dispatch, transfer, _ := newPayrollTestDispatch(t, workforce.PayrollDispatchConfig{Auto: true, Channel: workforce.PayrollChannelSFTP, Format: "csv"})

	rollback := context.Canceled
	err := testpkg.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if err := dispatch.QueueAfterMonthClose(txCtx, workforce.PayrollDispatchTrigger{
			Year: 2026, Month: 9, ClosedStaff: 4, ActorAccountID: 31,
		}); err != nil {
			return err
		}
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	overview := overviewIn(t, ctx, dispatch)
	assert.Nil(t, overview.Latest, "the dispatch commits with the close or not at all")

	closeSeptember(t, ctx, dispatch)
	_, err = dispatch.RunDue(ctx)
	require.NoError(t, err)
	closeSeptember(t, ctx, dispatch)
	_, err = dispatch.RunDue(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"zeiterfassung_alle_2026-09.csv", "zeiterfassung_alle_2026-09_korrektur-1.csv"}, transfer.sent)
	overview = overviewIn(t, ctx, dispatch)
	require.NotNil(t, overview.Latest)
	assert.True(t, overview.Latest.Correction)
}

func TestPayrollDispatchReservesCorrectionNumbersForQueuedFiles(t *testing.T) {
	t.Parallel()

	ctx := testpkg.Ctx(t)
	dispatch, transfer, _ := newPayrollTestDispatch(t, workforce.PayrollDispatchConfig{Auto: true, Channel: workforce.PayrollChannelSFTP, Format: "csv"})

	closeSeptember(t, ctx, dispatch)
	_, err := dispatch.RunDue(ctx)
	require.NoError(t, err)
	closeSeptember(t, ctx, dispatch)
	closeSeptember(t, ctx, dispatch)

	_, err = dispatch.RunDue(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"zeiterfassung_alle_2026-09.csv",
		"zeiterfassung_alle_2026-09_korrektur-1.csv",
		"zeiterfassung_alle_2026-09_korrektur-2.csv",
	}, transfer.sent)

	overview := overviewIn(t, ctx, dispatch)
	require.NotNil(t, overview.Latest)
	assert.True(t, overview.Latest.Correction)
}

func TestPayrollDispatchBlockedNoticeReferencesStoredDispatch(t *testing.T) {
	t.Parallel()

	ctx := testpkg.Ctx(t)
	dispatch, _, mail := newPayrollTestDispatch(t, workforce.PayrollDispatchConfig{Auto: true, Channel: workforce.PayrollChannelEmail, Format: "csv"})
	closeSeptember(t, ctx, dispatch)

	require.Len(t, mail.problems, 1)
	assert.Positive(t, mail.problems[0].ReferenceID)
	var storedID int64
	require.NoError(t, testpkg.SetupTestDB(t).NewRaw(`SELECT id FROM active.payroll_dispatches WHERE tenant_id = ?`, testpkg.TenantIDFromContext(ctx)).Scan(ctx, &storedID))
	assert.Equal(t, storedID, mail.problems[0].ReferenceID)
}
