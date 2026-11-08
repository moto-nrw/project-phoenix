package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// A single download must not load the other invoices or their PDF bodies.
type singleCareLetterStore struct {
	CareLetterStore
	t       *testing.T
	details map[string]any
}

func (s *singleCareLetterStore) CareRun(_ context.Context, organizationID, runID int64, lock bool) (parentbilling.CareRun, error) {
	assert.Equal(s.t, int64(7), organizationID)
	assert.Equal(s.t, int64(11), runID)
	assert.False(s.t, lock)
	return parentbilling.CareRun{ID: runID, Status: parentbilling.RunReleased}, nil
}

func (s *singleCareLetterStore) CareRunLetters(context.Context, int64, int64) (*parentbilling.CareLetterSet, error) {
	return nil, errors.New("individual PDF must not load the entire run")
}

func (s *singleCareLetterStore) CareLetterRecord(_ context.Context, organizationID, runID, invoiceID int64, kind string) (parentbilling.CareLetterRecord, error) {
	assert.Equal(s.t, int64(7), organizationID)
	assert.Equal(s.t, int64(11), runID)
	assert.Equal(s.t, int64(13), invoiceID)
	assert.Equal(s.t, parentbilling.CareLetterCancellation, kind)
	return parentbilling.CareLetterRecord{ID: 17, InvoiceID: invoiceID, Kind: kind,
		File: &parentbilling.BillingLetterFile{Filename: "storno.pdf", ContentType: "application/pdf", Data: []byte("%PDF-one")}}, nil
}

func (s *singleCareLetterStore) InsertCareRunEvent(_ context.Context, organizationID, runID, accountID int64, action string, details map[string]any) error {
	assert.Equal(s.t, int64(7), organizationID)
	assert.Equal(s.t, int64(11), runID)
	assert.Equal(s.t, int64(19), accountID)
	assert.Equal(s.t, runEventRead, action)
	s.details = details
	return nil
}

func TestCareLetterLoadsOnlyTheRequestedPDFAndAuditsIt(t *testing.T) {
	t.Parallel()
	store := &singleCareLetterStore{t: t}
	letters := &CareLetters{store: store}
	file, err := letters.CareLetter(context.Background(), parentbilling.HolidayReviewer{OrganizationID: 7, AccountID: 19}, 11, 13, parentbilling.CareLetterCancellation)
	require.NoError(t, err)
	assert.Equal(t, "storno.pdf", file.Filename)
	assert.Equal(t, []byte("%PDF-one"), file.Data)
	assert.Equal(t, map[string]any{"letter": "download", "invoice_id": int64(13), "letter_id": int64(17)}, store.details)
}
