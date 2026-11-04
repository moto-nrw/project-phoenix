package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The transfer workflow (#3050). The rules under test are the audit ones: a
// failure is never recorded or reported as a success, and an attempt that
// cannot be recorded is not a success either.

type fakeResolver struct {
	target domain.Target
	state  domain.TargetState
	err    error
}

func (f fakeResolver) Resolve(context.Context) (domain.Target, error) {
	if f.err != nil {
		return domain.Target{}, f.err
	}
	return f.target, nil
}

func (f fakeResolver) State(context.Context) (domain.TargetState, error) {
	if f.err != nil {
		return domain.TargetState{}, f.err
	}
	return f.state, nil
}

type fakeUploader struct {
	err      error
	calls    int
	filename string
	data     []byte
	pending  *fakePendingUpload
}

func (f *fakeUploader) Prepare(_ context.Context, _ domain.Target, filename string, data []byte) (func() error, func() error, error) {
	f.calls++
	f.filename = filename
	f.data = data
	if f.err != nil {
		return nil, nil, f.err
	}
	if f.pending == nil {
		f.pending = &fakePendingUpload{}
	}
	return f.pending.Commit, f.pending.Rollback, nil
}

type fakePendingUpload struct {
	commits   int
	rollbacks int
}

func (f *fakePendingUpload) Commit() error   { f.commits++; return nil }
func (f *fakePendingUpload) Rollback() error { f.rollbacks++; return nil }

func newTestService(resolver fakeResolver, uploader *fakeUploader, journal *fakeJournal) *Service {
	runtime := ports.CommandRuntime{
		Active:       func(context.Context) bool { return true },
		Detached:     func(ctx context.Context) context.Context { return ctx },
		WithinTenant: func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
	}
	return New(resolver, uploader, journal, runtime, nil)
}

type fakeJournal struct {
	entries     []domain.JournalEntry
	err         error
	completeErr error
}

func (f *fakeJournal) Begin(context.Context, domain.JournalEntry) (int64, error) { return 1, f.err }
func (f *fakeJournal) Claim(context.Context, int64) (string, error)              { return "one-shot-fence", nil }
func (f *fakeJournal) Complete(_ context.Context, _ int64, _ string, entry domain.JournalEntry) error {
	f.entries = append(f.entries, entry)
	if f.completeErr != nil {
		return f.completeErr
	}
	return f.err
}
func (f *fakeJournal) Record(_ context.Context, entry domain.JournalEntry) error {
	f.entries = append(f.entries, entry)
	return f.err
}

// reasonedFailure is a transport error that names its reason, the way the
// SFTP adapter's sentinels do.
type reasonedFailure struct{ code string }

func (r reasonedFailure) Error() string          { return "transport failed: " + r.code }
func (r reasonedFailure) TransferReason() string { return r.code }

func configuredTarget() domain.Target {
	return domain.Target{
		Host:            "dateien.beispiel.de",
		Port:            22,
		Username:        "lohn-export",
		Password:        "example-test-password",
		RemoteDirectory: "/upload/lohn",
	}
}

func testRequest() exporttransfer.Request {
	return exporttransfer.Request{
		Kind:           "staff_time_tracking",
		Format:         "datev_lodas",
		Filename:       "zeitkonten-2026-08.txt",
		Data:           []byte("inhalt"),
		ActorAccountID: 42,
		ActorName:      "A. Beispiel",
	}
}

func TestTransfer_SuccessIsRecordedWithoutCredentials(t *testing.T) {
	t.Parallel()

	journal := &fakeJournal{}
	uploader := &fakeUploader{}
	service := newTestService(fakeResolver{target: configuredTarget()}, uploader, journal)

	result, err := service.Transfer(context.Background(), testRequest())
	require.NoError(t, err)
	assert.True(t, result.Transferred)
	assert.Empty(t, result.Reason, "a success carries no reason")
	assert.Equal(t, "dateien.beispiel.de", result.TargetHost)

	require.Len(t, journal.entries, 1)
	entry := journal.entries[0]
	assert.True(t, entry.Success)
	assert.Equal(t, int64(42), entry.ActorAccountID)
	assert.Equal(t, "A. Beispiel", entry.ActorName)
	assert.Equal(t, "zeitkonten-2026-08.txt", entry.Filename)
	assert.Equal(t, int64(len(testRequest().Data)), entry.ByteSize)
	assert.Equal(t, "dateien.beispiel.de", entry.TargetHost)
	assert.Equal(t, "/upload/lohn", entry.TargetDirectory)
	assert.Equal(t, 1, uploader.pending.commits)
	assert.Zero(t, uploader.pending.rollbacks)
	// The journal entry has no field for them, which is the point — this
	// assertion pins that the shape never grows one.
	assert.NotContains(t, entry.Filename, "example-test-password")
}

func TestTransfer_FailureIsRecordedAsFailureWithItsReason(t *testing.T) {
	t.Parallel()

	journal := &fakeJournal{}
	uploader := &fakeUploader{err: reasonedFailure{code: domain.ReasonHostKey}}
	service := newTestService(fakeResolver{target: configuredTarget()}, uploader, journal)

	result, err := service.Transfer(context.Background(), testRequest())
	require.NoError(t, err, "a counterpart that refuses is a normal answer, not a server error")
	assert.False(t, result.Transferred)
	assert.Equal(t, domain.ReasonHostKey, result.Reason)

	require.Len(t, journal.entries, 1)
	assert.False(t, journal.entries[0].Success)
	assert.Equal(t, domain.ReasonHostKey, journal.entries[0].Reason)
}

func TestTransfer_UnsettledAttemptIsNotAnOrdinaryUnavailableTransfer(t *testing.T) {
	t.Parallel()

	completeErr := errors.New("journal outcome unavailable")
	journal := &fakeJournal{completeErr: completeErr}
	uploader := &fakeUploader{}
	service := newTestService(fakeResolver{target: configuredTarget()}, uploader, journal)

	_, err := service.Transfer(context.Background(), testRequest())
	require.ErrorIs(t, err, completeErr)
	assert.ErrorIs(t, err, exporttransfer.ErrUnknownSettlement)
	assert.NotErrorIs(t, err, ErrUnavailable,
		"the recovery worker, not a caller retry, must settle an attempt whose remote outcome may be unknown")
	assert.Equal(t, 1, uploader.calls)
	assert.Equal(t, 1, uploader.pending.commits)
}

// An unrecognized transport error must not be presented as a known, harmless
// condition.
func TestTransfer_UnnamedFailureBecomesAnInternalReason(t *testing.T) {
	t.Parallel()

	journal := &fakeJournal{}
	uploader := &fakeUploader{err: errors.New("something nobody classified")}
	service := newTestService(fakeResolver{target: configuredTarget()}, uploader, journal)

	result, err := service.Transfer(context.Background(), testRequest())
	require.NoError(t, err)
	assert.False(t, result.Transferred)
	assert.Equal(t, domain.ReasonInternal, result.Reason)
}

// Nothing was attempted, so nothing is recorded — and no connection is opened.
func TestTransfer_UnconfiguredTargetAttemptsAndRecordsNothing(t *testing.T) {
	t.Parallel()

	journal := &fakeJournal{}
	uploader := &fakeUploader{}
	service := newTestService(fakeResolver{err: domain.ErrNotConfigured}, uploader, journal)

	result, err := service.Transfer(context.Background(), testRequest())
	require.NoError(t, err)
	assert.False(t, result.Transferred)
	assert.Equal(t, domain.ReasonNotConfigured, result.Reason)
	assert.Zero(t, uploader.calls, "an unconfigured target must not be dialed")
	assert.Empty(t, journal.entries)
}

// Evidence must commit before any remote operation can start.
func TestTransfer_UnrecordableAttemptIsNotReportedAsSuccess(t *testing.T) {
	t.Parallel()

	journal := &fakeJournal{err: errors.New("journal unavailable")}
	uploader := &fakeUploader{}
	service := newTestService(fakeResolver{target: configuredTarget()}, uploader, journal)

	result, err := service.Transfer(context.Background(), testRequest())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnavailable)
	assert.False(t, result.Transferred)
	assert.Zero(t, uploader.calls, "an unrecordable intent must never touch the remote file")
}

// A settings store that cannot answer is not an unconfigured school.
func TestTransfer_ResolverFailureIsUnavailableNotUnconfigured(t *testing.T) {
	t.Parallel()

	boom := errors.New("settings unavailable")
	service := newTestService(fakeResolver{err: boom}, &fakeUploader{}, &fakeJournal{})

	_, err := service.Transfer(context.Background(), testRequest())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnavailable)
	assert.ErrorIs(t, err, boom)
}

// The capability never builds a file: whatever it is handed is what travels.
func TestTransfer_SendsTheFileItWasGivenUnchanged(t *testing.T) {
	t.Parallel()

	uploader := &fakeUploader{}
	service := newTestService(fakeResolver{target: configuredTarget()}, uploader, &fakeJournal{})

	request := testRequest()
	request.Data = []byte("Personalnummer;Lohnart\r\n4711;100\r\n")
	_, err := service.Transfer(context.Background(), request)
	require.NoError(t, err)

	assert.Equal(t, request.Filename, uploader.filename)
	assert.Equal(t, request.Data, uploader.data)
}

func TestStatus_PassesTheConfigurationThrough(t *testing.T) {
	t.Parallel()

	state := domain.TargetState{Enabled: true, Host: "dateien.beispiel.de", Port: 22, RemoteDirectory: "/upload/lohn"}
	service := newTestService(fakeResolver{state: state}, &fakeUploader{}, &fakeJournal{})

	got, err := service.Status(context.Background())
	require.NoError(t, err)
	assert.True(t, got.Ready)
	assert.Equal(t, "dateien.beispiel.de", got.Host)
}

func TestAccessAuditFailurePreventsTransfer(t *testing.T) {
	t.Parallel()
	uploader := &fakeUploader{}
	journal := &fakeJournal{}
	service := newTestService(fakeResolver{target: configuredTarget()}, uploader, journal)
	auditErr := errors.New("access audit unavailable")
	_, err := service.Transfer(context.Background(), exporttransfer.Request{
		PrepareFile: func(context.Context) (string, []byte, error) {
			return "", nil, auditErr
		},
	})
	require.ErrorIs(t, err, auditErr)
	assert.Zero(t, uploader.calls)
	assert.Empty(t, journal.entries)
}

func TestTransferUsesTheFilePreparedWithItsAccessAudit(t *testing.T) {
	t.Parallel()
	uploader := &fakeUploader{}
	journal := &fakeJournal{}
	service := newTestService(fakeResolver{target: configuredTarget()}, uploader, journal)
	result, err := service.Transfer(context.Background(), exporttransfer.Request{
		PrepareFile: func(context.Context) (string, []byte, error) {
			return "prepared.csv", []byte("prepared export"), nil
		},
	})
	require.NoError(t, err)
	require.True(t, result.Transferred)
	assert.Equal(t, "prepared.csv", uploader.filename)
	assert.Equal(t, []byte("prepared export"), uploader.data)
	require.Len(t, journal.entries, 1)
	assert.Equal(t, uploader.filename, journal.entries[0].Filename)
	assert.Equal(t, int64(len(uploader.data)), journal.entries[0].ByteSize)
}

// #4805: an automatic send names its month and the SFTP channel, so the
// journal answers which month left the school and how.
func TestTransfer_RecordsPeriodAndChannel(t *testing.T) {
	t.Parallel()

	journal := &fakeJournal{}
	service := newTestService(fakeResolver{target: configuredTarget()}, &fakeUploader{}, journal)
	request := testRequest()
	request.Period = "2026-08"

	_, err := service.Transfer(context.Background(), request)
	require.NoError(t, err)
	require.Len(t, journal.entries, 1)
	assert.Equal(t, "2026-08", journal.entries[0].Period)
	assert.Equal(t, domain.ChannelSFTP, journal.entries[0].Channel)
}

func TestRecordEmail_DeliveredAndFailedSends(t *testing.T) {
	t.Parallel()

	journal := &fakeJournal{}
	service := newTestService(fakeResolver{target: configuredTarget()}, &fakeUploader{}, journal)
	record := exporttransfer.EmailRecord{
		Kind: exporttransfer.KindStaffTimeTracking, Format: "csv", Period: "2026-08",
		Filename: "zeiterfassung_alle_2026-08.csv", ByteSize: 12, Recipient: "lohn@steuerbuero-beispiel.de",
		Delivered: true, ActorAccountID: 42, ActorName: "A. Beispiel",
	}
	require.NoError(t, service.RecordEmail(context.Background(), record))

	record.Delivered, record.Reason = false, exporttransfer.ReasonMailFailed
	require.NoError(t, service.RecordEmail(context.Background(), record))

	require.Len(t, journal.entries, 2)
	assert.True(t, journal.entries[0].Success)
	assert.Empty(t, journal.entries[0].Reason)
	assert.Equal(t, domain.ChannelEmail, journal.entries[0].Channel)
	assert.Equal(t, "lohn@steuerbuero-beispiel.de", journal.entries[0].TargetEmail)
	assert.Equal(t, "2026-08", journal.entries[0].Period)
	assert.False(t, journal.entries[1].Success)
	assert.Equal(t, domain.ReasonMailFailed, journal.entries[1].Reason)
}

func TestRecordEmail_RejectsIncompleteRecords(t *testing.T) {
	t.Parallel()

	journal := &fakeJournal{}
	service := newTestService(fakeResolver{target: configuredTarget()}, &fakeUploader{}, journal)

	err := service.RecordEmail(context.Background(), exporttransfer.EmailRecord{Filename: "datei.csv"})
	require.ErrorIs(t, err, ErrUnavailable, "a send without recipient says nothing about where the file went")

	err = service.RecordEmail(context.Background(), exporttransfer.EmailRecord{Filename: "datei.csv", Recipient: "a@b.de"})
	require.ErrorIs(t, err, ErrUnavailable, "a failure must carry its reason")
	assert.Empty(t, journal.entries)
}
