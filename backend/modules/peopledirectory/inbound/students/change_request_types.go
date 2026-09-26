package students

import "github.com/moto-nrw/project-phoenix/modules/requestreview"

// Wire names of the parent-request types, owned by the shared request-review
// projection (#2705). The lifecycle routes (mark-done, correct) classify
// their {kind} path segment by them.
const (
	requestTypeMasterData   = requestreview.TypeMasterData
	requestTypeCareSchedule = requestreview.TypeCareSchedule
	requestTypeOffering     = requestreview.TypeOffering
	requestTypeExcused      = requestreview.TypeExcused
)
