package collation

import "testing"

func TestSchoolClassLabel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		stored string
		want   string
	}{
		// #5101: the four spellings from the issue.
		{"1b", "Klasse 1b"},
		{"Klasse 1b", "Klasse 1b"},
		{"klasse 1b", "Klasse 1b"},
		{" 1b ", "Klasse 1b"},
		{" kLaSsE 3a ", "Klasse 3a"},
		{"Klasse1b", "Klasse 1b"},
		{"Klasse  1b", "Klasse 1b"},
		{"1. Klasse", "1. Klasse"},
		{"5 b/c", "Klasse 5 b/c"},
		{"Klassenfahrt", "Klasse Klassenfahrt"},
		{"Eulen", "Klasse Eulen"},
		{"Klasse", "Klasse"},
		{"", ""},
		{"   ", ""},
	}
	for _, tc := range cases {
		if got := SchoolClassLabel(tc.stored); got != tc.want {
			t.Errorf("SchoolClassLabel(%q) = %q, want %q", tc.stored, got, tc.want)
		}
	}
}
