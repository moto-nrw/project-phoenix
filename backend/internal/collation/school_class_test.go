package collation

import (
	"sort"
	"testing"
)

func TestCompareSchoolClasses(t *testing.T) {
	classes := []string{"10a", "2a", "1b", "Übergang", "1a", "", "Eulen", "2 c"}
	sort.SliceStable(classes, func(i, j int) bool {
		return CompareSchoolClasses(classes[i], classes[j]) < 0
	})
	want := []string{"1a", "1b", "2a", "2 c", "10a", "Eulen", "Übergang", ""}
	for i, w := range want {
		if classes[i] != w {
			t.Fatalf("position %d: got %q, want %q (full: %v)", i, classes[i], w, classes)
		}
	}
}

func TestCompareSchoolClassesSymmetry(t *testing.T) {
	cases := [][2]string{
		{"1a", "1a"},
		{"2a", "10a"},
		{"", "1a"},
		{"Eulen", "1a"},
	}
	for _, c := range cases {
		if got, want := CompareSchoolClasses(c[0], c[1]), -CompareSchoolClasses(c[1], c[0]); got != want {
			t.Errorf("CompareSchoolClasses(%q, %q) = %d, reverse = %d; not antisymmetric", c[0], c[1], got, -want)
		}
	}
	if CompareSchoolClasses("1a", "1a") != 0 {
		t.Error("equal classes must compare 0")
	}
}
