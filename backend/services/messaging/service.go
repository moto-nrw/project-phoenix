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
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const (
	maxMessageLen = 2000
	maxSubjectLen = 150
)

var (
	// ErrForbidden means the staff member may not read/write the thread.
	ErrForbidden = errors.New("messaging: forbidden")
	// ErrThreadNotFound means the thread id does not exist in the tenant.
	ErrThreadNotFound = errors.New("messaging: thread not found")
	// ErrEmptyBody means the message body was blank after trimming.
	ErrEmptyBody = errors.New("messaging: message body must not be empty")
	// ErrBodyTooLong means the message exceeded maxMessageLen.
	ErrBodyTooLong = errors.New("messaging: message body too long")
	// ErrEmptySubject means a new thread had a blank subject.
	ErrEmptySubject = errors.New("messaging: subject must not be empty")
	// ErrInvalidGuardian means the chosen recipient is not an account-holding
	// guardian of the child.
	ErrInvalidGuardian = errors.New("messaging: recipient is not a guardian of this child")
	// ErrMessagingDisabled means the feature flag is off for the tenant.
	ErrMessagingDisabled = errors.New("messaging: messaging disabled for this school")
)

// ThreadDetail is the chat-window payload: thread header (subject, child,
// guardian + relationship) plus the messages, oldest-first.
type ThreadDetail struct {
	ThreadID          int64
	Subject           string
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
	StartThread(ctx context.Context, studentID, guardianAccountID int64, subject, body string) (*ThreadDetail, error)
	ListGuardians(ctx context.Context, studentID int64) ([]*usersModels.MessageableGuardian, error)
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
	return rows, nil
}

func (s *service) UnreadThreadCount(ctx context.Context) (int, error) {
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
	if err != nil || student == nil {
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

func (s *service) buildThreadDetail(ctx context.Context, thread *usersModels.ParentMessageThread, accountID int64) (*ThreadDetail, error) {
	messages, err := s.messageRepo.ListByThread(ctx, thread.ID, 0)
	if err != nil {
		return nil, fmt.Errorf("messaging: list messages: %w", err)
	}
	detail := &ThreadDetail{
		ThreadID:          thread.ID,
		Subject:           thread.Subject,
		StudentID:         thread.StudentID,
		GuardianAccountID: thread.GuardianAccountID,
		Messages:          messages,
	}
	if proj, err := s.readRepo.FindThreadProjection(ctx, thread.ID, accountID); err == nil && proj != nil {
		detail.StudentName = proj.StudentName
		detail.GuardianName = proj.GuardianName
		detail.RelationshipType = proj.RelationshipType
	}
	return detail, nil
}

func (s *service) GetThread(ctx context.Context, threadID int64) (*ThreadDetail, error) {
	thread, err := s.loadAuthorizedThread(ctx, threadID)
	if err != nil {
		return nil, err
	}
	accountID := accountIDFromCtx(ctx)
	if err := s.readRepo.MarkRead(ctx, thread.TenantID, thread.ID, accountID); err != nil {
		return nil, fmt.Errorf("messaging: mark read: %w", err)
	}
	return s.buildThreadDetail(ctx, thread, accountID)
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

	accountID := accountIDFromCtx(ctx)
	if err := s.appendStaffMessage(ctx, thread, accountID, body); err != nil {
		return nil, err
	}

	messages, err := s.messageRepo.ListByThread(ctx, thread.ID, 0)
	if err != nil {
		return nil, fmt.Errorf("messaging: list messages: %w", err)
	}
	s.broadcast(thread.TenantID, thread.GuardianAccountID)
	return messages, nil
}

func (s *service) StartThread(ctx context.Context, studentID, guardianAccountID int64, subject, body string) (*ThreadDetail, error) {
	subject = strings.TrimSpace(subject)
	body = strings.TrimSpace(body)
	if subject == "" {
		return nil, ErrEmptySubject
	}
	if utf8.RuneCountInString(subject) > maxSubjectLen {
		subject = string([]rune(subject)[:maxSubjectLen])
	}
	if body == "" {
		return nil, ErrEmptyBody
	}
	if utf8.RuneCountInString(body) > maxMessageLen {
		return nil, ErrBodyTooLong
	}
	if err := s.canReadStudent(ctx, studentID); err != nil {
		return nil, err
	}
	if err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}

	// The recipient must be an account-holding guardian of this child.
	guardians, err := s.threadRepo.ListGuardiansForStudent(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("messaging: list guardians: %w", err)
	}
	if !containsGuardian(guardians, guardianAccountID) {
		return nil, ErrInvalidGuardian
	}

	accountID := accountIDFromCtx(ctx)
	tenantID := tenant.FromContext(ctx)
	thread := &usersModels.ParentMessageThread{
		StudentID:         studentID,
		GuardianAccountID: guardianAccountID,
		Subject:           subject,
	}
	thread.SetTenantID(tenantID)
	if err := s.threadRepo.Create(ctx, thread); err != nil {
		return nil, fmt.Errorf("messaging: create thread: %w", err)
	}
	if err := s.appendStaffMessage(ctx, thread, accountID, body); err != nil {
		return nil, err
	}
	s.broadcast(thread.TenantID, thread.GuardianAccountID)
	s.logger.Info("staff started parent thread",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("guardian_account_id", guardianAccountID),
		slog.Int64("tenant_id", thread.TenantID),
	)
	return s.buildThreadDetail(ctx, thread, accountID)
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

// appendStaffMessage writes a staff message into the thread, stamps the
// sender's name, and updates the thread's last-activity fields. The caller has
// already authorized and validated the body.
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
	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return fmt.Errorf("messaging: create message: %w", err)
	}
	now := time.Now()
	kind := usersModels.ParentMessageSenderStaff
	thread.LastMessageAt = &now
	thread.LastSenderKind = &kind
	if err := s.threadRepo.Update(ctx, thread); err != nil {
		return fmt.Errorf("messaging: update thread: %w", err)
	}
	if err := s.readRepo.MarkRead(ctx, thread.TenantID, thread.ID, accountID); err != nil {
		return fmt.Errorf("messaging: mark read: %w", err)
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

func (s *service) broadcast(tenantID, guardianAccountID int64) {
	if s.broadcaster == nil || tenantID <= 0 {
		return
	}
	gid := strconv.FormatInt(guardianAccountID, 10)
	event := realtime.NewEvent(realtime.EventParentMessage, "", realtime.EventData{Source: &gid})
	if err := s.broadcaster.BroadcastParentMessage(tenantID, guardianAccountID, event); err != nil {
		s.logger.Warn("messaging: failed to broadcast parent message",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("guardian_account_id", guardianAccountID),
			slog.String("error", err.Error()),
		)
	}
}

func containsGuardian(guardians []*usersModels.MessageableGuardian, accountID int64) bool {
	for _, g := range guardians {
		if g != nil && g.AccountID == accountID {
			return true
		}
	}
	return false
}
