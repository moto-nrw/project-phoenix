package users

import "net/http"

func (rs *GuardianResource) listPendingInvitations(w http.ResponseWriter, r *http.Request) {
	invitations, err := rs.runtime.ListPendingInvitations(r.Context())
	if err != nil {
		rs.fail(w, r, FailureInternal, err)
		return
	}
	responses := make([]map[string]any, 0, len(invitations))
	for _, invitation := range invitations {
		response := map[string]any{
			"id":                  invitation.ID,
			"guardian_profile_id": invitation.GuardianProfileID,
			"created_at":          invitation.CreatedAt,
			"expires_at":          invitation.ExpiresAt,
			"email_sent_at":       invitation.EmailSentAt,
			"email_error":         invitation.EmailError,
			"email_retry_count":   invitation.EmailRetryCount,
		}
		// The link of a pending invitation is stored as hash only (#4902);
		// no listing can hand it out, not even to the seeder.
		responses = append(responses, response)
	}
	rs.succeed(w, r, http.StatusOK, responses, "Pending invitations retrieved successfully")
}
