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
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/notifications"
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
	// ErrHandledBoundaryRequired means the replying client did not identify the
	// message snapshot its team reply covers.
	ErrHandledBoundaryRequired = errors.New("messaging: handled message boundary required")
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

// Service is the staff-side messaging service.
type Service struct {
	Config
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

	// Notifier and Preferences push a staff reply to the guardian's devices
	// (#1671). Both optional and both required together: without the consent
	// service there is nobody who agreed, so the push is skipped rather than
	// sent past consent.
	Notifier    notifications.Service
	Preferences notifications.PreferenceService

	// Outbox, GuardianProfiles, Schools and ParentsURL queue the guardian e-mail
	// for a new OGS message (#2307). All are optional so bare-constructed unit
	// test services remain silent. LoginImages only decorates the mail header.
	Outbox           platformModels.OutboxEnqueuer
	GuardianProfiles GuardianProfileFinder
	Schools          SchoolFinder
	LoginImages      LoginImageResolver
	ParentsURL       string
}

// NewService wires a staff messaging service.
func NewService(cfg Config) *Service {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Service{Config: cfg}
}

func (s *Service) scope(ctx context.Context) bool {
	perms := jwt.PermissionsFromCtx(ctx)
	return authorize.ResolveStudentReadScope(ctx, perms, s.UserContext)
}

func accountIDFromCtx(ctx context.Context) int64 {
	return int64(jwt.ClaimsFromCtx(ctx).ID)
}

func (s *Service) ListInbox(ctx context.Context, onlyUnread bool) ([]*usersModels.InboxThread, error) {
	accountID := accountIDFromCtx(ctx)
	allStudents := s.scope(ctx)
	rows, err := s.ReadRepo.ListInboxForStaff(ctx, accountID, allStudents, onlyUnread)
	if err != nil {
		return nil, fmt.Errorf("messaging: list inbox: %w", err)
	}
	s.suppressDisabledUnread(ctx, rows)
	// suppressDisabledUnread zeroes every unread badge when the school has turned
	// messaging off. In the "Nur ungelesen" view that would otherwise leave rows
	// with no visible badge — an internally contradictory "unread" list (the rows
	// were SELECTed for having unread, then their badges were suppressed). Drop the
	// now-badgeless rows so the filter and the suppressed badges agree. When
	// messaging is enabled the badges are intact (every onlyUnread row has count
	// >= 1), so this is a no-op there.
	if onlyUnread {
		rows = keepUnread(rows)
	}
	return rows, nil
}

// keepUnread returns only the rows that still carry an unread badge, preserving
// order. Used by the onlyUnread inbox after suppressDisabledUnread so a disabled
// school's filtered list does not render badgeless "unread" rows.
func keepUnread(rows []*usersModels.InboxThread) []*usersModels.InboxThread {
	out := make([]*usersModels.InboxThread, 0, len(rows))
	for _, row := range rows {
		if row.UnreadCount > 0 {
			out = append(out, row)
		}
	}
	return out
}

// suppressDisabledUnread zeroes the unread count on every inbox/student-card row
// when the school has turned parent messaging off, so the red row pills agree
// with the darkened sidebar badge (UnreadMessageCount returns 0 when disabled).
// Without it a disabled school with historical unread guardian messages shows a
// dark badge but lit pills — the exact disagreement the parent side's
// suppressDisabledUnread was built to prevent. History stays readable; only the
// unread signal is suppressed. The staff service runs inside the request's tenant
// transaction, so a single resolve covers every row. The "is messaging enabled"
// decision (and its fail-OPEN direction: a resolve failure leaves the rows
// untouched, briefly over-counting rather than hiding a real unread) lives in
// parentmessaging.MessagingEnabled, shared with the badge and the parent side so
// the four gate sites can never drift to disagreeing answers.
func (s *Service) suppressDisabledUnread(ctx context.Context, rows []*usersModels.InboxThread) {
	if len(rows) == 0 {
		return
	}
	if parentmessaging.MessagingEnabled(ctx, s.Settings, s.Logger) {
		return
	}
	for _, row := range rows {
		row.UnreadCount = 0
	}
}

func (s *Service) UnreadMessageCount(ctx context.Context) (int, error) {
	// Darken the staff sidebar badge when the school has turned parent messaging
	// off: the inbox history stays readable (ListInbox/GetThread are NOT gated),
	// but a disabled feature must not keep lighting up a red unread count. Sends
	// are already blocked by requireEnabled on the write paths. The enabled check
	// (and its fail-OPEN direction) is shared via parentmessaging.MessagingEnabled
	// so the badge, the row pills, and the write paths agree on one answer.
	if !parentmessaging.MessagingEnabled(ctx, s.Settings, s.Logger) {
		return 0, nil
	}
	accountID := accountIDFromCtx(ctx)
	allStudents := s.scope(ctx)
	count, err := s.ReadRepo.UnreadMessageCountForStaff(ctx, accountID, allStudents)
	if err != nil {
		return 0, fmt.Errorf("messaging: unread count: %w", err)
	}
	return count, nil
}

// canReadStudent loads the student and checks the staff member's read access.
func (s *Service) canReadStudent(ctx context.Context, studentID int64) error {
	student, err := s.Persons.GetStudentByID(ctx, studentID)
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
	if student.IsAlumnus() {
		return ErrForbidden
	}
	perms := jwt.PermissionsFromCtx(ctx)
	if !authorize.CanReadStudent(ctx, perms, student, s.UserContext) {
		return ErrForbidden
	}
	return nil
}

// loadAuthorizedThread fetches the thread and enforces staff read access to its
// child. Returns ErrThreadNotFound / ErrForbidden as appropriate.
func (s *Service) loadAuthorizedThread(ctx context.Context, threadID int64) (*usersModels.ParentMessageThread, error) {
	thread, err := s.ThreadRepo.FindByID(ctx, threadID)
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
func (s *Service) requireLinkedGuardian(ctx context.Context, thread *usersModels.ParentMessageThread) error {
	guardians, err := s.ThreadRepo.ListGuardiansForStudent(ctx, thread.StudentID)
	if err != nil {
		return fmt.Errorf("messaging: guardian link check: %w", err)
	}
	if !containsGuardian(guardians, thread.GuardianAccountID) {
		return ErrGuardianAccessRevoked
	}
	return nil
}

// buildDetailFromMessages assembles the chat-window payload (read receipts,
// header) from an already-fetched message snapshot, so the read paths can
// mark-read off the SAME snapshot they return (see markReadAndBuild).
func (s *Service) buildDetailFromMessages(ctx context.Context, thread *usersModels.ParentMessageThread, messages []*usersModels.ParentMessage) (*ThreadDetail, error) {
	// "OGS hat gelesen" receipt: flag guardian messages a staff member has read.
	// Shared with the parent side via parentmessaging.DecorateReadReceipts so the
	// receipt rule can't drift between the two chats (the staff reader's own read
	// is excluded by passing the thread's guardian as the "other" account).
	parentmessaging.DecorateReadReceipts(ctx, s.ReadRepo, s.Logger, thread.ID, thread.GuardianAccountID, messages)
	// "Gelesen" receipt on the staff's OWN messages: flag staff messages the
	// guardian has read, using the guardian's read cursor. Symmetric to the
	// "OGS hat gelesen" receipt above so each side sees when the other has read.
	parentmessaging.DecorateGuardianReadReceipts(ctx, s.ReadRepo, s.Logger, thread.ID, messages)
	detail := &ThreadDetail{
		ThreadID:          thread.ID,
		StudentID:         thread.StudentID,
		GuardianAccountID: thread.GuardianAccountID,
		Messages:          messages,
	}
	// A lookup error is a request failure, not blank metadata: GetThread has
	// already advanced the read cursor by this point, so silently returning an
	// empty-header detail would mark the thread read while handing the client an
	// incomplete chat. A genuinely missing header (nil, no error) is fine to
	// leave blank — the thread simply has no resolvable student/guardian names.
	header, err := s.ReadRepo.FindThreadHeader(ctx, thread.ID)
	if err != nil {
		return nil, fmt.Errorf("messaging: thread header: %w", err)
	}
	if header != nil {
		detail.StudentName = header.StudentName
		detail.GuardianName = header.GuardianName
		detail.RelationshipType = header.RelationshipType
	}
	return detail, nil
}

// markReadAndBuild lists the thread's messages, advances the staff reader's read
// cursor only up to the newest message actually fetched (never NOW()), and
// returns the chat detail built from that same snapshot. The mark-to-newest rule
// (and why NOW() would silently drop a just-committed guardian message from the
// staff badge) lives in parentmessaging.MarkReadToNewest, shared with the parent
// side so the two portals can't drift.
// markReadAndBuild also reports whether the staff reader's cursor ADVANCED, so the
// pure-read callers (GetThread, OpenThread) can fire a read-receipt SSE wake-up to
// the guardian's open chat only on a real move. The send callers (StartThread,
// PostMessage) ignore it — their new-message broadcast already refreshes receipts.
func (s *Service) markReadAndBuild(ctx context.Context, thread *usersModels.ParentMessageThread) (*ThreadDetail, bool, error) {
	messages, err := s.MessageRepo.ListByThread(ctx, thread.ID, 0)
	if err != nil {
		return nil, false, fmt.Errorf("messaging: list messages: %w", err)
	}
	advanced, err := parentmessaging.MarkReadToNewest(ctx, s.ReadRepo, thread.TenantID, thread.ID, accountIDFromCtx(ctx), true, messages)
	if err != nil {
		return nil, false, fmt.Errorf("messaging: mark read: %w", err)
	}
	detail, err := s.buildDetailFromMessages(ctx, thread, messages)
	if err != nil {
		return nil, false, err
	}
	return detail, advanced, nil
}

func (s *Service) GetThread(ctx context.Context, threadID int64) (*ThreadDetail, error) {
	thread, err := s.loadAuthorizedThread(ctx, threadID)
	if err != nil {
		return nil, err
	}
	detail, advanced, err := s.markReadAndBuild(ctx, thread)
	if err != nil {
		return nil, err
	}
	if advanced {
		// Staff just read guardian messages — wake the guardian's open chat so its
		// "Gelesen" receipts update live (only on a real advance, never looping).
		s.broadcastReadAfterCommit(ctx, thread)
	}
	return detail, nil
}

// PostMessage appends a staff reply and returns the refreshed thread messages.
func (s *Service) PostMessage(ctx context.Context, threadID int64, body string, handledUpToMessageID int64) ([]*usersModels.ParentMessage, error) {
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
	visibleMessages, err := s.MessageRepo.ListByThread(ctx, thread.ID, 0)
	if err != nil {
		return nil, fmt.Errorf("messaging: list visible messages: %w", err)
	}
	boundaryFound := handledUpToMessageID <= 0
	for _, message := range visibleMessages {
		if message.ID == handledUpToMessageID {
			boundaryFound = true
		}
	}
	if !boundaryFound {
		return nil, ErrHandledBoundaryRequired
	}
	if handledUpToMessageID <= 0 {
		for _, message := range visibleMessages {
			if usersModels.IsCounterpartMessage(message, true) {
				return nil, ErrHandledBoundaryRequired
			}
		}
	}

	accountID := accountIDFromCtx(ctx)
	message, err := s.appendStaffMessage(ctx, thread, accountID, body)
	if err != nil {
		return nil, err
	}
	if err := s.notifyGuardianEmail(ctx, thread, message.ID); err != nil {
		return nil, err
	}

	messages, err := s.MessageRepo.ListByThread(ctx, thread.ID, 0)
	if err != nil {
		return nil, fmt.Errorf("messaging: list messages: %w", err)
	}
	if err := parentmessaging.MarkStaffHandledToVisible(ctx, s.ReadRepo, thread.TenantID, thread.ID, handledUpToMessageID, visibleMessages); err != nil {
		return nil, fmt.Errorf("messaging: mark handled: %w", err)
	}
	// Advance the staff reader's cursor over this returned snapshot, exactly as the
	// GET path (markReadAndBuild) does. The client applies this list with
	// revalidate:false, so a guardian message that became visible since the last GET
	// (an SSE echo missed/delayed before this send) would otherwise stay lit in the
	// sidebar/inbox unread count though it is already on screen. Bounded to the
	// newest GUARDIAN row in the snapshot (never NOW(), never our own just-sent
	// message), so it can't leap the cursor to ~now and swallow a guardian message
	// committing concurrently in a still-open tx. See parentmessaging.MarkReadToNewest.
	if _, err := parentmessaging.MarkReadToNewest(ctx, s.ReadRepo, thread.TenantID, thread.ID, accountID, true, messages); err != nil {
		return nil, fmt.Errorf("messaging: mark read: %w", err)
	}
	// Re-stamp the "Gelesen" receipts on the returned snapshot: the client applies
	// this list optimistically (revalidate:false), so without it the staff's older,
	// guardian-read messages would lose their receipt until the next GET/SSE refresh.
	// The just-sent message is unread by the guardian (cursor unchanged by our send),
	// so it correctly stays unstamped.
	parentmessaging.DecorateGuardianReadReceipts(ctx, s.ReadRepo, s.Logger, thread.ID, messages)
	s.broadcastAfterCommit(ctx, thread)
	s.notifyGuardianDevice(ctx, thread)
	return messages, nil
}

// authorizeThreadParticipants enforces the shared precondition for opening or
// starting a conversation: the staffer may read the child, messaging is enabled
// for the tenant, and the chosen recipient is an account-holding guardian of the
// child. StartThread and OpenThread both run it so their access rules cannot
// drift apart. Must run inside the tenant transaction.
func (s *Service) authorizeThreadParticipants(ctx context.Context, studentID, guardianAccountID int64) error {
	if err := s.canReadStudent(ctx, studentID); err != nil {
		return err
	}
	if err := s.requireEnabled(ctx); err != nil {
		return err
	}
	// The recipient must be an account-holding guardian of this child.
	guardians, err := s.ThreadRepo.ListGuardiansForStudent(ctx, studentID)
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
func (s *Service) StartThread(ctx context.Context, studentID, guardianAccountID int64, body string) (*ThreadDetail, error) {
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
	thread, err := s.ThreadRepo.GetOrCreate(ctx, tenant.FromContext(ctx), studentID, guardianAccountID)
	if err != nil {
		return nil, fmt.Errorf("messaging: get-or-create thread: %w", err)
	}
	message, err := s.appendStaffMessage(ctx, thread, accountID, body)
	if err != nil {
		return nil, err
	}
	if err := s.notifyGuardianEmail(ctx, thread, message.ID); err != nil {
		return nil, err
	}
	messages, err := s.MessageRepo.ListByThread(ctx, thread.ID, 0)
	if err != nil {
		return nil, fmt.Errorf("messaging: list messages: %w", err)
	}
	s.broadcastAfterCommit(ctx, thread)
	s.notifyGuardianDevice(ctx, thread)
	s.Logger.Info("staff sent parent message",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("guardian_account_id", guardianAccountID),
		slog.Int64("tenant_id", thread.TenantID),
	)
	// Mark the returned snapshot read, exactly as PostMessage/OpenThread do. When
	// this hits an already-existing conversation with unread guardian messages, the
	// detail we hand back has just shown them to the staffer; advancing the cursor
	// here keeps the inbox/sidebar unread count from staying lit after the SSE
	// refetch. Snapshot-bounded (never NOW()) via markReadAndBuild. The advance flag
	// is ignored: this is a SEND path, and broadcastAfterCommit above already wakes
	// the guardian with the new message (which refreshes receipts too).
	if _, err := parentmessaging.MarkReadToNewest(ctx, s.ReadRepo, thread.TenantID, thread.ID, accountID, true, messages); err != nil {
		return nil, fmt.Errorf("messaging: mark read: %w", err)
	}
	detail, err := s.buildDetailFromMessages(ctx, thread, messages)
	return detail, err
}

// OpenThread get-or-creates the (student, guardian) conversation and returns
// it ready for the chat window, marking it read — the "open the chat" entry
// point for the staff WhatsApp-style flow. No message is sent; a freshly
// created thread has no messages and is filtered out of the inbox until the
// first reply, so opening a chat never litters the inbox.
func (s *Service) OpenThread(ctx context.Context, studentID, guardianAccountID int64) (*ThreadDetail, error) {
	if err := s.authorizeThreadParticipants(ctx, studentID, guardianAccountID); err != nil {
		return nil, err
	}

	thread, err := s.ThreadRepo.GetOrCreate(ctx, tenant.FromContext(ctx), studentID, guardianAccountID)
	if err != nil {
		return nil, fmt.Errorf("messaging: get-or-create thread: %w", err)
	}
	detail, advanced, err := s.markReadAndBuild(ctx, thread)
	if err != nil {
		return nil, err
	}
	if advanced {
		// Opening an existing conversation with unread guardian messages reads them —
		// wake the guardian's open chat so its "Gelesen" receipts update live.
		s.broadcastReadAfterCommit(ctx, thread)
	}
	return detail, nil
}

func (s *Service) ListGuardians(ctx context.Context, studentID int64) ([]*usersModels.MessageableGuardian, error) {
	if err := s.canReadStudent(ctx, studentID); err != nil {
		return nil, err
	}
	guardians, err := s.ThreadRepo.ListGuardiansForStudent(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("messaging: list guardians: %w", err)
	}
	return guardians, nil
}

// ListStudentThreads returns the staff view of one child's conversations
// (newest activity first), for the student-detail card. Authorizes read
// access to the child, then filters server-side instead of having the card
// fetch the whole tenant inbox.
func (s *Service) ListStudentThreads(ctx context.Context, studentID int64) ([]*usersModels.InboxThread, error) {
	if err := s.canReadStudent(ctx, studentID); err != nil {
		return nil, err
	}
	rows, err := s.ReadRepo.ListThreadsForStudent(ctx, accountIDFromCtx(ctx), studentID)
	if err != nil {
		return nil, fmt.Errorf("messaging: list student threads: %w", err)
	}
	s.suppressDisabledUnread(ctx, rows)
	return rows, nil
}

// appendStaffMessage writes a staff message into the thread, stamps the
// sender's name, and updates the thread's last-activity fields. The caller has
// already authorized and validated the body. The append invariant (and why it
// does NOT move the sender's read cursor) is shared with the parent side via
// parentmessaging.AppendMessage (one home for the "drive off the DB-stamped
// created_at" rule).
func (s *Service) appendStaffMessage(ctx context.Context, thread *usersModels.ParentMessageThread, accountID int64, body string) (*usersModels.ParentMessage, error) {
	msg := &usersModels.ParentMessage{
		ThreadID:         thread.ID,
		StudentID:        thread.StudentID,
		SenderAccountID:  accountID,
		SenderKind:       usersModels.ParentMessageSenderStaff,
		SenderName:       s.resolveStaffName(ctx, accountID),
		StaffNameVisible: s.staffNameVisibleToParents(ctx),
		Body:             body,
		Kind:             usersModels.ParentMessageKindMessage,
	}
	msg.SetTenantID(thread.TenantID)
	if err := parentmessaging.AppendMessage(ctx, s.MessageRepo, s.ThreadRepo, msg); err != nil {
		return nil, fmt.Errorf("messaging: append staff message: %w", err)
	}
	return msg, nil
}

// requireEnabled blocks writes (reply, new thread, open) when the school has
// turned messaging off. It fails OPEN on a transient resolve error (via
// parentmessaging.MessagingEnabled): a config-DB blip must not 500 every reply
// while the badge and inbox keep rendering unread — the write side now agrees
// with the read side instead of diverging. A genuine disabled flag still returns
// ErrMessagingDisabled (-> 403).
func (s *Service) requireEnabled(ctx context.Context) error {
	if !parentmessaging.MessagingEnabled(ctx, s.Settings, s.Logger) {
		return ErrMessagingDisabled
	}
	return nil
}

// resolveStaffName returns the staff member's display name, "OGS-Team" if none.
func (s *Service) resolveStaffName(ctx context.Context, accountID int64) string {
	name := "OGS-Team"
	if s.Persons == nil {
		return name
	}
	person, err := s.Persons.FindByAccountID(ctx, accountID)
	if err != nil || person == nil {
		return name
	}
	if full := strings.TrimSpace(person.FirstName + " " + person.LastName); full != "" {
		name = full
	}
	return name
}

// staffNameVisibleToParents resolves whether team replies should attribute the
// individual staff member to guardians, to be FROZEN onto the message at send
// time (users.parent_messages.staff_name_visible). It is read only here, on the
// write path: the parent-facing read path trusts the stamped column, so a later
// toggle never rewrites history. Unlike MessagingEnabled it fails CLOSED (false,
// anonymous) on a transient config-DB error: the flag exposes staff personal
// data and is frozen per message, so a blip must not permanently reveal a name
// for a school that explicitly opted out. Anonymizing one message is
// recoverable and privacy-safe; a wrongful disclosure is neither.
func (s *Service) staffNameVisibleToParents(ctx context.Context) bool {
	if s.Settings == nil {
		return false
	}
	visible, err := s.Settings.ResolveBool(ctx, configModels.KeyParentMessageStaffNameVisible)
	if err != nil {
		s.Logger.Warn("messaging: resolve staff-name visibility failed, defaulting to anonymous",
			slog.String("error", err.Error()),
		)
		return false
	}
	return visible
}

// broadcastAfterCommit queues the SSE wake-up to fire only AFTER the request
// transaction commits, mirroring the parent side (parent_messaging_service.go).
// TenantTxMiddleware commits after the handler returns, so broadcasting inline
// would let a woken client refetch the pre-commit snapshot (stale read), and a
// 5xx rollback would fire a phantom "new message" for a row that never
// persisted. Scalars are captured now because the thread pointer may be mutated
// after this call returns.
func (s *Service) broadcastAfterCommit(ctx context.Context, thread *usersModels.ParentMessageThread) {
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

func (s *Service) broadcastValues(tenantID, guardianAccountID, threadID, studentID int64) {
	parentmessaging.Broadcast(s.Broadcaster, s.Logger, tenantID, guardianAccountID, threadID, studentID)
}

// notifyGuardianDevice pushes a staff reply to the guardian's registered devices
// (#1671). Unlike the SSE wake-up next to it this is NOT wrapped in an
// after-commit hook: Notify defers its own fan-out until the surrounding tenant
// transaction commits, and the consent read it performs has to happen inside
// that transaction to be RLS-scoped at all.
//
// The audience carries the child (Audience.StudentID) so the devices are looked
// up under parent_portal.access for THAT child. The send path checks this here,
// in the request transaction, and the delivery transaction checks it again from
// the row it sees: a school can revoke a guardian's access to a child in the
// moments between the two, and the second answer is the one the push is sent on.
//
// The copy names neither the child nor the sender. A push payload leaves the
// backend and is rendered on a lock screen; the thread behind the deep link is
// authenticated, and that is where the details belong.
func (s *Service) notifyGuardianDevice(ctx context.Context, thread *usersModels.ParentMessageThread) {
	if thread == nil || s.Notifier == nil || s.Preferences == nil {
		return
	}
	if thread.GuardianAccountID <= 0 {
		return
	}

	optedIn, err := s.Preferences.FilterOptedIn(ctx, notifications.TypeParentMessage, []int64{thread.GuardianAccountID})
	if err != nil {
		s.Logger.Warn("messaging: filter opted-in guardian failed",
			slog.Int64("thread_id", thread.ID),
			slog.String("error", err.Error()),
		)
		return
	}
	if len(optedIn) == 0 {
		return
	}
	locale := ""
	if s.GuardianProfiles != nil {
		profile, profileErr := s.GuardianProfiles.FindByAccountID(ctx, thread.GuardianAccountID)
		if profileErr != nil {
			s.Logger.Warn("messaging: load guardian locale for push failed",
				slog.Int64("thread_id", thread.ID),
				slog.String("error", profileErr.Error()),
			)
		} else if profile != nil && profile.PortalLocale != nil {
			locale = *profile.PortalLocale
		}
	}
	title, notificationBody := notifications.ParentMessageCopy(locale)

	err = s.Notifier.Notify(ctx, notifications.Event{
		Type:     notifications.TypeParentMessage,
		Title:    title,
		Body:     notificationBody,
		DeepLink: "/messages",
		Priority: notifications.PriorityNormal,
		Audience: notifications.Audience{
			TenantID:           thread.TenantID,
			Scope:              notifications.ScopeGuardian,
			GuardianAccountIDs: optedIn,
			StudentIDs:         []int64{thread.StudentID},
		},
	})
	switch {
	case errors.Is(err, notifications.ErrDisabled), errors.Is(err, notifications.ErrOutsideActiveWindow):
		s.Logger.Info("messaging: guardian push suppressed by tenant notification gate",
			slog.Int64("thread_id", thread.ID),
			slog.String("reason", err.Error()),
		)
	case err != nil:
		s.Logger.Warn("messaging: guardian push failed",
			slog.Int64("thread_id", thread.ID),
			slog.String("error", err.Error()),
		)
	}
}

// broadcastReadAfterCommit queues the read-receipt SSE wake-up to fire only AFTER
// the request transaction commits, mirroring broadcastAfterCommit. The guardian's
// open chat refreshes its "Gelesen" receipts; staff tabs get a (sanitized)
// receipt-refresh nudge. Callers fire it only when the staff cursor actually
// advanced, so it never loops with the refetch it triggers.
func (s *Service) broadcastReadAfterCommit(ctx context.Context, thread *usersModels.ParentMessageThread) {
	if thread == nil {
		return
	}
	tenantID := thread.TenantID
	guardianAccountID := thread.GuardianAccountID
	threadID := thread.ID
	studentID := thread.StudentID
	tenant.RegisterAfterCommit(ctx, func() {
		parentmessaging.BroadcastRead(s.Broadcaster, s.Logger, tenantID, guardianAccountID, threadID, studentID)
	})
}

func containsGuardian(guardians []*usersModels.MessageableGuardian, accountID int64) bool {
	for _, g := range guardians {
		if g != nil && g.AccountID == accountID {
			return true
		}
	}
	return false
}
