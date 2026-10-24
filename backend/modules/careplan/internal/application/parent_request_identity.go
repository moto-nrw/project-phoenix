package application

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

type ParentCareRequestRecords interface {
	FindCareScheduleRequest(context.Context, int64, bool) (carerequests.Request, error)
}

type ParentCareRequestReads struct {
	Records ParentCareRequestRecords
	Missing func() error
}

func (r ParentCareRequestReads) CareRequestIdentity(ctx context.Context, id int64) (*carerequests.ParentCareRequestIdentity, error) {
	row, err := r.Records.FindCareScheduleRequest(ctx, id, false)
	if errors.Is(err, carerequests.ErrNotFound) {
		return nil, r.Missing()
	}
	if err != nil {
		return nil, err
	}
	// Retain the decoding the previous row reader performed before exposing identity.
	var payload map[string]any
	if err := json.Unmarshal(row.Payload, &payload); err != nil {
		return nil, err
	}
	if len(row.DecisionSnapshot) > 0 && string(row.DecisionSnapshot) != "null" {
		var snapshot carerequests.DecisionSnapshot
		if err := json.Unmarshal(row.DecisionSnapshot, &snapshot); err != nil {
			return nil, parentRequestDecodeError(err)
		}
	}
	return &carerequests.ParentCareRequestIdentity{SubmittedBy: row.SubmittedBy, StudentID: row.StudentID, RequestKind: row.RequestKind}, nil
}

// Persisted snapshot diagnostics are existing problem-detail text. Keep the
// retained Go type names while preserving the native JSON error as the cause.
func parentRequestDecodeError(err error) error {
	var mismatch *json.UnmarshalTypeError
	if !errors.As(err, &mismatch) {
		return err
	}
	text := strings.NewReplacer(
		"carerequests.DecisionSnapshot", "schedule.CareRequestDecisionSnapshot",
		"carerequests.SnapshotEntry", "schedule.CareRequestSnapshotEntry",
		"DecisionSnapshot", "CareRequestDecisionSnapshot",
		"SnapshotEntry", "CareRequestSnapshotEntry",
	).Replace(err.Error())
	return parentRequestDecodeFailure{error: err, text: text}
}

type parentRequestDecodeFailure struct {
	error
	text string
}

func (e parentRequestDecodeFailure) Error() string { return e.text }
func (e parentRequestDecodeFailure) Unwrap() error { return e.error }
