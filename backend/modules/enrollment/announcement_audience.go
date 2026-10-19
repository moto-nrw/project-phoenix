package enrollment

import "context"

// PendingAnnouncementApplicant identifies the account or pre-account email that
// has an undecided, non-withdrawn application at the caller's school.
type PendingAnnouncementApplicant struct {
	TenantID          int64  `json:"tenant_id"`
	GuardianFirstName string `json:"guardian_first_name"`
	GuardianLastName  string `json:"guardian_last_name"`
	GuardianAccountID *int64 `json:"guardian_account_id"`
	GuardianEmail     string `json:"guardian_email"`
}

// PendingAnnouncementApplicantsForSchools serves a parent feed's explicit school
// scope. Cross-school callers supply their authorized admin transaction.
func (m *Module) PendingAnnouncementApplicantsForSchools(ctx context.Context, schoolIDs []int64) ([]PendingAnnouncementApplicant, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.PendingAnnouncementApplicantsForSchools(ctx, schoolIDs)
}

func (m *Module) PendingAnnouncementApplicants(ctx context.Context) ([]PendingAnnouncementApplicant, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.PendingAnnouncementApplicants(ctx)
}
