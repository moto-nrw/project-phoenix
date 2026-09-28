package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/require"
)

// The flows decode the owner's requests and children, whose answers arrive as
// raw JSON. A corrupt or invalid stored row fails the whole read instead of
// yielding a partial batch.

type carePeriodReadResult struct {
	periods []*enrollment.StudentCarePeriod
	err     error
}

func (r carePeriodReadResult) StudentCarePeriods(context.Context, int64) ([]*enrollment.StudentCarePeriod, error) {
	return r.periods, r.err
}

func TestCarePeriodReadPreservesFailureWithoutPartialResults(t *testing.T) {
	t.Parallel()
	failure := errors.New("care period read failed")
	valid := &enrollment.StudentCarePeriod{ServiceStartDate: "2026-09-01", ServiceEndDate: "2027-08-31"}
	for _, tc := range []struct {
		name   string
		result carePeriodReadResult
	}{
		{name: "owner error with partial rows", result: carePeriodReadResult{periods: []*enrollment.StudentCarePeriod{valid}, err: failure}},
		{name: "invalid start after valid row", result: carePeriodReadResult{periods: []*enrollment.StudentCarePeriod{valid, {ServiceStartDate: "2026-02-30", ServiceEndDate: "2027-08-31"}}}},
		{name: "invalid end after valid row", result: carePeriodReadResult{periods: []*enrollment.StudentCarePeriod{valid, {ServiceStartDate: "2026-09-01", ServiceEndDate: "2027-02-30"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			periods, err := enrollment.StudentCarePeriodRecords(t.Context(), tc.result, 0)
			require.Error(t, err)
			require.Nil(t, periods, "failed reads must not expose a partial care-period history")
			if tc.result.err != nil {
				require.ErrorIs(t, err, failure)
			}
		})
	}
}

func TestRequestBatchRejectsMalformedStoredJSON(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"consent", "legal", "custom", "source"} {
		t.Run(field, func(t *testing.T) {
			invalid := json.RawMessage(`{"unfinished":`)
			row := &enrollment.Request{ID: 2}
			switch field {
			case "consent":
				row.ConsentFlags = invalid
			case "legal":
				row.LegalBlocksSnapshot = invalid
			case "custom":
				row.CustomData = invalid
			case "source":
				row.SourceMetadata = invalid
			}
			rows, err := requestValues([]*enrollment.Request{{ID: 1}, row})
			var syntaxError *json.SyntaxError
			require.ErrorAs(t, err, &syntaxError)
			require.Nil(t, rows, "a corrupt row must not return a successful partial batch")
		})
	}
}

func TestChildBatchRejectsMalformedStoredJSON(t *testing.T) {
	t.Parallel()
	rows, err := childValues([]*enrollment.RequestChild{
		{ID: 1, DateOfBirth: "2018-04-15"},
		{ID: 2, DateOfBirth: "2018-04-15", CustomData: json.RawMessage(`{"unfinished":`)},
	})
	var syntaxError *json.SyntaxError
	require.ErrorAs(t, err, &syntaxError)
	require.Nil(t, rows, "a corrupt row must not return a successful partial batch")
}

func TestChildBatchRejectsInvalidDates(t *testing.T) {
	t.Parallel()
	invalid := enrollment.Date("2027-02-30")
	for _, row := range []*enrollment.RequestChild{
		{ID: 2, DateOfBirth: invalid},
		{ID: 2, DateOfBirth: "2018-04-15", ActivateOn: &invalid},
	} {
		rows, err := childValues([]*enrollment.RequestChild{
			{ID: 1, DateOfBirth: "2018-04-15"}, row,
		})
		require.Error(t, err)
		require.Nil(t, rows, "invalid dates must not yield a partial child batch")
	}
}

func TestOwnerOfferingSelectionPreservesLegacyJSON(t *testing.T) {
	t.Parallel()
	for _, populated := range []bool{false, true} {
		name := "optional_fields_omitted"
		if populated {
			name = "dated_selection"
		}
		t.Run(name, func(t *testing.T) {
			row := &enrollment.RequestChildOfferingRecord{RequestChildID: 9007199254740993, CareOfferingID: 9007199254740995}
			row.ID = 9007199254740997
			row.TenantID = 9007199254740999
			row.CreatedAt = time.Date(2027, 3, 28, 0, 0, 0, 0, time.UTC)
			row.UpdatedAt = row.CreatedAt
			if populated {
				from, until := calendar.Date("2027-03-28"), calendar.Date("2027-10-31")
				note := "selection history"
				row.ValidFrom, row.ValidUntil, row.Notes = &from, &until, &note
				row.SelectedDays = []string{"mon", "wed"}
				row.ManualSelectedDays = []string{"mon"}
				row.AutomaticSelectedDays = []string{"wed"}
			}
			before, err := json.Marshal(row)
			require.NoError(t, err)
			var migrated enrollment.RequestChildOffering
			require.NoError(t, json.Unmarshal(before, &migrated))
			require.Equal(t, row.ID, migrated.ID)
			require.Equal(t, row.RequestChildID, migrated.RequestChildID)
			after, err := json.Marshal(migrated)
			require.NoError(t, err)
			require.JSONEq(t, string(before), string(after))
			converted := enrollment.RequestChildOfferingRecordsOf([]*enrollment.RequestChildOffering{&migrated})
			require.Equal(t, row, converted[0])
			if populated {
				require.Equal(t, enrollment.Date("2027-03-28"), *migrated.ValidFrom)
				require.Equal(t, enrollment.Date("2027-10-31"), *migrated.ValidUntil)
			}
		})
	}
}
