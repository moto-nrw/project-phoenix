package enrollment

// PendingAnnouncementApplicant identifies the account or pre-account email that
// has an undecided, non-withdrawn application at the caller's school.
type PendingAnnouncementApplicant struct {
	TenantID          int64  `json:"tenant_id"`
	GuardianFirstName string `json:"guardian_first_name"`
	GuardianLastName  string `json:"guardian_last_name"`
	GuardianAccountID *int64 `json:"guardian_account_id"`
	GuardianEmail     string `json:"guardian_email"`
}
