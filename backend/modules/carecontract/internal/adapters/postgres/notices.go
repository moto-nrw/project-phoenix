package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The declarations of a contract (#4682): withdrawal and termination, kept
// in care_contract.contract_notices with the confirmation as sent.

var _ ports.NoticeStore = Store{}

type noticeRow struct {
	bun.BaseModel `bun:"table:care_contract.contract_notices"`

	DispatchedOn           *calendar.Date `bun:"dispatched_on,type:date"`
	Kind                   string         `bun:"kind"`
	NoticeKind             *string        `bun:"notice_kind"`
	Channel                string         `bun:"channel"`
	DeclaredAt             time.Time      `bun:"declared_at"`
	ReceivedOn             calendar.Date  `bun:"received_on,type:date"`
	EndsOn                 *calendar.Date `bun:"ends_on,type:date"`
	Reason                 *string        `bun:"reason"`
	Recipient              string         `bun:"recipient_email"`
	ConfirmationText       string         `bun:"confirmation_text"`
	ID                     int64          `bun:"id"`
	EndDecision            *string        `bun:"end_decision"`
	EndDecidedAt           *time.Time     `bun:"end_decided_at"`
	EndNoticeText          *string        `bun:"end_notice_text"`
	EndNoticeNotSentReason *string        `bun:"end_notice_not_sent_reason"`
	EndNoticeOutbox        *int64         `bun:"end_notice_outbox_id"`
	AccountID              *int64         `bun:"account_id"`
}

// NoticeState reads the first and last day of the contract's effective
// versions, the withdrawal notice and the declaration made, in one
// statement.
func (Store) NoticeState(ctx context.Context, organizationID, contractID int64) (ports.NoticeState, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return ports.NoticeState{}, err
	}
	var row struct {
		ContractEnd *calendar.Date `bun:"contract_end,type:date"`
		InformedAt  *time.Time     `bun:"withdrawal_informed_at"`
		EarlyStart  bool           `bun:"early_start_requested"`
		StudentID   *int64         `bun:"student_id"`
		Party       *int64         `bun:"party_account_id"`
	}
	err = db.NewRaw(`SELECT c.withdrawal_informed_at, c.early_start_requested, c.student_id,
			(SELECT p.account_id FROM care_contract.contract_parties AS p
				WHERE p.organization_id = c.organization_id AND p.contract_id = c.id ORDER BY p.id LIMIT 1) AS party_account_id,
			(SELECT v.ends_on FROM care_contract.contract_versions AS v
				WHERE v.organization_id = c.organization_id AND v.contract_id = c.id
					AND v.status IN `+effectiveStatusSQL+`
				ORDER BY v.effective_from DESC LIMIT 1) AS contract_end
		FROM care_contract.contracts AS c WHERE c.organization_id = ? AND c.id = ?`,
		organizationID, contractID).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.NoticeState{}, carecontract.ErrContractNotFound
	}
	if err != nil {
		return ports.NoticeState{}, fmt.Errorf("care contract postgres: read notice state: %w", err)
	}
	state := ports.NoticeState{ContractEnd: row.ContractEnd, InformedAt: row.InformedAt, EarlyStart: row.EarlyStart,
		StudentID: row.StudentID}
	if row.Party != nil {
		state.Accounts = append(state.Accounts, *row.Party)
	}
	if err := readNotice(ctx, db, organizationID, contractID, &state); err != nil {
		return ports.NoticeState{}, err
	}
	return state, nil
}

// text is the value of a nullable text column, "" for NULL.
func text(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// readNotice adds the contract's newest declaration to state, with the
// account that declared among the accounts the contract names.
func readNotice(ctx context.Context, db bun.IDB, organizationID, contractID int64, state *ports.NoticeState) error {
	var notices []noticeRow
	if err := db.NewRaw(`SELECT id, account_id, kind, notice_kind, channel, declared_at, received_on, dispatched_on, ends_on, reason, recipient_email, confirmation_text,
			end_decision, end_decided_at, end_notice_text, end_notice_outbox_id,
			(SELECT e.details->>'notice_not_sent_reason' FROM care_contract.contract_events AS e
				WHERE e.organization_id = n.organization_id AND e.contract_id = n.contract_id
					AND e.action IN ('set_end', 'reject_reason') AND e.details->>'notice_id' = n.id::text
				ORDER BY e.id DESC LIMIT 1) AS end_notice_not_sent_reason
		FROM care_contract.contract_notices AS n WHERE organization_id = ? AND contract_id = ? ORDER BY id DESC LIMIT 1`,
		organizationID, contractID).Scan(ctx, &notices); err != nil {
		return fmt.Errorf("care contract postgres: read notice: %w", err)
	}
	if len(notices) != 1 {
		return nil
	}
	notice := notices[0]
	state.Notice = &carecontract.Notice{
		Kind: notice.Kind, Channel: notice.Channel, DeclaredAt: notice.DeclaredAt, ReceivedOn: notice.ReceivedOn,
		DispatchedOn: notice.DispatchedOn, EndsOn: notice.EndsOn, Recipient: notice.Recipient, ConfirmationText: notice.ConfirmationText,
		EndDecidedAt: notice.EndDecidedAt, EndNoticeSent: notice.EndNoticeOutbox != nil,
		TerminationKind: text(notice.NoticeKind), Reason: text(notice.Reason),
		EndDecision: text(notice.EndDecision), EndNoticeText: text(notice.EndNoticeText),
		EndNoticeNotSentReason: text(notice.EndNoticeNotSentReason),
	}
	state.NoticeID = notice.ID
	if notice.AccountID != nil && !slices.Contains(state.Accounts, *notice.AccountID) {
		state.Accounts = append(state.Accounts, *notice.AccountID)
	}
	return nil
}

// InsertNotice stores the declaration and moves the contract to its status.
// It changes the contract by its ID alone: the caller's row lock, taken
// before the state was read, keeps a second declaration out (the races in
// notice_race_test.go go red without it).
func (Store) InsertNotice(ctx context.Context, organizationID int64, notice ports.NewNotice) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	var terminationKind *string
	status := carecontract.StatusWithdrawn
	if notice.Kind == carecontract.NoticeTermination {
		terminationKind, status = &notice.TerminationKind, carecontract.StatusTerminated
	}
	var account *int64
	if notice.AccountID > 0 {
		account = &notice.AccountID
	}
	var reason *string
	if notice.Reason != "" {
		reason = &notice.Reason
	}
	var id int64
	if err := db.NewRaw(`INSERT INTO care_contract.contract_notices (organization_id, contract_id, kind, notice_kind, channel,
			account_id, declared_at, received_on, dispatched_on, ends_on, reason, recipient_email, confirmation_text)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		organizationID, notice.ContractID, notice.Kind, terminationKind, notice.Channel, account, notice.DeclaredAt,
		notice.ReceivedOn, notice.DispatchedOn, notice.EndsOn, reason, notice.Recipient, notice.ConfirmationText).Scan(ctx, &id); err != nil {
		return 0, fmt.Errorf("care contract postgres: insert notice: %w", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE care_contract.contracts SET status = ?, ends_on = ?
		WHERE organization_id = ? AND id = ?`, status, notice.EndsOn, organizationID, notice.ContractID); err != nil {
		return 0, fmt.Errorf("care contract postgres: end contract: %w", err)
	}
	return id, nil
}

// SetNoticeEnd decides the end of an extraordinary termination the Träger
// checked: on the declaration whose end is open, with the decision and the
// notice to the parents, and on the terminated contract, both of
// organizationID. Both statements run before either count is checked, so
// each guard (the Träger, the open end) shows on its own row; a count other
// than one rolls the caller's transaction back.
func (Store) SetNoticeEnd(ctx context.Context, organizationID, contractID int64, end ports.NoticeEnd) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	var account *int64
	if end.AccountID > 0 {
		account = &end.AccountID
	}
	endsOn := end.EndsOn
	notice, err := db.ExecContext(ctx, `UPDATE care_contract.contract_notices
		SET ends_on = ?, end_decision = ?, end_decided_by = ?, end_decided_at = ?, end_notice_text = ?
		WHERE organization_id = ? AND contract_id = ? AND notice_kind = ? AND ends_on IS NULL`,
		endsOn, end.Decision, account, end.DecidedAt, end.Text, organizationID, contractID, carecontract.TerminationExtraordinary)
	if err != nil {
		return fmt.Errorf("care contract postgres: set notice end: %w", err)
	}
	contract, err := db.ExecContext(ctx, `UPDATE care_contract.contracts SET ends_on = ?
		WHERE organization_id = ? AND id = ? AND status = ? AND ends_on IS NULL`,
		endsOn, organizationID, contractID, carecontract.StatusTerminated)
	if err != nil {
		return fmt.Errorf("care contract postgres: set contract end: %w", err)
	}
	for _, result := range []sql.Result{notice, contract} {
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			return carecontract.ErrNoticeEndNotOpen
		}
	}
	return nil
}

// SetNoticeOutbox records the confirmation mail of a declaration.
func (Store) SetNoticeOutbox(ctx context.Context, organizationID, noticeID, outboxID int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `UPDATE care_contract.contract_notices SET outbox_id = ?
		WHERE organization_id = ? AND id = ?`, outboxID, organizationID, noticeID); err != nil {
		return fmt.Errorf("care contract postgres: record notice mail: %w", err)
	}
	return nil
}

// SetEndNoticeOutbox records the mail that told the parents the decided end.
func (Store) SetEndNoticeOutbox(ctx context.Context, organizationID, noticeID, outboxID int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `UPDATE care_contract.contract_notices SET end_notice_outbox_id = ?
		WHERE organization_id = ? AND id = ?`, outboxID, organizationID, noticeID); err != nil {
		return fmt.Errorf("care contract postgres: record end notice mail: %w", err)
	}
	return nil
}

// InboxContractID tells whether the contract was sent to the reader's
// address and its child is one the reader may act for.
func (Store) InboxContractID(ctx context.Context, organizationID, contractID int64, reader carecontract.InboxReader) (bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return false, err
	}
	schools, students := inboxChildScope(reader.Children)
	var found []int64
	if err := db.NewRaw(`SELECT c.id FROM care_contract.contracts AS c
		WHERE c.organization_id = ? AND c.id = ? AND EXISTS (SELECT 1 FROM care_contract.contract_mailings AS m
			WHERE m.organization_id = c.organization_id AND m.contract_id = c.id AND lower(m.recipient_email) = lower(?))
			AND `+inboxChildScopeSQL, organizationID, contractID, reader.Email,
		pgdialect.Array(schools), pgdialect.Array(students)).Scan(ctx, &found); err != nil {
		return false, fmt.Errorf("care contract postgres: read inbox contract: %w", err)
	}
	return len(found) == 1, nil
}

// MarkWithdrawalInformed retains the first proven provision independently of
// outbox retention and later sends. Callers supply delivery or download evidence.
func (Store) MarkWithdrawalInformed(ctx context.Context, organizationID, contractID int64, at time.Time) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `UPDATE care_contract.contracts SET withdrawal_informed_at = LEAST(withdrawal_informed_at, ?)
		WHERE organization_id = ? AND id = ?`, at, organizationID, contractID)
	if err != nil {
		return fmt.Errorf("care contract postgres: record notice provision: %w", err)
	}
	return nil
}
