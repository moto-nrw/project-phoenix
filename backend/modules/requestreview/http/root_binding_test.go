package requestreviewhttp_test

import (
	"github.com/moto-ogs/project-phoenix/api"
	subject "github.com/moto-ogs/project-phoenix/modules/requestreview/http"
)

// testResource builds the resource over cfg with the bindings the root adds.
func testResource(cfg subject.ResourceConfig) *subject.Resource {
	return subject.NewResource(api.NewReviewHTTPUnitResource(cfg).ResourceConfig)
}
