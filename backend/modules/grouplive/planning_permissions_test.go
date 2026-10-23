package grouplive

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlanningPermissionProtectsEveryNoteRepresentation(t *testing.T) {
	t.Parallel()
	for _, allowed := range []bool{false, true} {
		f := &fakes{
			caller: func(context.Context) (Caller, error) {
				return Caller{CanReadGroups: true, CanReadStudentPlans: allowed}, nil
			},
			supervisedGroups: func(context.Context) ([]GroupRecord, error) { return twoGroups(), nil },
			members: func(context.Context, int64) ([]RosterStudent, error) {
				return []RosterStudent{{ID: 11, FirstName: "Mila", PhotoPath: new("/uploads/student-photos/test.jpg")}}, nil
			},
			photosEnabled: func(context.Context) (bool, error) { return true, nil },
			arrivals: func(_ context.Context, ids []int64, _ Date) (map[int64]Arrival, error) {
				if !allowed {
					require.Empty(t, ids, "a guest must not load student plans")
				}
				return map[int64]Arrival{11: {Notes: "Ankunft vertraulich", DayNotes: []string{"Tagesnotiz vertraulich"}}}, nil
			},
			pickups: func(context.Context, []int64, Date) (map[int64]Pickup, error) {
				return map[int64]Pickup{11: {Notes: "Abholung vertraulich", DayNotes: []DayNote{{Content: "Abholnotiz vertraulich"}}}}, nil
			},
			decide: func(DayInputs) DayDecision {
				return DayDecision{Reason: DayReasonArrivalException, ExceptionNotes: "Ausnahme vertraulich"}
			},
		}
		projection, err := newService(f).LiveGroup(context.Background(), 1)
		require.NoError(t, err)
		require.Len(t, projection.Students, 1)
		require.Equal(t, "Mila", projection.Students[0].FirstName)
		require.NotEmpty(t, projection.Students[0].PhotoURL)
		body, err := json.Marshal(projection)
		require.NoError(t, err)
		for _, note := range []string{"Ankunft vertraulich", "Tagesnotiz vertraulich", "Abholung vertraulich", "Abholnotiz vertraulich", "Ausnahme vertraulich"} {
			if allowed {
				require.Contains(t, string(body), note)
			} else {
				require.NotContains(t, string(body), note)
			}
		}
	}
}
