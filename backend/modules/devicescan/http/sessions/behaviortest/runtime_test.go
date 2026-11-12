package sessions_test

import (
	"context"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/devicescan"
	sessionsAPI "github.com/moto-ogs/project-phoenix/modules/devicescan/http/sessions"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

func testRuntime() sessionsAPI.Runtime {
	return sessionsAPI.Runtime{
		ParseID:       testpkg.ParseHTTPIDParam,
		Authenticated: func(ctx context.Context) bool { _, _, ok := testutil.DeviceIdentity(ctx); return ok },
		Success:       testutil.RespondSuccess,
		Failure: func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest, status int, err error, message string) {
			testutil.RespondCoded(w, r, status, "", err, nil, message)
		},
		Conflict: func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest, message string, info devicescan.ConflictInfoResponse) {
			render.Status(r, http.StatusConflict)
			render.JSON(w, r, map[string]any{"status": "error", "error": message, "details": info})
		},
		MarkRollback: testutil.MarkRollback,
	}
}
