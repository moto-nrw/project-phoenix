package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// MailStore persists the sends of the letters of a Träger in its Träger
// transaction.
type MailStore interface {
	BillingRun(ctx context.Context, organizationID, runID int64) (parentbilling.BillingRun, error)
	LockBillingRun(ctx context.Context, organizationID, runID int64) (parentbilling.BillingRun, error)
	RunDocuments(ctx context.Context, organizationID, runID int64) ([]parentbilling.BillingDocument, error)
	RunLetters(ctx context.Context, organizationID, runID int64) (*parentbilling.BillingLetters, error)
	DocumentLetter(ctx context.Context, organizationID, runID, documentID int64) (parentbilling.BillingLetterFile, error)
	// RunLetterFiles reads the stored letters of a run in the order of
	// their invoice numbers.
	RunLetterFiles(ctx context.Context, organizationID, runID int64) ([]parentbilling.BillingDocumentLetter, error)
	// RunMailings reads every send of the letters of a run in the order they
	// were stored under the run lock; the last one is the current send.
	RunMailings(ctx context.Context, organizationID, runID int64) ([]parentbilling.BillingLetterSend, error)
	InsertMailing(ctx context.Context, organizationID, runID, accountID int64, mailing parentbilling.BillingLetterSend) (parentbilling.BillingLetterSend, error)
	InsertRunEvent(ctx context.Context, organizationID, runID, accountID int64, action string, details map[string]any) error
}

// LetterCombiner combines rendered letters into one PDF to print.
type LetterCombiner interface {
	CombineLetters(letters [][]byte, title, filenameBase string) (parentbilling.BillingLetterFile, error)
}

// BillingLetterMail serves parentbilling.BillingLetterMailService.
type BillingLetterMail struct {
	store    MailStore
	post     parentbilling.BillingLetterPost
	combiner LetterCombiner
}

var _ parentbilling.BillingLetterMailService = (*BillingLetterMail)(nil)

// NewBillingLetterMail builds the sending and printing of the letters.
func NewBillingLetterMail(store MailStore, post parentbilling.BillingLetterPost, combiner LetterCombiner) *BillingLetterMail {
	if store == nil || post == nil || combiner == nil {
		panic("parent billing: the store, the post and the combiner are required")
	}
	return &BillingLetterMail{store: store, post: post, combiner: combiner}
}

// letterRun is a released run with its letters, its invoices and the sends
// of every invoice.
type letterRun struct {
	run       parentbilling.BillingRun
	documents []parentbilling.BillingDocument
	mailings  map[int64][]parentbilling.BillingLetterSend
}

// LetterMailings reads the mail state of every family of a released run
// and records the read.
func (m *BillingLetterMail) LetterMailings(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64) (parentbilling.BillingLetterMailings, error) {
	current, err := m.letterRun(ctx, reviewer, runID, false)
	if err != nil {
		return parentbilling.BillingLetterMailings{}, err
	}
	states, err := m.states(ctx, reviewer.OrganizationID, current.run.ID)
	if err != nil {
		return parentbilling.BillingLetterMailings{}, err
	}
	mailings := parentbilling.BillingLetterMailings{Documents: make([]parentbilling.BillingLetterMailing, len(current.documents))}
	for i, document := range current.documents {
		mailings.Documents[i] = documentMailing(document, current.mailings[document.ID], states)
	}
	if err := m.recordRead(ctx, reviewer, current.run.ID, map[string]any{"mail": "status"}); err != nil {
		return parentbilling.BillingLetterMailings{}, err
	}
	return mailings, nil
}

// MailBillingLetters mails the letter of every family with an address and
// no mail yet. The run is held while it does, so a second series waits and
// then sends nothing twice.
func (m *BillingLetterMail) MailBillingLetters(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64) (parentbilling.BillingLetterSeries, error) {
	current, err := m.letterRun(ctx, reviewer, runID, true)
	if err != nil {
		return parentbilling.BillingLetterSeries{}, err
	}
	files, err := m.store.RunLetterFiles(ctx, reviewer.OrganizationID, current.run.ID)
	if err != nil {
		return parentbilling.BillingLetterSeries{}, fmt.Errorf("parent billing: read letters: %w", err)
	}
	byDocument := make(map[int64]parentbilling.BillingLetterFile, len(files))
	for _, file := range files {
		byDocument[file.DocumentID] = file.File
	}
	traeger, err := m.traegerName(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.BillingLetterSeries{}, err
	}
	var series parentbilling.BillingLetterSeries
	for _, document := range current.documents {
		switch {
		case len(current.mailings[document.ID]) > 0:
			series.Already++
			continue
		case strings.TrimSpace(document.RecipientEmail) == "":
			series.WithoutEmail++
			continue
		}
		file, ok := byDocument[document.ID]
		if !ok {
			return parentbilling.BillingLetterSeries{}, parentbilling.ErrBillingLetterNotFound
		}
		if _, err := m.send(ctx, reviewer, current, document, file, traeger); err != nil {
			return parentbilling.BillingLetterSeries{}, err
		}
		series.Queued++
	}
	if err := m.recordRead(ctx, reviewer, current.run.ID, map[string]any{
		"mail": "series", "queued": series.Queued, "already": series.Already, "without_email": series.WithoutEmail,
	}); err != nil {
		return parentbilling.BillingLetterSeries{}, err
	}
	return series, nil
}

// MailBillingLetter mails the letter of one family, first or again. A send
// while the last mail still waits is refused, so a second click sends
// nothing twice.
func (m *BillingLetterMail) MailBillingLetter(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID, documentID int64) (parentbilling.BillingLetterMailing, error) {
	if documentID <= 0 {
		return parentbilling.BillingLetterMailing{}, parentbilling.ErrBillingLetterNotFound
	}
	current, err := m.letterRun(ctx, reviewer, runID, true)
	if err != nil {
		return parentbilling.BillingLetterMailing{}, err
	}
	document, ok := findDocument(current.documents, documentID)
	if !ok {
		return parentbilling.BillingLetterMailing{}, parentbilling.ErrBillingLetterNotFound
	}
	if strings.TrimSpace(document.RecipientEmail) == "" {
		return parentbilling.BillingLetterMailing{}, parentbilling.ErrRecipientEmailMissing
	}
	states, err := m.states(ctx, reviewer.OrganizationID, current.run.ID)
	if err != nil {
		return parentbilling.BillingLetterMailing{}, err
	}
	if documentMailing(document, current.mailings[document.ID], states).Status == parentbilling.LetterMailQueued {
		return parentbilling.BillingLetterMailing{}, parentbilling.ErrLetterMailPending
	}
	file, err := m.store.DocumentLetter(ctx, reviewer.OrganizationID, current.run.ID, document.ID)
	if err != nil {
		return parentbilling.BillingLetterMailing{}, err
	}
	traeger, err := m.traegerName(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.BillingLetterMailing{}, err
	}
	mailing, err := m.send(ctx, reviewer, current, document, file, traeger)
	if err != nil {
		return parentbilling.BillingLetterMailing{}, err
	}
	// The mail just waits on the outbox.
	states[mailing.DeliveryID] = parentbilling.BillingLetterMailState{DeliveryID: mailing.DeliveryID, Status: parentbilling.LetterMailQueued}
	mailings := append(current.mailings[document.ID], mailing)
	return documentMailing(document, mailings, states), nil
}

// PrintBillingLetters combines the letters of every family of a released
// run, in the order of their invoice numbers, and records the read.
func (m *BillingLetterMail) PrintBillingLetters(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64) (parentbilling.BillingLetterFile, error) {
	current, err := m.letterRun(ctx, reviewer, runID, false)
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	files, err := m.store.RunLetterFiles(ctx, reviewer.OrganizationID, current.run.ID)
	if err != nil {
		return parentbilling.BillingLetterFile{}, fmt.Errorf("parent billing: read letters: %w", err)
	}
	if len(files) == 0 {
		return parentbilling.BillingLetterFile{}, parentbilling.ErrBillingLettersMissing
	}
	letters := make([][]byte, len(files))
	for i, file := range files {
		letters[i] = file.File.Data
	}
	name := "Bestätigungsschreiben " + current.run.PeriodName
	file, err := m.combiner.CombineLetters(letters, name, name)
	if err != nil {
		return parentbilling.BillingLetterFile{}, fmt.Errorf("parent billing: combine letters: %w", err)
	}
	if err := m.recordRead(ctx, reviewer, current.run.ID, map[string]any{"letter": "print", "document_count": len(files)}); err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	return file, nil
}

// letterRun reads a released run whose letters are created, with its
// invoices and their sends; lock holds the run.
func (m *BillingLetterMail) letterRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, lock bool) (letterRun, error) {
	if err := validReviewer(reviewer); err != nil {
		return letterRun{}, err
	}
	if runID <= 0 {
		return letterRun{}, parentbilling.ErrBillingRunNotFound
	}
	read := m.store.BillingRun
	if lock {
		read = m.store.LockBillingRun
	}
	run, err := read(ctx, reviewer.OrganizationID, runID)
	if err != nil {
		return letterRun{}, err
	}
	if run.Status != parentbilling.RunReleased {
		return letterRun{}, parentbilling.ErrBillingRunNotReleased
	}
	letters, err := m.store.RunLetters(ctx, reviewer.OrganizationID, run.ID)
	if err != nil {
		return letterRun{}, fmt.Errorf("parent billing: read letters: %w", err)
	}
	if letters == nil {
		return letterRun{}, parentbilling.ErrBillingLettersMissing
	}
	documents, err := m.store.RunDocuments(ctx, reviewer.OrganizationID, run.ID)
	if err != nil {
		return letterRun{}, fmt.Errorf("parent billing: read invoices: %w", err)
	}
	stored, err := m.store.RunMailings(ctx, reviewer.OrganizationID, run.ID)
	if err != nil {
		return letterRun{}, fmt.Errorf("parent billing: read mails: %w", err)
	}
	mailings := map[int64][]parentbilling.BillingLetterSend{}
	for _, mailing := range stored {
		mailings[mailing.DocumentID] = append(mailings[mailing.DocumentID], mailing)
	}
	return letterRun{run: run, documents: documents, mailings: mailings}, nil
}

// send puts the mail of one letter on the outbox and stores the send in
// the same Träger transaction. The key counts the sends of the letter, so
// each send is one outbox entry.
func (m *BillingLetterMail) send(ctx context.Context, reviewer parentbilling.HolidayReviewer, current letterRun, document parentbilling.BillingDocument,
	file parentbilling.BillingLetterFile, traeger string) (parentbilling.BillingLetterSend, error) {
	recipient := strings.TrimSpace(document.RecipientEmail)
	deliveryID, err := m.post.EnqueueLetterMail(ctx, parentbilling.BillingLetterMail{
		OrganizationID: reviewer.OrganizationID, RunID: current.run.ID, DocumentID: document.ID, Recipient: recipient,
		IdempotencyKey: fmt.Sprintf("parent-billing-letter:%d:%d", document.ID, len(current.mailings[document.ID])+1),
		GuardianName:   strings.TrimSpace(document.RecipientFirstName + " " + document.RecipientLastName),
		TraegerName:    traeger, PeriodName: current.run.PeriodName, Number: document.Number,
		Filename: file.Filename, PDF: file.Data,
	})
	if err != nil {
		return parentbilling.BillingLetterSend{}, fmt.Errorf("parent billing: enqueue letter mail: %w", err)
	}
	mailing, err := m.store.InsertMailing(ctx, reviewer.OrganizationID, current.run.ID, reviewer.AccountID,
		parentbilling.BillingLetterSend{DocumentID: document.ID, DeliveryID: deliveryID, RecipientEmail: recipient})
	if err != nil {
		return parentbilling.BillingLetterSend{}, fmt.Errorf("parent billing: record letter mail: %w", err)
	}
	// The send also puts the letter in the inbox of the parents portal; the
	// push that says so goes with the first send only (ADR 0126).
	if err := m.post.EnqueueDocumentNotice(ctx, parentbilling.BillingDocumentNotice{
		OrganizationID: reviewer.OrganizationID, DocumentID: document.ID, RecipientEmail: recipient,
	}); err != nil {
		return parentbilling.BillingLetterSend{}, fmt.Errorf("parent billing: enqueue document notice: %w", err)
	}
	return mailing, nil
}

// traegerName is the first line of the letterhead, the name the letter
// closes with and the mail is sent with.
func (m *BillingLetterMail) traegerName(ctx context.Context, organizationID int64) (string, error) {
	text, err := m.post.SenderText(ctx, organizationID)
	if err != nil {
		return "", fmt.Errorf("parent billing: read letterhead: %w", err)
	}
	return firstLine(textLines(text)), nil
}

// states reads the mails of a run Delivery still keeps by outbox entry.
func (m *BillingLetterMail) states(ctx context.Context, organizationID, runID int64) (map[int64]parentbilling.BillingLetterMailState, error) {
	states, err := m.post.LetterMailStates(ctx, organizationID, runID)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read mail states: %w", err)
	}
	byID := make(map[int64]parentbilling.BillingLetterMailState, len(states))
	for _, state := range states {
		byID[state.DeliveryID] = state
	}
	return byID, nil
}

func (m *BillingLetterMail) recordRead(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, details map[string]any) error {
	if err := m.store.InsertRunEvent(ctx, reviewer.OrganizationID, runID, reviewer.AccountID, runEventRead, details); err != nil {
		return fmt.Errorf("parent billing: record letter read: %w", err)
	}
	return nil
}

// documentMailing is the state of the last send of a letter. A send whose
// outbox entry Delivery no longer keeps has an unknown result.
func documentMailing(document parentbilling.BillingDocument, mailings []parentbilling.BillingLetterSend, states map[int64]parentbilling.BillingLetterMailState) parentbilling.BillingLetterMailing {
	mailing := parentbilling.BillingLetterMailing{
		DocumentID: document.ID, Number: document.Number, RecipientFirstName: document.RecipientFirstName,
		RecipientLastName: document.RecipientLastName, RecipientEmail: document.RecipientEmail,
		Status: parentbilling.LetterMailNotSent, Sends: len(mailings),
	}
	if len(mailings) == 0 {
		return mailing
	}
	last := mailings[len(mailings)-1]
	requested := last.CreatedAt
	mailing.RequestedAt = &requested
	state, ok := states[last.DeliveryID]
	if !ok {
		mailing.Status = parentbilling.LetterMailUnknown
		return mailing
	}
	mailing.Status, mailing.Attempts, mailing.SentAt = state.Status, state.Attempts, state.SentAt
	return mailing
}

func findDocument(documents []parentbilling.BillingDocument, documentID int64) (parentbilling.BillingDocument, bool) {
	for _, document := range documents {
		if document.ID == documentID {
			return document, true
		}
	}
	return parentbilling.BillingDocument{}, false
}
