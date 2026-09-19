package displayorder

import "testing"

// The display order is what a class list reads like: the grade leads and
// sorts numerically, then the names in German dictionary order, then the ID
// so equal names never swap between two listings.
func TestCompareClassListEntryOrdersClassThenGermanName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b ClassListEntry
		want int
	}{
		{"grade 2 before grade 10", entry("2a", "Zander", "Zoe", 2), entry("10a", "Berg", "Ben", 3), -1},
		{"Ä sorts with A", entry("2a", "Ärger", "Anna", 1), entry("2a", "Zander", "Zoe", 2), -1},
		{"first name breaks the tie", entry("2a", "Berg", "Anna", 9), entry("2a", "Berg", "Ben", 1), -1},
		{"equal names fall back to the ID", entry("2a", "Berg", "Ben", 1), entry("2a", "Berg", "Ben", 2), -1},
		{"identical rows compare equal", entry("2a", "Berg", "Ben", 1), entry("2a", "Berg", "Ben", 1), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := compare(tt.a, tt.b)
			if sign(got) != tt.want {
				t.Fatalf("compare(%v, %v) = %d, want sign %d", tt.a, tt.b, got, tt.want)
			}
			if back := compare(tt.b, tt.a); sign(back) != -tt.want {
				t.Fatalf("compare is not antisymmetric: %d and %d", got, back)
			}
		})
	}
}

func entry(class, last, first string, id int64) ClassListEntry {
	return ClassListEntry{SchoolClass: class, LastName: last, FirstName: first, ID: id}
}

func compare(a, b ClassListEntry) int { return CompareClassListEntry(a, b) }

func sign(value int) int {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	}
	return 0
}
