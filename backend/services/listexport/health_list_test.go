package listexport

import (
	"testing"
)

func columnIDs(columns []Column) []ColumnID {
	ids := make([]ColumnID, 0, len(columns))
	for _, column := range columns {
		ids = append(ids, column.ID)
	}
	return ids
}

func equalIDs(a, b []ColumnID) bool {
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
		ids  []ColumnID
		want []ColumnID
	}{
		{
			name: "empty request prints every column",
			ids:  nil,
			want: []ColumnID{ColumnName, ColumnSchoolClass, ColumnGroup, ColumnHealthInfo},
		},
		{
			name: "class and group can be dropped, name and note cannot",
			ids:  []ColumnID{ColumnGroup},
			want: []ColumnID{ColumnName, ColumnGroup, ColumnHealthInfo},
		},
		{
			name: "columns foreign to the list are ignored",
			ids:  []ColumnID{ColumnBirthday, ColumnContactPhone, ColumnSchoolClass},
			want: []ColumnID{ColumnName, ColumnSchoolClass, ColumnHealthInfo},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := columnIDs(ResolveColumns(tt.ids, PresetHealthList))
			if !equalIDs(got, tt.want) {
				t.Fatalf("columns = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHealthListColumnLabel(t *testing.T) {
	t.Parallel()

	columns := HealthListColumns(nil)
	last := columns[len(columns)-1]
	if last.ID != ColumnHealthInfo || last.Label != "Gesundheitsinformationen" {
		t.Fatalf("last column = %+v, want the health column labelled Gesundheitsinformationen", last)
	}
}

// Every other preset keeps the health column out, even when a caller asks
// for it by id.
func TestOtherPresetsNeverResolveHealthColumn(t *testing.T) {
	t.Parallel()

	for _, preset := range []Preset{
		PresetOGSWeekly, PresetOGSCompact, PresetClassRoster, PresetDailyPlanning,
		PresetAttendanceSnapshot, PresetPickupList, PresetBlankChecklist, PresetBirthdayList,
	} {
		for _, column := range ResolveColumns([]ColumnID{ColumnName, ColumnHealthInfo}, preset) {
			if column.ID == ColumnHealthInfo {
				t.Fatalf("preset %q resolved the health column", preset)
			}
		}
	}
}
