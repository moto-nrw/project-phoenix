package collation

import (
	"sort"
	"testing"
)

func TestCompareSchoolClasses(t *testing.T) {
	t.Parallel()

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

func TestCompareSchoolClassesKlassePrefixSortsNumerically(t *testing.T) {
	t.Parallel()

	classes := []string{"Klasse 10a", "3a", "Klassenfahrt", "Klasse 2a", "1b", "klasse 4b"}
	sort.SliceStable(classes, func(i, j int) bool {
		return CompareSchoolClasses(classes[i], classes[j]) < 0
	})
	// "Klasse N…" interleaves numerically with bare grades; labels without
	// a grade after the prefix ("Klassenfahrt") stay string-collated.
	want := []string{"1b", "Klasse 2a", "3a", "klasse 4b", "Klasse 10a", "Klassenfahrt"}
	for i, w := range want {
		if classes[i] != w {
			t.Fatalf("position %d: got %q, want %q (full: %v)", i, classes[i], w, classes)
		}
	}
}

func TestCompareSchoolClassesLogicalClassEquivalence(t *testing.T) {
	t.Parallel()

	// Grouped exports treat comparator equality as "same logical class";
	// pin the variants that must merge into one section.
	equal := [][2]string{
		{"1a", "1A"},
		{"1 a", "1a"},
		{"Klasse 2a", "2a"},
		{"klasse 2A", "Klasse 2a"},
	}
	for _, c := range equal {
		if got := CompareSchoolClasses(c[0], c[1]); got != 0 {
			t.Errorf("CompareSchoolClasses(%q, %q) = %d, want 0", c[0], c[1], got)
		}
	}
	if CompareSchoolClasses("1a", "1b") == 0 {
		t.Error("distinct classes must not compare equal")
	}
}

func TestCompareSchoolClassesPrefixVariantsStayAdjacentAmongMixedLabels(t *testing.T) {
	t.Parallel()

	// Regression: "2a" and "Klasse 2a" compare equal, so with non-grade
	// labels ("Eulen") in the mix they must still end up adjacent — the
	// old fallback compared originals across forms ("2a" < "Eulen" <
	// "Klasse 2a"), which broke the ordering contract and could split one
	// logical class into duplicate grouped-export sections.
	classes := []string{"Eulen", "2a", "Klasse 2a", "10b", "Füchse"}
	sort.SliceStable(classes, func(i, j int) bool {
		return CompareSchoolClasses(classes[i], classes[j]) < 0
	})
	want := []string{"2a", "Klasse 2a", "10b", "Eulen", "Füchse"}
	for i, w := range want {
		if classes[i] != w {
			t.Fatalf("position %d: got %q, want %q (full: %v)", i, classes[i], w, classes)
		}
	}
}

func TestCompareSchoolClassesTransitivity(t *testing.T) {
	t.Parallel()

	labels := []string{"2a", "Klasse 2a", "Eulen", "1b", "Klasse 10a", "Klassenfahrt", "Übergang", "2 c", ""}
	for _, x := range labels {
		for _, y := range labels {
			for _, z := range labels {
				xy := CompareSchoolClasses(x, y)
				yz := CompareSchoolClasses(y, z)
				xz := CompareSchoolClasses(x, z)
				if xy <= 0 && yz <= 0 && xz > 0 {
					t.Errorf("not transitive: %q <= %q <= %q but cmp(%q, %q) = %d", x, y, z, x, z, xz)
				}
				if xy == 0 && yz == 0 && xz != 0 {
					t.Errorf("equality not transitive: %q == %q == %q but cmp(%q, %q) = %d", x, y, z, x, z, xz)
				}
			}
		}
	}
}

func TestCompareSchoolClassesSymmetry(t *testing.T) {
	t.Parallel()

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
