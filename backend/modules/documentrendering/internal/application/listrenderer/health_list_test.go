package listexport

import (
	"testing"

	listrecords "github.com/moto-nrw/project-phoenix/modules/documentrendering/records"
)

func columnIDs(columns []listrecords.Column) []listrecords.ColumnID {
	ids := make([]listrecords.ColumnID, 0, len(columns))
	for _, column := range columns {
		ids = append(ids, column.ID)
	}
	return ids
}

func equalIDs(a, b []listrecords.ColumnID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// The Gesundheitsliste (#3323) is the only child list that resolves the health
// column, and it always prints it next to the name.
func TestHealthListColumns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ids  []listrecords.ColumnID
		want []listrecords.ColumnID
	}{
		{
			name: "empty request prints every column",
			ids:  nil,
			want: []listrecords.ColumnID{listrecords.ColumnName, listrecords.ColumnSchoolClass, listrecords.ColumnGroup, listrecords.ColumnHealthInfo},
		},
		{
			name: "class and group can be dropped, name and note cannot",
			ids:  []listrecords.ColumnID{listrecords.ColumnGroup},
			want: []listrecords.ColumnID{listrecords.ColumnName, listrecords.ColumnGroup, listrecords.ColumnHealthInfo},
		},
		{
			name: "columns foreign to the list are ignored",
			ids:  []listrecords.ColumnID{listrecords.ColumnBirthday, listrecords.ColumnContactPhone, listrecords.ColumnSchoolClass},
			want: []listrecords.ColumnID{listrecords.ColumnName, listrecords.ColumnSchoolClass, listrecords.ColumnHealthInfo},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := columnIDs(listrecords.ResolveColumns(tt.ids, listrecords.PresetHealthList))
			if !equalIDs(got, tt.want) {
				t.Fatalf("columns = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHealthListColumnLabel(t *testing.T) {
	t.Parallel()

	columns := listrecords.HealthListColumns(nil)
	last := columns[len(columns)-1]
	if last.ID != listrecords.ColumnHealthInfo || last.Label != "Gesundheitsinformationen" {
		t.Fatalf("last column = %+v, want the health column labelled Gesundheitsinformationen", last)
	}
}

// Every other preset keeps the health column out, even when a caller asks
// for it by id.
func TestOtherPresetsNeverResolveHealthColumn(t *testing.T) {
	t.Parallel()

	for _, preset := range []listrecords.Preset{
		listrecords.PresetOGSWeekly, listrecords.PresetOGSCompact, listrecords.PresetClassRoster, listrecords.PresetDailyPlanning,
		listrecords.PresetAttendanceSnapshot, listrecords.PresetPickupList, listrecords.PresetBlankChecklist, listrecords.PresetBirthdayList,
	} {
		for _, column := range listrecords.ResolveColumns([]listrecords.ColumnID{listrecords.ColumnName, listrecords.ColumnHealthInfo}, preset) {
			if column.ID == listrecords.ColumnHealthInfo {
				t.Fatalf("preset %q resolved the health column", preset)
			}
		}
	}
}
