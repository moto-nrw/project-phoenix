package peopledirectory

import "context"

// FamilyProtectionQuery reads the current privacy flag without exposing the
// audit event, its reason, or the account that changed it.
type FamilyProtectionQuery interface {
	CurrentFamilyProtection(context.Context, []int64) (map[int64]bool, error)
}

// CurrentFamilyProtection returns the latest event's flag for each requested
// child in the tenant. Children without events are absent, meaning unprotected.
func (m *Module) CurrentFamilyProtection(ctx context.Context, studentIDs []int64) (map[int64]bool, error) {
	for _, id := range studentIDs {
		if id <= 0 {
			return nil, &InvalidStudentError{Reason: "family protection requires positive student IDs"}
		}
	}
	if len(studentIDs) == 0 {
		return map[int64]bool{}, nil
	}
	return m.engine.CurrentFamilyProtection(ctx, uniquePositive(studentIDs))
}
