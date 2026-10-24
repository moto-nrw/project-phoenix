package presenceprojectionhttp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// classHandoverFake answers the Kindersuche's class handover port (#4794).
type classHandoverFake struct {
	absences map[int64]ClassHandoverAbsence
	err      error
	asked    []int64
	day      calendar.Date
	calls    int
}

func (f *classHandoverFake) HandoverAbsences(_ context.Context, ids []int64, day calendar.Date) (map[int64]ClassHandoverAbsence, error) {
	f.calls++
	f.asked, f.day = ids, day
	return f.absences, f.err
}

// TestKindersucheMarksChildrenAbsentAtTheHandover pins that the page's
// children carry today's handover absence, read once and only with
// users:read.
func TestKindersucheMarksChildrenAbsentAtTheHandover(t *testing.T) {
	t.Parallel()
	day := calendar.NewDate(2026, 8, 19)
	absence := ClassHandoverAbsence{SchoolClass: "2b", HandedOverAt: time.Date(2026, 8, 19, 11, 45, 0, 0, time.UTC), HandedOverBy: "Petra Lehrkraft"}
	staff := testpkg.WithPermissionsContext(context.Background(), []string{"users:read"})

	reader := &classHandoverFake{absences: map[int64]ClassHandoverAbsence{2: absence}}
	responses := []StudentResponse{{ID: 1}, {ID: 2}}
	require.NoError(t, (&Resource{ResourceConfig{ClassHandovers: reader}}).enrichWithHandoverAbsences(staff, responses, day))
	assert.Equal(t, []int64{1, 2}, reader.asked)
	assert.Equal(t, day, reader.day)
	assert.Nil(t, responses[0].HandoverAbsent)
	assert.Equal(t, &absence, responses[1].HandoverAbsent)
	assert.Equal(t, &absence, slimStudentResponses(responses, day)[1].HandoverAbsent, "the slim list carries it")

	guest := &classHandoverFake{absences: reader.absences}
	responses = []StudentResponse{{ID: 2}}
	require.NoError(t, (&Resource{ResourceConfig{ClassHandovers: guest}}).enrichWithHandoverAbsences(
		testpkg.WithPermissionsContext(context.Background(), []string{"students:read"}), responses, day))
	assert.Zero(t, guest.calls, "like sick and excused, only with users:read")
	assert.Nil(t, responses[0].HandoverAbsent)

	failing := &classHandoverFake{err: errors.New("presence unavailable")}
	err := (&Resource{ResourceConfig{ClassHandovers: failing}}).enrichWithHandoverAbsences(staff, []StudentResponse{{ID: 2}}, day)
	require.ErrorContains(t, err, "load class handover absences")
}
