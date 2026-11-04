package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// ParentDocumentStore reads the inbox of one recipient in the Träger
// transaction and records each read in the trail of its run.
type ParentDocumentStore interface {
	RecipientDocuments(ctx context.Context, organizationID int64, email string) ([]parentbilling.ParentDocument, error)
	RecipientDocumentLetter(ctx context.Context, organizationID, documentID int64, email string) (int64, parentbilling.BillingLetterFile, error)
	InsertRunEvent(ctx context.Context, organizationID, runID, accountID int64, action string, details map[string]any) error
}

// ParentDocuments serves parentbilling.ParentDocuments (#4671, ADR 0126).
type ParentDocuments struct {
	store ParentDocumentStore
}

var _ parentbilling.ParentDocuments = (*ParentDocuments)(nil)

// NewParentDocuments builds the inbox of the parents portal.
func NewParentDocuments(store ParentDocumentStore) *ParentDocuments {
	if store == nil {
		panic("parent billing: the store is required")
	}
	return &ParentDocuments{store: store}
}

// ListParentDocuments lists the sent documents of the Träger addressed to
// the reader and records the read in the trail of every run it shows.
func (p *ParentDocuments) ListParentDocuments(ctx context.Context, organizationID int64, reader parentbilling.ParentDocumentReader) ([]parentbilling.ParentDocument, error) {
	email, err := readerAddress(organizationID, reader)
	if err != nil {
		return nil, err
	}
	documents, err := p.store.RecipientDocuments(ctx, organizationID, email)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read inbox: %w", err)
	}
	byRun := map[int64][]int64{}
	var runs []int64
	for _, document := range documents {
		if _, seen := byRun[document.RunID]; !seen {
			runs = append(runs, document.RunID)
		}
		byRun[document.RunID] = append(byRun[document.RunID], document.DocumentID)
	}
	for _, runID := range runs {
		if err := p.store.InsertRunEvent(ctx, organizationID, runID, reader.AccountID, runEventRead,
			map[string]any{"inbox": "list", "document_ids": byRun[runID]}); err != nil {
			return nil, fmt.Errorf("parent billing: record inbox read: %w", err)
		}
	}
	return documents, nil
}

// ParentDocumentLetter reads the stored letter of one sent document
// addressed to the reader and records the read in the trail of its run.
func (p *ParentDocuments) ParentDocumentLetter(ctx context.Context, organizationID, documentID int64, reader parentbilling.ParentDocumentReader) (parentbilling.BillingLetterFile, error) {
	email, err := readerAddress(organizationID, reader)
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	if documentID <= 0 {
		return parentbilling.BillingLetterFile{}, parentbilling.ErrParentDocumentNotFound
	}
	runID, file, err := p.store.RecipientDocumentLetter(ctx, organizationID, documentID, email)
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	if err := p.store.InsertRunEvent(ctx, organizationID, runID, reader.AccountID, runEventRead,
		map[string]any{"inbox": "download", "document_id": documentID}); err != nil {
		return parentbilling.BillingLetterFile{}, fmt.Errorf("parent billing: record inbox read: %w", err)
	}
	return file, nil
}

// readerAddress is the address that selects the documents of the reader.
// An account without an address reads nothing.
func readerAddress(organizationID int64, reader parentbilling.ParentDocumentReader) (string, error) {
	email := strings.TrimSpace(reader.Email)
	if organizationID <= 0 || reader.AccountID <= 0 || email == "" {
		return "", parentbilling.ErrParentDocumentNotFound
	}
	return email, nil
}
