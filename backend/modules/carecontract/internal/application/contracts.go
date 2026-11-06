package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// maxSendBatch bounds a send of several contracts in one request.
const maxSendBatch = 200

// Contracts serves carecontract.Contracts in the caller's Träger
// transaction.
type Contracts struct {
	store       Store
	letterheads carecontract.Letterheads
	renderer    LetterRenderer
	post        carecontract.Post
	clock       Clock
}

var _ carecontract.Contracts = (*Contracts)(nil)

// NewContracts builds the contracts of the Träger-Büro.
func NewContracts(store Store, letterheads carecontract.Letterheads, renderer LetterRenderer, post carecontract.Post, clock Clock) *Contracts {
	if store == nil || letterheads == nil || renderer == nil || post == nil {
		panic("care contract: the store, the letterhead, the renderer and the post are required")
	}
	return &Contracts{store: store, letterheads: letterheads, renderer: renderer, post: post, clock: clock}
}

// ListContracts lists the contracts of the filter and records the read of
// every contract it shows.
func (c *Contracts) ListContracts(ctx context.Context, reviewer carecontract.Reviewer, filter carecontract.ContractFilter) (carecontract.ContractList, error) {
	if err := checkReviewer(reviewer); err != nil {
		return carecontract.ContractList{}, err
	}
	if filter.Status != "" && !knownStatus(filter.Status) {
		return carecontract.ContractList{}, &carecontract.ValidationError{Fields: map[string]string{"status": "unknown"}}
	}
	if filter.Offset < 0 || filter.Limit < 0 || filter.Limit > 100 {
		return carecontract.ContractList{}, &carecontract.ValidationError{Fields: map[string]string{"contracts": "invalid_pagination"}}
	}
	if filter.Limit == 0 {
		filter.Limit = 50
	}
	filter.Search = strings.TrimSpace(filter.Search)
	contracts, err := c.store.ListContracts(ctx, reviewer.OrganizationID, filter)
	if err != nil {
		return carecontract.ContractList{}, fmt.Errorf("care contract: list: %w", err)
	}
	hasMore := len(contracts) > filter.Limit
	if hasMore {
		contracts = contracts[:filter.Limit]
	}
	counts, err := c.store.CountContracts(ctx, reviewer.OrganizationID)
	if err != nil {
		return carecontract.ContractList{}, fmt.Errorf("care contract: count: %w", err)
	}
	if len(contracts) > 0 {
		ids := make([]int64, len(contracts))
		for i, contract := range contracts {
			ids[i] = contract.ID
		}
		if err := c.store.InsertEvents(ctx, reviewer.OrganizationID, []Event{{
			AccountID: reviewer.AccountID, Action: ActionRead,
			Details: map[string]any{"view": "list", "contract_ids": ids},
		}}); err != nil {
			return carecontract.ContractList{}, fmt.Errorf("care contract: record list read: %w", err)
		}
	}
	return carecontract.ContractList{Contracts: contracts, Counts: counts, HasMore: hasMore}, nil
}

// Contract reads one contract with the state of its mails.
func (c *Contracts) Contract(ctx context.Context, reviewer carecontract.Reviewer, contractID int64) (carecontract.ContractDetail, error) {
	if err := checkReviewer(reviewer); err != nil {
		return carecontract.ContractDetail{}, err
	}
	record, err := c.store.Contract(ctx, reviewer.OrganizationID, contractID, false)
	if err != nil {
		return carecontract.ContractDetail{}, err
	}
	detail, err := c.detail(ctx, reviewer, record)
	if err != nil {
		return carecontract.ContractDetail{}, err
	}
	if err := c.record(ctx, reviewer, record, ActionRead, map[string]any{"view": "detail"}); err != nil {
		return carecontract.ContractDetail{}, err
	}
	return detail, nil
}

func (c *Contracts) detail(ctx context.Context, reviewer carecontract.Reviewer, record ContractRecord) (carecontract.ContractDetail, error) {
	mailings, err := c.mailings(ctx, reviewer.OrganizationID, record.Summary.ID)
	if err != nil {
		return carecontract.ContractDetail{}, err
	}
	return carecontract.ContractDetail{
		ContractSummary: record.Summary, Version: record.Version, Parties: record.Parties,
		Mailings: mailings, DocumentFrozen: record.DocumentID != nil,
	}, nil
}

// mailings are the sends of the contract, newest first, with the state of
// their mail. A mail the outbox no longer holds is unknown (ADR 0123).
func (c *Contracts) mailings(ctx context.Context, organizationID, contractID int64) ([]carecontract.Mailing, error) {
	stored, err := c.store.Mailings(ctx, organizationID, contractID)
	if err != nil || len(stored) == 0 {
		return []carecontract.Mailing{}, err
	}
	states, err := c.post.ContractMailStates(ctx, organizationID, contractID)
	if err != nil {
		return nil, fmt.Errorf("care contract: mail states: %w", err)
	}
	byOutbox := make(map[int64]string, len(states))
	for _, state := range states {
		byOutbox[state.OutboxID] = state.State
	}
	mailings := make([]carecontract.Mailing, len(stored))
	for i, mailing := range stored {
		state, ok := byOutbox[mailing.OutboxID]
		if !ok {
			state = carecontract.MailUnknown
		}
		mailings[i] = carecontract.Mailing{Recipient: mailing.Recipient, SentAt: mailing.SentAt, State: state, OutboxID: mailing.OutboxID}
	}
	return mailings, nil
}

// ContractDocument answers the stored PDF of a sent contract and, for a
// draft, a preview with the current template that stores nothing.
func (c *Contracts) ContractDocument(ctx context.Context, reviewer carecontract.Reviewer, contractID int64) (carecontract.File, error) {
	if err := checkReviewer(reviewer); err != nil {
		return carecontract.File{}, err
	}
	record, err := c.store.Contract(ctx, reviewer.OrganizationID, contractID, false)
	if err != nil {
		return carecontract.File{}, err
	}
	var file carecontract.File
	if record.DocumentID != nil {
		file, err = c.store.Document(ctx, reviewer.OrganizationID, contractID)
	} else {
		file, _, err = c.render(ctx, reviewer, record)
	}
	if err != nil {
		return carecontract.File{}, err
	}
	view := "document"
	if record.DocumentID == nil {
		view = "preview"
	}
	if err := c.record(ctx, reviewer, record, ActionRead, map[string]any{"view": view}); err != nil {
		return carecontract.File{}, err
	}
	return file, nil
}

func (c *Contracts) render(ctx context.Context, reviewer carecontract.Reviewer, record ContractRecord) (carecontract.File, int, error) {
	head, err := c.letterheads.Letterhead(ctx, reviewer)
	if err != nil {
		return carecontract.File{}, 0, fmt.Errorf("care contract: letterhead: %w", err)
	}
	file, err := c.renderer.RenderContract(contractLetter(record, head), documentFilenameBase(record))
	if err != nil {
		return carecontract.File{}, 0, fmt.Errorf("care contract: render: %w", err)
	}
	return file, head.Template.Version, nil
}

// SendContracts sends every contract of the list once. A contract that
// cannot be sent is left out with its reason; the others go.
func (c *Contracts) SendContracts(ctx context.Context, reviewer carecontract.Reviewer, contractIDs []int64) (carecontract.SendResult, error) {
	if err := checkReviewer(reviewer); err != nil {
		return carecontract.SendResult{}, err
	}
	ids := uniqueIDs(contractIDs)
	if len(ids) == 0 || len(ids) > maxSendBatch {
		return carecontract.SendResult{}, &carecontract.ValidationError{Fields: map[string]string{"contract_ids": "count"}}
	}
	result := carecontract.SendResult{Sent: []int64{}, Skipped: []carecontract.SendSkip{}}
	for _, id := range ids {
		err := c.send(ctx, reviewer, id)
		switch {
		case err == nil:
			result.Sent = append(result.Sent, id)
		case errors.Is(err, carecontract.ErrRecipientMissing):
			result.Skipped = append(result.Skipped, carecontract.SendSkip{ID: id, Reason: carecontract.SkipRecipientMissing})
		case errors.Is(err, carecontract.ErrMailPending):
			result.Skipped = append(result.Skipped, carecontract.SendSkip{ID: id, Reason: carecontract.SkipMailPending})
		case errors.Is(err, carecontract.ErrContractNotSendable):
			result.Skipped = append(result.Skipped, carecontract.SendSkip{ID: id, Reason: carecontract.SkipNotSendable})
		default:
			return carecontract.SendResult{}, err
		}
	}
	return result, nil
}

// send freezes the PDF of a draft and sends it, or sends the stored PDF of
// a sent contract again. The contract row is locked first, so two sends of
// one contract run one after the other and freeze one PDF.
func (c *Contracts) send(ctx context.Context, reviewer carecontract.Reviewer, contractID int64) error {
	record, err := c.store.Contract(ctx, reviewer.OrganizationID, contractID, true)
	if err != nil {
		return err
	}
	if record.Summary.Status != carecontract.StatusDraft && record.Summary.Status != carecontract.StatusSent {
		return carecontract.ErrContractNotSendable
	}
	party, ok := recipient(record.Parties)
	if !ok {
		return carecontract.ErrRecipientMissing
	}
	if err := c.checkNoPendingMail(ctx, reviewer.OrganizationID, contractID); err != nil {
		return err
	}
	file, documentID, err := c.frozenDocument(ctx, reviewer, record)
	if err != nil {
		return err
	}
	head, err := c.letterheads.Letterhead(ctx, reviewer)
	if err != nil {
		return fmt.Errorf("care contract: letterhead: %w", err)
	}
	stored, err := c.store.Mailings(ctx, reviewer.OrganizationID, contractID)
	if err != nil {
		return err
	}
	outboxID, err := c.post.EnqueueContractMail(ctx, carecontract.ContractMail{
		OrganizationID: reviewer.OrganizationID, ContractID: contractID, Recipient: party.Email,
		GuardianName: fullName(party.FirstName, party.LastName), TraegerName: firstLine(textLines(head.SenderText)),
		ChildName: fullName(record.Version.ChildFirstName, record.Version.ChildLastName),
		Modules:   joinGerman(ModuleNames(record.Version.Services)), Filename: file.Filename, PDF: file.Data,
		IdempotencyKey: fmt.Sprintf("care-contract:%d:%d", contractID, len(stored)+1),
	})
	if err != nil {
		return fmt.Errorf("care contract: enqueue mail: %w", err)
	}
	if err := c.store.InsertMailing(ctx, reviewer.OrganizationID, reviewer.AccountID, contractID, documentID, outboxID, party.Email); err != nil {
		return err
	}
	if record.Summary.Status == carecontract.StatusDraft {
		if err := c.store.MarkSent(ctx, reviewer.OrganizationID, contractID, c.clock.now()); err != nil {
			return err
		}
	}
	return c.record(ctx, reviewer, record, ActionSend, map[string]any{"document_id": documentID, "again": len(stored) > 0})
}

func (c *Contracts) checkNoPendingMail(ctx context.Context, organizationID, contractID int64) error {
	mailings, err := c.mailings(ctx, organizationID, contractID)
	if err != nil {
		return err
	}
	if len(mailings) > 0 && mailings[0].State == carecontract.MailQueued {
		return carecontract.ErrMailPending
	}
	return nil
}

// frozenDocument is the stored PDF, rendered and stored at the first send.
func (c *Contracts) frozenDocument(ctx context.Context, reviewer carecontract.Reviewer, record ContractRecord) (carecontract.File, int64, error) {
	if record.DocumentID != nil {
		file, err := c.store.Document(ctx, reviewer.OrganizationID, record.Summary.ID)
		return file, *record.DocumentID, err
	}
	file, templateVersion, err := c.render(ctx, reviewer, record)
	if err != nil {
		return carecontract.File{}, 0, err
	}
	id, err := c.store.InsertDocument(ctx, reviewer.OrganizationID, reviewer.AccountID, record, templateVersion, file)
	return file, id, err
}

// DiscardContract removes a draft. The trail keeps the entry.
func (c *Contracts) DiscardContract(ctx context.Context, reviewer carecontract.Reviewer, contractID int64) error {
	if err := checkReviewer(reviewer); err != nil {
		return err
	}
	record, err := c.store.Contract(ctx, reviewer.OrganizationID, contractID, true)
	if err != nil {
		return err
	}
	if record.Summary.Status != carecontract.StatusDraft {
		return carecontract.ErrContractNotDraft
	}
	deleted, err := c.store.DeleteDraft(ctx, reviewer.OrganizationID, contractID)
	if err != nil {
		return err
	}
	if !deleted {
		return carecontract.ErrContractNotDraft
	}
	return c.record(ctx, reviewer, record, ActionDiscard, map[string]any{"status": record.Summary.Status})
}

func (c *Contracts) record(ctx context.Context, reviewer carecontract.Reviewer, record ContractRecord, action string, details map[string]any) error {
	contractID, schoolID := record.Summary.ID, record.Summary.SchoolID
	if err := c.store.InsertEvents(ctx, reviewer.OrganizationID, []Event{{
		ContractID: &contractID, SchoolID: &schoolID, AccountID: reviewer.AccountID, Action: action, Details: details,
	}}); err != nil {
		return fmt.Errorf("care contract: record %s: %w", action, err)
	}
	return nil
}

// recipient is the first party with an address.
func recipient(parties []carecontract.Party) (carecontract.Party, bool) {
	for _, party := range parties {
		if strings.TrimSpace(party.Email) != "" {
			return party, true
		}
	}
	return carecontract.Party{}, false
}

func checkReviewer(reviewer carecontract.Reviewer) error {
	if reviewer.OrganizationID <= 0 || reviewer.AccountID <= 0 {
		return errors.New("care contract: the reviewer needs a Träger and an account")
	}
	return nil
}

func knownStatus(status string) bool {
	return slices.Contains([]string{
		carecontract.StatusDraft, carecontract.StatusSent, carecontract.StatusSigned,
		carecontract.StatusActive, carecontract.StatusEnded,
	}, status)
}

func uniqueIDs(ids []int64) []int64 {
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 && !slices.Contains(result, id) {
			result = append(result, id)
		}
	}
	return result
}
