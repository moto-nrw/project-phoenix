package ports

import (
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

type RequestEditRecords interface {
	Find(context.Context, int64, bool) (carerequests.Request, error)
	UpdatePending(context.Context, int64, json.RawMessage) error
}

type RequestEditEvents interface {
	RecordGuardianEdit(context.Context, *carerequests.Request, int64) error
}
