package httpadapter

import (
	"strings"
	"testing"
)

// The Document Rendering column ids the room snapshot fills. The literals
// pin the column contract of the rendered list.
const (
	columnRoomName       = "room_name"
	columnRoomChildCount = "room_child_count"
	columnChecklist      = "checklist"
	columnStudentName    = "student_name"
)

func TestBuildRoomSnapshotRowsGroupsLocationsAndStudents(t *testing.T) {
	t.Parallel()

	locations := []roomSnapshotLocation{
		{
			Name:        "Raum 101",
			Status:      "Belegt",
			Building:    "Stadthaus",
			Floor:       "Etage 2",
			Activity:    "Freispiel",
			Supervision: "Petra Huber",
			ChildCount:  2,
			StudentIDs:  []int64{43, 42},
		},
		{
			Name:       "Unterwegs",
			Status:     "Unterwegs",
			ChildCount: 0,
			IsTransit:  true,
		},
	}
	students := map[int64]roomSnapshotStudent{
		42: {ID: 42, Name: "Mila Albrecht", SchoolClass: "Klasse 3b", GroupName: "Regenbogengruppe"},
		43: {ID: 43, Name: "Kevin Anders", SchoolClass: "Klasse 3a", GroupName: "Wiesengruppe"},
	}

	rows := buildRoomSnapshotRows(locations, students)

	if len(rows) != 3 {
		t.Fatalf("rows len = %d, want 3", len(rows))
	}
	first := rows[0].Values
	if first[columnRoomName] != "Raum 101" {
		t.Fatalf("room name = %q", first[columnRoomName])
	}
	if first[columnRoomChildCount] != "2" {
		t.Fatalf("child count = %q", first[columnRoomChildCount])
	}
	if first[columnStudentName] != "Kevin Anders" {
		t.Fatalf("first student row = %q", first[columnStudentName])
	}
	if first[columnChecklist] != "" {
		t.Fatalf("checklist marker = %q", first[columnChecklist])
	}
	if rows[1].Values[columnRoomName] != "Raum 101" {
		t.Fatalf("second room name = %q", rows[1].Values[columnRoomName])
	}
	if rows[2].Values[columnRoomName] != "Unterwegs" {
		t.Fatalf("empty location row = %q", rows[2].Values[columnRoomName])
	}
	if rows[2].Values[columnStudentName] != "" {
		t.Fatalf("empty location student = %q", rows[2].Values[columnStudentName])
	}
}

func TestRoomSnapshotSubtitleIncludesTransit(t *testing.T) {
	t.Parallel()

	subtitle := roomSnapshotSubtitle([]roomSnapshotLocation{
		{Name: "Raum 101", ChildCount: 12},
		{Name: "Unterwegs", ChildCount: 3, IsTransit: true},
	})

	if !strings.Contains(subtitle, "1 Räume plus Unterwegs") {
		t.Fatalf("subtitle = %q", subtitle)
	}
	if !strings.Contains(subtitle, "15 Kinder") {
		t.Fatalf("subtitle = %q", subtitle)
	}
}

func TestBuildRoomSnapshotRowsGermanNameOrder(t *testing.T) {
	t.Parallel()

	locations := []roomSnapshotLocation{
		{Name: "Raum 101", StudentIDs: []int64{1, 2, 3, 4}},
	}
	students := map[int64]roomSnapshotStudent{
		1: {ID: 1, Name: "Jan Zimmermann"},
		2: {ID: 2, Name: "Emre Özdemir"},
		3: {ID: 3, Name: "lena ärmel"},
		4: {ID: 4, Name: "Ben Anders"},
	}

	rows := buildRoomSnapshotRows(locations, students)

	want := []string{"Ben Anders", "Emre Özdemir", "Jan Zimmermann", "lena ärmel"}
	if len(rows) != len(want) {
		t.Fatalf("row count = %d, want %d", len(rows), len(want))
	}
	for i, name := range want {
		if got := rows[i].Values[columnStudentName]; got != name {
			t.Fatalf("row %d student = %q, want %q", i, got, name)
		}
	}
}
