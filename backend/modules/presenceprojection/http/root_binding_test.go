package presenceprojectionhttp_test

import (
	"github.com/moto-ogs/project-phoenix/api"
	subject "github.com/moto-ogs/project-phoenix/modules/presenceprojection/http"
)

func init() {
	subject.BindTestConfig = func(cfg subject.ResourceConfig) subject.ResourceConfig {
		s := api.NewProjectionHTTPUnitResource(cfg)
		return s.ResourceConfig
	}
}
