package api

import (
	"context"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// ReadOnlyPreviewMiddleware enforces the write block for admin staff-view
// preview tokens (#2893). Requests without a read-only claim pass untouched;
// with one, only safe methods (GET/HEAD/OPTIONS, minus the denylisted
// state-changing GETs) and the allowlisted
// read-only POST routes go through. Mounted group-wide right after
// sessiontoken.Authenticator: it reads only the parsed claims, does no DB work, and
// rejecting here means no tenant transaction is ever opened for a blocked
// write.
func ReadOnlyPreviewMiddleware(next http.Handler) http.Handler {
	return common.ReadOnlyPreviewGuard(readOnlyPreview)(next)
}

func readOnlyPreview(ctx context.Context) bool {
	claims := sessiontoken.ClaimsFromCtx(ctx)
	return claims.IsReadOnlyPreview()
}
