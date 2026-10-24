package data

import (
	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func testRuntime() Runtime {
	return Runtime{
		ParseID: testpkg.ParseHTTPIDParam,
		Device:  testutil.DeviceIdentity,
		Success: testutil.RespondSuccess,
		Failure: func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest, status int, err error, message string) {
			testutil.RespondCoded(w, r, status, "", err, nil, message)
		},
	}
}
