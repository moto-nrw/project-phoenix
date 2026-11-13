package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

// The handover of a finance export (ADR 0218 section 6): the same producer
// as the download, a repeated content refused before a number is taken, and
// the outcome settled on the protocol entry the producer wrote.

type handoverExports struct {
	parentbilling.FinanceExports
	made int
}

func (e *handoverExports) SollstellungFile(context.Context, parentbilling.HolidayReviewer, string, int64) (parentbilling.ExportedFile, error) {
	e.made++
	return parentbilling.ExportedFile{Entry: parentbilling.ExportProtocolEntry{ID: 41, FileName: "sollstellung-110.csv"}, Data: []byte("a;b\r\n")}, nil
}

func (e *handoverExports) DatevFile(context.Context, parentbilling.HolidayReviewer, string, int64, parentbilling.DatevFileOptions,
) (parentbilling.ExportedFile, error) {
	e.made++
	return parentbilling.ExportedFile{}, parentbilling.ErrDatevNoNewBookings
}

type handoverDebits struct {
	parentbilling.SEPADebits
}

func (handoverDebits) SEPADebitFile(context.Context, parentbilling.HolidayReviewer, string, int64) (parentbilling.SEPADebitFile, error) {
	return parentbilling.SEPADebitFile{Filename: "lastschrift-112-M1.xml", Data: []byte("<xml/>"), ExportFileID: 43}, nil
}

type handoverStore struct {
	repeated bool
	// askedFor is the recipient the repeat check asked about.
	askedFor []int64
	inserted []int64
	targets  []parentbilling.HandoverDestination
	settled  map[int64]string
	reasons  map[int64]string
}

func (s *handoverStore) SameContentHandedOver(_ context.Context, _, _, recipientID int64) (bool, error) {
	s.askedFor = append(s.askedFor, recipientID)
	return s.repeated, nil
}

func (s *handoverStore) InsertExportFileHandover(_ context.Context, _, fileID int64, destination parentbilling.HandoverDestination) error {
	s.inserted = append(s.inserted, fileID)
	s.targets = append(s.targets, destination)
	return nil
}

func (s *handoverStore) SettleExportFileHandover(_ context.Context, _, fileID int64, state, reason string) error {
	if s.settled == nil {
		s.settled, s.reasons = map[int64]string{}, map[int64]string{}
	}
	s.settled[fileID], s.reasons[fileID] = state, reason
	return nil
}

// handoverTransport runs the producer and the settlement the way Export
// Transfer does, around an outcome the test picks.
type handoverTransport struct {
	outcome   parentbilling.FinanceHandoverOutcome
	prepareOK bool
}

func (t *handoverTransport) Handover(ctx context.Context, request parentbilling.FinanceHandoverRequest) (parentbilling.FinanceHandoverOutcome, error) {
	if t.outcome.NotReady {
		return t.outcome, nil
	}
	if _, _, err := request.Prepare(ctx, steuerbuero); err != nil {
		return parentbilling.FinanceHandoverOutcome{}, err
	}
	t.prepareOK = true
	if !t.outcome.Uncertain {
		if err := request.Settle(ctx, t.outcome.Transferred, t.outcome.Reason); err != nil {
			return parentbilling.FinanceHandoverOutcome{}, err
		}
	}
	return t.outcome, nil
}

var handoverReviewer = parentbilling.HolidayReviewer{OrganizationID: 7, AccountID: 9}

// steuerbuero is the recipient the transport resolved for the format.
var steuerbuero = parentbilling.HandoverDestination{RecipientID: 12, RecipientName: "Steuerbüro", Host: "steuerbuero.example.org", Directory: "/eingang"}

func sollHandover(repeat bool) parentbilling.HandoverRequest {
	return parentbilling.HandoverRequest{Format: parentbilling.ExportFormatSollstellung, RunKind: parentbilling.DebitRunCare, RunID: 3, Repeat: repeat}
}

func TestHandoverSettlesTheProtocolEntryOfItsFile(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		outcome parentbilling.FinanceHandoverOutcome
		state   string
		reason  string
	}{
		"transferred": {parentbilling.FinanceHandoverOutcome{Transferred: true}, parentbilling.HandoverTransferred, ""},
		"failed":      {parentbilling.FinanceHandoverOutcome{Reason: "connection_failed"}, parentbilling.HandoverFailed, "connection_failed"},
	} {
		store := &handoverStore{}
		handovers := NewExportHandovers(&handoverExports{}, handoverDebits{}, store, &handoverTransport{outcome: tc.outcome})
		result, err := handovers.HandOver(context.Background(), handoverReviewer, sollHandover(false))
		require.NoError(t, err, name)
		assert.Equal(t, tc.state, result.State, name)
		assert.Equal(t, "sollstellung-110.csv", result.FileName, name)
		assert.Equal(t, "steuerbuero.example.org", result.TargetHost, name)
		assert.Equal(t, "Steuerbüro", result.RecipientName, name)
		assert.Equal(t, []int64{41}, store.inserted, name)
		assert.Equal(t, []parentbilling.HandoverDestination{steuerbuero}, store.targets, "the handover records the recipient the transport resolved")
		assert.Equal(t, []int64{12}, store.askedFor, "the repeat check asks about that recipient")
		assert.Equal(t, tc.state, store.settled[41], name)
		assert.Equal(t, tc.reason, store.reasons[41], name)
	}
}

func TestHandoverOfAnUnrecordedOutcomeIsUnknownAndStaysPending(t *testing.T) {
	t.Parallel()
	store := &handoverStore{}
	handovers := NewExportHandovers(&handoverExports{}, handoverDebits{}, store,
		&handoverTransport{outcome: parentbilling.FinanceHandoverOutcome{Uncertain: true}})
	result, err := handovers.HandOver(context.Background(), handoverReviewer, sollHandover(false))
	require.NoError(t, err)
	assert.Equal(t, parentbilling.HandoverUnknown, result.State)
	assert.Empty(t, store.settled, "an unknown outcome is never written as transferred or failed")
}

func TestHandoverOfARepeatedContentNeedsRepeat(t *testing.T) {
	t.Parallel()
	store := &handoverStore{repeated: true}
	transport := &handoverTransport{outcome: parentbilling.FinanceHandoverOutcome{Transferred: true}}
	handovers := NewExportHandovers(&handoverExports{}, handoverDebits{}, store, transport)
	_, err := handovers.HandOver(context.Background(), handoverReviewer, sollHandover(false))
	require.ErrorIs(t, err, parentbilling.ErrExportAlreadyHandedOver)
	assert.Empty(t, store.inserted, "a refused handover records nothing")
	result, err := handovers.HandOver(context.Background(), handoverReviewer, sollHandover(true))
	require.NoError(t, err)
	assert.Equal(t, parentbilling.HandoverTransferred, result.State)
}

func TestHandoverWithoutRecipientMakesNoFile(t *testing.T) {
	t.Parallel()
	exports := &handoverExports{}
	handovers := NewExportHandovers(exports, handoverDebits{}, &handoverStore{},
		&handoverTransport{outcome: parentbilling.FinanceHandoverOutcome{NotReady: true}})
	_, err := handovers.HandOver(context.Background(), handoverReviewer, sollHandover(false))
	require.ErrorIs(t, err, parentbilling.ErrExportRecipientNotReady)
	assert.Zero(t, exports.made)
}

func TestHandoverKeepsTheRefusalOfTheDownload(t *testing.T) {
	t.Parallel()
	handovers := NewExportHandovers(&handoverExports{}, handoverDebits{}, &handoverStore{},
		&handoverTransport{outcome: parentbilling.FinanceHandoverOutcome{Transferred: true}})
	_, err := handovers.HandOver(context.Background(), handoverReviewer,
		parentbilling.HandoverRequest{Format: parentbilling.ExportFormatDatev, RunKind: parentbilling.DebitRunCare, RunID: 3})
	require.ErrorIs(t, err, parentbilling.ErrDatevNoNewBookings)
	_, err = handovers.HandOver(context.Background(), handoverReviewer,
		parentbilling.HandoverRequest{Format: "simba", RunKind: parentbilling.DebitRunCare, RunID: 3})
	require.ErrorIs(t, err, parentbilling.ErrExportHandoverFormat)
	_, err = handovers.HandOver(context.Background(), parentbilling.HolidayReviewer{}, sollHandover(false))
	require.Error(t, err)
	assert.False(t, errors.Is(err, parentbilling.ErrExportHandoverFormat))
}

func TestHandoverOfTheDebitFileSettlesItsProtocolEntry(t *testing.T) {
	t.Parallel()
	store := &handoverStore{}
	handovers := NewExportHandovers(&handoverExports{}, handoverDebits{}, store,
		&handoverTransport{outcome: parentbilling.FinanceHandoverOutcome{Transferred: true}})
	result, err := handovers.HandOver(context.Background(), handoverReviewer,
		parentbilling.HandoverRequest{Format: parentbilling.ExportFormatPain008, RunKind: parentbilling.DebitRunCare, RunID: 3})
	require.NoError(t, err)
	assert.Equal(t, "lastschrift-112-M1.xml", result.FileName)
	assert.Equal(t, parentbilling.HandoverTransferred, store.settled[43])
}
