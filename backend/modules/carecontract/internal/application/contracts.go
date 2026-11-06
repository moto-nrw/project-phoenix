package application

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// maxSendBatch bounds a send of several contracts in one request.
const maxSendBatch = 200

// Contracts serves carecontract.Contracts. The writes run in the caller's
// Träger transaction. The reads (list, detail, PDF) open their own units: the
// Träger transaction for the contracts, then the tenant transaction of every
// school whose contracts they showed, for that school's access log (ADR 0088
// section 7, ADR 0084 section 7). No transaction holds both (ADR 0105).
type Contracts struct {
	store       Store
	letterheads carecontract.Letterheads
	renderer    LetterRenderer
	post        carecontract.Post
	clock       Clock
	runtime     carecontract.Runtime
	access      carecontract.SchoolAccessLog
}

// ContractPorts are the root-bound ports of the contracts.
type ContractPorts struct {
	Letterheads carecontract.Letterheads
	Post        carecontract.Post
	Runtime     carecontract.Runtime
	Access      carecontract.SchoolAccessLog
}

var _ carecontract.Contracts = (*Contracts)(nil)

// NewContracts builds the contracts of the Träger-Büro.
func NewContracts(store Store, renderer LetterRenderer, ports ContractPorts, clock Clock) *Contracts {
	if store == nil || renderer == nil || ports.Letterheads == nil || ports.Post == nil || ports.Runtime == nil || ports.Access == nil {
		panic("care contract: the store, the renderer, the letterhead, the post, the runtime and the access log are required")
	}
	return &Contracts{store: store, letterheads: ports.Letterheads, renderer: renderer, post: ports.Post, clock: clock,
		runtime: ports.Runtime, access: ports.Access}
}

// ListContracts lists the contracts of the filter, records the read in the
// trail and in the access log of every school it shows contracts of.
func (c *Contracts) ListContracts(ctx context.Context, reviewer carecontract.Reviewer, filter carecontract.ContractFilter) (carecontract.ContractList, error) {
	if err := checkReviewer(reviewer); err != nil {
		return carecontract.ContractList{}, err
	}
	var list carecontract.ContractList
	err := c.runtime.WithinTraeger(ctx, reviewer.OrganizationID, func(txCtx context.Context) error {
		var err error
		list, err = c.listContracts(txCtx, reviewer, filter)
		return err
	})
	if err != nil {
		return carecontract.ContractList{}, err
	}
	bySchool := map[int64][]int64{}
	for _, contract := range list.Contracts {
		bySchool[contract.SchoolID] = append(bySchool[contract.SchoolID], contract.ID)
	}
	if err := c.logSchools(ctx, reviewer, "care_contract_list", bySchool); err != nil {
		return carecontract.ContractList{}, err
	}
	return list, nil
}

// Contract reads one contract with the state of its mails and records the
// read in the access log of its school.
func (c *Contracts) Contract(ctx context.Context, reviewer carecontract.Reviewer, contractID int64) (carecontract.ContractDetail, error) {
	if err := checkReviewer(reviewer); err != nil {
		return carecontract.ContractDetail{}, err
	}
	var detail carecontract.ContractDetail
	err := c.runtime.WithinTraeger(ctx, reviewer.OrganizationID, func(txCtx context.Context) error {
		var err error
		detail, err = c.contract(txCtx, reviewer, contractID)
		return err
	})
	if err != nil {
		return carecontract.ContractDetail{}, err
	}
	return detail, c.logSchools(ctx, reviewer, "care_contract_detail", map[int64][]int64{detail.SchoolID: {detail.ID}})
}

// ContractDocument answers the PDF and records the read in the access log
// of the contract's school.
func (c *Contracts) ContractDocument(ctx context.Context, reviewer carecontract.Reviewer, contractID int64) (carecontract.File, error) {
	if err := checkReviewer(reviewer); err != nil {
		return carecontract.File{}, err
	}
	var file carecontract.File
	var schoolID int64
	err := c.runtime.WithinTraeger(ctx, reviewer.OrganizationID, func(txCtx context.Context) error {
		var err error
		file, schoolID, err = c.contractDocument(txCtx, reviewer, contractID)
		return err
	})
	if err != nil {
		return carecontract.File{}, err
	}
	return file, c.logSchools(ctx, reviewer, "care_contract_document", map[int64][]int64{schoolID: {contractID}})
}

// logSchools writes one entry per school into that school's access log, in
// the school's own tenant transaction, after the Träger transaction ended.
// A failed entry fails the read: no contract leaves without its entry.
func (c *Contracts) logSchools(ctx context.Context, reviewer carecontract.Reviewer, view string, bySchool map[int64][]int64) error {
	schools := make([]int64, 0, len(bySchool))
	for school := range bySchool {
		if school > 0 {
			schools = append(schools, school)
		}
	}
	slices.Sort(schools)
	for _, school := range schools {
		ids := bySchool[school]
		err := c.runtime.WithinSchool(ctx, school, func(txCtx context.Context) error {
			return c.access.RecordAccess(txCtx, carecontract.SchoolAccess{
				OrganizationID: reviewer.OrganizationID, AccountID: reviewer.AccountID,
				Data: carecontract.SchoolAccessContracts, View: view, Rows: len(ids), ContractIDs: ids,
			})
		})
		if err != nil {
			return fmt.Errorf("care contract: access log of school %d: %w", school, err)
		}
	}
	return nil
}

func (c *Contracts) listContracts(ctx context.Context, reviewer carecontract.Reviewer, filter carecontract.ContractFilter) (carecontract.ContractList, error) {
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

func (c *Contracts) contract(ctx context.Context, reviewer carecontract.Reviewer, contractID int64) (carecontract.ContractDetail, error) {
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
	versions, err := c.store.Versions(ctx, reviewer.OrganizationID, record.Summary.ID)
	if err != nil {
		return carecontract.ContractDetail{}, err
	}
	return carecontract.ContractDetail{
		ContractSummary: record.Summary, Version: record.Version, Parties: record.Parties,
		Mailings: mailings, DocumentFrozen: record.DocumentID != nil, Versions: versions,
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

// contractDocument answers the stored PDF of a sent contract and, for a
// draft, a preview with the current template that stores nothing, with the
// contract's school.
func (c *Contracts) contractDocument(ctx context.Context, reviewer carecontract.Reviewer, contractID int64) (carecontract.File, int64, error) {
	if err := checkReviewer(reviewer); err != nil {
		return carecontract.File{}, 0, err
	}
	record, err := c.store.Contract(ctx, reviewer.OrganizationID, contractID, false)
	if err != nil {
		return carecontract.File{}, 0, err
	}
	var file carecontract.File
	if record.DocumentID != nil {
		file, _, err = c.store.VersionDocument(ctx, reviewer.OrganizationID, record.Version.ID)
	} else {
		file, _, err = c.render(ctx, reviewer, record)
	}
	if err != nil {
		return carecontract.File{}, 0, err
	}
	view := "document"
	if record.DocumentID == nil {
		view = "preview"
	}
	if err := c.record(ctx, reviewer, record, ActionRead, map[string]any{"view": view}); err != nil {
		return carecontract.File{}, 0, err
	}
	return file, record.Summary.SchoolID, nil
}

func (c *Contracts) render(ctx context.Context, reviewer carecontract.Reviewer, record ContractRecord) (carecontract.File, int, error) {
	head, err := c.letterheads.Letterhead(ctx, reviewer)
	if err != nil {
		return carecontract.File{}, 0, fmt.Errorf("care contract: letterhead: %w", err)
	}
	if firstText(record.Version.Terms.ContractPartner, firstLine(textLines(head.SenderText))) == "" {
		return carecontract.File{}, 0, &carecontract.ValidationError{Fields: map[string]string{"contract_partner": "required"}}
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
	return c.sendContracts(ctx, reviewer, ids, "")
}

// ResendContract changes the delivery destination only after a failed send.
func (c *Contracts) ResendContract(ctx context.Context, reviewer carecontract.Reviewer, contractID int64, email string) (carecontract.SendResult, error) {
	if err := checkReviewer(reviewer); err != nil {
		return carecontract.SendResult{}, err
	}
	if contractID <= 0 {
		return carecontract.SendResult{}, &carecontract.ValidationError{Fields: map[string]string{"contract_ids": "count"}}
	}
	address := strings.TrimSpace(email)
	parsed, err := mail.ParseAddress(address)
	if err != nil || address == "" || len(address) > 320 || strings.ContainsAny(email, "\r\n") || parsed.Address != address {
		return carecontract.SendResult{}, &carecontract.ValidationError{Fields: map[string]string{"recipient_email": "invalid"}}
	}
	return c.sendContracts(ctx, reviewer, []int64{contractID}, address)
}

func (c *Contracts) sendContracts(ctx context.Context, reviewer carecontract.Reviewer, ids []int64, destination string) (carecontract.SendResult, error) {
	result := carecontract.SendResult{Sent: []int64{}, Skipped: []carecontract.SendSkip{}}
	for _, id := range ids {
		err := c.send(ctx, reviewer, id, destination)
		switch {
		case err == nil:
			result.Sent = append(result.Sent, id)
		case missingContractPartner(err):
			result.Skipped = append(result.Skipped, carecontract.SendSkip{ID: id, Reason: carecontract.SkipPartnerMissing})
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

func missingContractPartner(err error) bool {
	var invalid *carecontract.ValidationError
	return errors.As(err, &invalid) && invalid.Fields["contract_partner"] == "required"
}

// send freezes the PDF of a draft and sends it, or sends the stored PDF of
// a sent contract again. The contract row is locked first, so two sends of
// one contract run one after the other and freeze one PDF.
func (c *Contracts) send(ctx context.Context, reviewer carecontract.Reviewer, contractID int64, destination string) error {
	record, err := c.store.Contract(ctx, reviewer.OrganizationID, contractID, true)
	if err != nil {
		return err
	}
	return c.deliver(ctx, reviewer, record, destination)
}

// deliver sends the contract read under its row lock: it freezes the PDF of
// a draft, enqueues the mail and records the send.
func (c *Contracts) deliver(ctx context.Context, reviewer carecontract.Reviewer, record ContractRecord, destination string) error {
	contractID := record.Summary.ID
	if record.Summary.Status != carecontract.StatusDraft && record.Summary.Status != carecontract.StatusSent {
		return carecontract.ErrContractNotSendable
	}
	party, previousSends, err := c.deliveryParty(ctx, reviewer.OrganizationID, record, destination)
	if err != nil {
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
	outboxID, err := c.post.EnqueueContractMail(ctx, carecontract.ContractMail{
		OrganizationID: reviewer.OrganizationID, ContractID: contractID, Recipient: party.Email,
		GuardianName: fullName(party.FirstName, party.LastName), TraegerName: firstLine(textLines(head.SenderText)),
		ChildName: fullName(record.Version.ChildFirstName, record.Version.ChildLastName),
		Modules:   joinGerman(ModuleNames(record.Version.Services)), Filename: file.Filename, PDF: file.Data,
		IdempotencyKey: fmt.Sprintf("care-contract:%d:%d", contractID, previousSends+1),
		Concluded:      record.Concluded != nil,
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
	return c.record(ctx, reviewer, record, ActionSend, map[string]any{"document_id": documentID, "again": previousSends > 0})
}

// deliveryParty keeps delivery corrections separate from the frozen party.
// A failed destination is corrected by a new mailing, never by editing history.
func (c *Contracts) deliveryParty(ctx context.Context, organizationID int64, record ContractRecord, destination string) (carecontract.Party, int, error) {
	party, ok := recipient(record.Parties)
	if !ok {
		return carecontract.Party{}, 0, carecontract.ErrRecipientMissing
	}
	mailings, err := c.mailings(ctx, organizationID, record.Summary.ID)
	if err != nil {
		return carecontract.Party{}, 0, err
	}
	if len(mailings) > 0 && mailings[0].State == carecontract.MailQueued {
		return carecontract.Party{}, 0, carecontract.ErrMailPending
	}
	if destination != "" && (record.Summary.Status != carecontract.StatusSent || record.DocumentID == nil || len(mailings) == 0 || mailings[0].State != carecontract.MailFailed) {
		return carecontract.Party{}, 0, carecontract.ErrContractNotSendable
	}
	if len(mailings) > 0 {
		party.Email = mailings[0].Recipient
	}
	if destination != "" {
		party.Email = destination
	}
	return party, len(mailings), nil
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

// uniqueIDs keeps every positive ID once, in ascending order. A send locks
// each contract row until its transaction ends; the fixed order keeps two
// overlapping batches, [1, 2] and [2, 1], from waiting on each other.
func uniqueIDs(ids []int64) []int64 {
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 && !slices.Contains(result, id) {
			result = append(result, id)
		}
	}
	slices.Sort(result)
	return result
}
