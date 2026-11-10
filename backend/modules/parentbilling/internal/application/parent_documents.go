package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// ParentDocumentStore reads the inbox of one recipient in the Träger
// transaction and records each read in the trail of its run.
type ParentDocumentStore interface {
	RecipientDocuments(ctx context.Context, organizationID int64, email string, children []parentbilling.ParentDocumentChild) ([]parentbilling.ParentDocument, error)
	// RecipientDocumentLetter also tells whether the run is a monthly run.
	RecipientDocumentLetter(ctx context.Context, organizationID, documentID int64, email string, children []parentbilling.ParentDocumentChild) (int64, bool, parentbilling.BillingLetterFile, error)
	InsertRunEvent(ctx context.Context, organizationID, runID, accountID int64, action string, details map[string]any) error
	// InsertCareRunEvent records the read of a letter of a monthly run (#4684).
	InsertCareRunEvent(ctx context.Context, organizationID, runID, accountID int64, action string, details map[string]any) error
	// The original of a sent Jahresbescheid of the reader (#6347);
	// RecipientDocuments lists them with the letters.
	RecipientCareNoticeLetter(ctx context.Context, organizationID, documentID int64, email string, children []parentbilling.ParentDocumentChild) (int64, parentbilling.BillingLetterFile, error)
	InsertCareNoticeRunEvent(ctx context.Context, organizationID, runID, accountID int64, action string, details map[string]any) error
}

// ParentDocuments serves parentbilling.ParentDocuments (#4671, ADR 0126).
type ParentDocuments struct {
	parentbilling.DocumentPushValidity
	// The parents' statement of the children of their family (#5918).
	*FamilyNumbers
	store ParentDocumentStore
}

var _ parentbilling.ParentDocuments = (*ParentDocuments)(nil)

// NewParentDocuments builds the inbox of the parents portal.
func NewParentDocuments(store interface {
	ParentDocumentStore
	parentbilling.DocumentPushValidity
	FamilyChildrenStore
}) *ParentDocuments {
	if store == nil {
		panic("parent billing: the store is required")
	}
	return &ParentDocuments{store: store, DocumentPushValidity: store, FamilyNumbers: NewParentFamilyChildren(store)}
}

// ListParentDocuments lists the sent documents of the Träger addressed to
// the reader and records the read in the trail of every run it shows.
func (p *ParentDocuments) ListParentDocuments(ctx context.Context, organizationID int64, reader parentbilling.ParentDocumentReader) ([]parentbilling.ParentDocument, error) {
	email, err := readerAddress(organizationID, reader)
	if err != nil {
		return nil, err
	}
	documents, err := p.store.RecipientDocuments(ctx, organizationID, email, reader.Children)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read inbox: %w", err)
	}
	byRun := map[runKey][]int64{}
	var runs []runKey
	for _, document := range documents {
		key := runKey{id: document.RunID, trail: documentTrail(document)}
		if _, seen := byRun[key]; !seen {
			runs = append(runs, key)
		}
		byRun[key] = append(byRun[key], document.DocumentID)
	}
	for _, run := range runs {
		if err := p.recordRead(ctx, organizationID, run.id, run.trail, reader.AccountID,
			map[string]any{"inbox": "list", "document_ids": byRun[run]}); err != nil {
			return nil, err
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
	trail := trailNotice
	runID, file, err := p.store.RecipientCareNoticeLetter(ctx, organizationID, documentID, email, reader.Children)
	if errors.Is(err, parentbilling.ErrParentDocumentNotFound) {
		var care bool
		runID, care, file, err = p.store.RecipientDocumentLetter(ctx, organizationID, documentID, email, reader.Children)
		trail = trailHoliday
		if care {
			trail = trailCare
		}
	}
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	if err := p.recordRead(ctx, organizationID, runID, trail, reader.AccountID,
		map[string]any{"inbox": "download", "document_id": documentID}); err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	return file, nil
}

// The trails a read of the inbox lands in: of the holiday run, the monthly
// run or the run of the Jahresbescheide.
const (
	trailHoliday = "holiday"
	trailCare    = "care"
	trailNotice  = "notice"
)

type runKey struct {
	id    int64
	trail string
}

func documentTrail(document parentbilling.ParentDocument) string {
	switch {
	case document.NoticeRun:
		return trailNotice
	case document.CareRun:
		return trailCare
	}
	return trailHoliday
}

// recordRead writes the read into the trail of the holiday, monthly or
// notice run.
func (p *ParentDocuments) recordRead(ctx context.Context, organizationID, runID int64, trail string, accountID int64, details map[string]any) error {
	insert := p.store.InsertRunEvent
	switch trail {
	case trailCare:
		insert = p.store.InsertCareRunEvent
	case trailNotice:
		insert = p.store.InsertCareNoticeRunEvent
	}
	if err := insert(ctx, organizationID, runID, accountID, runEventRead, details); err != nil {
		return fmt.Errorf("parent billing: record inbox read: %w", err)
	}
	return nil
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
