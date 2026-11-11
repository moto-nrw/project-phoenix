package care

import "context"

// ParentConferenceChild resolves a child for the Elternsprechtag (#4801).
// Seeing the coming days of the child's class needs parent_portal.access;
// booking, moving or cancelling a slot commits the family to a meeting and
// needs parent_portal.conference.book on the same relationship. The calendar
// keeps the class check; this flow keeps the relationship check before the
// calendar opens its unit of work.
func (s *Service) ParentConferenceChild(ctx context.Context, accountID, studentID int64, book bool) (*Child, error) {
	permission := GuardianPermissionPortalAccess
	if book {
		permission = GuardianPermissionConferenceBook
	}
	return s.ResolvePermittedChild(ctx, accountID, studentID, permission)
}

// WithParentConferenceBooking runs a booking, move or cancellation of an
// Elternsprechtag slot in the child's school unit behind the child write
// guard (#6529): parent_portal.conference.book, the relationship and the
// school membership hold until the calendar's write commits.
func (s *Service) WithParentConferenceBooking(ctx context.Context, accountID, studentID int64, write func(ctx context.Context, child *Child) error) error {
	child, err := s.ParentConferenceChild(ctx, accountID, studentID, true)
	if err != nil {
		return err
	}
	return s.WithChildWrite(ctx, accountID, studentID, child, childWriteNeeds(GuardianPermissionConferenceBook), func(txCtx context.Context) error {
		return write(txCtx, child)
	})
}
