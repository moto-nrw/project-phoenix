package timetracking

import "context"

// CleanupOrphanedStaffDocumentFiles runs Workforce's recovery pass over the
// stored staff document files (workforce.StaffDocumentFileRecovery).
func (rs *StaffAdminResource) CleanupOrphanedStaffDocumentFiles(ctx context.Context) (int, error) {
	return rs.documentFiles.CleanupOrphanedStaffDocumentFiles(ctx)
}
