package owners

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/stretchr/testify/require"
)

type nativeStudentReadProbe struct {
	row   *peopledirectory.StudentSnapshot
	err   error
	calls []string
	ids   []int64
}

func (p *nativeStudentReadProbe) StudentSnapshotByID(_ context.Context, id int64) (*peopledirectory.StudentSnapshot, error) {
	p.calls = append(p.calls, "read")
	p.ids = append(p.ids, id)
	return p.row, p.err
}
func (p *nativeStudentReadProbe) StudentSnapshotForUpdate(_ context.Context, id int64) (*peopledirectory.StudentSnapshot, error) {
	p.calls = append(p.calls, "lock")
	p.ids = append(p.ids, id)
	return p.row, p.err
}

func TestParentStudentReadsPreserveLockAndErrorPaths(t *testing.T) {
	t.Parallel()
	source := &nativeStudentReadProbe{}
	reads := NewParentStudentReads(source)
	ctx := context.Background()
	id := int64(23)
	row, err := reads.FindByID(ctx, id)
	require.NoError(t, err)
	require.Nil(t, row)
	failure := errors.New("owner read failed")
	source.err = failure
	row, err = reads.FindByIDForUpdate(ctx, id)
	require.ErrorIs(t, err, failure)
	require.Nil(t, row)
	require.Equal(t, []string{"read", "lock"}, source.calls)
	require.Equal(t, []int64{id, id}, source.ids)
}

func TestParentStudentSnapshotRetainsDepartureAndPersonRelations(t *testing.T) {
	t.Parallel()
	for _, baseline := range []*peopledirectory.StudentDepartureSnapshot{nil, {}, {AllowedDepartureModes: []byte(`{"mon":null,"tue":[],"wed":["pickup","bus"]}`), DepartureDays: []byte(`{}`), PickupDays: []byte(`{"wed":false}`)}} {
		source := &peopledirectory.StudentSnapshot{AllowedDepartureModes: []byte(`{"mon":null,"tue":[],"wed":["pickup","bus"]}`), DepartureDays: []byte(`{}`), PickupDays: []byte(`{"wed":false}`), DepartureBaseline: baseline, Person: &peopledirectory.DirectoryPersonSnapshot{FirstName: "Mia", LastName: "Meier"}}
		got, err := parentStudentSnapshot(source)
		require.NoError(t, err)
		require.NotNil(t, got.Person)
		require.Equal(t, "Mia", got.Person.FirstName)
		require.NotNil(t, got.DepartureDays)
		require.Empty(t, got.DepartureDays)
		require.Nil(t, got.BusDays)
		require.Nil(t, got.AllowedDepartureModes["mon"])
		require.NotNil(t, got.AllowedDepartureModes["tue"])
		require.Empty(t, got.AllowedDepartureModes["tue"])
		require.False(t, got.PickupDays["wed"])
		if baseline == nil {
			require.Nil(t, got.DepartureBaseline)
			continue
		}
		require.NotNil(t, got.DepartureBaseline)
		if len(baseline.AllowedDepartureModes) == 0 {
			require.Nil(t, got.DepartureBaseline.AllowedDepartureModes)
			continue
		}
		require.True(t, reflect.DeepEqual(got.AllowedDepartureModes, got.DepartureBaseline.AllowedDepartureModes))
		got.AllowedDepartureModes["wed"][0] = "alone"
		require.Equal(t, "pickup", string(got.DepartureBaseline.AllowedDepartureModes["wed"][0]))
	}
}
