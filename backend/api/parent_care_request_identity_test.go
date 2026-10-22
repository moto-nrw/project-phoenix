package api

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type parentCareRequestRecordStub struct {
	row    careplan.CareScheduleChangeRequest
	err    error
	onFind func(context.Context, int64, bool)
}

func (s *parentCareRequestRecordStub) FindCareScheduleRequest(ctx context.Context, id int64, lock bool) (careplan.CareScheduleChangeRequest, error) {
	if s.onFind != nil {
		s.onFind(ctx, id, lock)
	}
	return s.row, s.err
}

func TestParentCareRequestIdentityPreservesReadContract(t *testing.T) {
	t.Parallel()
	// Distinct 64-bit boundary values detect truncation and swapped identity fields.
	source := &parentCareRequestRecordStub{row: careplan.CareScheduleChangeRequest{
		SubmittedBy: math.MaxInt64, StudentID: math.MaxInt64 - 1,
		RequestKind: "weekly_schedule", Payload: json.RawMessage(`{"weekdays":[]}`),
	}}
	var requestID int64 = math.MaxInt64 - 2
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	source.onFind = func(got context.Context, id int64, lock bool) {
		calls++
		assert.Same(t, ctx, got)
		assert.Equal(t, requestID, id)
		assert.False(t, lock)
	}
	reader := owners.NewParentCareRequestReads(source)
	submittedBy, studentID, kind, err := reader.FindIdentity(ctx, requestID)
	require.NoError(t, err)
	assert.Equal(t, source.row.SubmittedBy, submittedBy)
	assert.Equal(t, source.row.StudentID, studentID)
	assert.Equal(t, source.row.RequestKind, kind)
	assert.Equal(t, 1, calls)
	// A missing request is the sharing routes' 404 (#4599), no longer the
	// retained missing-record error that rendered as 500.
	source.err = careplan.ErrCareScheduleRequestNotFound
	_, _, _, err = reader.FindIdentity(ctx, requestID)
	require.ErrorIs(t, err, ParentRequestSharingNotFound)
	require.EqualError(t, err, ParentRequestSharingNotFound.Error())
	source.err = errors.New("database unavailable")
	_, _, _, err = reader.FindIdentity(ctx, requestID)
	assert.ErrorIs(t, err, source.err)
	assert.Nil(t, owners.NewParentCareRequestReads(nil))
}

func TestParentCareRequestIdentityPreservesStoredSnapshotDiagnostics(t *testing.T) {
	t.Parallel()
	// Golden details were observed by running the retained snapshot decoder.
	for _, tc := range []struct{ raw, detail string }{
		{`[]`, `json: cannot unmarshal array into Go value of type schedule.CareRequestDecisionSnapshot`},
		{`{"diff":{}}`, `json: cannot unmarshal object into Go struct field CareRequestDecisionSnapshot.diff of type []schedule.CareRequestSnapshotEntry`},
		{`{"diff":[{"weekday":"bad"}]}`, `json: cannot unmarshal string into Go struct field CareRequestDecisionSnapshot.diff.0.weekday of type int`},
		{`{"diff":[{"old_modes":42}]}`, `json: cannot unmarshal number into Go struct field CareRequestDecisionSnapshot.diff.0.old_modes of type []string`},
		{`{`, `unexpected end of JSON input`},
		{`null`, ``},
		{`{"diff":[]}`, ``},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			source := &parentCareRequestRecordStub{row: careplan.CareScheduleChangeRequest{Payload: json.RawMessage(`{}`), DecisionSnapshot: json.RawMessage(tc.raw)}}
			var requestID int64
			_, _, _, err := owners.NewParentCareRequestReads(source).FindIdentity(context.Background(), requestID)
			if tc.detail == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tc.detail)
			if tc.raw != "{" {
				var mismatch *json.UnmarshalTypeError
				assert.ErrorAs(t, err, &mismatch)
			}
		})
	}
}
