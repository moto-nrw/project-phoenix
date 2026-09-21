package carerequests

import (
	"context"
	"encoding/json"
)

type Diffs interface {
	Weekly(context.Context, int64, json.RawMessage) ([]DiffEntry, error)
	Snapshot(context.Context, *Request) *DecisionSnapshot
}
