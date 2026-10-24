package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type handoverSourceFake struct {
	handovers []studentpresence.ClassHandover
	err       error
}

func (f handoverSourceFake) ClassHandoversOn(context.Context, calendar.Date) ([]studentpresence.ClassHandover, error) {
	return f.handovers, f.err
}

type staffNamerFake struct {
	names map[int64]string
	asked [][]int64
	err   error
}

func (f *staffNamerFake) StaffNames(_ context.Context, ids []int64) (map[int64]string, error) {
	f.asked = append(f.asked, ids)
	return f.names, f.err
}

// TestClassHandoversNameTheLehrkraftOnce pins the root join for the OGS
// views (#4794): only the asked children a handover named, "da" or "nicht
// da", the moment in Berlin time, and one name read per request, skipped
// when no handover named an asked child.
func TestClassHandoversNameTheLehrkraftOnce(t *testing.T) {
	t.Parallel()
	day := calendar.NewDate(2026, 8, 19)
	at := time.Date(2026, 8, 19, 9, 45, 0, 0, time.UTC)
	source := handoverSourceFake{handovers: []studentpresence.ClassHandover{
		{SchoolClass: "2b", HandedOverAt: at, HandedOverByStaffID: 7, PresentStudentIDs: []int64{4}, AbsentStudentIDs: []int64{1, 2}},
		{SchoolClass: "3a", HandedOverAt: at, HandedOverByStaffID: 8, PresentStudentIDs: []int64{5}, AbsentStudentIDs: []int64{}},
		{SchoolClass: "4c", HandedOverAt: at, AbsentStudentIDs: []int64{3}},
	}}
	staff := &staffNamerFake{names: map[int64]string{7: "Petra Lehrkraft"}}

	got, err := loadClassHandovers(context.Background(), classHandoverReads{handovers: source, staff: staff}, []int64{1, 2, 3, 4, 9}, day)
	require.NoError(t, err)
	require.Len(t, staff.asked, 1, "one name read")
	assert.Equal(t, []int64{7}, staff.asked[0], "each Lehrkraft once, nobody without an asked child")
	require.Len(t, got, 4)
	assert.Equal(t, childClassHandover{schoolClass: "2b", handedOverAt: at.In(calendar.Berlin), handedOverBy: "Petra Lehrkraft", absent: true}, got[1])
	assert.Equal(t, childClassHandover{schoolClass: "2b", handedOverAt: at.In(calendar.Berlin), handedOverBy: "Petra Lehrkraft"}, got[4], "handed over and there")
	assert.True(t, got[3].absent)
	assert.Empty(t, got[3].handedOverBy, "a removed staff member leaves the handover unnamed")
	assert.NotContains(t, got, int64(9), "not handed over")
	assert.Equal(t, calendar.Berlin, got[1].handedOverAt.Location())

	unnamed := &staffNamerFake{}
	got, err = loadClassHandovers(context.Background(), classHandoverReads{handovers: source, staff: unnamed}, []int64{9}, day)
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Empty(t, unnamed.asked, "no name read without a handed-over child")

	_, err = loadClassHandovers(context.Background(), classHandoverReads{handovers: handoverSourceFake{err: errors.New("down")}, staff: unnamed}, []int64{1}, day)
	require.Error(t, err)
	_, err = loadClassHandovers(context.Background(), classHandoverReads{handovers: source, staff: &staffNamerFake{err: errors.New("down")}}, []int64{1}, day)
	require.Error(t, err)
}
