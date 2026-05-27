package rooms

import (
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/services/listexport"
)

func TestBuildRoomSnapshotRowsGroupsLocationsAndStudents(t *testing.T) {
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
	if first[listexport.ColumnRoomName] != "Raum 101" {
		t.Fatalf("room name = %q", first[listexport.ColumnRoomName])
	}
	if first[listexport.ColumnRoomChildCount] != "2" {
		t.Fatalf("child count = %q", first[listexport.ColumnRoomChildCount])
	}
	if first[listexport.ColumnStudentName] != "Kevin Anders" {
		t.Fatalf("first student row = %q", first[listexport.ColumnStudentName])
	}
	if first[listexport.ColumnChecklist] != "" {
		t.Fatalf("checklist marker = %q", first[listexport.ColumnChecklist])
	}
	if rows[1].Values[listexport.ColumnRoomName] != "Raum 101" {
		t.Fatalf("second room name = %q", rows[1].Values[listexport.ColumnRoomName])
	}
	if rows[2].Values[listexport.ColumnRoomName] != "Unterwegs" {
		t.Fatalf("empty location row = %q", rows[2].Values[listexport.ColumnRoomName])
	}
	if rows[2].Values[listexport.ColumnStudentName] != "" {
		t.Fatalf("empty location student = %q", rows[2].Values[listexport.ColumnStudentName])
	}
}

func TestRoomSnapshotSubtitleIncludesTransit(t *testing.T) {
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
