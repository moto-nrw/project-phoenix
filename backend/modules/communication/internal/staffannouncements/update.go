package announcement

import (
	"context"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

// prepareUpdate validates an editable announcement before changing its stored
// fields. A declaration with submissions keeps its delivery mode so its
// status and guardian proof remain reachable.
func (s *service) prepareUpdate(ctx context.Context, a *usersModels.ParentAnnouncement, in *Input) ([]*usersModels.ParentAnnouncementTarget, []*usersModels.ParentAnnouncementOption, error) {
	targets, err := normalizeInput(in)
	if err != nil {
		return nil, nil, err
	}
	if err := s.guardDeclarationModeChange(ctx, a, in.DeliveryMode); err != nil {
		return nil, nil, err
	}
	if err := s.guardDeclarationSettingsChange(ctx, a, in); err != nil {
		return nil, nil, err
	}
	options, err := normalizePollOptions(in)
	if err != nil {
		return nil, nil, err
	}
	if err := normalizeReminder(in); err != nil {
		return nil, nil, err
	}
	return targets, options, nil
}
