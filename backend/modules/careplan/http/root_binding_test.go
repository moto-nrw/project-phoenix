package careplanhttp_test

import (
	"github.com/moto-nrw/project-phoenix/api"
	subject "github.com/moto-nrw/project-phoenix/modules/careplan/http"
)

func init() {
	subject.BindTestConfig = func(cfg subject.ResourceConfig) subject.ResourceConfig {
		s := api.NewCareHTTPUnitResource(cfg)
		if cfg.NotifyAbsenceReported != nil {
			s.NotifyAbsenceReported = cfg.NotifyAbsenceReported
		}
		return s.ResourceConfig
	}
}
