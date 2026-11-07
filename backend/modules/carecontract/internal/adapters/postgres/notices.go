package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The declarations of a contract (#4682): withdrawal and termination, kept
// in care_contract.contract_notices with the confirmation as sent.

var _ ports.NoticeStore = Store{}

type noticeRow struct {
	bun.BaseModel `bun:"table:care_contract.contract_notices"`

	DispatchedOn     *calendar.Date `bun:"dispatched_on,type:date"`
	Kind             string         `bun:"kind"`
	NoticeKind       *string        `bun:"notice_kind"`
	Channel          string         `bun:"channel"`
	DeclaredAt       time.Time      `bun:"declared_at"`
	ReceivedOn       calendar.Date  `bun:"received_on,type:date"`
	EndsOn           *calendar.Date `bun:"ends_on,type:date"`
	Reason           *string        `bun:"reason"`
	Recipient        string         `bun:"recipient_email"`
	ConfirmationText string         `bun:"confirmation_text"`
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
		FirstFrom   *calendar.Date `bun:"first_from,type:date"`
		ContractEnd *calendar.Date `bun:"contract_end,type:date"`
		InformedAt  *time.Time     `bun:"withdrawal_informed_at"`
		EarlyStart  bool           `bun:"early_start_requested"`
	}
	err = db.NewRaw(`SELECT c.withdrawal_informed_at, c.early_start_requested,
			(SELECT MIN(v.effective_from) FROM care_contract.contract_versions AS v
				WHERE v.organization_id = c.organization_id AND v.contract_id = c.id
					AND v.status IN `+effectiveStatusSQL+`) AS first_from,
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
	state := ports.NoticeState{FirstFrom: row.FirstFrom, ContractEnd: row.ContractEnd, InformedAt: row.InformedAt, EarlyStart: row.EarlyStart}
	var notices []noticeRow
	if err := db.NewRaw(`SELECT kind, notice_kind, channel, declared_at, received_on, dispatched_on, ends_on, reason, recipient_email, confirmation_text
		FROM care_contract.contract_notices WHERE organization_id = ? AND contract_id = ? ORDER BY id DESC LIMIT 1`,
		organizationID, contractID).Scan(ctx, &notices); err != nil {
		return ports.NoticeState{}, fmt.Errorf("care contract postgres: read notice: %w", err)
	}
	if len(notices) == 1 {
		notice := notices[0]
		state.Notice = &carecontract.Notice{
			Kind: notice.Kind, Channel: notice.Channel, DeclaredAt: notice.DeclaredAt, ReceivedOn: notice.ReceivedOn,
			DispatchedOn: notice.DispatchedOn, EndsOn: notice.EndsOn, Recipient: notice.Recipient, ConfirmationText: notice.ConfirmationText,
		}
		if notice.NoticeKind != nil {
			state.Notice.TerminationKind = *notice.NoticeKind
		}
		if notice.Reason != nil {
			state.Notice.Reason = *notice.Reason
		}
	}
	return state, nil
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

// SetNoticeEnd sets the end of an extraordinary termination the Träger
// checked: on the declaration whose end is open and on the terminated
// contract, both of organizationID. Both statements run before either
// count is checked, so each guard (the Träger, the open end) shows on its
// own row; a count other than one rolls the caller's transaction back.
func (Store) SetNoticeEnd(ctx context.Context, organizationID, contractID int64, endsOn calendar.Date) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	notice, err := db.ExecContext(ctx, `UPDATE care_contract.contract_notices SET ends_on = ?
		WHERE organization_id = ? AND contract_id = ? AND notice_kind = ? AND ends_on IS NULL`,
		endsOn, organizationID, contractID, carecontract.TerminationExtraordinary)
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
