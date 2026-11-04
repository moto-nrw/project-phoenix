package application

import (
	"context"
)

// prepareCreate validates a new announcement: the common fields, the shape of
// a class letter (#4800), the poll options and the reminder.
func prepareCreate(ctx context.Context, in *Input) ([]*StaffParentAnnouncementTarget, []*StaffParentAnnouncementOption, error) {
	targets, err := normalizeInput(in)
	if err != nil {
		return nil, nil, err
	}
	if err := normalizeClassLetter(ctx, in, in.Sender, targets); err != nil {
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

// prepareUpdate validates an editable announcement before changing its stored
// fields. A declaration with submissions keeps its delivery mode so its
// status and guardian proof remain reachable.
func (s *service) prepareUpdate(ctx context.Context, a *StaffParentAnnouncement, in *Input) ([]*StaffParentAnnouncementTarget, []*StaffParentAnnouncementOption, error) {
	targets, err := normalizeInput(in)
	if err != nil {
		return nil, nil, err
	}
	// An edit keeps the stored sender: it never moves a class letter to
	// another class or author name (#4800).
	if err := normalizeClassLetter(ctx, in, classLetterSenderOf(a), targets); err != nil {
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
