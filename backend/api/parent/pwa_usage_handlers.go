package parent

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// reportPWAUsage records that the parent's session runs in PWA standalone
// display mode, fanning out to every school the account is actively linked
// to (#2189). Idempotent: repeated reports only advance last_seen_at.
func (rs *Resource) reportPWAUsage(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(jwt.ErrTokenUnauthorized))
		return
	}
	if err := rs.PWAUsageService.ReportParent(r.Context(), int64(claims.ID)); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("App-Nutzung konnte nicht gespeichert werden.", err))
		return
	}
	common.Respond(w, r, http.StatusNoContent, nil, "")
}
