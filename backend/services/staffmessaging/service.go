// Package staffmessaging implements the OGS-internal colleague chat (#2598):
// a 1:1 conversation between two staff accounts of the same school.
//
// It deliberately does NOT reuse services/messaging. That service carries the
// parent-OGS rules — which child a staff member may see, whether the guardian
// is still linked, whether a staff name may be shown to parents — none of which
// exist here. The two share the schema SHAPE (thread / message / read cursor)
// and the transport (SSE + Web Push), not the authorization model.
//
// Authorization here is membership and nothing else: you can read and write a
// conversation exactly when you are one of its participants. No role, no
// permission and no admin flag substitutes for that.
package staffmessaging

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// maxMessageLen caps one message in runes. Mirrors the parent-OGS chat limit so
// the shared frontend composer can enforce one number for both surfaces.
const maxMessageLen = 2000

var (
	// ErrMessagingDisabled is returned when the school has the internal
	// messenger switched off.
	ErrMessagingDisabled = errors.New("staffmessaging: internal messaging disabled for this school")
	// ErrNotParticipant is returned when the caller is not part of the thread.
	// Deliberately indistinguishable from "thread does not exist" at the HTTP
	// boundary, so a probe cannot enumerate other people's conversations.
	ErrNotParticipant = errors.New("staffmessaging: not a participant of this thread")
	// ErrThreadNotFound is returned when no thread has that id in the tenant.
	ErrThreadNotFound = errors.New("staffmessaging: thread not found")
	// ErrRecipientNotAvailable is returned when the addressed account may not be
	// written to: it left the school, or it is not a staff account at all (a
	// guardian account carries an active tenant mapping too).
	ErrRecipientNotAvailable = errors.New("staffmessaging: recipient is not an active member of this school")
	// ErrCounterpartUnavailable is returned when the OTHER side of an existing
	// conversation is no longer a messageable colleague. The thread stays
	// readable; only appending to it stops.
	ErrCounterpartUnavailable = errors.New("staffmessaging: the other participant is no longer available")
	// ErrSelfConversation is returned when someone addresses themselves.
	ErrSelfConversation = errors.New("staffmessaging: cannot start a conversation with yourself")
	// ErrEmptyMessage is returned for a blank body.
	ErrEmptyMessage = errors.New("staffmessaging: message body must not be empty")
	// ErrMessageTooLong is returned for a body over maxMessageLen runes.
	ErrMessageTooLong = errors.New("staffmessaging: message body is too long")
	// ErrNoActor is returned when the request carries no account id.
	ErrNoActor = errors.New("staffmessaging: no authenticated account in context")
)

// ThreadDetail is the chat-window payload: who the conversation is with, plus
// the messages oldest-first.
type ThreadDetail struct {
	ThreadID             int64
	CounterpartAccountID int64
	CounterpartName      string
	// CounterpartRoleKind: one of the usersModels.StaffRoleKind constants.
	CounterpartRoleKind string
	Messages            []*usersModels.StaffMessage
}

// Service is the OGS-internal messaging service.
type Service struct {
	Config
}

// Config is the dependency-injection bundle.
type Config struct {
	ThreadRepo  usersModels.StaffMessageThreadRepository
	MessageRepo usersModels.StaffMessageRepository
	ReadRepo    usersModels.StaffMessageReadRepository
	Persons     userService.PersonService
	Settings    configService.SettingsService
	Broadcaster realtime.Broadcaster
	DB          *bun.DB
	Logger      *slog.Logger

	// Notifier and Preferences push a message to the recipient's devices. Both
	// optional and both required together: without the preference service there
	// is nobody who consented, so the push is skipped rather than sent past
	// consent.
	Notifier    notifications.Service
	Preferences notifications.PreferenceService
}

// NewService wires an internal messaging service.
func NewService(cfg Config) *Service {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Service{Config: cfg}
}

// actor returns the authenticated account id for the request.
func (s *Service) actor(ctx context.Context) (int64, error) {
	if id := jwt.ActorAccountIDFromCtx(ctx); id != nil && *id != 0 {
		return *id, nil
	}
	return 0, ErrNoActor
}

// requireEnabled gates every entry point on the school's feature switch.
//
// Unlike the parent-OGS messenger this fails CLOSED on a settings error. There
// the inbox must never half-disable for a school that uses it daily, so a blip
// counts as enabled. Here the opposite is true: a school that has not switched
// the internal messenger on must not start exchanging staff messages because a
// settings read hiccuped. A blocked send is recoverable; an unwanted internal
// chat channel at a school that declined it is not.
func (s *Service) requireEnabled(ctx context.Context) error {
	if s.Settings == nil {
		return ErrMessagingDisabled
	}
	enabled, err := s.Settings.ResolveBool(ctx, configModels.KeyStaffMessagingEnabled)
	if err != nil {
		s.Logger.Warn("staffmessaging: resolve enabled failed, failing closed",
			slog.String("key", configModels.KeyStaffMessagingEnabled),
			slog.String("error", err.Error()),
		)
		return ErrMessagingDisabled
	}
	if !enabled {
		return ErrMessagingDisabled
	}
	return nil
}

// ListInbox returns the caller's conversations, newest activity first.
func (s *Service) ListInbox(ctx context.Context, onlyUnread bool) ([]*usersModels.StaffInboxThread, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}
	accountID, err := s.actor(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.ReadRepo.ListInbox(ctx, accountID, onlyUnread)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.CounterpartAccountID)
	}
	kinds, err := s.ReadRepo.StaffRoleKinds(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		row.CounterpartRoleKind = kinds[row.CounterpartAccountID]
	}
	return rows, nil
}

// UnreadMessageCount is the caller's sidebar badge.
func (s *Service) UnreadMessageCount(ctx context.Context) (int, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return 0, err
	}
	accountID, err := s.actor(ctx)
	if err != nil {
		return 0, err
	}
	return s.ReadRepo.UnreadCount(ctx, accountID)
}

// ListMessageableStaff returns the colleagues the caller may write to.
func (s *Service) ListMessageableStaff(ctx context.Context) ([]*usersModels.MessageableStaff, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}
	accountID, err := s.actor(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.ReadRepo.ListMessageableStaff(ctx, accountID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.AccountID)
	}
	kinds, err := s.ReadRepo.StaffRoleKinds(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		row.RoleKind = kinds[row.AccountID]
	}
	return rows, nil
}

// authorizeThread loads a thread and verifies the caller takes part in it.
// Both "no such thread" and "not yours" surface as the same 404 upstream.
func (s *Service) authorizeThread(ctx context.Context, threadID, accountID int64) (*usersModels.StaffMessageThread, error) {
	thread, err := s.ThreadRepo.FindByID(ctx, threadID)
	if err != nil {
		return nil, err
	}
	if thread == nil {
		return nil, ErrThreadNotFound
	}
	member, err := s.ThreadRepo.IsParticipant(ctx, threadID, accountID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrNotParticipant
	}
	return thread, nil
}

// OpenThread returns the conversation with the given colleague, creating it
// when it does not exist yet. This is the "write to someone" entry point; the
// thread carries no message until the first send.
func (s *Service) OpenThread(ctx context.Context, counterpartAccountID int64) (*ThreadDetail, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}
	accountID, err := s.actor(ctx)
	if err != nil {
		return nil, err
	}
	if counterpartAccountID == accountID {
		return nil, ErrSelfConversation
	}

	// Both sides must be active members of THIS school. Checking the caller too
	// is not redundant: a JWT stays valid for its lifetime, so a staff member
	// deactivated mid-session still carries a usable token.
	if err := s.requireActiveMember(ctx, accountID); err != nil {
		return nil, err
	}
	if err := s.requireActiveMember(ctx, counterpartAccountID); err != nil {
		return nil, err
	}

	thread, err := s.ThreadRepo.GetOrCreateDirect(ctx, accountID, counterpartAccountID)
	if err != nil {
		return nil, err
	}
	return s.buildDetail(ctx, thread, accountID)
}

// requireReachableCounterparts verifies every participant except the caller is
// still a messageable colleague.
func (s *Service) requireReachableCounterparts(ctx context.Context, threadID, callerAccountID int64) error {
	participants, err := s.ThreadRepo.ParticipantAccountIDs(ctx, threadID)
	if err != nil {
		return err
	}
	for _, id := range participants {
		if id == callerAccountID {
			continue
		}
		ok, err := s.ReadRepo.IsMessageableStaff(ctx, id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrCounterpartUnavailable
		}
	}
	return nil
}

func (s *Service) requireActiveMember(ctx context.Context, accountID int64) error {
	active, err := s.ReadRepo.IsMessageableStaff(ctx, accountID)
	if err != nil {
		return err
	}
	if !active {
		return ErrRecipientNotAvailable
	}
	return nil
}

// GetThread returns the conversation and marks it read for the caller up to the
// newest message it returns.
func (s *Service) GetThread(ctx context.Context, threadID int64) (*ThreadDetail, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}
	accountID, err := s.actor(ctx)
	if err != nil {
		return nil, err
	}
	thread, err := s.authorizeThread(ctx, threadID, accountID)
	if err != nil {
		return nil, err
	}

	detail, err := s.buildDetail(ctx, thread, accountID)
	if err != nil {
		return nil, err
	}

	// Advance the read cursor to the newest message actually returned — not to
	// "now". Anything that commits between this read and the cursor write stays
	// unread, which is the safe direction: a message shown as unread once more
	// is harmless, a silently swallowed one is not.
	if n := len(detail.Messages); n > 0 {
		newest := detail.Messages[n-1]
		if err := s.ReadRepo.MarkReadUpTo(ctx, threadID, accountID, newest.CreatedAt, newest.ID); err != nil {
			return nil, err
		}
	}
	return detail, nil
}

// PostMessage appends a message to a conversation the caller takes part in.
func (s *Service) PostMessage(ctx context.Context, threadID int64, body string) (*usersModels.StaffMessage, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}
	accountID, err := s.actor(ctx)
	if err != nil {
		return nil, err
	}
	trimmed, err := validateBody(body)
	if err != nil {
		return nil, err
	}
	// Still a colleague? A JWT stays valid for its lifetime, so someone
	// deactivated (or offboarded) mid-session keeps a usable token for up to
	// AUTH_JWT_EXPIRY and could go on writing into existing conversations.
	// OpenThread already checks this; the send path is the other write door and
	// needs the same lock.
	//
	// Reads deliberately stay open: seeing the history one already had is
	// harmless, adding to it after leaving the school is not.
	if err := s.requireActiveMember(ctx, accountID); err != nil {
		return nil, err
	}

	thread, err := s.authorizeThread(ctx, threadID, accountID)
	if err != nil {
		return nil, err
	}

	// Is the OTHER side still reachable? Membership in the thread is a
	// historical fact and says nothing about today. Without this check a
	// conversation whose counterpart left the school keeps accepting messages
	// that nobody will ever answer - and worse: because reads stay open, that
	// person's still-valid token would keep DELIVERING them. Leaving history
	// readable was meant for the past, not for ongoing traffic.
	if err := s.requireReachableCounterparts(ctx, threadID, accountID); err != nil {
		return nil, err
	}

	// Serialize appends for this thread so message order matches commit order.
	if err := s.ThreadRepo.LockForMessageAppend(ctx, threadID); err != nil {
		return nil, err
	}

	message := &usersModels.StaffMessage{
		ThreadID:        threadID,
		SenderAccountID: accountID,
		SenderName:      s.resolveSenderName(ctx, accountID),
		Body:            trimmed,
	}
	if err := s.MessageRepo.Create(ctx, message); err != nil {
		return nil, err
	}
	if err := s.ThreadRepo.TouchLastMessage(ctx, threadID, message.CreatedAt, message.ID, accountID, trimmed); err != nil {
		return nil, err
	}

	// Deliberately NO read-cursor advance here.
	//
	// The tempting version ("you have read what you just wrote") is both
	// unnecessary and wrong. Unnecessary: the unread predicate already excludes
	// the reader's own messages (sender_account_id <> reader), so a just-sent
	// message can never count against its author. Wrong: the cursor is a
	// thread-wide watermark, so jumping it to the new message drags it past
	// everything the counterpart sent while this one was being typed - silently
	// clearing their unread signal for messages the sender never saw.
	// TestSendingDoesNotSwallowIncomingUnread pins this.

	s.notifyAfterCommit(ctx, thread, message, accountID)
	return message, nil
}

// validateBody trims and range-checks a message body.
func validateBody(body string) (string, error) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return "", ErrEmptyMessage
	}
	if len([]rune(trimmed)) > maxMessageLen {
		return "", ErrMessageTooLong
	}
	return trimmed, nil
}

// resolveSenderName freezes the sender's display name onto the message, so the
// history stays readable after a rename or deactivation.
func (s *Service) resolveSenderName(ctx context.Context, accountID int64) string {
	const fallback = "Unbekannt"
	if s.Persons == nil {
		return fallback
	}
	person, err := s.Persons.FindByAccountID(ctx, accountID)
	if err != nil || person == nil {
		return fallback
	}
	if full := strings.TrimSpace(person.FirstName + " " + person.LastName); full != "" {
		return full
	}
	return fallback
}

// buildDetail assembles the chat window payload for one viewer.
func (s *Service) buildDetail(ctx context.Context, thread *usersModels.StaffMessageThread, viewerAccountID int64) (*ThreadDetail, error) {
	messages, err := s.MessageRepo.ListByThread(ctx, thread.ID)
	if err != nil {
		return nil, err
	}
	participants, err := s.ThreadRepo.ParticipantAccountIDs(ctx, thread.ID)
	if err != nil {
		return nil, err
	}

	detail := &ThreadDetail{ThreadID: thread.ID, Messages: messages}
	for _, id := range participants {
		if id != viewerAccountID {
			detail.CounterpartAccountID = id
			detail.CounterpartName = s.resolveSenderName(ctx, id)
			break
		}
	}
	if detail.CounterpartAccountID != 0 {
		kinds, err := s.ReadRepo.StaffRoleKinds(ctx, []int64{detail.CounterpartAccountID})
		if err != nil {
			return nil, err
		}
		detail.CounterpartRoleKind = kinds[detail.CounterpartAccountID]
	}
	return detail, nil
}
