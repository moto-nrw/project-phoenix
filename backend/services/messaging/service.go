// Package messaging holds the staff-side (tenant portal) business logic for
// the parent-OGS messaging feature: the central inbox, per-thread chats, and
// staff replies / new conversations. A thread is one conversation between the
// OGS and a single guardian about one child (email-like, identified by a
// subject). The parent side lives in services/parent; both operate on the same
// users.parent_message_* tables.
//
// All methods run inside the request's tenant transaction (routes are mounted
// with TenantTxMiddleware), so tenant scoping is enforced by RLS.
package messaging

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const maxMessageLen = 2000

var (
	// ErrForbidden means the staff member may not read/write the thread.
	ErrForbidden = errors.New("messaging: forbidden")
	// ErrThreadNotFound means the thread id does not exist in the tenant.
	ErrThreadNotFound = errors.New("messaging: thread not found")
	// ErrEmptyBody means the message body was blank after trimming.
	ErrEmptyBody = errors.New("messaging: message body must not be empty")
	// ErrBodyTooLong means the message exceeded maxMessageLen.
	ErrBodyTooLong = errors.New("messaging: message body too long")
	// ErrInvalidGuardian means the chosen recipient is not an account-holding
	// guardian of the child.
	ErrInvalidGuardian = errors.New("messaging: recipient is not a guardian of this child")
	// ErrGuardianAccessRevoked means the thread's guardian is no longer a linked
	// guardian of the child with parent_portal.access. Staff keep read access to
	// historical threads, but may not write/broadcast to a recipient the parent
	// APIs now hide.
	ErrGuardianAccessRevoked = errors.New("messaging: recipient no longer has access to this child")
	// ErrMessagingDisabled means the feature flag is off for the tenant.
	ErrMessagingDisabled = errors.New("messaging: messaging disabled for this school")
)

// ThreadDetail is the chat-window payload: conversation header (child,
// guardian + relationship) plus the messages, oldest-first.
type ThreadDetail struct {
	ThreadID          int64
	StudentID         int64
	StudentName       string
	GuardianAccountID int64
	GuardianName      string
	RelationshipType  string
	Messages          []*usersModels.ParentMessage
}

// Service is the staff-side messaging contract.
type Service interface {
	ListInbox(ctx context.Context, onlyUnread bool) ([]*usersModels.InboxThread, error)
	UnreadThreadCount(ctx context.Context) (int, error)
	GetThread(ctx context.Context, threadID int64) (*ThreadDetail, error)
	PostMessage(ctx context.Context, threadID int64, body string) ([]*usersModels.ParentMessage, error)
	StartThread(ctx context.Context, studentID, guardianAccountID int64, body string) (*ThreadDetail, error)
	// OpenThread get-or-creates the conversation for a (student, guardian) pair
	// and returns it (with history if any), without sending a message — the
	// "open the chat" entry point for the staff WhatsApp-style flow. The empty
	// thread it may create stays hidden from the inbox until the first message.
	OpenThread(ctx context.Context, studentID, guardianAccountID int64) (*ThreadDetail, error)
	ListGuardians(ctx context.Context, studentID int64) ([]*usersModels.MessageableGuardian, error)
	// ListStudentThreads returns the staff view of one child's conversations
	// (newest activity first), for the student-detail card. Authorizes read
	// access to the child, then filters server-side instead of having the card
	// fetch the whole tenant inbox.
	ListStudentThreads(ctx context.Context, studentID int64) ([]*usersModels.InboxThread, error)
}

type service struct {
	threadRepo  usersModels.ParentMessageThreadRepository
	messageRepo usersModels.ParentMessageRepository
	readRepo    usersModels.ParentMessageReadRepository

	persons     userService.PersonService
	userContext userContextService.UserContextService
	settings    configService.SettingsService
	broadcaster realtime.Broadcaster

	db     *bun.DB
	logger *slog.Logger
}

// Config is the dependency-injection bundle.
type Config struct {
	ThreadRepo  usersModels.ParentMessageThreadRepository
	MessageRepo usersModels.ParentMessageRepository
	ReadRepo    usersModels.ParentMessageReadRepository
	Persons     userService.PersonService
	UserContext userContextService.UserContextService
	Settings    configService.SettingsService
	Broadcaster realtime.Broadcaster
	DB          *bun.DB
	Logger      *slog.Logger
}

// NewService wires a staff messaging service.
func NewService(cfg Config) Service {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &service{
		threadRepo:  cfg.ThreadRepo,
		messageRepo: cfg.MessageRepo,
		readRepo:    cfg.ReadRepo,
		persons:     cfg.Persons,
		userContext: cfg.UserContext,
		settings:    cfg.Settings,
		broadcaster: cfg.Broadcaster,
		db:          cfg.DB,
		logger:      logger,
	}
}

func (s *service) scope(ctx context.Context) (bool, []int64) {
	perms := jwt.PermissionsFromCtx(ctx)
	return authorize.ResolveStudentReadScope(ctx, perms, s.userContext, s.settings, s.logger)
}

func accountIDFromCtx(ctx context.Context) int64 {
	return int64(jwt.ClaimsFromCtx(ctx).ID)
}

func (s *service) ListInbox(ctx context.Context, onlyUnread bool) ([]*usersModels.InboxThread, error) {
	accountID := accountIDFromCtx(ctx)
	allStudents, groupIDs := s.scope(ctx)
	rows, err := s.readRepo.ListInboxForStaff(ctx, accountID, allStudents, groupIDs, onlyUnread)
	if err != nil {
		return nil, fmt.Errorf("messaging: list inbox: %w", err)
	}
	s.suppressDisabledUnread(ctx, rows)
	return rows, nil
}

// suppressDisabledUnread zeroes the unread count on every inbox/student-card row
// when the school has turned parent messaging off, so the red row pills agree
// with the darkened sidebar badge (UnreadThreadCount returns 0 when disabled).
// Without it a disabled school with historical unread guardian messages shows a
// dark badge but lit pills — the exact disagreement the parent side's
// suppressDisabledUnread was built to prevent. History stays readable; only the
// unread signal is suppressed. The staff service runs inside the request's tenant
// transaction, so a single ResolveBool covers every row. A resolve failure leaves
// the rows untouched — briefly over-counting beats hiding a real unread behind a
// transient settings hiccup (mirrors the parent side's fail-open).
func (s *service) suppressDisabledUnread(ctx context.Context, rows []*usersModels.InboxThread) {
	if len(rows) == 0 {
		return
	}
	enabled, err := s.settings.ResolveBool(ctx, configModels.KeyParentNotesEnabled)
	if err != nil {
		s.logger.Warn("messaging: resolve messaging setting for inbox failed",
			slog.String("error", err.Error()),
		)
		return
	}
	if enabled {
		return
	}
	for _, row := range rows {
		row.UnreadCount = 0
	}
}

func (s *service) UnreadThreadCount(ctx context.Context) (int, error) {
	// Darken the staff sidebar badge when the school has turned parent messaging
	// off: the inbox history stays readable (ListInbox/GetThread are NOT gated),
	// but a disabled feature must not keep lighting up a red unread count. Sends
	// are already blocked by requireEnabled on the write paths.
	if err := s.requireEnabled(ctx); err != nil {
		if errors.Is(err, ErrMessagingDisabled) {
			return 0, nil
		}
		return 0, err
	}
	accountID := accountIDFromCtx(ctx)
	allStudents, groupIDs := s.scope(ctx)
	count, err := s.readRepo.UnreadThreadCountForStaff(ctx, accountID, allStudents, groupIDs)
	if err != nil {
		return 0, fmt.Errorf("messaging: unread count: %w", err)
	}
	return count, nil
}

// canReadStudent loads the student and checks the staff member's read access.
func (s *service) canReadStudent(ctx context.Context, studentID int64) error {
	student, err := s.persons.GetStudentByID(ctx, studentID)
	if err != nil {
		// A missing/out-of-tenant student (stale search result, hand-crafted id)
		// is an authorization decision, not a server fault: GetStudentByID wraps
		// sql.ErrNoRows for a row that does not exist, so treat that as 403.
		if errors.Is(err, sql.ErrNoRows) {
			return ErrForbidden
		}
		// A transient lookup failure (DB blip/timeout) is NOT an authorization
		// decision: surface it as a server error (→ 500, retryable) rather than a
		// permanent ErrForbidden (403) that tells staff they may never read this
		// thread.
		return fmt.Errorf("messaging: load student for read check: %w", err)
	}
	if student == nil {
		return ErrForbidden
	}
	perms := jwt.PermissionsFromCtx(ctx)
	if !authorize.CanReadStudent(ctx, perms, student, s.userContext, s.settings, s.logger) {
		return ErrForbidden
	}
	return nil
}

// loadAuthorizedThread fetches the thread and enforces staff read access to its
// child. Returns ErrThreadNotFound / ErrForbidden as appropriate.
func (s *service) loadAuthorizedThread(ctx context.Context, threadID int64) (*usersModels.ParentMessageThread, error) {
	thread, err := s.threadRepo.FindByID(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("messaging: find thread: %w", err)
	}
	if thread == nil {
		return nil, ErrThreadNotFound
	}
	if err := s.canReadStudent(ctx, thread.StudentID); err != nil {
		return nil, err
	}
	return thread, nil
}

// requireLinkedGuardian verifies the thread's guardian is STILL a linked
// guardian of the child with parent_portal.access. Staff retain READ access to
// a historical thread after a guardian is unlinked or downgraded (so chat
// history stays visible), but writing or broadcasting to it would (a) create
// messages the recipient can no longer legitimately read and (b) leak
// thread/student activity to the revoked account via the parent SSE event. The
// parent reads reject these same threads (resolveOwnedChild/resolvePermittedChild
// require parent_portal.access) and ListGuardiansForStudent applies the
// identical JSONB containment filter, so the staff write path agrees with the
// parent read path. Must run inside the tenant transaction.
func (s *service) requireLinkedGuardian(ctx context.Context, thread *usersModels.ParentMessageThread) error {
	guardians, err := s.threadRepo.ListGuardiansForStudent(ctx, thread.StudentID)
	if err != nil {
		return fmt.Errorf("messaging: guardian link check: %w", err)
	}
	if !containsGuardian(guardians, thread.GuardianAccountID) {
		return ErrGuardianAccessRevoked
	}
	return nil
}

// buildThreadDetail snapshots the thread's messages and builds the chat detail.
// Used by the send path (StartThread), which does not advance the read cursor
// off the listed snapshot.
func (s *service) buildThreadDetail(ctx context.Context, thread *usersModels.ParentMessageThread) (*ThreadDetail, error) {
	messages, err := s.messageRepo.ListByThread(ctx, thread.ID, 0)
	if err != nil {
		return nil, fmt.Errorf("messaging: list messages: %w", err)
	}
	return s.buildDetailFromMessages(ctx, thread, messages), nil
}

// buildDetailFromMessages assembles the chat-window payload (read receipts,
// header, request diffs) from an already-fetched message snapshot, so the read
// paths can mark-read off the SAME snapshot they return (see markReadAndBuild).
func (s *service) buildDetailFromMessages(ctx context.Context, thread *usersModels.ParentMessageThread, messages []*usersModels.ParentMessage) *ThreadDetail {
	// "OGS hat gelesen" receipt: flag guardian messages a staff member has read.
	// Shared with the parent side via parentmessaging.DecorateReadReceipts so the
	// receipt rule can't drift between the two chats (the staff reader's own read
	// is excluded by passing the thread's guardian as the "other" account).
	parentmessaging.DecorateReadReceipts(ctx, s.readRepo, s.logger, thread.ID, thread.GuardianAccountID, messages)
	detail := &ThreadDetail{
		ThreadID:          thread.ID,
		StudentID:         thread.StudentID,
		GuardianAccountID: thread.GuardianAccountID,
		Messages:          messages,
	}
	if header, err := s.readRepo.FindThreadHeader(ctx, thread.ID); err == nil && header != nil {
		detail.StudentName = header.StudentName
		detail.GuardianName = header.GuardianName
		detail.RelationshipType = header.RelationshipType
	}
	return detail
}

// markReadAndBuild lists the thread's messages, advances the staff reader's read
// cursor only up to the newest message actually fetched (never NOW()), and
// returns the chat detail built from that same snapshot. Marking to NOW() would
// advance the cursor past a guardian message that committed between the snapshot
// and the mark, silently dropping it from the staff unread badge though staff
// never saw it — the same hazard the parent side guards against (see parent
// GetChildConversation). Messages are oldest-first, so the last is the newest.
func (s *service) markReadAndBuild(ctx context.Context, thread *usersModels.ParentMessageThread) (*ThreadDetail, error) {
	messages, err := s.messageRepo.ListByThread(ctx, thread.ID, 0)
	if err != nil {
		return nil, fmt.Errorf("messaging: list messages: %w", err)
	}
	accountID := accountIDFromCtx(ctx)
	if len(messages) > 0 {
		newest := messages[len(messages)-1]
		if err := s.readRepo.MarkReadUpTo(ctx, thread.TenantID, thread.ID, accountID, newest.CreatedAt, newest.ID); err != nil {
			return nil, fmt.Errorf("messaging: mark read: %w", err)
		}
	}
	// Empty snapshot: leave the cursor untouched. Marking read to NOW() on an empty
	// get-or-created thread would advance the cursor past a guardian message that
	// commits between this empty ListByThread snapshot and the mark, dropping it
	// from the staff unread badge though staff never saw it.
	return s.buildDetailFromMessages(ctx, thread, messages), nil
}

func (s *service) GetThread(ctx context.Context, threadID int64) (*ThreadDetail, error) {
	thread, err := s.loadAuthorizedThread(ctx, threadID)
	if err != nil {
		return nil, err
	}
	return s.markReadAndBuild(ctx, thread)
}

func (s *service) PostMessage(ctx context.Context, threadID int64, body string) ([]*usersModels.ParentMessage, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, ErrEmptyBody
	}
	if utf8.RuneCountInString(body) > maxMessageLen {
		return nil, ErrBodyTooLong
	}
	thread, err := s.loadAuthorizedThread(ctx, threadID)
	if err != nil {
		return nil, err
	}
	if err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}
	// Refuse to send to a guardian who has been unlinked / lost parent_portal.access
	// since the thread was created — the parent APIs hide such threads, so a reply
	// would be unreadable and the SSE wake-up would leak activity to a revoked account.
	if err := s.requireLinkedGuardian(ctx, thread); err != nil {
		return nil, err
	}

	accountID := accountIDFromCtx(ctx)
	if err := s.appendStaffMessage(ctx, thread, accountID, body); err != nil {
		return nil, err
	}

	messages, err := s.messageRepo.ListByThread(ctx, thread.ID, 0)
	if err != nil {
		return nil, fmt.Errorf("messaging: list messages: %w", err)
	}
	s.broadcastAfterCommit(ctx, thread)
	return messages, nil
}

// authorizeThreadParticipants enforces the shared precondition for opening or
// starting a conversation: the staffer may read the child, messaging is enabled
// for the tenant, and the chosen recipient is an account-holding guardian of the
// child. StartThread and OpenThread both run it so their access rules cannot
// drift apart. Must run inside the tenant transaction.
func (s *service) authorizeThreadParticipants(ctx context.Context, studentID, guardianAccountID int64) error {
	if err := s.canReadStudent(ctx, studentID); err != nil {
		return err
	}
	if err := s.requireEnabled(ctx); err != nil {
		return err
	}
	// The recipient must be an account-holding guardian of this child.
	guardians, err := s.threadRepo.ListGuardiansForStudent(ctx, studentID)
	if err != nil {
		return fmt.Errorf("messaging: list guardians: %w", err)
	}
	if !containsGuardian(guardians, guardianAccountID) {
		return ErrInvalidGuardian
	}
	return nil
}

// StartThread sends the OGS's first message to a guardian about a child. The
// conversation is get-or-create: if one already exists for the (student,
// guardian) pair the message is appended to it instead of opening a second.
func (s *service) StartThread(ctx context.Context, studentID, guardianAccountID int64, body string) (*ThreadDetail, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, ErrEmptyBody
	}
	if utf8.RuneCountInString(body) > maxMessageLen {
		return nil, ErrBodyTooLong
	}
	if err := s.authorizeThreadParticipants(ctx, studentID, guardianAccountID); err != nil {
		return nil, err
	}

	accountID := accountIDFromCtx(ctx)
	thread, err := s.threadRepo.GetOrCreate(ctx, tenant.FromContext(ctx), studentID, guardianAccountID)
	if err != nil {
		return nil, fmt.Errorf("messaging: get-or-create thread: %w", err)
	}
	if err := s.appendStaffMessage(ctx, thread, accountID, body); err != nil {
		return nil, err
	}
	s.broadcastAfterCommit(ctx, thread)
	s.logger.Info("staff sent parent message",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("guardian_account_id", guardianAccountID),
		slog.Int64("tenant_id", thread.TenantID),
	)
	return s.buildThreadDetail(ctx, thread)
}

// OpenThread get-or-creates the (student, guardian) conversation and returns
// it ready for the chat window, marking it read. No message is sent; a freshly
// created thread has no messages and is filtered out of the inbox until the
// first reply, so opening a chat never litters the inbox.
func (s *service) OpenThread(ctx context.Context, studentID, guardianAccountID int64) (*ThreadDetail, error) {
	if err := s.authorizeThreadParticipants(ctx, studentID, guardianAccountID); err != nil {
		return nil, err
	}

	thread, err := s.threadRepo.GetOrCreate(ctx, tenant.FromContext(ctx), studentID, guardianAccountID)
	if err != nil {
		return nil, fmt.Errorf("messaging: get-or-create thread: %w", err)
	}
	return s.markReadAndBuild(ctx, thread)
}

func (s *service) ListGuardians(ctx context.Context, studentID int64) ([]*usersModels.MessageableGuardian, error) {
	if err := s.canReadStudent(ctx, studentID); err != nil {
		return nil, err
	}
	guardians, err := s.threadRepo.ListGuardiansForStudent(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("messaging: list guardians: %w", err)
	}
	return guardians, nil
}

func (s *service) ListStudentThreads(ctx context.Context, studentID int64) ([]*usersModels.InboxThread, error) {
	if err := s.canReadStudent(ctx, studentID); err != nil {
		return nil, err
	}
	rows, err := s.readRepo.ListThreadsForStudent(ctx, accountIDFromCtx(ctx), studentID)
	if err != nil {
		return nil, fmt.Errorf("messaging: list student threads: %w", err)
	}
	s.suppressDisabledUnread(ctx, rows)
	return rows, nil
}

// appendStaffMessage writes a staff message into the thread, stamps the
// sender's name, and updates the thread's last-activity fields. The caller has
// already authorized and validated the body. The append + read-cursor invariant
// is shared with the parent side via parentmessaging.AppendMessage (one home for
// the "drive off the DB-stamped created_at" rule).
func (s *service) appendStaffMessage(ctx context.Context, thread *usersModels.ParentMessageThread, accountID int64, body string) error {
	msg := &usersModels.ParentMessage{
		ThreadID:        thread.ID,
		StudentID:       thread.StudentID,
		SenderAccountID: accountID,
		SenderKind:      usersModels.ParentMessageSenderStaff,
		SenderName:      s.resolveStaffName(ctx, accountID),
		Body:            body,
	}
	msg.SetTenantID(thread.TenantID)
	if err := parentmessaging.AppendMessage(ctx, s.messageRepo, s.threadRepo, s.readRepo, msg); err != nil {
		return fmt.Errorf("messaging: append staff message: %w", err)
	}
	return nil
}

func (s *service) requireEnabled(ctx context.Context) error {
	enabled, err := s.settings.ResolveBool(ctx, configModels.KeyParentNotesEnabled)
	if err != nil {
		return fmt.Errorf("messaging: resolve setting: %w", err)
	}
	if !enabled {
		return ErrMessagingDisabled
	}
	return nil
}

// resolveStaffName returns the staff member's display name, "OGS-Team" if none.
func (s *service) resolveStaffName(ctx context.Context, accountID int64) string {
	name := "OGS-Team"
	if s.persons == nil {
		return name
	}
	person, err := s.persons.FindByAccountID(ctx, accountID)
	if err != nil || person == nil {
		return name
	}
	if full := strings.TrimSpace(person.FirstName + " " + person.LastName); full != "" {
		name = full
	}
	return name
}

// broadcastAfterCommit queues the SSE wake-up to fire only AFTER the request
// transaction commits, mirroring the parent side (parent_messaging_service.go).
// TenantTxMiddleware commits after the handler returns, so broadcasting inline
// would let a woken client refetch the pre-commit snapshot (stale read), and a
// 5xx rollback would fire a phantom "new message" for a row that never
// persisted. Scalars are captured now because the thread pointer may be mutated
// after this call returns.
func (s *service) broadcastAfterCommit(ctx context.Context, thread *usersModels.ParentMessageThread) {
	if thread == nil {
		return
	}
	tenantID := thread.TenantID
	guardianAccountID := thread.GuardianAccountID
	threadID := thread.ID
	studentID := thread.StudentID
	tenant.RegisterAfterCommit(ctx, func() {
		s.broadcastValues(tenantID, guardianAccountID, threadID, studentID)
	})
}

func (s *service) broadcastValues(tenantID, guardianAccountID, threadID, studentID int64) {
	parentmessaging.Broadcast(s.broadcaster, s.logger, tenantID, guardianAccountID, threadID, studentID)
}

func containsGuardian(guardians []*usersModels.MessageableGuardian, accountID int64) bool {
	for _, g := range guardians {
		if g != nil && g.AccountID == accountID {
			return true
		}
	}
	return false
}
