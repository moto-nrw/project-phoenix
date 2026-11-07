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

// The trail actions of the decided end of an extraordinary termination.
const (
	ActionSetEnd       = ports.ActionSetEnd
	ActionRejectReason = ports.ActionRejectReason
)

// endDecision is how the Träger-Büro decides the end of an extraordinary
// termination: the day it set (EndDecisionSet), or none, when it does not
// accept the reason and the declaration holds as an ordinary termination
// (EndDecisionReinterpreted). expected, when set, is the end the office was
// shown for a refusal.
type endDecision struct {
	decision string
	endsOn   calendar.Date
	expected calendar.Date
	// permitted are the addresses that may receive the notice: those of
	// the guardians whose relationship to the child still grants
	// carecontract.GuardianPermissionEnrollmentSubmit, read before the
	// decision in the school's transaction.
	permitted []string
}

// SetNoticeEnd sets the last day of a contract the parents terminated
// extraordinarily, once the Träger-Büro checked their reason (#4682,
// § 312k Abs. 2 BGB). The end lies on or after the day the declaration
// arrived. The parents get the last day in Textform.
func (n *Notices) SetNoticeEnd(ctx context.Context, reviewer carecontract.Reviewer, contractID int64, endsOn calendar.Date) (carecontract.NoticeOptions, error) {
	return n.decideEndInTraeger(ctx, reviewer, contractID, endDecision{decision: carecontract.EndDecisionSet, endsOn: endsOn})
}

// RejectNoticeReason records that the Träger-Büro does not accept the reason
// of an extraordinary termination (#4682, decision of 07.11.2026): the
// declaration holds as an ordinary termination to the next notice date from
// its receipt (§ 140 BGB), and the parents get the new end in Textform.
func (n *Notices) RejectNoticeReason(ctx context.Context, reviewer carecontract.Reviewer, contractID int64, expectedEndsOn calendar.Date) (carecontract.NoticeOptions, error) {
	return n.decideEndInTraeger(ctx, reviewer, contractID, endDecision{decision: carecontract.EndDecisionReinterpreted, expected: expectedEndsOn})
}

func (n *Notices) decideEndInTraeger(ctx context.Context, reviewer carecontract.Reviewer, contractID int64, decision endDecision) (carecontract.NoticeOptions, error) {
	if err := checkReviewer(reviewer); err != nil {
		return carecontract.NoticeOptions{}, err
	}
	if err := n.requireOffice(); err != nil {
		return carecontract.NoticeOptions{}, err
	}
	permitted, err := n.permittedAddresses(ctx, reviewer, contractID)
	if err != nil {
		return carecontract.NoticeOptions{}, err
	}
	decision.permitted = permitted
	var options carecontract.NoticeOptions
	err = n.office.runtime.WithinTraeger(ctx, reviewer.OrganizationID, func(txCtx context.Context) error {
		var err error
		options, err = n.decideEnd(txCtx, reviewer, contractID, decision)
		return err
	})
	return options, err
}

// permittedAddresses reads who may still receive the notice: the contract's
// child and accounts in the Träger transaction, then those accounts' guardian
// relationships to the child in the school's tenant transaction, never both
// in one (ADR 0105). A child that is no student yet, or a contract without
// an account, has no relationship to check, so no address qualifies. A
// relationship revoked after this read and before the decision commits
// still gets the mail; the window is the length of one request.
func (n *Notices) permittedAddresses(ctx context.Context, reviewer carecontract.Reviewer, contractID int64) ([]string, error) {
	if n.addressees == nil {
		return nil, errors.New("care contract: the notice addressees are not configured")
	}
	var schoolID int64
	var state ports.NoticeState
	err := n.office.runtime.WithinTraeger(ctx, reviewer.OrganizationID, func(txCtx context.Context) error {
		record, err := n.store.Contract(txCtx, reviewer.OrganizationID, contractID, false)
		if err != nil {
			return err
		}
		schoolID = record.Summary.SchoolID
		state, err = n.notices.NoticeState(txCtx, reviewer.OrganizationID, contractID)
		return err
	})
	if err != nil || state.StudentID == nil || len(state.Accounts) == 0 {
		return nil, err
	}
	var permitted []string
	err = n.office.runtime.WithinSchool(ctx, schoolID, func(schoolCtx context.Context) error {
		var err error
		permitted, err = n.addressees.PermittedAddresses(schoolCtx, *state.StudentID, state.Accounts)
		return err
	})
	return permitted, err
}

// decideEnd decides the end in the caller's Träger transaction. It takes
// the contract's row lock first, as every declaration does, so a second
// decision (the same or the other one) waits and then finds no end open;
// the store's guard on the open end refuses it as well. The notice to the
// parents is enqueued once, keyed to the declaration.
func (n *Notices) decideEnd(ctx context.Context, reviewer carecontract.Reviewer, contractID int64, decision endDecision) (carecontract.NoticeOptions, error) {
	org := reviewer.OrganizationID
	record, err := n.store.Contract(ctx, org, contractID, true)
	if err != nil {
		return carecontract.NoticeOptions{}, err
	}
	state, err := n.notices.NoticeState(ctx, org, contractID)
	if err != nil {
		return carecontract.NoticeOptions{}, err
	}
	endsOn := decision.endsOn
	if decision.decision == carecontract.EndDecisionReinterpreted {
		if !endIsOpen(record, state.Notice) {
			return carecontract.NoticeOptions{}, carecontract.ErrNoticeEndNotOpen
		}
		endsOn = reinterpretedEnd(record, state)
		if !decision.expected.IsZero() && decision.expected != endsOn {
			return carecontract.NoticeOptions{}, &carecontract.ValidationError{Fields: map[string]string{"expected_ends_on": "changed"}}
		}
	} else if _, err := openEnd(record, state.Notice, endsOn); err != nil {
		return carecontract.NoticeOptions{}, err
	}
	// A computed ordinary end may preserve a fixed term before receipt.
	// Only a manually chosen end must be on or after receipt.
	notice := state.Notice
	traeger, err := n.officeTraeger(ctx, reviewer, record)
	if err != nil {
		return carecontract.NoticeOptions{}, err
	}
	text, err := n.endNoticeText(ctx, org, record, notice, endsOn, traeger, decision.decision)
	if err != nil {
		return carecontract.NoticeOptions{}, err
	}
	if err := n.notices.SetNoticeEnd(ctx, org, contractID, ports.NoticeEnd{
		EndsOn: endsOn, Decision: decision.decision, AccountID: reviewer.AccountID, DecidedAt: n.clock.now(), Text: text,
	}); err != nil {
		return carecontract.NoticeOptions{}, err
	}
	if err := n.tellEnd(ctx, reviewer, record, state, decision, endsOn, traeger, text); err != nil {
		return carecontract.NoticeOptions{}, err
	}
	return n.options(ctx, org, record, n.today(), true)
}

// tellEnd enqueues the notice of the decided end once per declaration (its
// idempotency key), links the mail to the declaration and records the
// decision in the trail. The notice goes to the address of the
// confirmation, else the contract's newest mailing or party address, but
// only to one that still belongs to a guardian whose relationship to the
// child grants the enrollment submission. Without such an address no mail
// goes out; the decision stands and the office sees that the parents got
// no notice (Notice.EndNoticeSent).
func (n *Notices) tellEnd(ctx context.Context, reviewer carecontract.Reviewer, record ContractRecord, state ports.NoticeState,
	decision endDecision, endsOn calendar.Date, traeger, text string) error {
	org, notice := reviewer.OrganizationID, state.Notice
	party, _ := recipient(record.Parties)
	details := map[string]any{"ends_on": endsOn.String(), "termination_kind": notice.TerminationKind, "notice_id": state.NoticeID}
	address, err := n.endAddress(ctx, org, record, notice.Recipient, party.Email, decision.permitted)
	if err != nil {
		return err
	}
	details["notice_sent"] = address != ""
	if address != "" {
		outboxID, err := n.post.EnqueueNoticeMail(ctx, carecontract.NoticeMail{
			OrganizationID: org, ContractID: record.Summary.ID, Recipient: address,
			GuardianName: fullName(party.FirstName, party.LastName), TraegerName: traeger, Kind: notice.Kind,
			Decision: decision.decision, ConfirmationText: text, IdempotencyKey: fmt.Sprintf("care-contract-notice-end:%d", state.NoticeID),
		})
		if err != nil {
			return fmt.Errorf("care contract: enqueue end notice: %w", err)
		}
		if err := n.notices.SetEndNoticeOutbox(ctx, org, state.NoticeID, outboxID); err != nil {
			return err
		}
		details["outbox_id"] = outboxID
	}
	action := ActionSetEnd
	if decision.decision == carecontract.EndDecisionReinterpreted {
		action = ActionRejectReason
	}
	return n.trail(ctx, org, reviewer.AccountID, record, action, details)
}

// endAddress is the first of the confirmation's address, the contract's
// newest mailing address and the party's address that a permitted
// guardian holds, or "" when none does.
func (n *Notices) endAddress(ctx context.Context, organizationID int64, record ContractRecord, confirmed, partyEmail string, permitted []string) (string, error) {
	allowed := make(map[string]bool, len(permitted))
	for _, address := range permitted {
		if address = strings.ToLower(strings.TrimSpace(address)); address != "" {
			allowed[address] = true
		}
	}
	if len(allowed) == 0 {
		return "", nil
	}
	candidates := []string{confirmed}
	mailings, err := n.store.Mailings(ctx, organizationID, record.Summary.ID)
	if err != nil {
		return "", err
	}
	if len(mailings) > 0 {
		candidates = append(candidates, mailings[0].Recipient)
	}
	for _, candidate := range append(candidates, partyEmail) {
		if allowed[strings.ToLower(strings.TrimSpace(candidate))] {
			return strings.TrimSpace(candidate), nil
		}
	}
	return "", nil
}

// reinterpretedEnd is the last day of an extraordinary termination that
// holds as an ordinary one: the ordinary notice date counted from the day
// the declaration arrived, as an ordinary termination on that day would
// have ended (#4682).
func reinterpretedEnd(record ContractRecord, state ports.NoticeState) calendar.Date {
	ordinary, _ := noticeDates(record.Version.Terms, state.Notice.ReceivedOn, state.ContractEnd)
	return ordinary
}

// officeTraeger is the Träger's name as the office's letterhead gives it,
// or the contract partner of the terms.
func (n *Notices) officeTraeger(ctx context.Context, reviewer carecontract.Reviewer, record ContractRecord) (string, error) {
	head, err := n.office.letterheads.Letterhead(ctx, reviewer)
	if err != nil {
		return "", err
	}
	if traeger := strings.TrimSpace(firstLine(textLines(head.SenderText))); traeger != "" {
		return traeger, nil
	}
	return record.Version.Terms.ContractPartner, nil
}

// endNoticeText fills the Träger's template of the notice of a decided end.
func (n *Notices) endNoticeText(ctx context.Context, organizationID int64, record ContractRecord, notice *carecontract.Notice,
	endsOn calendar.Date, traeger, decision string) (string, error) {
	kind := carecontract.NoticeTemplateEndSet
	if decision == carecontract.EndDecisionReinterpreted {
		kind = carecontract.NoticeTemplateReasonRejected
	}
	template, err := n.templates.NoticeTemplate(ctx, organizationID, kind)
	if err != nil {
		return "", err
	}
	return endText(template, confirmationValues{
		Child: fullName(record.Version.ChildFirstName, record.Version.ChildLastName), Modules: joinGerman(ModuleNames(record.Version.Services)),
		School: record.Version.SchoolName, EndsOn: &endsOn, Traeger: traeger, Reason: notice.Reason,
		Received: receipt(ports.NewNotice{Channel: notice.Channel, DeclaredAt: notice.DeclaredAt, ReceivedOn: notice.ReceivedOn}),
	})
}

// openEnd is the extraordinary termination whose end is still open, when
// endsOn may end it: not before the day the declaration arrived.
func openEnd(record ContractRecord, notice *carecontract.Notice, endsOn calendar.Date) (*carecontract.Notice, error) {
	if !endIsOpen(record, notice) {
		return nil, carecontract.ErrNoticeEndNotOpen
	}
	if endsOn.IsZero() || endsOn.Before(notice.ReceivedOn) {
		return nil, &carecontract.ValidationError{Fields: map[string]string{"ends_on": "out_of_range"}}
	}
	return notice, nil
}

// endIsOpen tells whether the contract is terminated extraordinarily and
// its end is not decided yet.
func endIsOpen(record ContractRecord, notice *carecontract.Notice) bool {
	return record.Summary.Status == carecontract.StatusTerminated && notice != nil &&
		notice.TerminationKind == carecontract.TerminationExtraordinary && notice.EndsOn == nil
}
