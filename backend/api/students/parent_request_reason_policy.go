package students

import (
	"log/slog"
	"net/http"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// staffReasonRequired resolves operations.parent_request_reason_policy for the
// request's school and answers whether the DECIDING staff member must state a
// reason on an approval (#2267, story 28). Rejections always need one, whatever
// the policy says — that check lives in the domain services.
//
// A read failure falls back to the strictest reading: asking for a reason that
// was not required is a nuisance, dropping one that was required loses an
// explanation the family can never recover.
func (rs *Resource) staffReasonRequired(r *http.Request) bool {
	if rs.SettingsService == nil {
		return true
	}
	policy, err := rs.SettingsService.ResolveString(r.Context(), configModels.KeyParentRequestReasonPolicy)
	if err != nil {
		slog.WarnContext(r.Context(), "resolve parent request reason policy failed, requiring a reason",
			slog.String("error", err.Error()),
		)
		return true
	}
	return userService.ReasonRequiredFor(policy, true)
}
