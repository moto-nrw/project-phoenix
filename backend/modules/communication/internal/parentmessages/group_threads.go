package messaging

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

const (
	maxGroupThreadTitleLen = 120
	maxGroupTargetLabelLen = 120
	maxGroupClassNameLen   = 50
)

// GroupThreadStore is the persistence of the group threads (#4784).
type GroupThreadStore interface {
	CreateThread(ctx context.Context, target deliveryPorts.ParentGroupTarget, title string, createdBy *int64) (*deliveryPorts.ParentGroupThread, error)
	FindThread(ctx context.Context, id int64, lock bool) (*deliveryPorts.ParentGroupThread, error)
	AppendMessage(ctx context.Context, threadID int64, sender *int64, senderName, body string, recipients []deliveryPorts.ParentGroupRecipient) (*deliveryPorts.ParentGroupMessage, error)
	CloseThread(ctx context.Context, id, accountID int64) (bool, error)
	RecipientAccountIDs(ctx context.Context, threadID int64) ([]int64, error)
	ListMessages(ctx context.Context, threadID int64) ([]deliveryPorts.ParentGroupMessage, error)
	ListThreads(ctx context.Context) ([]deliveryPorts.ParentGroupThreadSummary, error)
	TitlesByIDs(ctx context.Context, ids []int64) (map[int64]string, error)
	ReadStatus(ctx context.Context, threadID int64) ([]deliveryPorts.ParentGroupChildRead, error)
}

// The group threads are an OGS-wide channel: the caller must read every
// child of the school, as for the reachable-children list (ADR 0079).
func (s *Service) requireGroupAuthor(ctx context.Context) error {
	if s.GroupThreads == nil || s.GroupAudience == nil {
		return fmt.Errorf("messaging: group threads are not configured")
	}
	if !s.scope(ctx) {
		return deliveryPorts.ErrParentGroupMessagingForbidden
	}
	return nil
}

// ListParentGroupThreads returns the school's group threads, newest first.
func (s *Service) ListParentGroupThreads(ctx context.Context) ([]deliveryPorts.ParentGroupThreadSummary, error) {
	if err := s.requireGroupAuthor(ctx); err != nil {
		return nil, err
	}
	rows, err := s.GroupThreads.ListThreads(ctx)
	if err != nil {
		return nil, fmt.Errorf("messaging: list group threads: %w", err)
	}
	return rows, nil
}

// PreviewParentGroupAudience lists the children a target reaches today and
// marks those without a reachable parent.
func (s *Service) PreviewParentGroupAudience(ctx context.Context, target deliveryPorts.ParentGroupTarget) ([]deliveryPorts.ParentGroupChild, error) {
	if err := s.requireGroupAuthor(ctx); err != nil {
		return nil, err
	}
	target, err := normalizeGroupTarget(target, false)
	if err != nil {
		return nil, err
	}
	children, err := s.GroupAudience.GroupAudience(ctx, s.runtime.TenantID(ctx), target)
	if err != nil {
		return nil, fmt.Errorf("messaging: group audience: %w", err)
	}
	out := make([]deliveryPorts.ParentGroupChild, 0, len(children))
	for _, child := range children {
		out = append(out, deliveryPorts.ParentGroupChild{
			StudentID: child.StudentID, FirstName: child.FirstName, LastName: child.LastName,
			SchoolClass: child.SchoolClass, Reachable: len(child.GuardianAccountIDs) > 0,
		})
	}
	return out, nil
}

// CreateParentGroupThread opens a group thread with its first message.
func (s *Service) CreateParentGroupThread(ctx context.Context, input deliveryPorts.ParentGroupThreadInput) (*deliveryPorts.ParentGroupThreadDetail, error) {
	if err := s.requireGroupAuthor(ctx); err != nil {
		return nil, err
	}
	target, err := normalizeGroupTarget(input.Target, true)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(input.Title)
	if title == "" || utf8.RuneCountInString(title) > maxGroupThreadTitleLen {
		return nil, deliveryPorts.ErrParentGroupThreadInvalid
	}
	body, err := normalizeGroupBody(input.Body)
	if err != nil {
		return nil, err
	}
	if err := s.requireEnabled(ctx); err != nil {
		return nil, deliveryPorts.ErrParentGroupMessagingDisabled
	}
	accountID := s.accountID(ctx)
	thread, err := s.GroupThreads.CreateThread(ctx, target, title, optionalAccount(accountID))
	if err != nil {
		return nil, fmt.Errorf("messaging: create group thread: %w", err)
	}
	if err := s.sendGroupMessage(ctx, thread, accountID, body); err != nil {
		return nil, err
	}
	return s.groupThreadDetail(ctx, thread)
}

// GetParentGroupThread returns a group thread with its messages and the
// families' read state.
func (s *Service) GetParentGroupThread(ctx context.Context, threadID int64) (*deliveryPorts.ParentGroupThreadDetail, error) {
	thread, err := s.loadGroupThread(ctx, threadID, false)
	if err != nil {
		return nil, err
	}
	return s.groupThreadDetail(ctx, thread)
}

// PostParentGroupMessage writes a further message into an open group thread.
// Its recipients are resolved anew: the families of the target today.
func (s *Service) PostParentGroupMessage(ctx context.Context, threadID int64, body string) (*deliveryPorts.ParentGroupThreadDetail, error) {
	body, err := normalizeGroupBody(body)
	if err != nil {
		return nil, err
	}
	thread, err := s.loadGroupThread(ctx, threadID, true)
	if err != nil {
		return nil, err
	}
	if thread.ClosedAt != nil {
		return nil, deliveryPorts.ErrParentGroupThreadClosed
	}
	if err := s.requireEnabled(ctx); err != nil {
		return nil, deliveryPorts.ErrParentGroupMessagingDisabled
	}
	if err := s.sendGroupMessage(ctx, thread, s.accountID(ctx), body); err != nil {
		return nil, err
	}
	return s.groupThreadDetail(ctx, thread)
}

// CloseParentGroupThread closes a group thread: families can no longer
// answer it. Closing a closed thread is a no-op.
func (s *Service) CloseParentGroupThread(ctx context.Context, threadID int64) (*deliveryPorts.ParentGroupThreadDetail, error) {
	thread, err := s.loadGroupThread(ctx, threadID, true)
	if err != nil {
		return nil, err
	}
	if thread.ClosedAt == nil {
		recipientAccountIDs, err := s.GroupThreads.RecipientAccountIDs(ctx, thread.ID)
		if err != nil {
			return nil, fmt.Errorf("messaging: list group thread recipients: %w", err)
		}
		if _, err := s.GroupThreads.CloseThread(ctx, thread.ID, s.accountID(ctx)); err != nil {
			return nil, fmt.Errorf("messaging: close group thread: %w", err)
		}
		if thread, err = s.GroupThreads.FindThread(ctx, threadID, false); err != nil {
			return nil, fmt.Errorf("messaging: reload group thread: %w", err)
		}
		s.broadcastGroupThreadAfterCommit(ctx, thread, recipientAccountIDs)
	}
	return s.groupThreadDetail(ctx, thread)
}

func (s *Service) loadGroupThread(ctx context.Context, threadID int64, lock bool) (*deliveryPorts.ParentGroupThread, error) {
	if err := s.requireGroupAuthor(ctx); err != nil {
		return nil, err
	}
	thread, err := s.GroupThreads.FindThread(ctx, threadID, lock)
	if err != nil {
		return nil, fmt.Errorf("messaging: find group thread: %w", err)
	}
	if thread == nil {
		return nil, deliveryPorts.ErrParentGroupThreadNotFound
	}
	return thread, nil
}

// sendGroupMessage resolves the target's reachable families, stores the
// message with them as recipients and notifies them. A message that would
// reach nobody is refused: the OGS would believe it informed the group.
func (s *Service) sendGroupMessage(ctx context.Context, thread *deliveryPorts.ParentGroupThread, accountID int64, body string) error {
	children, err := s.GroupAudience.GroupAudience(ctx, thread.TenantID, thread.Target)
	if err != nil {
		return fmt.Errorf("messaging: group audience: %w", err)
	}
	recipients := groupRecipients(children)
	if len(recipients) == 0 {
		return deliveryPorts.ErrParentGroupThreadNoRecipients
	}
	message, err := s.GroupThreads.AppendMessage(ctx, thread.ID, optionalAccount(accountID), s.resolveStaffName(ctx, accountID), body, recipients)
	if err != nil {
		return fmt.Errorf("messaging: append group message: %w", err)
	}
	thread.LastMessageAt = &message.CreatedAt
	if err := s.notifyGroupRecipients(ctx, thread, message.ID, recipients); err != nil {
		return err
	}
	s.broadcastGroupThreadAfterCommit(ctx, thread, recipientAccountIDs(recipients))
	return nil
}

func (s *Service) groupThreadDetail(ctx context.Context, thread *deliveryPorts.ParentGroupThread) (*deliveryPorts.ParentGroupThreadDetail, error) {
	messages, err := s.GroupThreads.ListMessages(ctx, thread.ID)
	if err != nil {
		return nil, fmt.Errorf("messaging: list group messages: %w", err)
	}
	children, err := s.groupReadChildren(ctx, thread)
	if err != nil {
		return nil, err
	}
	return &deliveryPorts.ParentGroupThreadDetail{
		Thread: *thread, Messages: messages,
		Children: children,
	}, nil
}

// decorateGroupThreadTitles names the group thread a guardian's message
// answers, so the OGS sees the reference in the 1:1 conversation.
func (s *Service) decorateGroupThreadTitles(ctx context.Context, messages []*ParentMessage) error {
	if s.GroupThreads == nil {
		return nil
	}
	ids := make([]int64, 0)
	for _, message := range messages {
		if message != nil && message.GroupThreadID != nil {
			ids = append(ids, *message.GroupThreadID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	titles, err := s.GroupThreads.TitlesByIDs(ctx, ids)
	if err != nil {
		return fmt.Errorf("messaging: group thread titles: %w", err)
	}
	for _, message := range messages {
		if message != nil && message.GroupThreadID != nil {
			message.GroupThreadTitle = titles[*message.GroupThreadID]
		}
	}
	return nil
}

func normalizeGroupTarget(target deliveryPorts.ParentGroupTarget, requireLabel bool) (deliveryPorts.ParentGroupTarget, error) {
	target.Label = strings.TrimSpace(target.Label)
	if requireLabel && (target.Label == "" || utf8.RuneCountInString(target.Label) > maxGroupTargetLabelLen) {
		return target, deliveryPorts.ErrParentGroupThreadInvalid
	}
	switch target.Type {
	case deliveryPorts.ParentGroupTargetClass:
		if target.RefText == nil {
			return target, deliveryPorts.ErrParentGroupThreadInvalid
		}
		name := strings.TrimSpace(*target.RefText)
		if name == "" || utf8.RuneCountInString(name) > maxGroupClassNameLen {
			return target, deliveryPorts.ErrParentGroupThreadInvalid
		}
		target.RefText, target.RefID = &name, nil
	case deliveryPorts.ParentGroupTargetGroup, deliveryPorts.ParentGroupTargetActivityGroup:
		if target.RefID == nil || *target.RefID <= 0 {
			return target, deliveryPorts.ErrParentGroupThreadInvalid
		}
		target.RefText = nil
	default:
		return target, deliveryPorts.ErrParentGroupThreadInvalid
	}
	return target, nil
}

func normalizeGroupBody(body string) (string, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", deliveryPorts.ErrParentGroupMessageEmptyBody
	}
	if utf8.RuneCountInString(body) > maxMessageLen {
		return "", deliveryPorts.ErrParentGroupMessageTooLong
	}
	return body, nil
}

func groupRecipients(children []deliveryPorts.ParentGroupAudienceChild) []deliveryPorts.ParentGroupRecipient {
	out := make([]deliveryPorts.ParentGroupRecipient, 0, len(children))
	for _, child := range children {
		for _, accountID := range child.GuardianAccountIDs {
			out = append(out, deliveryPorts.ParentGroupRecipient{StudentID: child.StudentID, GuardianAccountID: accountID})
		}
	}
	return out
}

func optionalAccount(accountID int64) *int64 {
	if accountID <= 0 {
		return nil
	}
	return &accountID
}

// groupReadChildren lists every child the thread reached with its read
// state and name, unread children first, then by name.
func (s *Service) groupReadChildren(ctx context.Context, thread *deliveryPorts.ParentGroupThread) ([]deliveryPorts.ParentGroupChild, error) {
	reads, err := s.GroupThreads.ReadStatus(ctx, thread.ID)
	if err != nil {
		return nil, fmt.Errorf("messaging: group read status: %w", err)
	}
	ids := make([]int64, 0, len(reads))
	for _, read := range reads {
		ids = append(ids, read.StudentID)
	}
	named, err := s.GroupAudience.GroupStudents(ctx, thread.TenantID, ids)
	if err != nil {
		return nil, fmt.Errorf("messaging: group read names: %w", err)
	}
	names := make(map[int64]deliveryPorts.ParentGroupChild, len(named))
	for _, child := range named {
		names[child.StudentID] = child
	}
	out := make([]deliveryPorts.ParentGroupChild, 0, len(reads))
	for _, read := range reads {
		child := names[read.StudentID]
		child.StudentID, child.Reachable, child.Read = read.StudentID, true, read.Read
		out = append(out, child)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Read != out[j].Read {
			return !out[i].Read
		}
		return strings.ToLower(out[i].LastName+" "+out[i].FirstName) < strings.ToLower(out[j].LastName+" "+out[j].FirstName)
	})
	return out, nil
}
