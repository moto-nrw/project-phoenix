package messaging

import (
	"context"
	"errors"
)

// Conversation audiences (#5200). They mirror Communication's values; the
// root binding carries them unchanged.
const (
	AudienceTeam       = "team"
	AudienceLeadership = "leadership"
)

var (
	// ErrInvalidAudience means the audience is neither team nor leadership,
	// or a group-thread answer named the leadership.
	ErrInvalidAudience = errors.New("parent: invalid conversation audience")
	// ErrLeadershipUnavailable means no new leadership conversation may begin:
	// the school switched the choice off or nobody holds the permission.
	ErrLeadershipUnavailable = errors.New("parent: leadership conversations are not available")
)

// conversationRef names one conversation for its realtime wake-up: the
// thread and its audience, which decides whose staff tabs wake (#5200).
type conversationRef struct {
	ID       int64
	Audience string
}

// LeadershipOption is the school's leadership option: whether it is switched
// on and the leadership persons' short names ("Swantje M.").
type LeadershipOption struct {
	Enabled bool
	Names   []string
}

// Available reports whether a new leadership conversation may begin.
func (o LeadershipOption) Available() bool {
	return o.Enabled && len(o.Names) > 0
}

// normalizeAudience maps an empty audience to the team conversation, so
// clients that send none keep writing to the team.
func normalizeAudience(audience string) (string, error) {
	switch audience {
	case "", AudienceTeam:
		return AudienceTeam, nil
	case AudienceLeadership:
		return AudienceLeadership, nil
	default:
		return "", ErrInvalidAudience
	}
}

// MessageTarget names where a guardian message goes: the audience ("" is
// the team) and the group thread it answers (0 for none).
type MessageTarget struct {
	Audience      string
	GroupThreadID int64
}

func firstTarget(targets []MessageTarget) MessageTarget {
	if len(targets) == 0 {
		return MessageTarget{}
	}
	return targets[0]
}

func firstAudience(audiences []string) string {
	if len(audiences) == 0 {
		return ""
	}
	return audiences[0]
}

// childConversationView resolves the guardian's child and builds the view of
// the audience's conversation: who reads it and, filled in the tenant
// transaction, the leadership option.
func (s *Service) childConversationView(ctx context.Context, accountID, studentID int64, audience string) (*Child, *MessageThreadView, error) {
	audience, err := normalizeAudience(audience)
	if err != nil {
		return nil, nil, err
	}
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, nil, err
	}
	return child, &MessageThreadView{
		StudentID: studentID, StudentName: child.StudentName, SchoolName: child.SchoolName,
		Audience: audience,
	}, nil
}

// findChildConversation stamps the leadership option on the view and loads
// the guardian's conversation of its audience, nil when none exists yet.
func (s *Service) findChildConversation(txCtx context.Context, view *MessageThreadView, accountID int64) (*ParentMessageThread, error) {
	guardianName, err := s.guardianName(txCtx, accountID)
	if err != nil {
		return nil, err
	}
	view.GuardianName = guardianName
	thread, err := s.MessageThreadRepo.FindByStudentGuardian(txCtx, view.StudentID, accountID, view.Audience)
	if err != nil {
		return nil, err
	}
	if err := s.decorateLeadership(txCtx, view, thread != nil); err != nil {
		return nil, err
	}
	return thread, nil
}

// normalizeMessageInput validates a guardian message: its audience, the team
// audience of a group-thread answer, and the body.
func normalizeMessageInput(body, audience string, groupThreadID int64) (string, string, error) {
	audience, err := normalizeAudience(audience)
	if err != nil {
		return "", "", err
	}
	if groupThreadID > 0 && audience != AudienceTeam {
		return "", "", ErrInvalidAudience
	}
	body, err = normalizeParentMessageBody(body)
	if err != nil {
		return "", "", err
	}
	return body, audience, nil
}

// decorateLeadership stamps the school's leadership option on the view and,
// for a leadership conversation, its readers: named while the conversation
// exists or the school offers the choice, never while it is off and no
// conversation exists. It runs in the child's tenant transaction.
func (s *Service) decorateLeadership(txCtx context.Context, view *MessageThreadView, conversationExists bool) error {
	leadership := view.Audience == AudienceLeadership
	option, err := s.Conversations.LeadershipOption(txCtx, leadership && conversationExists)
	if err != nil {
		return err
	}
	view.Leadership = option
	if leadership && (conversationExists || option.Available()) {
		view.LeadershipReaders = option.Names
	}
	return nil
}

// guardianConversation get-or-creates the guardian's conversation of the
// view's audience. A leadership conversation the guardian has not started
// yet needs the school's leadership option.
func (s *Service) guardianConversation(txCtx context.Context, tenantID, studentID, accountID int64, view *MessageThreadView) (*ParentMessageThread, error) {
	if view.Audience != AudienceLeadership {
		if err := s.decorateLeadership(txCtx, view, false); err != nil {
			return nil, err
		}
		return s.MessageThreadRepo.GetOrCreate(txCtx, tenantID, studentID, accountID, view.Audience)
	}
	existing, err := s.MessageThreadRepo.FindByStudentGuardian(txCtx, studentID, accountID, AudienceLeadership)
	if err != nil {
		return nil, err
	}
	// A new conversation needs the choice. A started one stays writable after
	// the school switched it off, but not once nobody is left to read it.
	if err := s.decorateLeadership(txCtx, view, existing != nil); err != nil {
		return nil, err
	}
	if len(view.Leadership.Names) == 0 || (existing == nil && !view.Leadership.Enabled) {
		return nil, ErrLeadershipUnavailable
	}
	return s.MessageThreadRepo.GetOrCreate(txCtx, tenantID, studentID, accountID, view.Audience)
}
