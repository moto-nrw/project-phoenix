package careplanhttp_test

import (
	"github.com/moto-ogs/project-phoenix/api"
	careplanhttp "github.com/moto-ogs/project-phoenix/modules/careplan/http"
)

// testResource binds the configuration the way the students mount does
// (api.NewStudentHTTPResources), keeping a test's own absence callback.
func testResource(cfg careplanhttp.ResourceConfig) *careplanhttp.Resource {
	bound := api.NewCareHTTPUnitResource(cfg)
	if cfg.NotifyAbsenceReported != nil {
		bound.NotifyAbsenceReported = cfg.NotifyAbsenceReported
	}
	return careplanhttp.NewResource(bound.ResourceConfig)
}
