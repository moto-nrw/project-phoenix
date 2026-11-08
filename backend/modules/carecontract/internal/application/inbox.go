package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Inbox serves carecontract.Inbox (ADR 0088 section 7, ADR 0126, ADR 0153):
// a contract lies in the inbox of the account its mail went to. The parents
// portal supplies the schools and children its relationship checks permit.
type Inbox struct {
	store Store
	// InboxNotices is the store's check for the push of a sent contract
	// (#5941): it records no read, so it needs no layer of its own.
	carecontract.InboxNotices
	carecontract.DocumentPushValidity
}

var (
	_ carecontract.Inbox        = (*Inbox)(nil)
	_ carecontract.InboxNotices = (*Inbox)(nil)
)

func NewInbox(store interface {
	Store
	carecontract.DocumentPushValidity
}) *Inbox {
	if store == nil {
		panic("care contract: the store is required")
	}
	return &Inbox{store: store, InboxNotices: store, DocumentPushValidity: store}
}

// InboxContracts lists the sent contracts addressed to the reader and
// records the read of each.
func (in *Inbox) InboxContracts(ctx context.Context, organizationID int64, reader carecontract.InboxReader) ([]carecontract.InboxContract, error) {
	email, err := inboxAddress(organizationID, reader)
	if err != nil {
		return nil, err
	}
	reader.Email = email
	rows, err := in.store.InboxContracts(ctx, organizationID, reader)
	if err != nil {
		return nil, fmt.Errorf("care contract: read inbox: %w", err)
	}
	if len(rows) == 0 {
		return []carecontract.InboxContract{}, nil
	}
	today := calendar.DateFromTime(time.Now())
	contracts, ids := make([]carecontract.InboxContract, len(rows)), make([]int64, len(rows))
	for i, row := range rows {
		contracts[i], ids[i] = row.Contract, row.Contract.ID
		contracts[i].CanWithdraw = row.Contract.CanEnd && withdrawalMayRun(row.Concluded, row.InformedAt, today)
	}
	if err := trail(ctx, in.store, organizationID, reader.AccountID, 0, 0, ActionRead, map[string]any{"inbox": "list", "contract_ids": ids}); err != nil {
		return nil, err
	}
	return contracts, nil
}

// InboxContractDocument reads the stored PDF of one sent contract addressed
// to the reader and records the read.
func (in *Inbox) InboxContractDocument(ctx context.Context, organizationID, contractID int64, reader carecontract.InboxReader) (carecontract.File, error) {
	email, err := inboxAddress(organizationID, reader)
	if err != nil {
		return carecontract.File{}, err
	}
	if contractID <= 0 {
		return carecontract.File{}, carecontract.ErrInboxDocumentNotFound
	}
	reader.Email = email
	file, err := in.store.InboxDocument(ctx, organizationID, contractID, reader)
	if err != nil {
		return carecontract.File{}, err
	}
	if file.WithdrawalNotice {
		if err := in.store.MarkWithdrawalInformed(ctx, organizationID, contractID, time.Now()); err != nil {
			return carecontract.File{}, err
		}
	}
	if err := trail(ctx, in.store, organizationID, reader.AccountID, contractID, 0, ActionRead, map[string]any{"inbox": "download"}); err != nil {
		return carecontract.File{}, err
	}
	return file, nil
}

func inboxAddress(organizationID int64, reader carecontract.InboxReader) (string, error) {
	email := strings.TrimSpace(reader.Email)
	if organizationID <= 0 || reader.AccountID <= 0 || email == "" {
		return "", carecontract.ErrInboxDocumentNotFound
	}
	return email, nil
}

// withdrawalMayRun tells whether the withdrawal period of a concluded
// contract may still run on day (#6105). informedAt is the earliest recorded
// provision of the notice; a later delivery proof only moves it earlier, so
// the end computed from it is never before the true end. A paper contract
// without a recorded notice has no known end yet: the withdrawal function
// stays reachable until the right expires (§ 356 Abs. 3 Satz 2 BGB) and the
// declaration page decides.
func withdrawalMayRun(concluded *carecontract.Concluded, informedAt *time.Time, day calendar.Date) bool {
	if concluded == nil {
		return false
	}
	until := withdrawalUntil(concluded, informedAt)
	if until == nil {
		expiry := nextWorkingDay(addMonths(calendar.DateFromTime(concluded.At), 12).AddDays(carecontract.WithdrawalDays))
		until = &expiry
	}
	return !day.After(*until)
}
