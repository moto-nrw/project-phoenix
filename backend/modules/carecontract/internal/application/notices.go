package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NoticeStore persists the declarations (#4682).
type NoticeStore = ports.NoticeStore

// The actions of the trail for a declaration.
const (
	ActionTerminate = ports.ActionTerminate
	ActionWithdraw  = ports.ActionWithdraw
)

// Notices serves carecontract.Notices (#4682): withdrawal and termination
// of a concluded contract, by the parents in the parents portal or by the
// Träger-Büro for a declaration it received.
type Notices struct {
	store     Store
	notices   NoticeStore
	templates carecontract.NoticeTemplates
	post      carecontract.NoticePost
	clock     Clock
	// office reads the letterhead, opens the Träger transaction and writes
	// the school's access log for the Träger-Büro; nil where only the
	// parents portal declares.
	office *Contracts
}

var _ carecontract.Notices = (*Notices)(nil)

// NoticePorts are the root-bound ports of the declarations.
type NoticePorts struct {
	Templates carecontract.NoticeTemplates
	Post      carecontract.NoticePost
}

// NewNotices builds the declarations. office, the contracts of the
// Träger-Büro, may be nil for the parents portal alone.
func NewNotices(store Store, notices NoticeStore, noticePorts NoticePorts, clock Clock, office *Contracts) *Notices {
	if store == nil || notices == nil || noticePorts.Templates == nil || noticePorts.Post == nil {
		panic("care contract: the store, the notice store, the templates and the post are required")
	}
	return &Notices{store: store, notices: notices, templates: noticePorts.Templates, post: noticePorts.Post, clock: clock, office: office}
}

// trail records one operation on the contract under the acting account.
func (n *Notices) trail(ctx context.Context, organizationID, accountID int64, record ContractRecord, action string, details map[string]any) error {
	contractID, schoolID := record.Summary.ID, record.Summary.SchoolID
	if err := n.store.InsertEvents(ctx, organizationID, []Event{{
		ContractID: &contractID, SchoolID: &schoolID, AccountID: accountID, Action: action, Details: details,
	}}); err != nil {
		return fmt.Errorf("care contract: record %s: %w", action, err)
	}
	return nil
}

func (n *Notices) requireOffice() error {
	if n.office == nil {
		return errors.New("care contract: the office declarations are not configured")
	}
	return nil
}

// declaration is one declaration with the party it comes from.
type declaration struct {
	organizationID int64
	accountID      int64
	channel        string
	contractID     int64
	input          carecontract.NoticeInput
	// recipient is the address the confirmation goes to: the declaring
	// parent's own, or, for the office, the contract party's.
	recipient string
	// traeger is the Träger's name in the confirmation.
	traeger string
}

// NoticeOptions reads what can be declared on a contract today and records
// the read in the trail and in the access log of the contract's school.
func (n *Notices) NoticeOptions(ctx context.Context, reviewer carecontract.Reviewer, contractID int64) (carecontract.NoticeOptions, error) {
	if err := checkReviewer(reviewer); err != nil {
		return carecontract.NoticeOptions{}, err
	}
	if err := n.requireOffice(); err != nil {
		return carecontract.NoticeOptions{}, err
	}
	var options carecontract.NoticeOptions
	var schoolID int64
	err := n.office.runtime.WithinTraeger(ctx, reviewer.OrganizationID, func(txCtx context.Context) error {
		record, err := n.store.Contract(txCtx, reviewer.OrganizationID, contractID, false)
		if err != nil {
			return err
		}
		schoolID = record.Summary.SchoolID
		if options, err = n.options(txCtx, reviewer.OrganizationID, record); err != nil {
			return err
		}
		return n.trail(txCtx, reviewer.OrganizationID, reviewer.AccountID, record, ActionRead, map[string]any{"view": "notice_options"})
	})
	if err != nil {
		return carecontract.NoticeOptions{}, err
	}
	return options, n.office.logSchools(ctx, reviewer, "care_contract_notice", map[int64][]int64{schoolID: {contractID}})
}

// DeclareNotice records a withdrawal or termination the Träger-Büro
// received in Textform, in its own Träger transaction, and confirms it to
// the contract party.
func (n *Notices) DeclareNotice(ctx context.Context, reviewer carecontract.Reviewer, contractID int64, input carecontract.NoticeInput) (carecontract.NoticeOptions, error) {
	if err := checkReviewer(reviewer); err != nil {
		return carecontract.NoticeOptions{}, err
	}
	if err := n.requireOffice(); err != nil {
		return carecontract.NoticeOptions{}, err
	}
	var options carecontract.NoticeOptions
	err := n.office.runtime.WithinTraeger(ctx, reviewer.OrganizationID, func(txCtx context.Context) error {
		head, err := n.office.letterheads.Letterhead(txCtx, reviewer)
		if err != nil {
			return err
		}
		options, err = n.declare(txCtx, declaration{
			organizationID: reviewer.OrganizationID, accountID: reviewer.AccountID, channel: carecontract.ChannelOffice,
			contractID: contractID, input: input, traeger: firstLine(textLines(head.SenderText)),
		})
		return err
	})
	return options, err
}

// ParentNoticeOptions reads, in the caller's Träger transaction, what the
// parents can declare on a contract sent to them, and records the read.
func (n *Notices) ParentNoticeOptions(ctx context.Context, organizationID, contractID int64, reader carecontract.InboxReader) (carecontract.NoticeOptions, error) {
	if err := n.parentContract(ctx, organizationID, contractID, &reader); err != nil {
		return carecontract.NoticeOptions{}, err
	}
	record, err := n.store.Contract(ctx, organizationID, contractID, false)
	if err != nil {
		return carecontract.NoticeOptions{}, err
	}
	options, err := n.options(ctx, organizationID, record)
	if err != nil {
		return carecontract.NoticeOptions{}, err
	}
	return options, n.trail(ctx, organizationID, reader.AccountID, record, ActionRead, map[string]any{"inbox": "notice_options"})
}

// ParentDeclareNotice records the parents' withdrawal or termination with
// the button of the parents portal, in the caller's Träger transaction, and
// confirms it to the declaring parent's address.
func (n *Notices) ParentDeclareNotice(ctx context.Context, organizationID, contractID int64, reader carecontract.InboxReader,
	traeger string, input carecontract.NoticeInput) (carecontract.NoticeOptions, error) {
	if err := n.parentContract(ctx, organizationID, contractID, &reader); err != nil {
		return carecontract.NoticeOptions{}, err
	}
	// The parents declare today; only the office dates a received letter.
	input.ReceivedOn = n.today()
	return n.declare(ctx, declaration{
		organizationID: organizationID, accountID: reader.AccountID, channel: carecontract.ChannelParents,
		contractID: contractID, input: input, recipient: reader.Email, traeger: traeger,
	})
}

// parentContract checks that the contract was sent to the reader's address
// and its child is one the reader's relationships let act.
func (n *Notices) parentContract(ctx context.Context, organizationID, contractID int64, reader *carecontract.InboxReader) error {
	email, err := inboxAddress(organizationID, *reader)
	if err != nil || contractID <= 0 {
		return carecontract.ErrInboxDocumentNotFound
	}
	reader.Email = email
	found, err := n.notices.InboxContractID(ctx, organizationID, contractID, *reader)
	if err != nil {
		return err
	}
	if !found {
		return carecontract.ErrInboxDocumentNotFound
	}
	return nil
}

func (n *Notices) today() calendar.Date {
	return calendar.DateFromTime(n.clock.now())
}

// declare is the one transition out of aktiv (#4682). It takes the row lock
// of the contract first and reads its versions after it, the order of the
// conclusion (#4680) and the amendment (#4681): a concurrent declaration or
// amendment waits for this transaction and then sees the ended contract.
func (n *Notices) declare(ctx context.Context, d declaration) (carecontract.NoticeOptions, error) {
	record, err := n.store.Contract(ctx, d.organizationID, d.contractID, true)
	if err != nil {
		return carecontract.NoticeOptions{}, err
	}
	state, err := n.notices.NoticeState(ctx, d.organizationID, d.contractID)
	if err != nil {
		return carecontract.NoticeOptions{}, err
	}
	if record.Summary.Status != carecontract.StatusActive || record.Concluded == nil {
		return carecontract.NoticeOptions{}, carecontract.ErrContractNotEndable
	}
	notice, err := n.newNotice(record, state, d)
	if err != nil {
		return carecontract.NoticeOptions{}, err
	}
	if err := n.confirm(ctx, record, d, &notice); err != nil {
		return carecontract.NoticeOptions{}, err
	}
	record.Summary.Status = carecontract.StatusWithdrawn
	if notice.Kind == carecontract.NoticeTermination {
		record.Summary.Status = carecontract.StatusTerminated
	}
	return n.options(ctx, d.organizationID, record)
}

// newNotice checks the declaration against the contract and dates it.
func (n *Notices) newNotice(record ContractRecord, state ports.NoticeState, d declaration) (ports.NewNotice, error) {
	received, err := n.receivedOn(record, d.input.ReceivedOn)
	if err != nil {
		return ports.NewNotice{}, err
	}
	notice := ports.NewNotice{
		ContractID: d.contractID, Kind: d.input.Kind, Channel: d.channel, AccountID: d.accountID,
		DeclaredAt: n.clock.now(), ReceivedOn: received,
	}
	switch d.input.Kind {
	case carecontract.NoticeTermination:
		ordinary, special := noticeDates(record.Version.Terms, received, state.ContractEnd)
		switch d.input.TerminationKind {
		case carecontract.TerminationOrdinary:
			notice.EndsOn = &ordinary
		case carecontract.TerminationSpecial:
			if special == nil {
				return ports.NewNotice{}, carecontract.ErrSpecialTerminationClosed
			}
			notice.EndsOn = special
		default:
			return ports.NewNotice{}, &carecontract.ValidationError{Fields: map[string]string{"termination_kind": "unknown"}}
		}
		notice.TerminationKind = d.input.TerminationKind
	case carecontract.NoticeWithdrawal:
		until := withdrawalUntil(record.Concluded, state.InformedAt)
		if until == nil || received.After(*until) {
			return ports.NewNotice{}, carecontract.ErrWithdrawalClosed
		}
	default:
		return ports.NewNotice{}, &carecontract.ValidationError{Fields: map[string]string{"kind": "unknown"}}
	}
	return notice, nil
}

// receivedOn is the day the declaration reached the Träger: not before the
// conclusion and not in the future.
func (n *Notices) receivedOn(record ContractRecord, received calendar.Date) (calendar.Date, error) {
	today := n.today()
	if received.IsZero() || received.After(today) || received.Before(calendar.DateFromTime(record.Concluded.At)) {
		return calendar.Date(""), &carecontract.ValidationError{Fields: map[string]string{"received_on": "out_of_range"}}
	}
	return received, nil
}

// confirm fills the Träger's confirmation, stores the declaration with it,
// enqueues the mail and records the declaration in the trail.
func (n *Notices) confirm(ctx context.Context, record ContractRecord, d declaration, notice *ports.NewNotice) error {
	address := d.recipient
	party, hasParty := recipient(record.Parties)
	if address == "" {
		if !hasParty {
			return carecontract.ErrRecipientMissing
		}
		address = party.Email
	}
	traeger := strings.TrimSpace(d.traeger)
	if traeger == "" {
		traeger = record.Version.Terms.ContractPartner
	}
	template, err := n.templates.NoticeTemplate(ctx, d.organizationID, notice.Kind)
	if err != nil {
		return err
	}
	text, err := confirmationText(template, confirmationValues{
		Child: fullName(record.Version.ChildFirstName, record.Version.ChildLastName), Modules: joinGerman(ModuleNames(record.Version.Services)),
		School: record.Version.SchoolName, EndsOn: notice.EndsOn, ReceivedOn: notice.ReceivedOn, Traeger: traeger,
	})
	if err != nil {
		return err
	}
	notice.Recipient, notice.ConfirmationText = address, text
	noticeID, err := n.notices.InsertNotice(ctx, d.organizationID, *notice)
	if err != nil {
		return err
	}
	outboxID, err := n.post.EnqueueNoticeMail(ctx, carecontract.NoticeMail{
		OrganizationID: d.organizationID, ContractID: d.contractID, Recipient: address,
		GuardianName: fullName(party.FirstName, party.LastName), TraegerName: traeger, Kind: notice.Kind,
		ConfirmationText: text, IdempotencyKey: fmt.Sprintf("care-contract-notice:%d", noticeID),
	})
	if err != nil {
		return fmt.Errorf("care contract: enqueue confirmation: %w", err)
	}
	if err := n.notices.SetNoticeOutbox(ctx, d.organizationID, noticeID, outboxID); err != nil {
		return err
	}
	action := ActionWithdraw
	details := map[string]any{"notice_id": noticeID, "channel": d.channel, "received_on": notice.ReceivedOn.String(),
		"version_id": record.Version.ID, "outbox_id": outboxID}
	if notice.Kind == carecontract.NoticeTermination {
		action = ActionTerminate
		details["termination_kind"], details["ends_on"] = notice.TerminationKind, notice.EndsOn.String()
	}
	return n.trail(ctx, d.organizationID, d.accountID, record, action, details)
}

// options is what can be declared on the contract today, or the declaration
// made.
func (n *Notices) options(ctx context.Context, organizationID int64, record ContractRecord) (carecontract.NoticeOptions, error) {
	state, err := n.notices.NoticeState(ctx, organizationID, record.Summary.ID)
	if err != nil {
		return carecontract.NoticeOptions{}, err
	}
	options := carecontract.NoticeOptions{
		ContractID: record.Summary.ID, Status: record.Summary.Status, SchoolName: record.Version.SchoolName,
		ChildName: fullName(record.Version.ChildFirstName, record.Version.ChildLastName),
		Modules:   joinGerman(ModuleNames(record.Version.Services)), Terms: NoticeText(record.Version.Terms),
		EarlyStart: state.EarlyStart, Notice: state.Notice,
	}
	if record.Summary.Status != carecontract.StatusActive || record.Concluded == nil {
		return options, nil
	}
	today := n.today()
	ordinary, special := noticeDates(record.Version.Terms, today, state.ContractEnd)
	options.OrdinaryEndsOn, options.SpecialEndsOn = &ordinary, special
	if until := withdrawalUntil(record.Concluded, state.InformedAt); until != nil && !today.After(*until) {
		options.WithdrawalUntil = until
	}
	return options, nil
}
