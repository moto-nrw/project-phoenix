package careplantest

import (
	"context"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
)

// SchoolWideReviewScope is the review scope of a caller who may decide every
// child's request. Suites use it where the production policy is not under
// test.
func SchoolWideReviewScope(context.Context) (carePlanCompose.ReviewScope, error) {
	return carePlanCompose.ReviewScope{SchoolWide: true}, nil
}
