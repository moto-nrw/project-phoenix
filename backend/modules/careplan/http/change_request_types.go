package careplanhttp

import (
	parentrequests "github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
)

// Wire names of the parent-request types, owned by the shared request-review
// projection (#2705). The lifecycle routes (mark-done, correct) classify
// their {kind} path segment by them.
const (
	requestTypeMasterData   = string(parentrequests.KindMasterData)
	requestTypeCareSchedule = string(parentrequests.KindCareSchedule)
	requestTypeOffering     = string(parentrequests.KindOffering)
	requestTypeExcused      = string(parentrequests.KindExcused)
)
