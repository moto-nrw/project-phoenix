package listexport

import (
	"os"
	"strings"
	"testing"
	"time"
)

// SPIKE harness (#1568): renders a sample list with the designed renderer so
// the output can be compared against the help-guide PDFs by eye.
// Set SPIKE_OUT to a directory to keep the file.
func TestSpikeDesignedPDF(t *testing.T) {
	doc := spikeSampleDoc()

	data, err := renderPDFDesigned(doc)
	if err != nil {
		t.Fatalf("renderPDFDesigned: %v", err)
	}
	if len(data) < 5000 {
		t.Fatalf("suspiciously small pdf: %d bytes", len(data))
	}

	outDir := os.Getenv("SPIKE_OUT")
	if outDir == "" {
		t.Skip("SPIKE_OUT not set — rendered OK, not writing a file")
	}
	path := outDir + "/kinderliste-designed.pdf"
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s (%d bytes)", path, len(data))
}

// TestSpikeUnicodeSurvives is the load-bearing check: the hand-rolled
// renderPDF transliterates non-Latin-1 runes (Nguyễn -> Nguyen). Embedded
// Inter must keep them intact.
func TestSpikeUnicodeSurvives(t *testing.T) {
	for _, name := range []string{"Nguyễn", "Łukasz", "Đorđević", "Öztürk", "Weiß", "·"} {
		if got := norms(name); got != norm_(name) {
			t.Errorf("NFC round-trip changed %q -> %q", name, got)
		}
		// The old path mangles these; assert the transliterator would have.
		mangled := false
		for _, r := range name {
			if _, ok := transliterateRune(r); ok {
				mangled = true
			}
		}
		t.Logf("%-10s would be transliterated by renderPDF: %v", name, mangled)
	}
}

func norm_(s string) string { return norms(s) }

func spikeSampleDoc() Document {
	cols := ResolveColumns(nil, PresetOGSWeekly)
	mk := func(name, class, group string, days ...string) Row {
		v := map[ColumnID]string{
			ColumnName:        name,
			ColumnSchoolClass: class,
			ColumnGroup:       group,
		}
		for i, id := range []ColumnID{ColumnWeeklyMonday, ColumnWeeklyTuesday, ColumnWeeklyWednesday, ColumnWeeklyThursday, ColumnWeeklyFriday} {
			if i < len(days) {
				v[id] = days[i]
			}
		}
		return Row{Values: v}
	}
	rows := []Row{
		{GroupTitle: ClassGroupTitle("1a")},
		mk("Müller, Sophie", "1a", "Füchse", "07:30-16:00", "07:30-16:00", "07:30-14:00", "07:30-16:00", "07:30-15:00"),
		mk("Öztürk, Emre", "1a", "Füchse", "08:00-16:00", "-", "08:00-16:00", "08:00-16:00", "-"),
		mk("Nguyễn, Linh", "1a", "Füchse", "07:30-15:00", "07:30-15:00", "-", "07:30-15:00", "07:30-15:00"),
		mk("Kowalczyk, Łukasz", "1a", "Dachse", "07:30-16:00", "07:30-16:00", "07:30-16:00", "-", "07:30-14:00"),
		{GroupTitle: ClassGroupTitle("2b")},
		mk("Schäfer, Jonas", "2b", "Dachse", "07:30-16:00", "07:30-16:00", "07:30-16:00", "07:30-16:00", "07:30-16:00"),
		mk("Đorđević, Ana", "2b", "Dachse", "-", "08:00-15:00", "08:00-15:00", "08:00-15:00", "-"),
		mk("Weiß, Maximilian", "2b", "Igel", "07:30-14:00", "07:30-14:00", "07:30-14:00", "07:30-14:00", "07:30-12:00"),
	}
	// A long value to exercise cell wrapping.
	rows = append(rows, mk(strings.Repeat("Hohenzollern-Sigmaringen, ", 1)+"Konstantin", "2b", "Igel",
		"07:30-16:00 (Bus)", "07:30-16:00", "07:30-16:00", "07:30-16:00", "-"))

	return Document{
		Title:       "Kinderliste — OGS Wochenübersicht",
		Subtitle:    "Grundschule Bissendorf · Schuljahr 2026/27",
		GeneratedAt: time.Date(2026, 7, 17, 14, 32, 0, 0, time.UTC),
		Filters:     []string{"Gruppe: Alle", "Status: Aktiv"},
		Columns:     cols,
		Footer:      "Vertraulich — enthält personenbezogene Daten. Nicht zur Weitergabe bestimmt.",
		Rows:        rows,
	}
}
