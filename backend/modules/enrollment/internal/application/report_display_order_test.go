package application

import (
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The report display order is a local copy of internal/collation. These cases
// mirror internal/collation's tests so the copy provably sorts the same way.

func TestCompareGermanMatchesCollation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b string
		want func(int) bool
	}{
		// DIN 5007-1 dictionary order: ä/ö/ü sort with a/o/u, not after z.
		{"Anders before Ärmel", "Anders", "Ärmel", isNegative},
		{"Ärmel before Becker", "Ärmel", "Becker", isNegative},
		{"Becker before Nowak", "Becker", "Nowak", isNegative},
		{"Nowak before Özdemir", "Nowak", "Özdemir", isNegative},
		{"Özdemir before Pauli", "Özdemir", "Pauli", isNegative},
		{"Pauli before Übel", "Pauli", "Übel", isNegative},
		{"Übel before Vogel", "Übel", "Vogel", isNegative},
		{"Vogel before Zimmermann", "Vogel", "Zimmermann", isNegative},
		// Case-insensitive; a lowercase name does not sort after uppercase ones.
		{"case ignored", "müller", "Müller", isZero},
		{"lowercase not last", "ahrens", "Becker", isNegative},
		// Dictionary order, not phone-book: diacritics stay significant.
		{"Mueller before Müller", "Mueller", "Müller", isNegative},
		{"Muller differs from Müller", "Muller", "Müller", isNonZero},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := compareGerman(tt.a, tt.b)
			assert.True(t, tt.want(got), "compareGerman(%q, %q) = %d", tt.a, tt.b, got)
		})
	}
}

func TestCompareGermanSortMatchesCollation(t *testing.T) {
	t.Parallel()

	names := []string{"Zimmermann", "Özdemir", "ahrens", "Ärmel", "Müller", "Mueller"}
	sort.SliceStable(names, func(i, j int) bool {
		return compareGerman(names[i], names[j]) < 0
	})
	assert.Equal(t, []string{"ahrens", "Ärmel", "Mueller", "Müller", "Özdemir", "Zimmermann"}, names)
}

func TestCompareGermanConcurrentUse(t *testing.T) {
	t.Parallel()

	// The pool must make concurrent comparisons safe; run with -race.
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				if compareGerman("Ärmel", "Becker") >= 0 {
					t.Error("expected Ärmel < Becker")
					return
				}
				if compareGermanNames("Müller", "Anna", "Müller", "Ben") >= 0 {
					t.Error("expected Anna < Ben on equal last name")
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestCompareGermanNamesMatchesCollation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                         string
		aLast, aFirst, bLast, bFirst string
		want                         int
	}{
		{"last name decides", "Ärmel", "Zoe", "Becker", "Anna", -1},
		{"equal last, first decides", "Müller", "Anna", "Müller", "Ömer", -1},
		{"fully equal", "Müller", "Anna", "müller", "anna", 0},
		{"prefixed last names", "Uhl", "Jan", "von Berg", "Jan", -1},
		{"von Berg before Weber", "von Berg", "Jan", "Weber", "Jan", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, compareGermanNames(tt.aLast, tt.aFirst, tt.bLast, tt.bFirst))
		})
	}
}

func TestCompareSchoolClassesSortMatchesCollation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		classes []string
		want    []string
	}{
		{
			name:    "grades numerically, then names, empty last",
			classes: []string{"10a", "2a", "1b", "Übergang", "1a", "", "Eulen", "2 c"},
			want:    []string{"1a", "1b", "2a", "2 c", "10a", "Eulen", "Übergang", ""},
		},
		{
			// "Klasse N…" interleaves numerically with bare grades; labels
			// without a grade after the prefix stay string-collated.
			name:    "Klasse prefix sorts numerically",
			classes: []string{"Klasse 10a", "3a", "Klassenfahrt", "Klasse 2a", "1b", "klasse 4b"},
			want:    []string{"1b", "Klasse 2a", "3a", "klasse 4b", "Klasse 10a", "Klassenfahrt"},
		},
		{
			// "2a" and "Klasse 2a" compare equal and stay adjacent among
			// labels without a grade.
			name:    "prefix variants stay adjacent among mixed labels",
			classes: []string{"Eulen", "2a", "Klasse 2a", "10b", "Füchse"},
			want:    []string{"2a", "Klasse 2a", "10b", "Eulen", "Füchse"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			classes := append([]string(nil), tt.classes...)
			sort.SliceStable(classes, func(i, j int) bool {
				return compareSchoolClasses(classes[i], classes[j]) < 0
			})
			assert.Equal(t, tt.want, classes)
		})
	}
}

func TestCompareSchoolClassesLogicalClassEquivalenceMatchesCollation(t *testing.T) {
	t.Parallel()

	// Grouped exports treat comparator equality as "same logical class".
	for _, c := range [][2]string{
		{"1a", "1A"},
		{"1 a", "1a"},
		{"Klasse 2a", "2a"},
		{"klasse 2A", "Klasse 2a"},
	} {
		assert.Zero(t, compareSchoolClasses(c[0], c[1]), "compareSchoolClasses(%q, %q)", c[0], c[1])
	}
	assert.NotZero(t, compareSchoolClasses("1a", "1b"), "distinct classes must not compare equal")
}

func TestCompareSchoolClassesTransitivityMatchesCollation(t *testing.T) {
	t.Parallel()

	labels := []string{"2a", "Klasse 2a", "Eulen", "1b", "Klasse 10a", "Klassenfahrt", "Übergang", "2 c", ""}
	for _, x := range labels {
		for _, y := range labels {
			for _, z := range labels {
				xy := compareSchoolClasses(x, y)
				yz := compareSchoolClasses(y, z)
				xz := compareSchoolClasses(x, z)
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

func TestCompareSchoolClassesSymmetryMatchesCollation(t *testing.T) {
	t.Parallel()

	for _, c := range [][2]string{
		{"1a", "1a"},
		{"2a", "10a"},
		{"", "1a"},
		{"Eulen", "1a"},
	} {
		assert.Equal(t, -compareSchoolClasses(c[1], c[0]), compareSchoolClasses(c[0], c[1]), "compareSchoolClasses(%q, %q) not antisymmetric", c[0], c[1])
	}
	assert.Zero(t, compareSchoolClasses("1a", "1a"), "equal classes must compare 0")
}

func isNegative(v int) bool { return v < 0 }
func isZero(v int) bool     { return v == 0 }
func isNonZero(v int) bool  { return v != 0 }
