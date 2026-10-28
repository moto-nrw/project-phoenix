package messaging

import (
	"context"
	"errors"
	"fmt"

	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

// errLeadersNotConfigured reports a service built without its leadership
// directory: a configuration error, never an empty leadership.
var errLeadersNotConfigured = errors.New("messaging: leadership directory is not configured")

// threadAudience is the conversation's audience; a thread value built without
// one is a team conversation, as every thread was before #5200.
func threadAudience(thread *ParentMessageThread) string {
	if thread == nil || thread.Audience == "" {
		return deliveryPorts.ParentAudienceTeam
	}
	return thread.Audience
}

// leaderNames resolves the leadership persons' short names when they are
// shown, so a changed role shows up on the next read.
func (s *Service) leaderNames(ctx context.Context) ([]string, error) {
	if s.Leaders == nil {
		return nil, errLeadersNotConfigured
	}
	leaders, err := s.Leaders.ParentMessageLeaders(ctx)
	if err != nil {
		return nil, fmt.Errorf("messaging: list leadership persons: %w", err)
	}
	return deliveryPorts.ParentMessageLeaderShortNames(leaders), nil
}

// leadershipAvailable reports whether a new leadership conversation may
// begin and names the leadership persons. A read failure is a request
// failure: the choice routes a family's message away from the team, so it is
// neither offered nor refused on a guess.
func (s *Service) leadershipAvailable(ctx context.Context) (bool, []string, error) {
	if s.Leaders == nil {
		return false, nil, errLeadersNotConfigured
	}
	choice, err := deliveryPorts.ResolveParentMessageLeadershipChoice(ctx, s.Settings, s.Leaders)
	if err != nil {
		return false, nil, fmt.Errorf("messaging: %w", err)
	}
	return choice.Available(), choice.ShortNames(), nil
}

func (s *Service) requireLeadershipAvailable(ctx context.Context) error {
	available, _, err := s.leadershipAvailable(ctx)
	if err != nil {
		return err
	}
	if !available {
		return ErrLeadershipUnavailable
	}
	return nil
}

// readerPreview says who on the school side would read a new conversation
// of the audience, and whether a new leadership conversation may begin. Only
// a leadership member learns the latter and the leadership names.
func (s *Service) readerPreview(ctx context.Context, audience string) (deliveryPorts.ParentMessageReaderPreview, error) {
	preview := deliveryPorts.ParentMessageReaderPreview{
		Audience: audience,
		School:   deliveryPorts.ParentMessageSchoolReaders{Kind: deliveryPorts.ParentMessageSchoolReadersKind(audience)},
	}
	if !s.readsLeadership(ctx) {
		return preview, nil
	}
	available, names, err := s.leadershipAvailable(ctx)
	if err != nil {
		return deliveryPorts.ParentMessageReaderPreview{}, err
	}
	preview.LeadershipAvailable = available
	if audience == deliveryPorts.ParentAudienceLeadership {
		preview.School.Names = names
	}
	return preview, nil
}

// reach is which conversations the caller lists, counts and opens. Without
// the leadership permission the leadership conversations do not exist for the
// caller: no row, no number, no event and 404 on their id.
func (s *Service) reach(ctx context.Context) StaffReach {
	if s.UserContext == nil {
		return StaffReach{}
	}
	perms := s.Session.Permissions(ctx)
	return StaffReach{
		AllStudents: s.UserContext.ReadsAllStudents(ctx, perms),
		Leadership:  s.UserContext.ReadsLeadershipMessages(ctx, perms),
	}
}

// readsLeadership reports whether the caller holds the leadership permission.
func (s *Service) readsLeadership(ctx context.Context) bool {
	if s.UserContext == nil {
		return false
	}
	return s.UserContext.ReadsLeadershipMessages(ctx, s.Session.Permissions(ctx))
}

// conversationForWrite get-or-creates the (student, guardian, audience)
// conversation for a staff member. Only a leadership member may open a
// leadership conversation, and a new one only while the school offers the
// choice (#5200); an existing one stays open even after it was switched off.
func (s *Service) conversationForWrite(ctx context.Context, studentID, guardianAccountID int64, audience string) (*ParentMessageThread, error) {
	audience, err := s.staffAudience(ctx, audience)
	if err != nil {
		return nil, err
	}
	if audience == deliveryPorts.ParentAudienceLeadership {
		existing, err := s.ThreadRepo.FindByStudentGuardian(ctx, studentID, guardianAccountID, audience)
		if err != nil {
			return nil, fmt.Errorf("messaging: find leadership thread: %w", err)
		}
		if existing == nil {
			if err := s.requireLeadershipAvailable(ctx); err != nil {
				return nil, err
			}
		}
	}
	thread, err := s.ThreadRepo.GetOrCreate(ctx, s.runtime.TenantID(ctx), studentID, guardianAccountID, audience)
	if err != nil {
		return nil, fmt.Errorf("messaging: get-or-create thread: %w", err)
	}
	return thread, nil
}

// staffAudience validates the audience a staff member asks for. A caller
// without the leadership permission may not name the leadership audience.
func (s *Service) staffAudience(ctx context.Context, audience string) (string, error) {
	normalized, ok := deliveryPorts.NormalizeParentMessageAudience(audience)
	if !ok {
		return "", ErrInvalidAudience
	}
	if normalized == deliveryPorts.ParentAudienceLeadership && !s.readsLeadership(ctx) {
		return "", ErrForbidden
	}
	return normalized, nil
}
