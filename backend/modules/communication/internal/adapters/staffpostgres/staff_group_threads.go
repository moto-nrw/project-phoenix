package staffpostgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
)

// CreateStaffGroupThread creates a named group conversation (#4776) with the
// creator and the members as participants. Every participant's history starts
// at creation, so a group never shows anything older than itself.
func (s *ThreadStore) CreateStaffGroupThread(ctx context.Context, name string, creatorAccountID int64, memberAccountIDs []int64) (*communication.StaffThreadRecord, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	if tenantID <= 0 {
		return nil, fmt.Errorf("create staff group thread: tenant is required")
	}
	creator := creatorAccountID
	thread := &staffThreadRow{
		TenantID:           tenantID,
		Kind:               domain.StaffMessageThreadKindGroup,
		Name:               &name,
		CreatedByAccountID: &creator,
	}
	if _, err := db.NewInsert().
		Model(thread).
		ModelTableExpr(staffThreadsTable).
		ExcludeColumn("created_at", "updated_at").
		// A group is not identified by its members: two groups with the same
		// people are two conversations, and members change. A random key
		// only has to be unique per school.
		Value("participant_key", "'group:' || gen_random_uuid()::text").
		Returning("id, participant_key, created_at, updated_at").
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("create staff group thread: %w", err)
	}
	accountIDs := append([]int64{creatorAccountID}, memberAccountIDs...)
	if err := s.insertGroupParticipants(ctx, thread.TenantID, thread.ID, accountIDs); err != nil {
		return nil, err
	}
	return thread.value(), nil
}

// insertGroupParticipants adds members whose visible history starts now.
// clock_timestamp() rather than NOW(): messages are stamped with the clock at
// their insert, so the join instant has to come from the same clock. An
// existing member keeps their row and with it their history.
func (s *ThreadStore) insertGroupParticipants(ctx context.Context, tenantID, threadID int64, accountIDs []int64) error {
	db, _, err := s.database(ctx)
	if err != nil {
		return err
	}
	seen := make(map[int64]bool, len(accountIDs))
	rows := make([]*staffParticipantRow, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		if seen[accountID] {
			continue
		}
		seen[accountID] = true
		rows = append(rows, &staffParticipantRow{TenantID: tenantID, ThreadID: threadID, AccountID: accountID})
	}
	if len(rows) == 0 {
		return nil
	}
	if _, err := db.NewInsert().
		Model(&rows).
		ModelTableExpr("users.staff_message_participants").
		Value("history_from", "clock_timestamp()").
		On("CONFLICT (thread_id, account_id) DO NOTHING").
		Exec(ctx); err != nil {
		return fmt.Errorf("add staff group participants: %w", err)
	}
	return nil
}

// RenameStaffGroupThread changes a group's display name. A direct thread is
// never renamed: the kind predicate leaves it untouched.
func (s *ThreadStore) RenameStaffGroupThread(ctx context.Context, threadID int64, name string) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	query := db.NewUpdate().
		Model((*staffThreadRow)(nil)).
		ModelTableExpr(tableExprStaffMessageThreadsAsThread).
		Set("name = ?", name).
		Where(`"staff_message_thread".id = ?`, threadID).
		Where(`"staff_message_thread".kind = ?`, domain.StaffMessageThreadKindGroup)
	query = withTenant(query, staffThreadAlias, tenantID)
	if _, err := query.Exec(ctx); err != nil {
		return fmt.Errorf("rename staff group thread: %w", err)
	}
	return nil
}

// AddStaffGroupParticipants adds members to an existing group.
func (s *ThreadStore) AddStaffGroupParticipants(ctx context.Context, threadID int64, accountIDs []int64) error {
	_, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	if tenantID <= 0 {
		return fmt.Errorf("add staff group participants: tenant is required")
	}
	return s.insertGroupParticipants(ctx, tenantID, threadID, accountIDs)
}

// RemoveStaffParticipant removes one member and their read cursor. Access
// ends with the row: membership is the only thing that grants it.
func (s *ThreadStore) RemoveStaffParticipant(ctx context.Context, threadID, accountID int64) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	participants := db.NewDelete().
		TableExpr(`users.staff_message_participants AS "participant"`).
		Where(`"participant".thread_id = ?`, threadID).
		Where(`"participant".account_id = ?`, accountID)
	participants = withTenant(participants, "participant", tenantID)
	if _, err := participants.Exec(ctx); err != nil {
		return fmt.Errorf("remove staff message participant: %w", err)
	}
	reads := db.NewDelete().
		TableExpr(`users.staff_message_reads AS "cursor"`).
		Where(`"cursor".thread_id = ?`, threadID).
		Where(`"cursor".account_id = ?`, accountID)
	reads = withTenant(reads, "cursor", tenantID)
	if _, err := reads.Exec(ctx); err != nil {
		return fmt.Errorf("remove staff message read cursor: %w", err)
	}
	return nil
}

type staffParticipantStateRow struct {
	AccountID         int64      `bun:"account_id"`
	HistoryFrom       *time.Time `bun:"history_from"`
	LastReadAt        *time.Time `bun:"last_read_at"`
	LastReadMessageID *int64     `bun:"last_read_message_id"`
}

// StaffParticipantStates returns every member with the start of their
// visible history and their read cursor, ascending by account.
func (s *ThreadStore) StaffParticipantStates(ctx context.Context, threadID int64) ([]*communication.StaffParticipantRecord, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []staffParticipantStateRow
	query := db.NewSelect().
		TableExpr(`users.staff_message_participants AS "participant"`).
		ColumnExpr(`"participant".account_id`).
		ColumnExpr(`"participant".history_from`).
		ColumnExpr(`"cursor".last_read_at`).
		ColumnExpr(`"cursor".last_read_message_id`).
		Join(`LEFT JOIN users.staff_message_reads AS "cursor"
			ON "cursor".thread_id = "participant".thread_id
			AND "cursor".account_id = "participant".account_id`).
		Where(`"participant".thread_id = ?`, threadID).
		OrderExpr(`"participant".account_id ASC`)
	query = withTenant(query, "participant", tenantID)
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list staff message participant states: %w", err)
	}
	out := make([]*communication.StaffParticipantRecord, 0, len(rows))
	for _, row := range rows {
		record := &communication.StaffParticipantRecord{AccountID: row.AccountID, HistoryFrom: row.HistoryFrom, LastReadAt: row.LastReadAt}
		if row.LastReadMessageID != nil {
			record.LastReadMessageID = *row.LastReadMessageID
		}
		out = append(out, record)
	}
	return out, nil
}

// DeleteStaffThread removes a conversation with its messages, members and
// cursors (ON DELETE CASCADE). Used when the last member leaves a group.
func (s *ThreadStore) DeleteStaffThread(ctx context.Context, threadID int64) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	query := db.NewDelete().
		Model((*staffThreadRow)(nil)).
		ModelTableExpr(tableExprStaffMessageThreadsAsThread).
		Where(`"staff_message_thread".id = ?`, threadID)
	query = withTenant(query, staffThreadAlias, tenantID)
	if _, err := query.Exec(ctx); err != nil {
		return fmt.Errorf("delete staff message thread: %w", err)
	}
	return nil
}
