package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const payrollTestTenant = int64(7)

var payrollTestNow = time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)

// fakePayrollStore keeps the dispatches of one tenant in memory and applies
// the lease fence the way the postgres store does.
type fakePayrollStore struct {
	rows    []domain.PayrollDispatch
	nextID  int64
	lease   int
	purged  bool
	opened  []string
	failFin bool
}

func (f *fakePayrollStore) NextPayrollDispatchSequence(_ context.Context, year, month int) (int, error) {
	next := 0
	for _, row := range f.rows {
		if row.Year == year && row.Month == month && row.Sequence >= next {
			next = row.Sequence + 1
		}
	}
	return next, nil
}

func (f *fakePayrollStore) InsertPayrollDispatch(_ context.Context, value domain.PayrollDispatch) error {
	f.nextID++
	value.ID = f.nextID
	value.CreatedAt = payrollTestNow
	f.rows = append(f.rows, value)
	return nil
}

func (f *fakePayrollStore) LatestPayrollDispatch(context.Context) (*domain.PayrollDispatch, error) {
	if len(f.rows) == 0 {
		return nil, nil
	}
	latest := f.rows[len(f.rows)-1]
	return &latest, nil
}

func (f *fakePayrollStore) ClaimPayrollDispatches(_ context.Context, now time.Time, _ time.Duration, limit int) ([]domain.PayrollDispatch, error) {
	var claims []domain.PayrollDispatch
	for i := range f.rows {
		row := &f.rows[i]
		if (row.State == workforce.PayrollDispatchPending || row.State == workforce.PayrollDispatchMailing) &&
			!row.NextAttemptAt.After(now) && row.LeaseToken == "" && len(claims) < limit {
			f.lease++
			row.LeaseToken = string(rune('a' + f.lease))
			claim := *row
			claim.FileData = nil
			claims = append(claims, claim)
		}
	}
	return claims, nil
}

func (f *fakePayrollStore) FinishPayrollDispatch(_ context.Context, value domain.PayrollDispatch, _ time.Time) (bool, error) {
	if f.failFin {
		return false, errors.New("database unavailable")
	}
	for i := range f.rows {
		row := &f.rows[i]
		if row.ID != value.ID || row.LeaseToken != value.LeaseToken {
			continue
		}
		file := row.FileData
		if value.FileData != nil {
			file = value.FileData
		} else if value.State != workforce.PayrollDispatchMailing && value.State != workforce.PayrollDispatchSent {
			file = nil
		}
		value.FileData, value.LeaseToken = file, ""
		*row = value
		return true, nil
	}
	return false, nil
}

func (f *fakePayrollStore) OpenPayrollDownload(_ context.Context, hash string, now time.Time) (*domain.PayrollDispatch, error) {
	f.opened = append(f.opened, hash)
	for i := range f.rows {
		row := &f.rows[i]
		if row.DownloadTokenHash == hash && row.DownloadedAt == nil && row.FileData != nil && row.DownloadExpiresAt.After(now) {
			opened := *row
			row.DownloadedAt, row.FileData = &now, nil
			return &opened, nil
		}
	}
	return nil, nil
}

func (f *fakePayrollStore) PurgePayrollDispatches(context.Context, time.Time, time.Time) error {
	f.purged = true
	return nil
}

type fakePayrollSettings struct {
	config PayrollDispatchConfig
	err    error
}

func (f fakePayrollSettings) PayrollDispatchConfig(context.Context) (PayrollDispatchConfig, error) {
	return f.config, f.err
}

type fakePayrollExport struct {
	report    *workforce.DatevExportReport
	reportErr error
	exportErr error
	exports   []int64
}

func (f *fakePayrollExport) ExportStaffTime(_ context.Context, request workforce.TimeExportRequest, actor int64, _ string) (*workforce.ExportFile, error) {
	if f.exportErr != nil {
		return nil, f.exportErr
	}
	f.exports = append(f.exports, actor)
	name := "zeiterfassung_alle_2026-09.csv"
	if request.Format == "datev_lodas" {
		name = "datev_lodas_2026-09.txt"
	}
	return &workforce.ExportFile{Data: []byte("Personalnummer;Stunden"), Filename: name, ContentType: "text/csv"}, nil
}

func (f *fakePayrollExport) DatevReport(context.Context, workforce.TimeExportRequest) (*workforce.DatevExportReport, error) {
	if f.reportErr != nil {
		return nil, f.reportErr
	}
	if f.report == nil {
		return &workforce.DatevExportReport{}, nil
	}
	return f.report, nil
}

type fakePayrollTransfer struct {
	outcomes []PayrollTransferOutcome
	sent     []string
	periods  []string
	emails   []PayrollEmailResult
}

func (f *fakePayrollTransfer) Transfer(ctx context.Context, request PayrollTransferRequest) (PayrollTransferOutcome, error) {
	outcome := PayrollTransferOutcome{Transferred: true}
	if len(f.outcomes) > 0 {
		outcome, f.outcomes = f.outcomes[0], f.outcomes[1:]
	}
	if outcome.NotConfigured {
		return outcome, nil
	}
	filename, _, err := request.Prepare(ctx)
	if err != nil {
		return PayrollTransferOutcome{}, err
	}
	f.sent = append(f.sent, filename)
	f.periods = append(f.periods, request.Period)
	return outcome, nil
}

func (f *fakePayrollTransfer) RecordEmail(_ context.Context, result PayrollEmailResult) error {
	f.emails = append(f.emails, result)
	return nil
}

type fakePayrollMail struct {
	links    []PayrollMailMessage
	state    string
	problems []PayrollMailMessage
}

func (f *fakePayrollMail) Enqueue(_ context.Context, message PayrollMailMessage) (int64, error) {
	if message.Type == PayrollLinkMailType {
		f.links = append(f.links, message)
	} else {
		f.problems = append(f.problems, message)
	}
	return 900 + int64(len(f.links)+len(f.problems)), nil
}

func (f *fakePayrollMail) State(context.Context, int64) (string, error) { return f.state, nil }

type fakePayrollSchools struct{}

func (fakePayrollSchools) School(context.Context, int64) (PayrollSchool, error) {
	return PayrollSchool{Name: "OGS Am Park", Subdomain: "park"}, nil
}

type fakePayrollManagers struct{}

func (fakePayrollManagers) TimeTrackingManagers(context.Context) ([]PayrollContact, error) {
	return []PayrollContact{
		{StaffID: 3, FirstName: "Lea", LastName: "Leitung", Email: "lea@ogs-am-park.de"},
		{StaffID: 4, FirstName: "Ohne", LastName: "Adresse"},
	}, nil
}

type fakePayrollTokens struct{}

func (fakePayrollTokens) NewToken() (string, string, error) {
	return "raw-token", "hash:raw-token", nil
}
func (fakePayrollTokens) Fingerprint(raw string) string { return "hash:" + raw }

type payrollHarness struct {
	service  *PayrollDispatch
	store    *fakePayrollStore
	export   *fakePayrollExport
	transfer *fakePayrollTransfer
	mail     *fakePayrollMail
	now      *time.Time
}

func newPayrollHarness(t *testing.T, config PayrollDispatchConfig) payrollHarness {
	t.Helper()
	now := payrollTestNow
	h := payrollHarness{store: &fakePayrollStore{}, export: &fakePayrollExport{}, transfer: &fakePayrollTransfer{},
		mail: &fakePayrollMail{state: PayrollMailPending}, now: &now}
	inTx := func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
	service, err := NewPayrollDispatch(PayrollDispatchDependencies{
		Store: h.store, Settings: fakePayrollSettings{config: config}, Export: h.export,
		Transfer: h.transfer, Mail: h.mail, Schools: fakePayrollSchools{}, Managers: fakePayrollManagers{},
		Tokens: fakePayrollTokens{}, FrontendURL: "https://moto.example/",
		Now: func() time.Time { return *h.now },
		Runtime: PayrollRuntime{
			TenantID: func(context.Context) (int64, error) { return payrollTestTenant, nil },
			Within:   inTx,
			WithinTenant: func(ctx context.Context, id int64, fn func(context.Context) error) error {
				if id != payrollTestTenant {
					return fn(ctx) // another school: the fake holds no rows of it
				}
				return fn(ctx)
			},
			Detached: func(ctx context.Context) context.Context { return ctx },
		},
	})
	require.NoError(t, err)
	h.service = service
	return h
}

func (h payrollHarness) close(t *testing.T) {
	t.Helper()
	require.NoError(t, h.service.QueueAfterMonthClose(context.Background(), workforce.PayrollDispatchTrigger{
		Year: 2026, Month: 9, ClosedStaff: 12, ActorAccountID: 31, ActorName: "Lea Leitung",
	}))
}

func (h payrollHarness) run(t *testing.T) int {
	t.Helper()
	finished, err := h.service.RunDue(context.Background())
	require.NoError(t, err)
	return finished
}

func sftpConfig(format string) PayrollDispatchConfig {
	return PayrollDispatchConfig{Auto: true, Channel: workforce.PayrollChannelSFTP, Format: format}
}

func TestPayrollDispatch_NothingQueuedWithoutTheSwitchOrAClosedPerson(t *testing.T) {
	t.Parallel()

	off := newPayrollHarness(t, PayrollDispatchConfig{Auto: false, Channel: "sftp", Format: "csv"})
	off.close(t)
	assert.Empty(t, off.store.rows, "the switch is off: nothing leaves the school")

	on := newPayrollHarness(t, sftpConfig("csv"))
	require.NoError(t, on.service.QueueAfterMonthClose(context.Background(), workforce.PayrollDispatchTrigger{
		Year: 2026, Month: 9, ClosedStaff: 0, ActorAccountID: 31,
	}))
	assert.Empty(t, on.store.rows, "a close that froze nobody changed nothing to send")
}

func TestPayrollDispatch_SettingsErrorFailsTheClose(t *testing.T) {
	t.Parallel()

	h := newPayrollHarness(t, sftpConfig("csv"))
	h.service.deps.Settings = fakePayrollSettings{err: errors.New("settings unavailable")}
	err := h.service.QueueAfterMonthClose(context.Background(), workforce.PayrollDispatchTrigger{
		Year: 2026, Month: 9, ClosedStaff: 3, ActorAccountID: 31,
	})
	require.Error(t, err, "an unreadable switch must not silently skip the payroll file")
	assert.Empty(t, h.store.rows)
}

func TestPayrollDispatch_SendsAfterTheCloseBySFTP(t *testing.T) {
	t.Parallel()

	h := newPayrollHarness(t, sftpConfig("csv"))
	h.close(t)
	require.Len(t, h.store.rows, 1)
	assert.Equal(t, workforce.PayrollDispatchPending, h.store.rows[0].State)

	assert.Equal(t, 1, h.run(t))
	row := h.store.rows[0]
	assert.Equal(t, workforce.PayrollDispatchSent, row.State)
	assert.Equal(t, 1, row.Attempts)
	assert.Equal(t, []string{"zeiterfassung_alle_2026-09.csv"}, h.transfer.sent)
	assert.Equal(t, []string{"2026-09"}, h.transfer.periods)
	assert.Equal(t, []int64{31}, h.export.exports, "the access audit names the person who closed the month")
	assert.Empty(t, h.mail.problems)
	assert.True(t, h.store.purged)
}

func TestPayrollDispatch_CorrectionCarriesTheHintInTheFilename(t *testing.T) {
	t.Parallel()

	h := newPayrollHarness(t, sftpConfig("datev_lodas"))
	h.close(t)
	h.run(t)
	// One person's month is reopened and closed again.
	require.NoError(t, h.service.QueueAfterMonthClose(context.Background(), workforce.PayrollDispatchTrigger{
		Year: 2026, Month: 9, ClosedStaff: 1, ActorAccountID: 31,
	}))
	h.run(t)

	assert.Equal(t, []string{"datev_lodas_2026-09.txt", "datev_lodas_2026-09_korrektur-1.txt"}, h.transfer.sent)
	overview, err := h.service.Overview(context.Background())
	require.NoError(t, err)
	require.NotNil(t, overview.Latest)
	assert.True(t, overview.Latest.Correction)
	assert.Equal(t, workforce.PayrollDispatchSent, overview.Latest.State)
}

func TestPayrollDispatch_IncompleteDatevBlocksAndNotifies(t *testing.T) {
	t.Parallel()

	for name, export := range map[string]*fakePayrollExport{
		"missing DATEV client":     {reportErr: workforce.ErrPayrollConfigIncomplete},
		"staff without number":     {report: &workforce.DatevExportReport{StaffSkipped: []workforce.DatevSkippedStaff{{LastName: "Muster"}}}},
		"Lohnart without a number": {report: &workforce.DatevExportReport{UnconfiguredCategories: []string{"krank"}}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newPayrollHarness(t, sftpConfig("datev_lug"))
			h.export = export
			h.service.deps.Export = export
			h.close(t)

			assert.Equal(t, 1, h.run(t))
			row := h.store.rows[0]
			assert.Equal(t, workforce.PayrollDispatchBlocked, row.State)
			assert.Equal(t, workforce.PayrollReasonDatevIncomplete, row.Reason)
			assert.Empty(t, h.transfer.sent, "nothing is sent")
			assert.Empty(t, export.exports, "no file is even built")
			require.Len(t, h.mail.problems, 1)
			assert.Equal(t, payrollReasonText(workforce.PayrollReasonDatevIncomplete), h.mail.problems[0].Content["ReasonText"])
		})
	}
}

func TestPayrollDispatch_UnconfiguredSFTPBlocks(t *testing.T) {
	t.Parallel()

	h := newPayrollHarness(t, sftpConfig("csv"))
	h.transfer.outcomes = []PayrollTransferOutcome{{NotConfigured: true}}
	h.close(t)
	h.run(t)

	assert.Equal(t, workforce.PayrollDispatchBlocked, h.store.rows[0].State)
	assert.Equal(t, workforce.PayrollReasonSFTPNotConfigured, h.store.rows[0].Reason)
	require.Len(t, h.mail.problems, 1)
}

func TestPayrollDispatch_RetriesThreeTimesThenNotifies(t *testing.T) {
	t.Parallel()

	h := newPayrollHarness(t, sftpConfig("csv"))
	failure := PayrollTransferOutcome{Reason: "connection_failed"}
	h.transfer.outcomes = []PayrollTransferOutcome{failure, failure, failure, failure}
	h.close(t)

	waits := []time.Duration{5 * time.Minute, 30 * time.Minute, 2 * time.Hour}
	for attempt, wait := range waits {
		assert.Zero(t, h.run(t), "attempt %d fails but is retried", attempt+1)
		row := h.store.rows[0]
		assert.Equal(t, workforce.PayrollDispatchPending, row.State)
		assert.Equal(t, attempt+1, row.Attempts)
		assert.Equal(t, "connection_failed", row.Reason)
		assert.Equal(t, h.now.Add(wait), row.NextAttemptAt)
		assert.Zero(t, h.run(t), "not due before the wait is over")
		*h.now = row.NextAttemptAt
	}
	assert.Empty(t, h.mail.problems, "nobody is bothered while retries remain")

	assert.Equal(t, 1, h.run(t))
	row := h.store.rows[0]
	assert.Equal(t, workforce.PayrollDispatchFailed, row.State)
	assert.Equal(t, workforce.PayrollDispatchMaxAttempts, row.Attempts)
	require.Len(t, h.mail.problems, 1, "one notice per manager with an address")
	notice := h.mail.problems[0]
	assert.Equal(t, payrollTestTenant, notice.TenantID)
	assert.Equal(t, "lea@ogs-am-park.de", notice.ToAddress)
	assert.Equal(t, "Monatsdatei September 2026 nicht verschickt", notice.Subject)
	assert.Equal(t, "Die Übertragung an das Lohnbüro hat viermal nicht geklappt.", notice.Content["ReasonText"])
	assert.Equal(t, "https://park.moto.example/payroll", notice.Content["LinkURL"])
}

func TestPayrollDispatch_SecondAttemptSucceeds(t *testing.T) {
	t.Parallel()

	h := newPayrollHarness(t, sftpConfig("csv"))
	h.transfer.outcomes = []PayrollTransferOutcome{{Reason: "upload_failed"}, {Transferred: true}}
	h.close(t)
	h.run(t)
	*h.now = h.store.rows[0].NextAttemptAt
	assert.Equal(t, 1, h.run(t))
	assert.Equal(t, workforce.PayrollDispatchSent, h.store.rows[0].State)
	assert.Empty(t, h.store.rows[0].Reason, "a sent dispatch carries no failure")
	assert.Empty(t, h.mail.problems)
}

func emailConfig(address string) PayrollDispatchConfig {
	return PayrollDispatchConfig{Auto: true, Channel: workforce.PayrollChannelEmail, Format: "csv", Email: address}
}

func TestPayrollDispatch_EmailSendsALinkNotTheFile(t *testing.T) {
	t.Parallel()

	h := newPayrollHarness(t, emailConfig("lohn@steuerbuero-beispiel.de"))
	h.close(t)
	assert.Zero(t, h.run(t), "the mail is queued, not yet sent")

	row := h.store.rows[0]
	assert.Equal(t, workforce.PayrollDispatchMailing, row.State)
	assert.Equal(t, "hash:raw-token", row.DownloadTokenHash, "only the fingerprint is stored")
	assert.Equal(t, payrollTestNow.Add(14*24*time.Hour), *row.DownloadExpiresAt)
	assert.NotEmpty(t, row.FileData)
	require.Len(t, h.mail.links, 1)
	link := h.mail.links[0]
	assert.Equal(t, "https://moto.example/lohnbuero/7.raw-token", link.Content["LinkURL"])
	assert.Equal(t, payrollTestTenant, link.TenantID)
	assert.Equal(t, "lohn@steuerbuero-beispiel.de", link.ToAddress)
	assert.Equal(t, "Monatsdatei September 2026", link.Subject)
	assert.Equal(t, "OGS Am Park", link.Content["SchoolName"])
	assert.Equal(t, *row.DownloadExpiresAt, link.ExpiresAt, "Delivery drops the mail once the link expired")
	assert.Equal(t, workforce.PayrollDispatchMaxAttempts, link.MaxAttempts, "the first send and three retries")
	assert.Empty(t, h.transfer.emails, "the journal entry waits for the mail server's answer")

	h.mail.state = PayrollMailSent
	*h.now = row.NextAttemptAt
	assert.Equal(t, 1, h.run(t))
	assert.Equal(t, workforce.PayrollDispatchSent, h.store.rows[0].State)
	require.Len(t, h.transfer.emails, 1)
	assert.True(t, h.transfer.emails[0].Delivered)
	assert.Equal(t, "2026-09", h.transfer.emails[0].Period)
	assert.NotEmpty(t, h.store.rows[0].FileData, "the link stays open after the mail was sent")

	download, err := h.service.Download(context.Background(), payrollTestTenant, "raw-token")
	require.NoError(t, err)
	assert.Equal(t, "zeiterfassung_alle_2026-09.csv", download.Filename)
	assert.Equal(t, []byte("Personalnummer;Stunden"), download.Data)

	_, err = h.service.Download(context.Background(), payrollTestTenant, "raw-token")
	require.ErrorIs(t, err, workforce.ErrPayrollDownloadUnavailable, "a link opens once")
	require.NotNil(t, h.store.rows[0].DownloadedAt)
}

func TestPayrollDispatch_FailedLinkMailIsJournaledAndNotified(t *testing.T) {
	t.Parallel()

	h := newPayrollHarness(t, emailConfig("lohn@steuerbuero-beispiel.de"))
	h.close(t)
	h.run(t)
	h.mail.state = PayrollMailFailed
	*h.now = h.store.rows[0].NextAttemptAt
	h.run(t)

	row := h.store.rows[0]
	assert.Equal(t, workforce.PayrollDispatchFailed, row.State)
	assert.Equal(t, workforce.PayrollReasonMailFailed, row.Reason)
	assert.Nil(t, row.FileData, "a link nobody received must not keep payroll data")
	require.Len(t, h.transfer.emails, 1)
	assert.False(t, h.transfer.emails[0].Delivered)
	require.Len(t, h.mail.problems, 1)
}

func TestPayrollDispatch_MissingAddressIsBlockedAtTheClose(t *testing.T) {
	t.Parallel()

	h := newPayrollHarness(t, emailConfig("  "))
	h.close(t)

	require.Len(t, h.store.rows, 1)
	assert.Equal(t, workforce.PayrollDispatchBlocked, h.store.rows[0].State)
	assert.Equal(t, workforce.PayrollReasonEmailMissing, h.store.rows[0].Reason)
	require.Len(t, h.mail.problems, 1, "the leadership learns at once")
	assert.Zero(t, h.run(t), "a blocked dispatch is never picked up")
}

func TestPayrollDispatch_UnknownLinkAnswersUnavailable(t *testing.T) {
	t.Parallel()

	h := newPayrollHarness(t, emailConfig("lohn@steuerbuero-beispiel.de"))
	for _, attempt := range []struct {
		tenant int64
		token  string
	}{{payrollTestTenant, "guessed"}, {payrollTestTenant, ""}, {0, "raw-token"}} {
		_, err := h.service.Download(context.Background(), attempt.tenant, attempt.token)
		require.ErrorIs(t, err, workforce.ErrPayrollDownloadUnavailable)
	}
}

func TestPayrollDispatch_BrokenPassCountsAsAFailedAttempt(t *testing.T) {
	t.Parallel()

	h := newPayrollHarness(t, emailConfig("lohn@steuerbuero-beispiel.de"))
	h.export.exportErr = errors.New("database unavailable")
	h.close(t)

	_, err := h.service.RunDue(context.Background())
	require.Error(t, err)
	row := h.store.rows[0]
	assert.Equal(t, workforce.PayrollDispatchPending, row.State)
	assert.Equal(t, 1, row.Attempts)
	assert.Equal(t, workforce.PayrollReasonInternal, row.Reason)
	assert.Empty(t, h.mail.links)
}

func TestPayrollFilename(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "a_2026-09.csv", payrollFilename("a_2026-09.csv", 0))
	assert.Equal(t, "a_2026-09_korrektur-2.xlsx", payrollFilename("a_2026-09.xlsx", 2))
	assert.Equal(t, "ohne_endung_korrektur-1", payrollFilename("ohne_endung", 1))
}
