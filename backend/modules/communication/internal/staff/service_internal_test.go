package announcement

import (
	"errors"
	"testing"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

func strptr(s string) *string { return &s }
func i64ptr(v int64) *int64   { return &v }

func TestNormalizeLinkURL(t *testing.T) {
	t.Parallel()

	valid := map[string]string{
		"https://schule.example.de/ausflug":   "https://schule.example.de/ausflug",
		"  https://schule.example.de/a  ":     "https://schule.example.de/a",
		"http://schule.example.de":            "http://schule.example.de",
		"https://example.de/pfad?x=1&y=2#top": "https://example.de/pfad?x=1&y=2#top",
	}
	for raw, want := range valid {
		got, err := normalizeLinkURL(strptr(raw))
		if err != nil {
			t.Fatalf("expected %q valid, got %v", raw, err)
		}
		if got == nil || *got != want {
			t.Fatalf("expected %q normalized to %q, got %v", raw, want, got)
		}
	}

	for _, empty := range []*string{nil, strptr(""), strptr("   ")} {
		got, err := normalizeLinkURL(empty)
		if err != nil || got != nil {
			t.Fatalf("expected empty link to collapse to nil, got %v / %v", got, err)
		}
	}

	invalid := []string{
		"javascript:alert(1)",
		"ftp://example.de/datei",
		"/relativer/pfad",
		"schule.example.de", // no scheme
		"https://",          // no host
	}
	for _, raw := range invalid {
		if _, err := normalizeLinkURL(strptr(raw)); !errors.Is(err, ErrValidation) {
			t.Fatalf("expected %q to fail validation, got %v", raw, err)
		}
	}

	long := "https://example.de/" + string(make([]byte, maxLinkURLLen))
	if _, err := normalizeLinkURL(&long); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected over-long link to fail validation, got %v", err)
	}
}

func TestNormalizeInput_Valid(t *testing.T) {
	t.Parallel()

	in := Input{
		Title:    "  Ausflug am Freitag  ",
		Body:     "  Bitte Regenjacke mitgeben.  ",
		Priority: "", // defaults to info
		Targets: []TargetInput{
			{TargetType: usersModels.AnnouncementTargetSchoolAll},
			{TargetType: usersModels.AnnouncementTargetClass, RefText: strptr(" 1a ")},
			{TargetType: usersModels.AnnouncementTargetGroup, RefID: i64ptr(7)},
			{TargetType: usersModels.AnnouncementTargetPendingEnrollment},
		},
	}
	targets, err := normalizeInput(&in)
	if err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}
	if in.Title != "Ausflug am Freitag" || in.Body != "Bitte Regenjacke mitgeben." {
		t.Fatalf("expected trimmed title/body, got %q / %q", in.Title, in.Body)
	}
	if in.Priority != usersModels.ParentAnnouncementPriorityInfo {
		t.Fatalf("expected default priority info, got %q", in.Priority)
	}
	if len(targets) != 4 {
		t.Fatalf("expected 4 targets, got %d", len(targets))
	}
	// class ref_text trimmed
	for _, target := range targets {
		if target.TargetType == usersModels.AnnouncementTargetClass {
			if target.TargetRefText == nil || *target.TargetRefText != "1a" {
				t.Fatalf("expected class ref_text '1a', got %v", target.TargetRefText)
			}
		}
	}
}

func TestNormalizeInput_Errors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   Input
	}{
		{"empty title", Input{Title: "  ", Body: "x", Targets: []TargetInput{{TargetType: usersModels.AnnouncementTargetSchoolAll}}}},
		{"empty body", Input{Title: "t", Body: "  ", Targets: []TargetInput{{TargetType: usersModels.AnnouncementTargetSchoolAll}}}},
		{"body too long", Input{Title: "t", Body: "x" + string(make([]byte, maxBodyLen)), Targets: []TargetInput{{TargetType: usersModels.AnnouncementTargetSchoolAll}}}},
		{"bad priority", Input{Title: "t", Body: "b", Priority: "urgent", Targets: []TargetInput{{TargetType: usersModels.AnnouncementTargetSchoolAll}}}},
		{"no targets", Input{Title: "t", Body: "b"}},
		{"unknown target", Input{Title: "t", Body: "b", Targets: []TargetInput{{TargetType: "everyone"}}}},
		{"class without text", Input{Title: "t", Body: "b", Targets: []TargetInput{{TargetType: usersModels.AnnouncementTargetClass}}}},
		{"group without id", Input{Title: "t", Body: "b", Targets: []TargetInput{{TargetType: usersModels.AnnouncementTargetGroup}}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := c.in
			if _, err := normalizeInput(&in); !errors.Is(err, ErrValidation) {
				t.Fatalf("expected ErrValidation, got %v", err)
			}
		})
	}
}

func TestNormalizeInput_DedupesTargets(t *testing.T) {
	t.Parallel()

	in := Input{
		Title: "t", Body: "b",
		Targets: []TargetInput{
			{TargetType: usersModels.AnnouncementTargetGroup, RefID: i64ptr(3)},
			{TargetType: usersModels.AnnouncementTargetGroup, RefID: i64ptr(3)},
			{TargetType: usersModels.AnnouncementTargetSchoolAll},
			{TargetType: usersModels.AnnouncementTargetSchoolAll},
		},
	}
	targets, err := normalizeInput(&in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 deduped targets, got %d", len(targets))
	}
}

func TestNormalizeInput_SendEmailPassesThrough(t *testing.T) {
	t.Parallel()

	in := Input{
		Title: "t", Body: "b", SendEmail: true,
		Targets: []TargetInput{{TargetType: usersModels.AnnouncementTargetSchoolAll}},
	}
	if _, err := normalizeInput(&in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !in.SendEmail {
		t.Fatalf("expected send_email to remain true")
	}
}
