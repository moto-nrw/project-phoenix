package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/ports"
)

type directoryPeopleFake struct {
	teachers []DirectoryTeacher
	calls    int
	filter   []int64
	err      error
	students []devicescan.DirectoryStudent
}

func (p *directoryPeopleFake) Teachers(context.Context) ([]DirectoryTeacher, error) {
	p.calls++
	return p.teachers, p.err
}

func TestDirectoryOmitsIncompleteTeacherRecords(t *testing.T) {
	t.Parallel()
	people := &directoryPeopleFake{teachers: []DirectoryTeacher{
		{StaffID: testStaffID, Person: &ports.Person{ID: testPersonID, FirstName: "Legacy", LastName: "Teacher"}},
		{StaffID: testStaffID + 1},
	}}
	directory := NewDirectory(people, nil, nil, fakePrincipals{device: testDevice()}, nil)
	teachers, err := directory.Teachers(context.Background())
	if err != nil || len(teachers) != 1 || teachers[0].StaffID != testStaffID || teachers[0].PersonID != testPersonID || teachers[0].DisplayName != "Legacy Teacher" {
		t.Fatalf("unexpected kiosk teacher roster: %+v, %v", teachers, err)
	}
}

func (p *directoryPeopleFake) Students(_ context.Context, filter []int64) ([]devicescan.DirectoryStudent, error) {
	p.calls++
	p.filter = filter
	return p.students, p.err
}

func TestDirectoryRejectsUnauthenticatedReads(t *testing.T) {
	t.Parallel()
	people := &directoryPeopleFake{}
	directory := NewDirectory(people, nil, nil, fakePrincipals{}, nil)
	if _, err := directory.Teachers(context.Background()); !errors.Is(err, devicescan.ErrDeviceUnauthorized) {
		t.Fatalf("teachers: expected missing-device rejection, got %v", err)
	}
	if _, err := directory.Students(context.Background(), nil); !errors.Is(err, devicescan.ErrDeviceUnauthorized) {
		t.Fatalf("students: expected missing-device rejection, got %v", err)
	}
	if people.calls != 0 {
		t.Fatalf("unauthenticated request read the roster %d times", people.calls)
	}
}

func TestDirectoryDistinguishesAbsentAndEmptyTeacherFilter(t *testing.T) {
	t.Parallel()
	people := &directoryPeopleFake{students: []devicescan.DirectoryStudent{{StudentID: testStudentID}}}
	directory := NewDirectory(people, nil, nil, fakePrincipals{device: testDevice()}, nil)
	all, err := directory.Students(context.Background(), nil)
	if err != nil || len(all) != 1 || people.calls != 1 || people.filter != nil {
		t.Fatalf("absent filter did not select all students: result=%v, error=%v, calls=%d", all, err, people.calls)
	}
	empty, err := directory.Students(context.Background(), []int64{})
	if err != nil || empty == nil || len(empty) != 0 || people.calls != 1 {
		t.Fatalf("empty filter must return an empty array without reading: result=%v, error=%v, calls=%d", empty, err, people.calls)
	}
}

func TestDirectoryPreservesFilteredLookupFailureAndDeduplication(t *testing.T) {
	t.Parallel()
	failure := errors.New("roster unavailable")
	people := &directoryPeopleFake{err: failure}
	directory := NewDirectory(people, nil, nil, fakePrincipals{device: testDevice()}, nil)
	if _, err := directory.Students(context.Background(), nil); !errors.Is(err, failure) {
		t.Fatalf("unfiltered lookup must propagate failure: %v", err)
	}
	if _, err := directory.Teachers(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("teacher lookup must propagate failure: %v", err)
	}
	filtered, err := directory.Students(context.Background(), []int64{testStaffID})
	if err != nil || filtered == nil || len(filtered) != 0 {
		t.Fatalf("filtered lookup must retain its empty-result failure contract: %v, %v", filtered, err)
	}
	people.err = nil
	people.students = []devicescan.DirectoryStudent{
		{StudentID: testStudentID, GroupName: "first"},
		{StudentID: testStudentID, GroupName: "last"},
	}
	filtered, err = directory.Students(context.Background(), []int64{testStaffID})
	if err != nil || len(filtered) != 1 || filtered[0].GroupName != "last" {
		t.Fatalf("filtered roster must deduplicate with the last row winning: %v, %v", filtered, err)
	}
}

// The hint „Gast aus …“ (PyrePortal#499) names the school of the device's
// tenant only, and a failed read leaves the roster usable without it.
func TestDirectoryNamesHolidayGuestSchoolsAndSurvivesAFailedRead(t *testing.T) {
	t.Parallel()
	people := &directoryPeopleFake{students: []devicescan.DirectoryStudent{
		{StudentID: testStudentID, SchoolClass: "Feriengast"}, {StudentID: testStudentID + 1, SchoolClass: "2b"},
	}}
	var asked []int64
	var failure error
	guests := func(_ context.Context, schoolID int64) (map[int64]string, error) {
		asked = append(asked, schoolID)
		return map[int64]string{testStudentID: "Grundschule Melle"}, failure
	}
	device := testDevice()
	device.TenantID = 4242
	directory := NewDirectory(people, nil, guests, fakePrincipals{device: device}, nil)
	for _, filter := range [][]int64{nil, {testStaffID}} {
		students, err := directory.Students(context.Background(), filter)
		if err != nil || len(students) != 2 {
			t.Fatalf("filter %v: unexpected roster %+v, %v", filter, students, err)
		}
		hints := map[int64]string{}
		for _, student := range students {
			hints[student.StudentID] = student.HolidayGuestSchool
		}
		if hints[testStudentID] != "Grundschule Melle" || hints[testStudentID+1] != "" {
			t.Fatalf("filter %v: unexpected hints %v", filter, hints)
		}
	}
	if len(asked) != 2 || asked[0] != device.TenantID || asked[1] != device.TenantID {
		t.Fatalf("hints must be read once per roster for the device's school, got %v", asked)
	}

	failure = errors.New("hints unavailable")
	people.students = []devicescan.DirectoryStudent{{StudentID: testStudentID}, {StudentID: testStudentID + 1}}
	students, err := directory.Students(context.Background(), nil)
	if err != nil || len(students) != 2 || students[0].HolidayGuestSchool != "" || students[1].HolidayGuestSchool != "" {
		t.Fatalf("a failed hint read must keep the roster without hints: %+v, %v", students, err)
	}

	people.students = nil
	asked = nil
	if _, err := directory.Students(context.Background(), nil); err != nil || len(asked) != 0 {
		t.Fatalf("an empty roster reads no hints: asked=%v, err=%v", asked, err)
	}
}
