package data_test

import (
	"github.com/moto-nrw/project-phoenix/api/testutil"
	dataAPI "github.com/moto-nrw/project-phoenix/modules/devicescan/http/data"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func testRuntime() dataAPI.Runtime {
	return dataAPI.Runtime{
		ParseID: testpkg.ParseHTTPIDParam,
		Device:  testutil.DeviceIdentity,
		Success: testutil.RespondSuccess,
		Failure: func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest, status int, err error, message string) {
			testutil.RespondCoded(w, r, status, "", err, nil, message)
		},
	}
}
