package collation

import (
	"sort"
	"sync"
	"testing"
)

func TestCompareGermanUmlautsGroupWithBaseLetters(t *testing.T) {
	// DIN 5007-1 dictionary order: ä/ö/ü sort with a/o/u, not after z.
	ordered := []string{
		"Anders",
		"Ärmel",
		"Becker",
		"Nowak",
		"Özdemir",
		"Pauli",
		"Übel",
		"Vogel",
		"Zimmermann",
	}
	for i := 0; i < len(ordered)-1; i++ {
		if CompareGerman(ordered[i], ordered[i+1]) >= 0 {
			t.Errorf("expected %q < %q", ordered[i], ordered[i+1])
		}
	}
}

func TestCompareGermanIsCaseInsensitive(t *testing.T) {
	if got := CompareGerman("müller", "Müller"); got != 0 {
		t.Errorf("CompareGerman(müller, Müller) = %d, want 0", got)
	}
	// A lowercase name must not sort after all uppercase names.
	if CompareGerman("ahrens", "Becker") >= 0 {
		t.Error("expected ahrens < Becker regardless of case")
	}
}

func TestCompareGermanKeepsDiacriticsSignificant(t *testing.T) {
	// Dictionary order (not phone-book): Mueller < Müller, and they are
	// distinct — IgnoreCase must not collapse diacritics like Loose would.
	if CompareGerman("Mueller", "Müller") >= 0 {
		t.Error("expected Mueller < Müller under DIN 5007-1")
	}
	if CompareGerman("Muller", "Müller") == 0 {
		t.Error("expected Muller != Müller (diacritics significant)")
	}
}

func TestCompareGermanNames(t *testing.T) {
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
			got := CompareGermanNames(tt.aLast, tt.aFirst, tt.bLast, tt.bFirst)
			if got != tt.want {
				t.Errorf("CompareGermanNames(%q,%q,%q,%q) = %d, want %d",
					tt.aLast, tt.aFirst, tt.bLast, tt.bFirst, got, tt.want)
			}
		})
	}
}

func TestCompareGermanSortIntegration(t *testing.T) {
	names := []string{"Zimmermann", "Özdemir", "ahrens", "Ärmel", "Müller", "Mueller"}
	sort.SliceStable(names, func(i, j int) bool {
		return CompareGerman(names[i], names[j]) < 0
	})
	want := []string{"ahrens", "Ärmel", "Mueller", "Müller", "Özdemir", "Zimmermann"}
	for i, name := range want {
		if names[i] != name {
			t.Fatalf("sorted order = %v, want %v", names, want)
		}
	}
}

func TestCompareGermanConcurrentUse(t *testing.T) {
	// The pool must make concurrent comparisons safe; run with -race.
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				if CompareGerman("Ärmel", "Becker") >= 0 {
					t.Error("expected Ärmel < Becker")
					return
				}
				if CompareGermanNames("Müller", "Anna", "Müller", "Ben") >= 0 {
					t.Error("expected Anna < Ben on equal last name")
					return
				}
			}
		}()
	}
	wg.Wait()
}
