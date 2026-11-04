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
