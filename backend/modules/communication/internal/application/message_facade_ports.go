package application

import (
	"context"
)

type messageObservation func(context.Context, string) (context.Context, func(error))
type ParentMessageService[I, T, M, G any] interface {
	ListInbox(context.Context, bool) ([]*I, error)
	UnreadMessageCount(context.Context) (int, error)
	GetThread(context.Context, int64) (*T, error)
	ListGuardians(context.Context, int64) ([]*G, error)
	ListReachableStudentIDs(context.Context) ([]int64, error)
	ListStudentThreads(context.Context, int64) ([]*I, error)
	StartThread(context.Context, int64, int64, string) (*T, error)
	OpenThread(context.Context, int64, int64) (*T, error)
	PostMessage(context.Context, int64, string, int64) ([]*M, error)
	MarkUnread(context.Context, int64) error
	MarkAllRead(context.Context) (int, error)
	CountScope(context.Context) (string, error)
	HasOwnGroups(context.Context) (bool, error)
	SetCountScope(context.Context, string) error
}
type ParentMessageRows[I, T, M, G any] struct {
	Inbox    func(*I) *InboxThread
	Thread   func(*T) (InboxThread, []*M)
	Message  func(*M) *ParentMessage
	Guardian func(*G) *MessageableGuardian
}
type StaffMessageService[I, T, M, G any, R retentionCounts] interface {
	ListInbox(context.Context, bool) ([]*I, error)
	UnreadMessageCount(context.Context) (int, error)
	ListMessageableStaff(context.Context) ([]*G, error)
	OpenThread(context.Context, int64) (*T, error)
	GetThread(context.Context, int64) (*T, error)
	PostMessage(context.Context, int64, string) (*M, error)
	CleanupExpiredMessages(context.Context) (R, error)
}
type StaffMessageRows[I, T, M, G any, R retentionCounts] struct {
	Inbox     func(*I) *StaffInboxThread
	Thread    func(*T) (StaffInboxThread, []*M)
	Message   func(*M) *StaffMessage
	Colleague func(*G) *MessageableStaff
}
type ParentMessageErrors struct {
	ErrForbidden               error
	ErrThreadNotFound          error
	ErrEmptyBody               error
	ErrBodyTooLong             error
	ErrHandledBoundaryRequired error
	ErrInvalidGuardian         error
	ErrGuardianAccessRevoked   error
	ErrMessagingDisabled       error
	ErrInvalidCountScope       error
}
type StaffMessageErrors struct {
	ErrMessagingDisabled      error
	ErrNotParticipant         error
	ErrThreadNotFound         error
	ErrRecipientNotAvailable  error
	ErrCounterpartUnavailable error
	ErrSelfConversation       error
	ErrEmptyMessage           error
	ErrMessageTooLong         error
	ErrNoActor                error
	ErrRetentionUnresolved    error
}
