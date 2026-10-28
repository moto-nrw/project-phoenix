package staffmessaging

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	values "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// Group chats (#4776): a named conversation with members from the staff and
// the Lehrkräfte of one school. Access is membership, exactly as in the direct
// chat; what is new is who may change the membership:
//   - OGS staff and the leadership start groups; a Lehrkraft takes part but
//     does not start one.
//   - the person who started a group and the leadership (role kind admin)
//     rename it and add or remove members, as long as they are members
//     themselves: access stays membership; every member may leave.
//   - a new member sees the history from the moment they joined.

const (
	// The thread kinds (domain.StaffMessageThreadKind*), stored in
	// users.staff_message_threads.kind.
	directKind = "direct"
	groupKind  = "group"

	// Role kinds of the colleague directory (peopledirectory StaffRoleKind*).
	roleKindAdmin     = "admin"
	roleKindLehrkraft = "lehrkraft"

	// maxGroupNameLen caps a group name in runes.
	maxGroupNameLen = 80
	// maxGroupMembers caps a group including its creator. A group of a whole
	// school is a Mitteilung, not a chat.
	maxGroupMembers = 100
)

var (
	// ErrGroupCreateForbidden: the caller may not start a group (a Lehrkraft).
	ErrGroupCreateForbidden = errors.New("staffmessaging: this role may not start a group chat")
	// ErrGroupManageForbidden: the caller neither started the group nor leads
	// the school.
	ErrGroupManageForbidden = errors.New("staffmessaging: only the creator and the leadership may manage this group")
	// ErrGroupNameInvalid: the name is blank or too long.
	ErrGroupNameInvalid = errors.New("staffmessaging: group name must not be empty or too long")
	// ErrGroupMembersRequired: a group needs at least one other member.
	ErrGroupMembersRequired = errors.New("staffmessaging: a group needs at least one other member")
	// ErrGroupTooLarge: more members than maxGroupMembers.
	ErrGroupTooLarge = errors.New("staffmessaging: too many members for one group")
)

// requireActiveGroupMember closes a group for a member whose account no
// longer counts as a colleague. Rendered like "no such thread".
func (s *Service) requireActiveGroupMember(ctx context.Context, thread *StaffMessageThread, accountID int64) error {
	if thread.Kind != groupKind {
		return nil
	}
	active, err := s.ReadRepo.IsMessageableStaff(ctx, accountID)
	if err != nil {
		return err
	}
	if !active {
		return ErrNotParticipant
	}
	return nil
}

func validateGroupName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || len([]rune(trimmed)) > maxGroupNameLen {
		return "", ErrGroupNameInvalid
	}
	return trimmed, nil
}

// roleKind classifies one account at the current school.
func (s *Service) roleKind(ctx context.Context, accountID int64) (string, error) {
	kinds, err := s.ReadRepo.StaffRoleKinds(ctx, []int64{accountID})
	if err != nil {
		return "", err
	}
	return kinds[accountID], nil
}

// activeCaller resolves the caller and checks the switch and that the caller
// is still a colleague at this school.
func (s *Service) activeCaller(ctx context.Context) (int64, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return 0, err
	}
	accountID, err := s.actor(ctx)
	if err != nil {
		return 0, err
	}
	if err := s.requireActiveMember(ctx, accountID); err != nil {
		return 0, err
	}
	return accountID, nil
}

// requireMessageable checks every new member against the colleague relation
// the recipient picker shows, with one directory read.
func (s *Service) requireMessageable(ctx context.Context, callerAccountID int64, accountIDs []int64) error {
	colleagues, err := s.ReadRepo.ListMessageableStaff(ctx, callerAccountID)
	if err != nil {
		return err
	}
	known := make(map[int64]bool, len(colleagues))
	for _, colleague := range colleagues {
		known[colleague.AccountID] = true
	}
	for _, id := range accountIDs {
		if !known[id] {
			return ErrRecipientNotAvailable
		}
	}
	return nil
}

// distinctOthers drops duplicates, non-positive ids and the caller.
func distinctOthers(accountIDs []int64, callerAccountID int64) []int64 {
	seen := make(map[int64]bool, len(accountIDs))
	out := make([]int64, 0, len(accountIDs))
	for _, id := range accountIDs {
		if id <= 0 || id == callerAccountID || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// CreateGroup starts a group chat with the caller and the given colleagues.
func (s *Service) CreateGroup(ctx context.Context, name string, memberAccountIDs []int64) (*ThreadDetail, error) {
	accountID, err := s.activeCaller(ctx)
	if err != nil {
		return nil, err
	}
	kind, err := s.roleKind(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if kind == roleKindLehrkraft {
		return nil, ErrGroupCreateForbidden
	}
	trimmed, err := validateGroupName(name)
	if err != nil {
		return nil, err
	}
	members := distinctOthers(memberAccountIDs, accountID)
	if len(members) == 0 {
		return nil, ErrGroupMembersRequired
	}
	if len(members)+1 > maxGroupMembers {
		return nil, ErrGroupTooLarge
	}
	if err := s.requireMessageable(ctx, accountID, members); err != nil {
		return nil, err
	}
	record, err := s.Groups.CreateStaffGroupThread(ctx, trimmed, accountID, members)
	if err != nil {
		return nil, err
	}
	thread := (*StaffMessageThread)(values.StaffThreadFromRecord(record))
	s.broadcastMembershipChange(ctx, thread, accountID, nil)
	return s.buildDetail(ctx, thread, accountID)
}

// managedGroup authorizes a membership or name change: the caller is an
// active member of the group and either started it or leads the school.
func (s *Service) managedGroup(ctx context.Context, threadID int64) (*StaffMessageThread, int64, error) {
	accountID, err := s.activeCaller(ctx)
	if err != nil {
		return nil, 0, err
	}
	thread, err := s.authorizeThread(ctx, threadID, accountID)
	if err != nil {
		return nil, 0, err
	}
	if thread.Kind != groupKind {
		return nil, 0, ErrThreadNotFound
	}
	// Serialize membership changes on the thread row, so two additions cannot
	// both pass the size cap and a last leave cannot race an addition.
	if err := s.lockThread(ctx, threadID); err != nil {
		return nil, 0, err
	}
	// A removal may have committed while this request waited for the lock.
	// Revalidate membership while holding it before deciding management rights.
	thread, err = s.authorizeThread(ctx, threadID, accountID)
	if err != nil {
		return nil, 0, err
	}
	if thread.Kind != groupKind {
		return nil, 0, ErrThreadNotFound
	}
	manager, err := s.canManage(ctx, thread, accountID)
	if err != nil {
		return nil, 0, err
	}
	if !manager {
		return nil, 0, ErrGroupManageForbidden
	}
	return thread, accountID, nil
}

func (s *Service) canManage(ctx context.Context, thread *StaffMessageThread, accountID int64) (bool, error) {
	if thread.CreatedByAccountID != nil && *thread.CreatedByAccountID == accountID {
		return true, nil
	}
	kind, err := s.roleKind(ctx, accountID)
	if err != nil {
		return false, err
	}
	return kind == roleKindAdmin, nil
}

// RenameGroup changes the name of a group.
func (s *Service) RenameGroup(ctx context.Context, threadID int64, name string) (*ThreadDetail, error) {
	thread, accountID, err := s.managedGroup(ctx, threadID)
	if err != nil {
		return nil, err
	}
	trimmed, err := validateGroupName(name)
	if err != nil {
		return nil, err
	}
	if err := s.Groups.RenameStaffGroupThread(ctx, threadID, trimmed); err != nil {
		return nil, err
	}
	thread.Name = trimmed
	s.broadcastMembershipChange(ctx, thread, accountID, nil)
	return s.buildDetail(ctx, thread, accountID)
}

// AddGroupMembers adds colleagues to a group. They see the history from now.
func (s *Service) AddGroupMembers(ctx context.Context, threadID int64, accountIDs []int64) (*ThreadDetail, error) {
	thread, accountID, err := s.managedGroup(ctx, threadID)
	if err != nil {
		return nil, err
	}
	current, err := s.ThreadRepo.ParticipantAccountIDs(ctx, threadID)
	if err != nil {
		return nil, err
	}
	isMember := make(map[int64]bool, len(current))
	for _, id := range current {
		isMember[id] = true
	}
	added := make([]int64, 0, len(accountIDs))
	for _, id := range distinctOthers(accountIDs, accountID) {
		if !isMember[id] {
			added = append(added, id)
		}
	}
	if len(added) == 0 {
		return nil, ErrGroupMembersRequired
	}
	if len(current)+len(added) > maxGroupMembers {
		return nil, ErrGroupTooLarge
	}
	if err := s.requireMessageable(ctx, accountID, added); err != nil {
		return nil, err
	}
	if err := s.Groups.AddStaffGroupParticipants(ctx, threadID, added); err != nil {
		return nil, err
	}
	s.broadcastMembershipChange(ctx, thread, accountID, nil)
	return s.buildDetail(ctx, thread, accountID)
}

// RemoveGroupMember takes one member out of a group. Removing oneself is
// leaving, which every member may do.
func (s *Service) RemoveGroupMember(ctx context.Context, threadID, memberAccountID int64) (*ThreadDetail, error) {
	if caller, err := s.actor(ctx); err == nil && caller == memberAccountID {
		return nil, s.LeaveGroup(ctx, threadID)
	}
	thread, accountID, err := s.managedGroup(ctx, threadID)
	if err != nil {
		return nil, err
	}
	member, err := s.ThreadRepo.IsParticipant(ctx, threadID, memberAccountID)
	if err != nil {
		return nil, err
	}
	if member {
		if err := s.Groups.RemoveStaffParticipant(ctx, threadID, memberAccountID); err != nil {
			return nil, err
		}
		// The removed member's open tabs must learn that the group is gone.
		s.broadcastMembershipChange(ctx, thread, accountID, []int64{memberAccountID})
	}
	return s.buildDetail(ctx, thread, accountID)
}

// LeaveGroup removes the caller from a group. The last member to leave
// deletes the group with its messages.
//
// Deliberately no activeCaller: leaving must work for anyone still holding a
// membership, even when the chat was switched off meanwhile.
func (s *Service) LeaveGroup(ctx context.Context, threadID int64) error {
	accountID, err := s.actor(ctx)
	if err != nil {
		return err
	}
	thread, err := s.authorizeThread(ctx, threadID, accountID)
	if err != nil {
		return err
	}
	if thread.Kind != groupKind {
		return ErrThreadNotFound
	}
	if err := s.lockThread(ctx, threadID); err != nil {
		return err
	}
	// The caller might have been removed while waiting for the lock.
	thread, err = s.authorizeThread(ctx, threadID, accountID)
	if err != nil {
		return err
	}
	if thread.Kind != groupKind {
		return ErrThreadNotFound
	}
	if err := s.Groups.RemoveStaffParticipant(ctx, threadID, accountID); err != nil {
		return err
	}
	remaining, err := s.ThreadRepo.ParticipantAccountIDs(ctx, threadID)
	if err != nil {
		return err
	}
	if len(remaining) == 0 {
		if err := s.Groups.DeleteStaffThread(ctx, threadID); err != nil {
			return err
		}
		// Deleting the thread also deletes its participant rows, so use the
		// departing account captured above instead of resolving recipients again.
		s.broadcastMembershipRecipients(ctx, thread, accountID, []int64{accountID})
		return nil
	}
	s.broadcastMembershipChange(ctx, thread, accountID, []int64{accountID})
	return nil
}

// broadcastMembershipChange wakes the current members and anyone who just
// lost access, after commit. Their inboxes and open chats refetch; the event
// carries no content.
func (s *Service) broadcastMembershipChange(ctx context.Context, thread *StaffMessageThread, actorAccountID int64, formerMembers []int64) {
	if s.Broadcaster == nil {
		return
	}
	participants, err := s.ThreadRepo.ParticipantAccountIDs(ctx, thread.ID)
	if err != nil {
		s.Logger.Warn("staffmessaging: resolve participants for group broadcast failed",
			slog.Int64("thread_id", thread.ID),
			slog.String("error", err.Error()),
		)
		return
	}
	recipients := append(append([]int64{}, participants...), formerMembers...)
	s.broadcastMembershipRecipients(ctx, thread, actorAccountID, recipients)
}

func (s *Service) broadcastMembershipRecipients(ctx context.Context, thread *StaffMessageThread, actorAccountID int64, recipients []int64) {
	tenantID, threadID := thread.TenantID, thread.ID
	s.runtime.RegisterAfterCommit(ctx, func() {
		event := realtime.NewStaffMessageEvent(actorAccountID, threadID)
		if err := s.Broadcaster.BroadcastToStaffAccounts(tenantID, recipients, event); err != nil {
			s.Logger.Warn("staffmessaging: group broadcast failed",
				slog.Int64("thread_id", threadID),
				slog.String("error", err.Error()),
			)
		}
		if err := s.Broadcaster.BroadcastToSchoolAccounts(tenantID, recipients, event); err != nil {
			s.Logger.Warn("staffmessaging: group school broadcast failed",
				slog.Int64("thread_id", threadID),
				slog.String("error", err.Error()),
			)
		}
	})
}

// buildGroupDetail assembles a group's chat window for one viewer: the
// messages since the viewer joined, the members, and per message who read it.
func (s *Service) buildGroupDetail(ctx context.Context, thread *StaffMessageThread, viewerAccountID int64) (*ThreadDetail, error) {
	records, err := s.Groups.StaffParticipantStates(ctx, thread.ID)
	if err != nil {
		return nil, err
	}
	states := values.StaffParticipantStatesFromRecords(records)
	var viewerFrom *time.Time
	for _, state := range states {
		if state.AccountID == viewerAccountID {
			viewerFrom = state.HistoryFrom
		}
	}
	all, err := s.MessageRepo.ListByThread(ctx, thread.ID)
	if err != nil {
		return nil, err
	}
	messages := make([]*StaffMessage, 0, len(all))
	for _, message := range all {
		if viewerFrom == nil || !message.CreatedAt.Before(*viewerFrom) {
			messages = append(messages, message)
		}
	}
	members, err := s.groupMembers(ctx, viewerAccountID, states)
	if err != nil {
		return nil, err
	}
	manager, err := s.canManage(ctx, thread, viewerAccountID)
	if err != nil {
		return nil, err
	}
	return &ThreadDetail{
		ThreadID:  thread.ID,
		Kind:      groupKind,
		GroupName: thread.Name,
		CanManage: manager,
		Members:   members,
		Messages:  messages,
		Receipts:  readReceipts(messages, states),
	}, nil
}

// groupMembers names the members with one directory read; a member who is no
// longer a colleague falls back to the frozen-name lookup.
func (s *Service) groupMembers(ctx context.Context, viewerAccountID int64, states []*values.StaffParticipantState) ([]values.MessageableStaff, error) {
	colleagues, err := s.ReadRepo.ListMessageableStaff(ctx, viewerAccountID)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(colleagues))
	for _, colleague := range colleagues {
		names[colleague.AccountID] = colleague.Name
	}
	ids := make([]int64, 0, len(states))
	for _, state := range states {
		ids = append(ids, state.AccountID)
	}
	kinds, err := s.ReadRepo.StaffRoleKinds(ctx, ids)
	if err != nil {
		return nil, err
	}
	members := make([]values.MessageableStaff, 0, len(states))
	for _, state := range states {
		name, ok := names[state.AccountID]
		if !ok {
			name = s.resolveSenderName(ctx, state.AccountID)
		}
		members = append(members, values.MessageableStaff{AccountID: state.AccountID, Name: name, RoleKind: kinds[state.AccountID]})
	}
	return members, nil
}

// readReceipts computes, per message, which members read it. A member counts
// for a message when they are not its sender and had joined by the time it
// was written; they read it when their cursor is at or past it. The
// comparison is the same (created_at, id) tuple the unread count uses.
func readReceipts(messages []*StaffMessage, states []*values.StaffParticipantState) map[int64]values.StaffReadReceipt {
	out := make(map[int64]values.StaffReadReceipt, len(messages))
	for _, message := range messages {
		receipt := values.StaffReadReceipt{ReadBy: []int64{}, EligibleReaderAccountIDs: []int64{}}
		for _, state := range states {
			if state.AccountID == message.SenderAccountID {
				continue
			}
			if state.HistoryFrom != nil && message.CreatedAt.Before(*state.HistoryFrom) {
				continue
			}
			receipt.ReaderCount++
			receipt.EligibleReaderAccountIDs = append(receipt.EligibleReaderAccountIDs, state.AccountID)
			if cursorAtOrPast(state, message) {
				receipt.ReadBy = append(receipt.ReadBy, state.AccountID)
			}
		}
		out[message.ID] = receipt
	}
	return out
}

func (s *Service) refreshGroupReceipts(ctx context.Context, threadID int64, detail *ThreadDetail) error {
	records, err := s.Groups.StaffParticipantStates(ctx, threadID)
	if err != nil {
		return err
	}
	detail.Receipts = readReceipts(detail.Messages, values.StaffParticipantStatesFromRecords(records))
	return nil
}

// groupMessageReceipt returns the initial receipt for a message while the
// caller still holds the thread lock from PostMessage.
func (s *Service) groupMessageReceipt(ctx context.Context, threadID int64, message *StaffMessage) (*values.StaffReadReceipt, error) {
	records, err := s.Groups.StaffParticipantStates(ctx, threadID)
	if err != nil {
		return nil, err
	}
	receipt := readReceipts([]*StaffMessage{message}, values.StaffParticipantStatesFromRecords(records))[message.ID]
	return &receipt, nil
}

func cursorAtOrPast(state *values.StaffParticipantState, message *StaffMessage) bool {
	if state.LastReadAt == nil {
		return false
	}
	if state.LastReadAt.After(message.CreatedAt) {
		return true
	}
	return state.LastReadAt.Equal(message.CreatedAt) && state.LastReadMessageID >= message.ID
}
