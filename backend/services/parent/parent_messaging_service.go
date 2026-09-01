package parent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	notificationsSvc "github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
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
// every guardian-authored message the staff side has already read. BOTH the read
// path (GetChildConversation) and the write path (PostChildMessage) run their
// message snapshot through this, so an existing receipt survives a send instead
// of vanishing until the next full GET/SSE refresh. Delegates to the shared
// parentmessaging.DecorateReadReceipts so the receipt rule stays identical to the
// staff chat (one home for it, not two hand-mirrored copies).
func (s *service) decorateReadReceipts(ctx context.Context, threadID, guardianAccountID int64, messages []*usersModels.ParentMessage) {
	parentmessaging.DecorateReadReceipts(ctx, s.MessageReadRepo, s.Logger, threadID, guardianAccountID, messages)
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
	txErr := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		children, err := s.ChildRepo.ListByAccount(adminCtx, accountID)
		if err != nil {
			return err
		}
		tenantIDs := distinctTenantIDs(children)
		if len(tenantIDs) == 0 {
			return nil
		}
		rows, err := s.MessageReadRepo.ListThreadsForGuardianTenants(adminCtx, accountID, tenantIDs)
		if err != nil {
			return err
		}
		out = rows
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: list message threads: %w", txErr)
	}
	// Darken the per-row unread pill for a school that has turned messaging off, so
	// the row agrees with the sidebar badge (UnreadMessageCount counts only enabled
	// tenants). History stays readable — only the unread signal is suppressed. Only
	// the multi-child picker calls this, so the extra per-tenant flag reads stay off
	// the common single-child path.
	s.suppressDisabledUnread(ctx, out)
	return out, nil
}

// suppressDisabledUnread zeroes the unread count on rows whose tenant has parent
// messaging turned off, mirroring UnreadMessageCount's enabled-tenant filter so
// the row pills and the aggregate badge can never disagree. The enabled flag is
// resolved once per distinct tenant (cached in-loop). The fail-OPEN direction (a
// resolve failure counts the tenant as enabled — hiding a real unread behind a
// transient settings hiccup is worse than briefly over-counting) and the setting
// key both live in parentmessaging.MessagingEnabledForTenant, shared with the
// staff side. ResolveBoolForTenant opens its own tenant tx, so this runs OUTSIDE
// the admin tx above.
func (s *service) suppressDisabledUnread(ctx context.Context, rows []*usersModels.InboxThread) {
	enabled := make(map[int64]bool)
	for _, row := range rows {
		if row.UnreadCount == 0 || row.TenantID <= 0 {
			continue
		}
		on, cached := enabled[row.TenantID]
		if !cached {
			on = parentmessaging.MessagingEnabledForTenant(ctx, s.Settings, row.TenantID, s.Logger)
			enabled[row.TenantID] = on
		}
		if !on {
			row.UnreadCount = 0
		}
	}
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
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		rows, err := s.MessageReadRepo.ListThreadsForGuardianStudent(txCtx, accountID, studentID)
		if err != nil {
			return err
		}
		out = rows
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: list child threads: %w", txErr)
	}
	// Same suppression as ListMessageThreads: darken the per-row unread pill for a
	// school that has turned messaging off so the child-detail row agrees with the
	// sidebar badge (UnreadMessageCount counts only enabled tenants). Without it the
	// badge reads 0 while the child-detail panel still renders a red pill.
	s.suppressDisabledUnread(ctx, out)
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
	if txErr := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		children, err := s.ChildRepo.ListByAccount(adminCtx, accountID)
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
	// unread. The per-tenant resolve fails OPEN and shares its key with the
	// row-pill path via parentmessaging.MessagingEnabledForTenant (which wraps its
	// own tenant tx, so it runs outside the admin tx above): a transient settings
	// hiccup must not make the badge vanish while the thread-list rows keep showing
	// unread pills.
	enabledTenantIDs := make([]int64, 0, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		if parentmessaging.MessagingEnabledForTenant(ctx, s.Settings, tenantID, s.Logger) {
			enabledTenantIDs = append(enabledTenantIDs, tenantID)
		}
	}
	if len(enabledTenantIDs) == 0 {
		return 0, nil
	}

	total := 0
	txErr := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		count, err := s.MessageReadRepo.UnreadMessageCountForGuardianTenants(adminCtx, accountID, enabledTenantIDs)
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
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		thread, err := s.MessageThreadRepo.FindByStudentGuardian(txCtx, studentID, accountID)
		if err != nil {
			return err
		}
		if thread == nil {
			return nil // no conversation yet — empty view
		}
		view.ThreadID = thread.ID
		messages, err := s.MessageRepo.ListByThread(txCtx, thread.ID, 0)
		if err != nil {
			return err
		}
		// "OGS hat gelesen" receipt — decorate the snapshot through the shared path
		// so the write side (PostChildMessage) renders identical markers.
		s.decorateReadReceipts(txCtx, thread.ID, accountID, messages)
		view.Messages = messages
		// Mark read only up to the newest message actually returned, never NOW().
		// The mark-to-newest invariant (and the empty-conversation skip) lives in
		// parentmessaging.MarkReadToNewest, shared with the staff side so the two
		// portals' unread counts can't drift.
		advanced, err := parentmessaging.MarkReadToNewest(txCtx, s.MessageReadRepo, thread.TenantID, thread.ID, accountID, false, messages)
		if err != nil {
			return err
		}
		if advanced {
			// The guardian just read staff messages — wake the OGS side so its
			// staff-facing "von den Eltern gelesen" receipt updates live. After commit
			// and only on a real advance, so it can't loop with the refetch it triggers.
			tenantID := thread.TenantID
			threadID := thread.ID
			tenant.RegisterAfterCommit(txCtx, func() {
				s.broadcastReadReceipt(tenantID, accountID, threadID, studentID)
			})
		}
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
	// A child whose care at this school has ended keeps read access to what
	// happened, but nothing new can be submitted for them (#2487).
	if err := child.requireCareRunning(); err != nil {
		return nil, err
	}
	// Fail OPEN on a transient resolve error (via the shared helper) so a config-DB
	// blip does not 500 a guardian's reply while the badge and thread list keep
	// rendering unread — the write side agrees with the read side. A genuine
	// disabled flag still returns ErrNotesDisabled.
	if !parentmessaging.MessagingEnabledForTenant(ctx, s.Settings, child.tenantID, s.Logger) {
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
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		student, err := s.StudentRepo.FindByIDForUpdate(txCtx, studentID)
		if err != nil {
			return err
		}
		if student.CareEndedOn(s.todayDate()) {
			return ErrChildCareEnded
		}
		thread, err := s.MessageThreadRepo.GetOrCreate(txCtx, child.tenantID, studentID, accountID)
		if err != nil {
			return err
		}
		message, err := s.appendGuardianMessage(txCtx, thread, accountID, senderName, body)
		if err != nil {
			return err
		}
		view.ThreadID = thread.ID
		messages, err := s.MessageRepo.ListByThread(txCtx, thread.ID, 0)
		if err != nil {
			return err
		}
		// The frontend replaces its thread state with this returned view, so it must
		// carry the same "OGS hat gelesen" markers GetChildConversation produces —
		// otherwise existing receipts blink off until the next full GET/SSE refresh.
		s.decorateReadReceipts(txCtx, thread.ID, accountID, messages)
		view.Messages = messages
		// Advance the guardian's read cursor over this returned snapshot, exactly as
		// GetChildConversation does. The guardian is now viewing this list, so any
		// staff message that became visible since the last GET (e.g. a reply that
		// arrived while an SSE refresh was missed) must count as read — otherwise the
		// sidebar badge keeps counting a message already on screen until a separate
		// read-marking GET runs. Bounded to the newest STAFF row in the snapshot
		// (never NOW(), never our own just-sent message), so it can't leap the cursor
		// to ~now and swallow a staff message committing concurrently in a still-open
		// tx. See parentmessaging.MarkReadToNewest.
		if _, err := parentmessaging.MarkReadToNewest(txCtx, s.MessageReadRepo, thread.TenantID, thread.ID, accountID, false, messages); err != nil {
			return err
		}
		captured := child.tenantID
		capturedThread := view.ThreadID
		if s.ParentMessageNotifier != nil {
			if err := s.ParentMessageNotifier.NotifyStaffParentMessage(txCtx, notificationsSvc.StaffParentMessageReport{
				TenantID: captured, ThreadID: capturedThread, MessageID: message.ID,
				StudentID: studentID, ActorAccountID: accountID,
			}); err != nil {
				return err
			}
		}
		tenant.RegisterAfterCommit(txCtx, func() {
			s.broadcastParentMessage(captured, accountID, capturedThread, studentID)
		})
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: post child message: %w", txErr)
	}
	s.Logger.Info("parent sent message",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("tenant_id", child.tenantID),
	)
	return view, nil
}

// appendGuardianMessage writes a guardian message into the thread and updates the
// thread's last-activity fields. The append invariant (and why it does NOT move
// the sender's read cursor) is shared with the staff side via
// parentmessaging.AppendMessage (one home for the "drive off the DB-stamped
// created_at" rule).
func (s *service) appendGuardianMessage(ctx context.Context, thread *usersModels.ParentMessageThread, accountID int64, senderName, body string) (*usersModels.ParentMessage, error) {
	msg := &usersModels.ParentMessage{
		ThreadID:        thread.ID,
		StudentID:       thread.StudentID,
		SenderAccountID: accountID,
		SenderKind:      usersModels.ParentMessageSenderGuardian,
		SenderName:      senderName,
		Body:            body,
		Kind:            usersModels.ParentMessageKindMessage,
	}
	msg.SetTenantID(thread.TenantID)
	if err := parentmessaging.AppendMessage(ctx, s.MessageRepo, s.MessageThreadRepo, msg); err != nil {
		return nil, err
	}
	return msg, nil
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
	if s.GuardianProfileRepo == nil {
		return name, nil
	}
	if txErr := tenant.WithTenantTx(ctx, s.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		profile, err := s.GuardianProfileRepo.FindByAccountID(txCtx, accountID)
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
		s.Logger.Warn("parent: resolve guardian name failed",
			slog.Int64("account_id", accountID),
			slog.String("error", txErr.Error()),
		)
		return "", fmt.Errorf("parent: resolve guardian name: %w", txErr)
	}
	return name, nil
}

// broadcastParentMessage wakes the guardian's own tabs and the tenant's staff.
// threadID/studentID let an open chat skip refetching unrelated threads. The
// fan-out contract is shared with the staff side via parentmessaging.Broadcast.
func (s *service) broadcastParentMessage(tenantID, guardianAccountID, threadID, studentID int64) {
	parentmessaging.Broadcast(s.Broadcaster, s.Logger, tenantID, guardianAccountID, threadID, studentID)
}

// broadcastReadReceipt wakes the OGS side (and the guardian's own tabs) to refresh
// "Gelesen" receipts after the guardian read a conversation, over the shared
// receipt fan-out. Mirrors broadcastParentMessage but as a read-only event;
// callers fire it after commit and ONLY on a real cursor advance, so it can't loop
// with the receipt refetch it triggers.
func (s *service) broadcastReadReceipt(tenantID, guardianAccountID, threadID, studentID int64) {
	parentmessaging.BroadcastRead(s.Broadcaster, s.Logger, tenantID, guardianAccountID, threadID, studentID)
}
