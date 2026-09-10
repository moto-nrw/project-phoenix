package iot

import (
	"context"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/moto-nrw/project-phoenix/api/testutil"
)

func testRuntime() Runtime {
	return Runtime{
		Authenticated: func(ctx context.Context) bool { _, _, ok := testutil.DeviceIdentity(ctx); return ok },
		Success:       testutil.RespondSuccess,
		Failure: func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest, status int, err error, message string) {
			testutil.RespondCoded(w, r, status, "", err, nil, message)
		},
	}
}
