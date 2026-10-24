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
	handovers map[int64]ClassHandover
	err       error
	asked     []int64
	day       calendar.Date
	calls     int
}

func (f *classHandoverFake) ClassHandovers(_ context.Context, ids []int64, day calendar.Date) (map[int64]ClassHandover, error) {
	f.calls++
	f.asked, f.day = ids, day
	return f.handovers, f.err
}

// TestKindersucheCarriesTheClassHandover pins that the page's children carry
// today's handover, "da" or "nicht da", read once and only with users:read.
func TestKindersucheCarriesTheClassHandover(t *testing.T) {
	t.Parallel()
	day := calendar.NewDate(2026, 8, 19)
	at := time.Date(2026, 8, 19, 11, 45, 0, 0, time.UTC)
	absent := ClassHandover{SchoolClass: "2b", HandedOverAt: at, HandedOverBy: "Petra Lehrkraft", Absent: true}
	present := ClassHandover{SchoolClass: "2b", HandedOverAt: at, HandedOverBy: "Petra Lehrkraft"}
	staff := testpkg.WithPermissionsContext(context.Background(), []string{"users:read"})

	reader := &classHandoverFake{handovers: map[int64]ClassHandover{2: absent, 3: present}}
	responses := []StudentResponse{{ID: 1, SchoolClass: "1a"}, {ID: 2, SchoolClass: "2b"}, {ID: 3, SchoolClass: "2b"}}
	require.NoError(t, (&Resource{ResourceConfig{ClassHandovers: reader}}).enrichWithClassHandovers(staff, responses, day))
	assert.Equal(t, []int64{1, 2, 3}, reader.asked)
	assert.Equal(t, day, reader.day)
	assert.Nil(t, responses[0].ClassHandover)
	assert.Equal(t, &absent, responses[1].ClassHandover)
	assert.Equal(t, &present, responses[2].ClassHandover)
	assert.Equal(t, &absent, slimStudentResponses(responses, day)[1].ClassHandover, "the slim list carries it")

	guest := &classHandoverFake{handovers: reader.handovers}
	responses = []StudentResponse{{ID: 2}}
	require.NoError(t, (&Resource{ResourceConfig{ClassHandovers: guest}}).enrichWithClassHandovers(
		testpkg.WithPermissionsContext(context.Background(), []string{"students:read"}), responses, day))
	assert.Zero(t, guest.calls, "like sick and excused, only with users:read")
	assert.Nil(t, responses[0].ClassHandover)

	failing := &classHandoverFake{err: errors.New("presence unavailable")}
	err := (&Resource{ResourceConfig{ClassHandovers: failing}}).enrichWithClassHandovers(staff, []StudentResponse{{ID: 2}}, day)
	require.ErrorContains(t, err, "load class handovers")
}
