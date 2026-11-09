package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The stored originals of the Jahresbescheide and their delivery (#6347,
// decision A). They take the steps of the monthly letters (ADR 0165): the
// renderer and the letterhead of the Träger, the mail kind of the letters on
// Delivery's outbox, the push "Neues Dokument vom Träger" and the check of
// the recipient against the contract on every send. The PDF is rendered once
// from the issued notice and the letterhead of that moment; every later read,
// mail and inbox entry carries these stored bytes.

// Actions of the trail of a notice run for its originals.
const (
	noticeEventLetters = "letters"
	noticeEventMail    = "mail"
)

// CareNoticeLetterStore persists the originals of the released notice runs
// of a Träger in its Träger transaction.
type CareNoticeLetterStore interface {
	familyBillingReader
	CareNoticeRun(ctx context.Context, organizationID, runID int64, lock bool) (parentbilling.CareNoticeRun, error)
	CareNoticeRunNotices(ctx context.Context, organizationID, runID int64) ([]parentbilling.CareNotice, error)
	InsertCareNoticeRunEvent(ctx context.Context, organizationID, runID, accountID int64, action string, details map[string]any) error
	// InsertCareNoticeLetter reports a second PDF of a notice as
	// ErrBillingLettersExist.
	InsertCareNoticeLetter(ctx context.Context, organizationID, runID, noticeID int64, file parentbilling.BillingLetterFile) (parentbilling.CareNoticeLetterRecord, error)
	CareNoticeLetterRecords(ctx context.Context, organizationID, runID int64, withFiles bool) ([]parentbilling.CareNoticeLetterRecord, error)
	CareNoticeLetterRecord(ctx context.Context, organizationID, runID, noticeID int64) (parentbilling.CareNoticeLetterRecord, error)
	CareNoticeLetterMailings(ctx context.Context, organizationID, runID int64) ([]parentbilling.CareNoticeLetterSend, error)
	InsertCareNoticeLetterMailing(ctx context.Context, organizationID, runID, accountID int64, send parentbilling.CareNoticeLetterSend) (parentbilling.CareNoticeLetterSend, error)
}

// CareNoticeLetterPorts are the ports the root binds for the originals.
type CareNoticeLetterPorts struct {
	Letterheads parentbilling.BillingLetterheads
	Recipients  CareLetterRecipients
	Post        parentbilling.BillingLetterPost
	NoticePost  parentbilling.CareNoticeLetterPost
}

// CareNoticeLetters serves parentbilling.CareNoticeLetterService.
type CareNoticeLetters struct {
	store       CareNoticeLetterStore
	letterheads parentbilling.BillingLetterheads
	recipients  CareLetterRecipients
	post        parentbilling.BillingLetterPost
	noticePost  parentbilling.CareNoticeLetterPost
	renderer    LetterRenderer
	combiner    LetterCombiner
}

var _ parentbilling.CareNoticeLetterService = (*CareNoticeLetters)(nil)

// NewCareNoticeLetters builds the originals of the Jahresbescheide.
func NewCareNoticeLetters(store CareNoticeLetterStore, ports CareNoticeLetterPorts, renderer LetterRenderer, combiner LetterCombiner) *CareNoticeLetters {
	if store == nil || ports.Letterheads == nil || ports.Recipients == nil || ports.Post == nil || ports.NoticePost == nil ||
		renderer == nil || combiner == nil {
		panic("parent billing: the store, the ports, the renderer and the combiner are required")
	}
	return &CareNoticeLetters{store: store, letterheads: ports.Letterheads, recipients: ports.Recipients, post: ports.Post,
		noticePost: ports.NoticePost, renderer: renderer, combiner: combiner}
}

// noticeLetterRun is a released notice run with its notices, their stored
// originals and every send.
type noticeLetterRun struct {
	run     parentbilling.CareNoticeRun
	notices []parentbilling.CareNotice
	records map[int64]parentbilling.CareNoticeLetterRecord
	sends   map[int64][]parentbilling.CareNoticeLetterSend
}

func (c noticeLetterRun) byContract(contractID int64) (parentbilling.CareNotice, bool) {
	for _, notice := range c.notices {
		if notice.ContractID == contractID {
			return notice, true
		}
	}
	return parentbilling.CareNotice{}, false
}

// load reads a released run; lock holds it for a change, withFiles reads
// the PDFs.
func (l *CareNoticeLetters) load(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, lock, withFiles bool) (noticeLetterRun, error) {
	if err := validReviewer(reviewer); err != nil {
		return noticeLetterRun{}, err
	}
	if runID <= 0 {
		return noticeLetterRun{}, parentbilling.ErrCareNoticeRunNotFound
	}
	organizationID := reviewer.OrganizationID
	run, err := l.store.CareNoticeRun(ctx, organizationID, runID, lock)
	if err != nil {
		return noticeLetterRun{}, err
	}
	if run.Status != parentbilling.RunReleased {
		return noticeLetterRun{}, parentbilling.ErrBillingRunNotReleased
	}
	current := noticeLetterRun{run: run, records: map[int64]parentbilling.CareNoticeLetterRecord{},
		sends: map[int64][]parentbilling.CareNoticeLetterSend{}}
	if current.notices, err = l.store.CareNoticeRunNotices(ctx, organizationID, run.ID); err != nil {
		return noticeLetterRun{}, err
	}
	records, err := l.store.CareNoticeLetterRecords(ctx, organizationID, run.ID, withFiles)
	if err != nil {
		return noticeLetterRun{}, err
	}
	for _, record := range records {
		current.records[record.NoticeID] = record
	}
	sends, err := l.store.CareNoticeLetterMailings(ctx, organizationID, run.ID)
	if err != nil {
		return noticeLetterRun{}, err
	}
	for _, send := range sends {
		current.sends[send.LetterID] = append(current.sends[send.LetterID], send)
	}
	return current, nil
}

// CareNoticeLetters reads the state of every notice of a released run and
// records the read with the contracts it shows.
func (l *CareNoticeLetters) CareNoticeLetters(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64) (parentbilling.CareNoticeLetters, error) {
	current, err := l.load(ctx, reviewer, runID, false, false)
	if err != nil {
		return parentbilling.CareNoticeLetters{}, err
	}
	return l.overview(ctx, reviewer, current)
}

// CreateCareNoticeLetters renders the PDF of every issued notice of the run
// without one, on the letterhead of now, and stores it. A stored PDF stays:
// a later letterhead, template, address or contract never reaches it.
func (l *CareNoticeLetters) CreateCareNoticeLetters(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64) (parentbilling.CareNoticeLetters, error) {
	current, err := l.load(ctx, reviewer, runID, true, false)
	if err != nil {
		return parentbilling.CareNoticeLetters{}, err
	}
	if len(current.notices) == 0 {
		return parentbilling.CareNoticeLetters{}, parentbilling.ErrBillingRunEmpty
	}
	var missing []parentbilling.CareNotice
	for _, notice := range current.notices {
		if _, ok := current.records[notice.ID]; !ok {
			missing = append(missing, notice)
		}
	}
	if len(missing) == 0 {
		return parentbilling.CareNoticeLetters{}, parentbilling.ErrBillingLettersExist
	}
	head, err := l.letterheads.Letterhead(ctx, reviewer)
	if err != nil {
		return parentbilling.CareNoticeLetters{}, fmt.Errorf("parent billing: read letterhead: %w", err)
	}
	created := make([]int64, 0, len(missing))
	for _, notice := range missing {
		file, err := l.renderer.RenderLetter(feeNoticeLetter(notice, head), careNoticeFilename(notice))
		if err != nil {
			return parentbilling.CareNoticeLetters{}, fmt.Errorf("parent billing: render notice letter: %w", err)
		}
		record, err := l.store.InsertCareNoticeLetter(ctx, reviewer.OrganizationID, current.run.ID, notice.ID, file)
		if err != nil {
			return parentbilling.CareNoticeLetters{}, err
		}
		current.records[notice.ID] = record
		created = append(created, notice.ID)
	}
	if err := l.store.InsertCareNoticeRunEvent(ctx, reviewer.OrganizationID, current.run.ID, reviewer.AccountID, noticeEventLetters,
		map[string]any{"notice_ids": created, "contract_ids": noticeContracts(missing)}); err != nil {
		return parentbilling.CareNoticeLetters{}, fmt.Errorf("parent billing: record notice letters: %w", err)
	}
	return l.overview(ctx, reviewer, current)
}

// CareNoticePDF reads the stored original of the notice of a contract and
// records the download.
func (l *CareNoticeLetters) CareNoticePDF(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID, contractID int64) (parentbilling.BillingLetterFile, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	if runID <= 0 {
		return parentbilling.BillingLetterFile{}, parentbilling.ErrCareNoticeRunNotFound
	}
	run, err := l.store.CareNoticeRun(ctx, reviewer.OrganizationID, runID, false)
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	notices, err := l.store.CareNoticeRunNotices(ctx, reviewer.OrganizationID, run.ID)
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	current := noticeLetterRun{run: run, notices: notices}
	notice, ok := current.byContract(contractID)
	if !ok {
		return parentbilling.BillingLetterFile{}, parentbilling.ErrCareNoticeNotFound
	}
	record, err := l.store.CareNoticeLetterRecord(ctx, reviewer.OrganizationID, run.ID, notice.ID)
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	if err := l.recordRead(ctx, reviewer, run.ID, map[string]any{"letter": "download", "letter_id": record.ID,
		"contract_ids": []int64{contractID}}); err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	return *record.File, nil
}

// MailCareNoticeLetters mails every stored notice of the run with an
// address, no mail yet and a recipient the contract still bills. The run is
// held while it does, so a second series waits and then sends nothing twice.
func (l *CareNoticeLetters) MailCareNoticeLetters(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64) (parentbilling.BillingLetterSeries, error) {
	current, err := l.mailRun(ctx, reviewer, runID)
	if err != nil {
		return parentbilling.BillingLetterSeries{}, err
	}
	recipients, err := effectiveRecipients(ctx, l.recipients, l.store, reviewer, noticeContracts(current.notices))
	if err != nil {
		return parentbilling.BillingLetterSeries{}, err
	}
	traeger, err := l.traegerName(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.BillingLetterSeries{}, err
	}
	var series parentbilling.BillingLetterSeries
	var queued []int64
	for _, notice := range current.notices {
		record, ok := current.records[notice.ID]
		switch {
		case !ok:
			continue
		case len(current.sends[record.ID]) > 0:
			series.Already++
			continue
		case strings.TrimSpace(notice.Recipient.Email) == "":
			series.WithoutEmail++
			continue
		case !sameNoticeRecipient(notice, recipients):
			series.RecipientChanged++
			continue
		}
		if err := l.send(ctx, reviewer, &current, notice, traeger); err != nil {
			return parentbilling.BillingLetterSeries{}, err
		}
		series.Queued++
		queued = append(queued, record.ID)
	}
	if err := l.store.InsertCareNoticeRunEvent(ctx, reviewer.OrganizationID, current.run.ID, reviewer.AccountID, noticeEventMail,
		map[string]any{"queued": series.Queued, "already": series.Already, "without_email": series.WithoutEmail,
			"recipient_changed": series.RecipientChanged, "letter_ids": queued}); err != nil {
		return parentbilling.BillingLetterSeries{}, fmt.Errorf("parent billing: record notice mails: %w", err)
	}
	return series, nil
}

// MailCareNoticeLetter mails the notice of a contract, first or again. A send
// while the last mail still waits or is delayed, to an address the provider
// holds back, without an address or to a party the contract no longer bills
// is refused, so a second click sends nothing twice.
func (l *CareNoticeLetters) MailCareNoticeLetter(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID, contractID int64) (parentbilling.CareNoticeMailing, error) {
	current, err := l.mailRun(ctx, reviewer, runID)
	if err != nil {
		return parentbilling.CareNoticeMailing{}, err
	}
	notice, ok := current.byContract(contractID)
	if !ok {
		return parentbilling.CareNoticeMailing{}, parentbilling.ErrCareNoticeNotFound
	}
	record, ok := current.records[notice.ID]
	if !ok {
		return parentbilling.CareNoticeMailing{}, parentbilling.ErrBillingLettersMissing
	}
	if strings.TrimSpace(notice.Recipient.Email) == "" {
		return parentbilling.CareNoticeMailing{}, parentbilling.ErrRecipientEmailMissing
	}
	states, err := l.states(ctx, reviewer.OrganizationID, current.run.ID)
	if err != nil {
		return parentbilling.CareNoticeMailing{}, err
	}
	last := []parentbilling.CareNoticeMailing{noticeMailing(current, notice, states)}
	if err := l.refuseWhileWaiting(ctx, last); err != nil {
		return parentbilling.CareNoticeMailing{}, err
	}
	recipients, err := effectiveRecipients(ctx, l.recipients, l.store, reviewer, []int64{notice.ContractID})
	if err != nil {
		return parentbilling.CareNoticeMailing{}, err
	}
	if !sameNoticeRecipient(notice, recipients) {
		return parentbilling.CareNoticeMailing{}, parentbilling.ErrCareRecipientChanged
	}
	traeger, err := l.traegerName(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.CareNoticeMailing{}, err
	}
	if err := l.send(ctx, reviewer, &current, notice, traeger); err != nil {
		return parentbilling.CareNoticeMailing{}, err
	}
	sends := current.sends[record.ID]
	if err := l.store.InsertCareNoticeRunEvent(ctx, reviewer.OrganizationID, current.run.ID, reviewer.AccountID, noticeEventMail,
		map[string]any{"queued": 1, "letter_ids": []int64{record.ID}}); err != nil {
		return parentbilling.CareNoticeMailing{}, fmt.Errorf("parent billing: record notice mail: %w", err)
	}
	// The mail just waits on the outbox.
	latest := sends[len(sends)-1]
	states[latest.DeliveryID] = parentbilling.BillingLetterMailState{DeliveryID: latest.DeliveryID, Status: parentbilling.LetterMailQueued}
	return noticeMailing(current, notice, states), nil
}

// refuseWhileWaiting refuses another mail while the last one still waits, is
// delayed, or the provider still holds the address back.
func (l *CareNoticeLetters) refuseWhileWaiting(ctx context.Context, last []parentbilling.CareNoticeMailing) error {
	switch {
	case last[0].Status == parentbilling.LetterMailQueued:
		return parentbilling.ErrLetterMailPending
	case last[0].MailOutcome == parentbilling.LetterMailOutcomeDelayed:
		return parentbilling.ErrLetterMailDelayed
	}
	l.readProviderBlocks(ctx, last)
	switch last[0].ProviderBlock {
	case parentbilling.LetterProviderBlockActive:
		return parentbilling.ErrLetterMailBlocked
	case parentbilling.LetterProviderBlockUnreadable:
		return parentbilling.ErrLetterMailBlockUnreadable
	}
	return nil
}

// PrintCareNoticeLetters combines the stored originals of the run in the
// order of their numbers; CareLetterPrintWithoutEmail keeps only the notices
// without an address.
func (l *CareNoticeLetters) PrintCareNoticeLetters(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, only string) (parentbilling.BillingLetterFile, error) {
	if only != "" && only != parentbilling.CareLetterPrintWithoutEmail {
		return parentbilling.BillingLetterFile{}, validLetterKind("")
	}
	current, err := l.load(ctx, reviewer, runID, false, true)
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	var pdfs [][]byte
	var printed []int64
	for _, notice := range current.notices {
		record, ok := current.records[notice.ID]
		if !ok || (only == parentbilling.CareLetterPrintWithoutEmail && strings.TrimSpace(notice.Recipient.Email) != "") {
			continue
		}
		pdfs, printed = append(pdfs, record.File.Data), append(printed, record.ID)
	}
	if len(pdfs) == 0 {
		return parentbilling.BillingLetterFile{}, parentbilling.ErrBillingLetterNotFound
	}
	name := fmt.Sprintf("Jahresbescheide %d", current.run.Year)
	file, err := l.combiner.CombineLetters(pdfs, name, name)
	if err != nil {
		return parentbilling.BillingLetterFile{}, fmt.Errorf("parent billing: combine notice letters: %w", err)
	}
	if err := l.recordRead(ctx, reviewer, current.run.ID, map[string]any{"letter": "print", "only": only, "letter_ids": printed,
		"contract_ids": noticeContracts(current.notices)}); err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	return file, nil
}

func (l *CareNoticeLetters) mailRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64) (noticeLetterRun, error) {
	current, err := l.load(ctx, reviewer, runID, true, true)
	if err != nil {
		return noticeLetterRun{}, err
	}
	if len(current.records) == 0 {
		return noticeLetterRun{}, parentbilling.ErrBillingLettersMissing
	}
	return current, nil
}

// send puts the mail with the stored PDF on the outbox and stores the send in
// the same Träger transaction. The key counts the sends of the letter, so
// each send is one outbox entry; only the first send pushes to the inbox.
func (l *CareNoticeLetters) send(ctx context.Context, reviewer parentbilling.HolidayReviewer, current *noticeLetterRun,
	notice parentbilling.CareNotice, traeger string,
) error {
	record := current.records[notice.ID]
	recipient := strings.TrimSpace(notice.Recipient.Email)
	earlier := len(current.sends[record.ID])
	mailKind := parentbilling.MailDocumentCareFeeNotice
	if notice.Kind == parentbilling.NoticeKindChange {
		mailKind = parentbilling.MailDocumentCareChangeNotice
	}
	deliveryID, err := l.noticePost.EnqueueCareNoticeLetterMail(ctx, parentbilling.BillingLetterMail{
		OrganizationID: reviewer.OrganizationID, RunID: current.run.ID, DocumentID: record.ID, Recipient: recipient,
		IdempotencyKey: fmt.Sprintf("parent-billing-care-notice-letter:%d:%d", record.ID, earlier+1),
		GuardianName:   fullName(notice.Recipient.FirstName, notice.Recipient.LastName),
		TraegerName:    traeger, PeriodName: noticePeriodName(notice), Number: notice.Number,
		DocumentKind: mailKind, LetterKind: parentbilling.DocumentKindFeeNotice, Filename: record.File.Filename, PDF: record.File.Data,
	})
	if err != nil {
		return fmt.Errorf("parent billing: enqueue notice letter mail: %w", err)
	}
	send, err := l.store.InsertCareNoticeLetterMailing(ctx, reviewer.OrganizationID, current.run.ID, reviewer.AccountID,
		parentbilling.CareNoticeLetterSend{LetterID: record.ID, DeliveryID: deliveryID, RecipientEmail: recipient})
	if err != nil {
		return err
	}
	current.sends[record.ID] = append(current.sends[record.ID], send)
	if earlier > 0 {
		return nil
	}
	if err := l.post.EnqueueDocumentNotice(ctx, parentbilling.BillingDocumentNotice{
		OrganizationID: reviewer.OrganizationID, DocumentID: record.ID, RecipientEmail: recipient,
	}); err != nil {
		return fmt.Errorf("parent billing: enqueue document notice: %w", err)
	}
	return nil
}

// noticePeriodName names a notice as its mail and the inbox do:
// "Betreuung 2030".
func noticePeriodName(notice parentbilling.CareNotice) string {
	return fmt.Sprintf("Betreuung %d", notice.Year)
}

// sameNoticeRecipient tells whether the address the notice copied is still
// the effective billing address of the contract's family. A notice has no
// change of its own recipient, so the party of today decides alone.
func sameNoticeRecipient(notice parentbilling.CareNotice, recipients map[int64]parentbilling.CareRecipient) bool {
	now, ok := recipients[notice.ContractID]
	return ok && strings.EqualFold(strings.TrimSpace(now.Email), strings.TrimSpace(notice.Recipient.Email))
}

// overview builds the state of every notice and records the read.
func (l *CareNoticeLetters) overview(ctx context.Context, reviewer parentbilling.HolidayReviewer, current noticeLetterRun) (parentbilling.CareNoticeLetters, error) {
	states, err := l.states(ctx, reviewer.OrganizationID, current.run.ID)
	if err != nil {
		return parentbilling.CareNoticeLetters{}, err
	}
	mailings := make([]parentbilling.CareNoticeMailing, 0, len(current.notices))
	for _, notice := range current.notices {
		mailings = append(mailings, noticeMailing(current, notice, states))
	}
	l.readProviderBlocks(ctx, mailings)
	if err := l.recordRead(ctx, reviewer, current.run.ID, map[string]any{"letters": "status",
		"contract_ids": noticeContracts(current.notices)}); err != nil {
		return parentbilling.CareNoticeLetters{}, err
	}
	return parentbilling.CareNoticeLetters{Notices: mailings, Summary: noticeSummary(mailings)}, nil
}

// noticeMailing is a notice with its original and the state of its last
// send. A send whose outbox entry Delivery no longer keeps has an unknown
// result.
func noticeMailing(current noticeLetterRun, notice parentbilling.CareNotice, states map[int64]parentbilling.BillingLetterMailState) parentbilling.CareNoticeMailing {
	entry := parentbilling.CareNoticeMailing{
		NoticeID: notice.ID, ContractID: notice.ContractID, Kind: notice.Kind, Number: notice.Number,
		ChildFirstName: notice.ChildFirstName, ChildLastName: notice.ChildLastName, SchoolName: notice.SchoolName,
		RecipientFirstName: notice.Recipient.FirstName, RecipientLastName: notice.Recipient.LastName,
		RecipientEmail: notice.Recipient.Email, TotalCents: notice.TotalCents, InInbox: notice.StudentID != nil,
		Status: parentbilling.LetterMailNotSent,
	}
	if notice.Amends != nil {
		entry.AmendsNumber = notice.Amends.Number
	}
	if notice.AmendedBy != nil {
		entry.AmendedBy = notice.AmendedBy.Number
	}
	record, ok := current.records[notice.ID]
	if !ok {
		return entry
	}
	created := record.CreatedAt
	entry.HasLetter, entry.SHA256, entry.CreatedAt = true, record.SHA256, &created
	sends := current.sends[record.ID]
	entry.Sends = len(sends)
	if len(sends) == 0 {
		return entry
	}
	last := sends[len(sends)-1]
	requested := last.CreatedAt
	entry.RequestedAt = &requested
	state, known := states[last.DeliveryID]
	if !known {
		entry.Status = parentbilling.LetterMailUnknown
		return entry
	}
	entry.Status, entry.Attempts, entry.SentAt, entry.MailOutcome = state.Status, state.Attempts, state.SentAt, state.Outcome
	return entry
}

// noticeSummary counts the notices by their original and their last mail.
func noticeSummary(mailings []parentbilling.CareNoticeMailing) parentbilling.CareNoticeLetterSummary {
	summary := parentbilling.CareNoticeLetterSummary{Notices: len(mailings)}
	for _, mailing := range mailings {
		if !mailing.HasLetter {
			continue
		}
		summary.Letters++
		if !mailing.InInbox {
			summary.WithoutInbox++
		}
		switch {
		case careMailFailed(mailing.MailOutcome):
			summary.Failed++
		case mailing.MailOutcome == "delayed" || mailing.MailOutcome == "overdue":
			summary.Queued++
		case mailing.Status == parentbilling.LetterMailSent:
			summary.Sent++
		case mailing.Status == parentbilling.LetterMailQueued:
			summary.Queued++
		case mailing.Status == parentbilling.LetterMailFailed || mailing.Status == parentbilling.LetterMailUnknown:
			summary.Failed++
		default:
			summary.NotSent++
			if strings.TrimSpace(mailing.RecipientEmail) == "" {
				summary.WithoutEmail++
			}
		}
	}
	return summary
}

func (l *CareNoticeLetters) states(ctx context.Context, organizationID, runID int64) (map[int64]parentbilling.BillingLetterMailState, error) {
	states, err := l.noticePost.CareNoticeLetterMailStates(ctx, organizationID, runID)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read notice mail states: %w", err)
	}
	byID := make(map[int64]parentbilling.BillingLetterMailState, len(states))
	for _, state := range states {
		byID[state.DeliveryID] = state
	}
	return byID, nil
}

// readProviderBlocks asks once per address, and only about notices whose
// last mail left a block at the provider.
func (l *CareNoticeLetters) readProviderBlocks(ctx context.Context, mailings []parentbilling.CareNoticeMailing) {
	var addresses []string
	for _, mailing := range mailings {
		if parentbilling.LetterMailOutcomeHeldBack(mailing.MailOutcome) {
			addresses = append(addresses, mailing.RecipientEmail)
		}
	}
	if len(addresses) == 0 {
		return
	}
	blocks := l.post.LetterMailBlocks(ctx, addresses)
	for i := range mailings {
		if parentbilling.LetterMailOutcomeHeldBack(mailings[i].MailOutcome) {
			mailings[i].ProviderBlock = blocks[strings.ToLower(strings.TrimSpace(mailings[i].RecipientEmail))]
		}
	}
}

func (l *CareNoticeLetters) traegerName(ctx context.Context, organizationID int64) (string, error) {
	text, err := l.post.SenderText(ctx, organizationID)
	if err != nil {
		return "", fmt.Errorf("parent billing: read letterhead: %w", err)
	}
	return firstLine(textLines(text)), nil
}

func (l *CareNoticeLetters) recordRead(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, details map[string]any) error {
	if err := l.store.InsertCareNoticeRunEvent(ctx, reviewer.OrganizationID, runID, reviewer.AccountID, runEventRead, details); err != nil {
		return fmt.Errorf("parent billing: record notice letter read: %w", err)
	}
	return nil
}
