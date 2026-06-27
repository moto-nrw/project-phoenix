package parent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// MessageThreadView is the parent-facing chat payload: the conversation header
// (child + the OGS/school it belongs to) plus the messages, oldest-first. The
// counterpart the guardian talks to is always "the OGS [SchoolName]", never an
// individual staff member, so the handler can mask staff sender names.
//
// ThreadID is 0 when the guardian has not written about this child yet — the
// thread is created lazily on the first message (get-or-create).
type MessageThreadView struct {
	ThreadID    int64
	StudentID   int64
	StudentName string
	SchoolName  string
	Messages    []*usersModels.ParentMessage
}

// decorateReadReceipts stamps the "OGS hat gelesen" indicator (ReadByStaff) on
// every guardian-authored message the staff side has already read, using the
// newest staff read cursor in the thread. BOTH the read path
// (GetChildConversation) and the write path (PostChildMessage) run their message
// snapshot through this, so an existing receipt survives a send instead of
// vanishing until the next full GET/SSE refresh. A transient lookup failure is
// logged, not fatal — the indicator simply stays hidden until the next load.
func (s *service) decorateReadReceipts(ctx context.Context, threadID, guardianAccountID int64, messages []*usersModels.ParentMessage) {
	cutoff, err := s.messageReadRepo.LatestReadAtByOther(ctx, threadID, guardianAccountID)
	if err != nil {
		s.logger.Warn("parent: read-receipt lookup failed",
			slog.Int64("thread_id", threadID),
			slog.String("error", err.Error()),
		)
		return
	}
	usersModels.StampStaffReadReceipts(messages, cutoff)
}

// ListMessageThreads returns the guardian's conversations across all their
// children's schools, newest activity first. Children the guardian has not
// written about yet do not appear here — the frontend merges this with the
// full children list so every child is reachable.
func (s *service) ListMessageThreads(ctx context.Context, accountID int64) ([]*usersModels.InboxThread, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}
	// ONE cross-tenant admin transaction for BOTH the child lookup and the thread
	// query. The guardian's threads span every school their children attend, and
	// this fires on every parent_message SSE event and on window focus — resolving
	// children and querying their threads in a single SET ROLE + BEGIN/COMMIT
	// (instead of nesting ListChildrenForAccount's own admin tx inside a second
	// one) halves the round-trips. The thread query returns rows already globally
	// ordered (newest-activity first, nulls last).
	out := make([]*usersModels.InboxThread, 0)
	txErr := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		children, err := s.childRepo.ListByAccount(adminCtx, accountID)
		if err != nil {
			return err
		}
		tenantIDs := distinctTenantIDs(children)
		if len(tenantIDs) == 0 {
			return nil
		}
		rows, err := s.messageReadRepo.ListThreadsForGuardianTenants(adminCtx, accountID, tenantIDs)
		if err != nil {
			return err
		}
		out = rows
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: list message threads: %w", txErr)
	}
	return out, nil
}

// ListChildThreads returns the guardian's conversation(s) about ONE of their
// children — at most one per the chat model — newest activity first. The child
// detail page uses this so it stops fetching the guardian's whole cross-tenant
// inbox and filtering client-side. Ownership (and tenant) is resolved first;
// the per-child query then runs inside that child's tenant tx. Unlike
// GetChildConversation this does NOT mark the thread read.
func (s *service) ListChildThreads(ctx context.Context, accountID, studentID int64) ([]*usersModels.InboxThread, error) {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	out := make([]*usersModels.InboxThread, 0)
	txErr := tenant.WithTenantTx(ctx, s.db, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		rows, err := s.messageReadRepo.ListThreadsForGuardianStudent(txCtx, accountID, studentID)
		if err != nil {
			return err
		}
		out = rows
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: list child threads: %w", txErr)
	}
	return out, nil
}

// distinctTenantIDs collects the unique tenant IDs from the guardian's children
// — the schools they currently have a child at, used to scope cross-tenant
// thread/badge queries.
func distinctTenantIDs(children []*parentModels.ChildSummary) []int64 {
	seen := make(map[int64]bool, len(children))
	out := make([]int64, 0, len(children))
	for _, c := range children {
		if !seen[c.TenantID] {
			seen[c.TenantID] = true
			out = append(out, c.TenantID)
		}
	}
	return out
}

// UnreadMessageCount returns the guardian's total number of conversations with
// unread staff-side activity across all their children's schools — the parent
// portal's sidebar badge. Mirrors the staff UnreadThreadCount: a light per-tenant
// COUNT, instead of fetching every thread's full projection just to sum unreads.
func (s *service) UnreadMessageCount(ctx context.Context, accountID int64) (int, error) {
	if accountID <= 0 {
		return 0, fmt.Errorf("parent: account_id must be positive")
	}
	// Resolve the guardian's children (cross-tenant) in one admin transaction.
	var tenantIDs []int64
	if txErr := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		children, err := s.childRepo.ListByAccount(adminCtx, accountID)
		if err != nil {
			return err
		}
		tenantIDs = distinctTenantIDs(children)
		return nil
	}); txErr != nil {
		return 0, fmt.Errorf("parent: unread message count: %w", txErr)
	}
	if len(tenantIDs) == 0 {
		return 0, nil
	}

	// Only count schools where parent messaging is currently enabled, so the
	// portal badge goes dark for a school that has turned the feature off (the
	// guardian can still open and read the archived history — only the unread
	// signal is suppressed). The flag is per-tenant, so a guardian with one child
	// at an enabled school and one at a disabled school sees only the former's
	// unread. ResolveBoolForTenant wraps its own tenant tx, so it runs outside the
	// admin tx above.
	enabledTenantIDs := make([]int64, 0, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		on, err := s.settings.ResolveBoolForTenant(ctx, tenantID, configModels.KeyParentNotesEnabled)
		if err != nil {
			return 0, fmt.Errorf("parent: resolve messaging setting: %w", err)
		}
		if on {
			enabledTenantIDs = append(enabledTenantIDs, tenantID)
		}
	}
	if len(enabledTenantIDs) == 0 {
		return 0, nil
	}

	total := 0
	txErr := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		count, err := s.messageReadRepo.UnreadThreadCountForGuardianTenants(adminCtx, accountID, enabledTenantIDs)
		if err != nil {
			return err
		}
		total = count
		return nil
	})
	if txErr != nil {
		return 0, fmt.Errorf("parent: unread message count: %w", txErr)
	}
	return total, nil
}

// GetChildConversation returns the guardian's conversation about one owned
// child and marks it read. When no conversation exists yet it returns an empty
// view (ThreadID 0) so the chat window can open ready to write.
func (s *service) GetChildConversation(ctx context.Context, accountID, studentID int64) (*MessageThreadView, error) {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}

	view := &MessageThreadView{
		StudentID:   studentID,
		StudentName: child.studentName,
		SchoolName:  child.schoolName,
	}
	txErr := tenant.WithTenantTx(ctx, s.db, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		thread, err := s.messageThreadRepo.FindByStudentGuardian(txCtx, studentID, accountID)
		if err != nil {
			return err
		}
		if thread == nil {
			return nil // no conversation yet — empty view
		}
		view.ThreadID = thread.ID
		messages, err := s.messageRepo.ListByThread(txCtx, thread.ID, 0)
		if err != nil {
			return err
		}
		// "OGS hat gelesen" receipt — decorate the snapshot through the shared path
		// so the write side (PostChildMessage) renders identical markers.
		s.decorateReadReceipts(txCtx, thread.ID, accountID, messages)
		view.Messages = messages
		// Mark read only up to the newest message actually returned, not NOW(): a
		// staff message that commits between the ListByThread snapshot above and
		// this mark is absent from `messages` yet would fall under a NOW() cursor,
		// dropping it from the guardian's unread count even though it was never
		// shown (and the refetch that would heal it is what advanced the cursor).
		// Messages are ordered ASC, so the last element is the newest.
		if len(messages) > 0 {
			newest := messages[len(messages)-1]
			return s.messageReadRepo.MarkReadUpTo(txCtx, thread.TenantID, thread.ID, accountID, newest.CreatedAt, newest.ID)
		}
		// Empty conversation: do NOT create/advance the read cursor. A NOW() cursor
		// here would move past a staff message that commits between the empty
		// snapshot and this mark, so the parent's unread badge would never light up
		// for that first unseen message.
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: get child conversation: %w", txErr)
	}
	return view, nil
}

// PostChildMessage appends a guardian message to the child's conversation,
// creating the conversation on the first message (get-or-create).
func (s *service) PostChildMessage(ctx context.Context, accountID, studentID int64, body string) (*MessageThreadView, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, ErrEmptyNote
	}
	if utf8.RuneCountInString(body) > maxParentNoteLen {
		return nil, ErrNoteTooLong
	}

	// Posting a message is a per-child WRITE, so it requires parent_portal.notes.write
	// — NOT merely parent_portal.access (visibility). A pickup-only/emergency-contact
	// guardian has access but no write authority; the compose box is hidden for them
	// (ChildFeatures.NotesEnabled gates on the same permission), and this gate makes
	// the API agree with the UI instead of accepting a direct POST. See
	// .claude/rules/guardian-parent-permissions.md.
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionNotesWrite)
	if err != nil {
		return nil, err
	}
	enabled, err := s.settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentNotesEnabled)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve messaging setting: %w", err)
	}
	if !enabled {
		return nil, ErrNotesDisabled
	}

	senderName, err := s.resolveGuardianName(ctx, child.tenantID, accountID)
	if err != nil {
		return nil, err
	}
	view := &MessageThreadView{
		StudentID:   studentID,
		StudentName: child.studentName,
		SchoolName:  child.schoolName,
	}
	txErr := tenant.WithTenantTx(ctx, s.db, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		thread, err := s.messageThreadRepo.GetOrCreate(txCtx, child.tenantID, studentID, accountID)
		if err != nil {
			return err
		}
		if err := s.appendGuardianMessage(txCtx, thread, accountID, senderName, body); err != nil {
			return err
		}
		view.ThreadID = thread.ID
		messages, err := s.messageRepo.ListByThread(txCtx, thread.ID, 0)
		if err != nil {
			return err
		}
		// The frontend replaces its thread state with this returned view, so it must
		// carry the same "OGS hat gelesen" markers GetChildConversation produces —
		// otherwise existing receipts blink off until the next full GET/SSE refresh.
		s.decorateReadReceipts(txCtx, thread.ID, accountID, messages)
		view.Messages = messages
		captured := child.tenantID
		capturedThread := view.ThreadID
		tenant.RegisterAfterCommit(txCtx, func() {
			s.broadcastParentMessage(captured, accountID, capturedThread, studentID)
		})
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: post child message: %w", txErr)
	}
	s.logger.Info("parent sent message",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("tenant_id", child.tenantID),
	)
	return view, nil
}

// appendGuardianMessage writes a guardian message into the thread, updates the
// thread's last-activity fields, and marks it read for the sender.
func (s *service) appendGuardianMessage(ctx context.Context, thread *usersModels.ParentMessageThread, accountID int64, senderName, body string) error {
	msg := &usersModels.ParentMessage{
		ThreadID:        thread.ID,
		StudentID:       thread.StudentID,
		SenderAccountID: accountID,
		SenderKind:      usersModels.ParentMessageSenderGuardian,
		SenderName:      senderName,
		Body:            body,
	}
	msg.SetTenantID(thread.TenantID)
	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return err
	}
	// Drive the thread touch and read cursor off the inserted row's DB-stamped
	// created_at, NOT a Go time.Now(): messages.created_at is the Postgres clock,
	// so seeding from the app clock desyncs the two when the app host leads
	// Postgres (multi-host) — a staff reply within the skew window would be marked
	// read though never seen, and the monotonic preview guard could keep the older
	// message. Mirrors appendStaffMessage on the staff side.
	at := msg.CreatedAt
	// Guarded, monotonic update: a staff reply that committed with a newer instant
	// must keep owning the inbox preview/order, so this guardian send no-ops rather
	// than clobbering it when it is the older of the two (see TouchLastMessage).
	if err := s.messageThreadRepo.TouchLastMessage(ctx, thread.ID, at, usersModels.ParentMessageSenderGuardian, body); err != nil {
		return err
	}
	return s.messageReadRepo.MarkReadUpTo(ctx, thread.TenantID, thread.ID, accountID, at, msg.ID)
}

// resolveGuardianName returns the guardian's display name for the child's
// tenant, "Elternteil" if none. The lookup MUST be scoped to tenantID: a parent
// account is cross-tenant and owns one guardian_profiles row per school, so a
// global admin-scope lookup would pick an arbitrary school's profile (by the
// repository's ordering) and could denormalize school A's name onto a message
// sent in school B — wrong name, plus cross-tenant profile data leaked into the
// chat history permanently. Running inside the tenant tx lets RLS pin the lookup
// to this child's school.
func (s *service) resolveGuardianName(ctx context.Context, tenantID, accountID int64) (string, error) {
	name := "Elternteil"
	if s.guardianProfileRepo == nil {
		return name, nil
	}
	if txErr := tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		profile, err := s.guardianProfileRepo.FindByAccountID(txCtx, accountID)
		if err != nil {
			return err
		}
		// A genuinely missing/blank profile keeps the "Elternteil" default — that is
		// not an error. Only a real lookup FAILURE must propagate (below), so a
		// transient DB blip never denormalizes the wrong sender name onto the
		// persisted message forever.
		if profile == nil {
			return nil
		}
		if full := strings.TrimSpace(profile.FirstName + " " + profile.LastName); full != "" {
			name = full
		}
		return nil
	}); txErr != nil {
		s.logger.Warn("parent: resolve guardian name failed",
			slog.Int64("account_id", accountID),
			slog.String("error", txErr.Error()),
		)
		return "", fmt.Errorf("parent: resolve guardian name: %w", txErr)
	}
	return name, nil
}

// broadcastParentMessage wakes the guardian's own tabs and the tenant's staff.
// threadID/studentID let an open chat skip refetching unrelated threads.
func (s *service) broadcastParentMessage(tenantID, guardianAccountID, threadID, studentID int64) {
	if s.broadcaster == nil || tenantID <= 0 {
		return
	}
	event := realtime.NewParentMessageEvent(guardianAccountID, threadID, studentID)
	if err := s.broadcaster.BroadcastParentMessage(tenantID, guardianAccountID, event); err != nil {
		s.logger.Warn("parent: failed to broadcast parent message",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("guardian_account_id", guardianAccountID),
			slog.String("error", err.Error()),
		)
	}
}
