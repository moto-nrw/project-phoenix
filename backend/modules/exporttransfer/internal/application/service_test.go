package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/domain"
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
}

func (f *fakeUploader) Upload(_ context.Context, _ domain.Target, filename string, data []byte) error {
	f.calls++
	f.filename = filename
	f.data = data
	return f.err
}

type fakeJournal struct {
	entries []domain.JournalEntry
	err     error
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
		Password:        "s3hr-geheim",
		RemoteDirectory: "/upload/lohn",
	}
}

func testRequest() domain.Request {
	return domain.Request{
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
	service := New(fakeResolver{target: configuredTarget()}, &fakeUploader{}, journal, nil)

	result, err := service.Transfer(context.Background(), testRequest())
	require.NoError(t, err)
	assert.True(t, result.Success)
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
	// The journal entry has no field for them, which is the point — this
	// assertion pins that the shape never grows one.
	assert.NotContains(t, entry.Filename, "s3hr-geheim")
}

func TestTransfer_FailureIsRecordedAsFailureWithItsReason(t *testing.T) {
	t.Parallel()

	journal := &fakeJournal{}
	uploader := &fakeUploader{err: reasonedFailure{code: domain.ReasonHostKey}}
	service := New(fakeResolver{target: configuredTarget()}, uploader, journal, nil)

	result, err := service.Transfer(context.Background(), testRequest())
	require.NoError(t, err, "a counterpart that refuses is a normal answer, not a server error")
	assert.False(t, result.Success)
	assert.Equal(t, domain.ReasonHostKey, result.Reason)

	require.Len(t, journal.entries, 1)
	assert.False(t, journal.entries[0].Success)
	assert.Equal(t, domain.ReasonHostKey, journal.entries[0].Reason)
}

// An unrecognized transport error must not be presented as a known, harmless
// condition.
func TestTransfer_UnnamedFailureBecomesAnInternalReason(t *testing.T) {
	t.Parallel()

	journal := &fakeJournal{}
	uploader := &fakeUploader{err: errors.New("something nobody classified")}
	service := New(fakeResolver{target: configuredTarget()}, uploader, journal, nil)

	result, err := service.Transfer(context.Background(), testRequest())
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, domain.ReasonInternal, result.Reason)
}

// Nothing was attempted, so nothing is recorded — and no connection is opened.
func TestTransfer_UnconfiguredTargetAttemptsAndRecordsNothing(t *testing.T) {
	t.Parallel()

	journal := &fakeJournal{}
	uploader := &fakeUploader{}
	service := New(fakeResolver{err: domain.ErrNotConfigured}, uploader, journal, nil)

	result, err := service.Transfer(context.Background(), testRequest())
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, domain.ReasonNotConfigured, result.Reason)
	assert.Zero(t, uploader.calls, "an unconfigured target must not be dialed")
	assert.Empty(t, journal.entries)
}

// The file may well have arrived. A success that cannot be traced is exactly
// what the audit requirement exists to prevent, so it is reported as an error.
func TestTransfer_UnrecordableAttemptIsNotReportedAsSuccess(t *testing.T) {
	t.Parallel()

	journal := &fakeJournal{err: errors.New("journal unavailable")}
	service := New(fakeResolver{target: configuredTarget()}, &fakeUploader{}, journal, nil)

	result, err := service.Transfer(context.Background(), testRequest())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnavailable)
	assert.False(t, result.Success)
}

// A settings store that cannot answer is not an unconfigured school.
func TestTransfer_ResolverFailureIsUnavailableNotUnconfigured(t *testing.T) {
	t.Parallel()

	boom := errors.New("settings unavailable")
	service := New(fakeResolver{err: boom}, &fakeUploader{}, &fakeJournal{}, nil)

	_, err := service.Transfer(context.Background(), testRequest())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnavailable)
	assert.ErrorIs(t, err, boom)
}

// The capability never builds a file: whatever it is handed is what travels.
func TestTransfer_SendsTheFileItWasGivenUnchanged(t *testing.T) {
	t.Parallel()

	uploader := &fakeUploader{}
	service := New(fakeResolver{target: configuredTarget()}, uploader, &fakeJournal{}, nil)

	request := testRequest()
	request.Data = []byte("Personalnummer;Lohnart\r\n4711;100\r\n")
	_, err := service.Transfer(context.Background(), request)
	require.NoError(t, err)

	assert.Equal(t, request.Filename, uploader.filename)
	assert.Equal(t, request.Data, uploader.data)
}

func TestState_PassesTheConfigurationThrough(t *testing.T) {
	t.Parallel()

	state := domain.TargetState{Enabled: true, Host: "dateien.beispiel.de", Port: 22, RemoteDirectory: "/upload/lohn"}
	service := New(fakeResolver{state: state}, &fakeUploader{}, &fakeJournal{}, nil)

	got, err := service.State(context.Background())
	require.NoError(t, err)
	assert.True(t, got.Ready())
	assert.Equal(t, "dateien.beispiel.de", got.Host)
}
