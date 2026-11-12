package requestreviewhttp_test

import (
	"github.com/moto-ogs/project-phoenix/api"
	subject "github.com/moto-ogs/project-phoenix/modules/requestreview/http"
)

func init() {
	subject.BindTestConfig = func(cfg subject.ResourceConfig) subject.ResourceConfig {
		s := api.NewReviewHTTPUnitResource(cfg)
		return s.ResourceConfig
	}
}
