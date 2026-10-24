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

// TestClassHandoverAbsencesNameTheLehrkraftOnce pins the root join for the
// OGS views (#4794): only the asked children a handover marked "nicht da",
// the moment in Berlin time, and one name read per request, skipped when no
// asked child is marked.
func TestClassHandoverAbsencesNameTheLehrkraftOnce(t *testing.T) {
	t.Parallel()
	day := calendar.NewDate(2026, 8, 19)
	at := time.Date(2026, 8, 19, 9, 45, 0, 0, time.UTC)
	source := handoverSourceFake{handovers: []studentpresence.ClassHandover{
		{SchoolClass: "2b", HandedOverAt: at, HandedOverByStaffID: 7, AbsentStudentIDs: []int64{1, 2}},
		{SchoolClass: "3a", HandedOverAt: at, HandedOverByStaffID: 8, AbsentStudentIDs: []int64{}},
		{SchoolClass: "4c", HandedOverAt: at, AbsentStudentIDs: []int64{3}},
	}}
	staff := &staffNamerFake{names: map[int64]string{7: "Petra Lehrkraft"}}

	got, err := loadClassHandoverAbsences(context.Background(), classHandoverAbsenceReads{absences: source, staff: staff}, []int64{1, 2, 3, 9}, day)
	require.NoError(t, err)
	assert.Equal(t, [][]int64{{7}}, staff.asked, "one read, each Lehrkraft once, nobody without marked children")
	require.Len(t, got, 3)
	assert.Equal(t, "Petra Lehrkraft", got[1].handedOverBy)
	assert.Equal(t, "2b", got[2].schoolClass)
	assert.Empty(t, got[3].handedOverBy, "a removed staff member leaves the handover unnamed")
	assert.Equal(t, calendar.Berlin, got[1].handedOverAt.Location())

	unmarked := &staffNamerFake{}
	got, err = loadClassHandoverAbsences(context.Background(), classHandoverAbsenceReads{absences: source, staff: unmarked}, []int64{9}, day)
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Empty(t, unmarked.asked, "no name read without a marked child")

	_, err = loadClassHandoverAbsences(context.Background(), classHandoverAbsenceReads{absences: handoverSourceFake{err: errors.New("down")}, staff: unmarked}, []int64{1}, day)
	require.Error(t, err)
	_, err = loadClassHandoverAbsences(context.Background(), classHandoverAbsenceReads{absences: source, staff: &staffNamerFake{err: errors.New("down")}}, []int64{1}, day)
	require.Error(t, err)
}
