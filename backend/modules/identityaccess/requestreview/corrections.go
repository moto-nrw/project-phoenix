package requestreview

import "context"

// CorrectionsAllowed preserves the direct-correction history's staff gate.
// Unlike parent-request review, it is not controlled by group-leader settings.
func CorrectionsAllowed(ctx context.Context, adminWildcard bool, currentStaff func(context.Context) (bool, error)) bool {
	if adminWildcard {
		return true
	}
	if currentStaff == nil {
		return false
	}
	staff, err := currentStaff(ctx)
	return err == nil && staff
}
