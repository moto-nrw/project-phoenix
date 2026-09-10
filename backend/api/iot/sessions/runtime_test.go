package sessions_test

import (
	"context"

	sessionsAPI "github.com/moto-nrw/project-phoenix/api/iot/sessions"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func testRuntime() sessionsAPI.Runtime {
	return sessionsAPI.Runtime{
		ParseID:       testpkg.ParseHTTPIDParam,
		Authenticated: func(ctx context.Context) bool { _, _, ok := testutil.DeviceIdentity(ctx); return ok },
		Success:       testutil.RespondSuccess,
		Failure: func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest, status int, err error, message string) {
			testutil.RespondCoded(w, r, status, "", err, nil, message)
		},
		MarkRollback: testutil.MarkRollback,
	}
}
