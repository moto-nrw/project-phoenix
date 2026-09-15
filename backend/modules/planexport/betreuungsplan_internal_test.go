package planexport

import (
	"context"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/services/listexport"
)

func TestBetreuungsplanRendersBlockRoomStaffAndChildren(t *testing.T) {
	t.Parallel()

	service, renderer := newBetreuungsplanService(
		[]*Instance{instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)},
		[]*InstanceStaff{instanceStaff(11, 7), instanceStaff(11, 8)},
		map[int64]int{11: 24},
	)

	if _, err := service.ExportBetreuungsplan(context.Background(), careParams()); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}

	cell := cellFor(t, renderer.doc, "Mensa", listexport.ColumnPlanMonday)
	want := "12:00–13:00 · Speisesaal\nKessener, F., Müller, A.\n24 Kinder"
	if cell != want {
		t.Fatalf("cell = %q, want %q", cell, want)
	}
}

func TestBetreuungsplanShowsStaffRoomOverride(t *testing.T) {
	t.Parallel()

	overridden := instanceStaff(11, 8)
	overridden.RoomID = ptr(int64(4))
	service, renderer := newBetreuungsplanService(
		[]*Instance{instance(11, monday, clock(12, 0), clock(13, 0), "Lernzeit", 3)},
		[]*InstanceStaff{instanceStaff(11, 7), overridden}, nil,
	)

	if _, err := service.ExportBetreuungsplan(context.Background(), careParams()); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}
	cell := cellFor(t, renderer.doc, "Lernzeit", listexport.ColumnPlanMonday)
	if !strings.Contains(cell, "Müller, A. (Gruppenraum 1)") {
		t.Fatalf("cell = %q, want the staff room override", cell)
	}
}

// A block nobody is assigned to yet must still print — this is exactly the
// case StaffScheduleOverview cannot see, which is why the care plan reads
// the instances directly.
func TestBetreuungsplanShowsBlockWithoutStaff(t *testing.T) {
	t.Parallel()

	service, renderer := newBetreuungsplanService(
		[]*Instance{instance(11, monday, clock(14, 30), clock(15, 30), "Lernzeit", 4)},
		nil,
		nil,
	)

	if _, err := service.ExportBetreuungsplan(context.Background(), careParams()); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}
	cell := cellFor(t, renderer.doc, "Lernzeit", listexport.ColumnPlanMonday)
	if !strings.Contains(cell, "Kein Personal eingeteilt") {
		t.Fatalf("cell = %q, want the unstaffed block stated", cell)
	}
}

// A cancelled block stays on the sheet — "entfällt" is what the reader came
// for — but its reason is internal.
func TestBetreuungsplanCancelledBlockHidesReasonOnNotice(t *testing.T) {
	t.Parallel()

	cancelled := instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)
	cancelled.Cancelled = true
	cancelled.CancelReason = ptr("Personalmangel")

	for _, tc := range []struct {
		variant    Variant
		wantReason bool
	}{
		{VariantNotice, false},
		{VariantInternal, true},
	} {
		t.Run(string(tc.variant), func(t *testing.T) {
			service, renderer := newBetreuungsplanService(
				[]*Instance{cancelled}, nil, nil)
			params := careParams()
			params.Variant = tc.variant
			if _, err := service.ExportBetreuungsplan(context.Background(), params); err != nil {
				t.Fatalf("ExportBetreuungsplan: %v", err)
			}

			cell := cellFor(t, renderer.doc, "Mensa", listexport.ColumnPlanMonday)
			if !strings.Contains(cell, "entfällt") {
				t.Fatalf("cell = %q, want the cancellation stated", cell)
			}
			if got := strings.Contains(cell, "Personalmangel"); got != tc.wantReason {
				t.Fatalf("cell = %q, reason present = %v, want %v", cell, got, tc.wantReason)
			}
		})
	}
}

// Absent staff are not at the block: the wall sheet drops them, the internal
// one marks them.
func TestBetreuungsplanAbsentStaffOnlyMarkedInternally(t *testing.T) {
	t.Parallel()

	absent := instanceStaff(11, 8)
	absent.IsAbsent = true

	service, renderer := newBetreuungsplanService(
		[]*Instance{instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)},
		[]*InstanceStaff{instanceStaff(11, 7), absent},
		nil,
	)
	if _, err := service.ExportBetreuungsplan(context.Background(), careParams()); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}
	if cell := cellFor(t, renderer.doc, "Mensa", listexport.ColumnPlanMonday); strings.Contains(cell, "Müller") {
		t.Fatalf("notice cell = %q, want the absent staff member left out", cell)
	}

	service, renderer = newBetreuungsplanService(
		[]*Instance{instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)},
		[]*InstanceStaff{instanceStaff(11, 7), absent},
		nil,
	)
	params := careParams()
	params.Variant = VariantInternal
	if _, err := service.ExportBetreuungsplan(context.Background(), params); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}
	if cell := cellFor(t, renderer.doc, "Mensa", listexport.ColumnPlanMonday); !strings.Contains(cell, "Müller, A. (abwesend)") {
		t.Fatalf("internal cell = %q, want the absence marked", cell)
	}
}

// Rows read down the day, the way the hand-kept Excel sheets do.
func TestBetreuungsplanOrdersOfferingsByEarliestStart(t *testing.T) {
	t.Parallel()

	service, renderer := newBetreuungsplanService(
		[]*Instance{
			instance(12, monday, clock(14, 30), clock(15, 30), "Lernzeit", 4),
			instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3),
			instance(13, tuesday, clock(7, 30), clock(8, 0), "Randstunde", 4),
		},
		nil,
		nil,
	)
	if _, err := service.ExportBetreuungsplan(context.Background(), careParams()); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}
	labels := rowLabels(renderer.doc)
	want := []string{"Randstunde", "Mensa", "Lernzeit"}
	if len(labels) != len(want) {
		t.Fatalf("rows = %v, want %v", labels, want)
	}
	for i, label := range want {
		if labels[i] != label {
			t.Fatalf("rows = %v, want %v", labels, want)
		}
	}
}

// Angebot names are not unique. Two separately configured Angebote that
// happen to share a name are two rows, or the sheet prints one row that is
// neither of them.
func TestBetreuungsplanKeepsSameNamedOfferingsApart(t *testing.T) {
	t.Parallel()

	first := instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)
	first.ActivityGroupID = ptr(int64(41))
	second := instance(12, monday, clock(13, 0), clock(14, 0), "Mensa", 4)
	second.ActivityGroupID = ptr(int64(42))

	service, renderer := newBetreuungsplanService(
		[]*Instance{first, second}, nil, nil)
	if _, err := service.ExportBetreuungsplan(context.Background(), careParams()); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}

	labels := rowLabels(renderer.doc)
	if len(labels) != 2 || labels[0] != "Mensa" || labels[1] != "Mensa" {
		t.Fatalf("rows = %v, want two Mensa rows", labels)
	}
	cells := []string{
		listexport.StripStyleMarkers(renderer.doc.Rows[0].Values[listexport.ColumnPlanMonday]),
		listexport.StripStyleMarkers(renderer.doc.Rows[1].Values[listexport.ColumnPlanMonday]),
	}
	if !strings.Contains(cells[0], "12:00–13:00") || strings.Contains(cells[0], "13:00–14:00") {
		t.Fatalf("first cell = %q, want only the first Angebot", cells[0])
	}
	if !strings.Contains(cells[1], "13:00–14:00") || strings.Contains(cells[1], "12:00–13:00") {
		t.Fatalf("second cell = %q, want only the second Angebot", cells[1])
	}
}

// A spontaneous block has no Angebot to be identified by, so its repeats
// across the week still collect into one row.
func TestBetreuungsplanGroupsSpontaneousBlocksByTitle(t *testing.T) {
	t.Parallel()

	service, renderer := newBetreuungsplanService(
		[]*Instance{
			instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3),
			instance(12, tuesday, clock(12, 0), clock(13, 0), "Mensa", 3),
		}, nil, nil)
	if _, err := service.ExportBetreuungsplan(context.Background(), careParams()); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}
	if labels := rowLabels(renderer.doc); len(labels) != 1 || labels[0] != "Mensa" {
		t.Fatalf("rows = %v, want a single Mensa row", labels)
	}
}

// What somebody wrote about the day belongs to the team, not to the wall.
func TestBetreuungsplanNoteOnlyOnInternalSheet(t *testing.T) {
	t.Parallel()

	noted := instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)
	noted.Notes = ptr("Zweite Gruppe isst später")

	for _, tc := range []struct {
		variant  Variant
		wantNote bool
	}{
		{VariantNotice, false},
		{VariantInternal, true},
	} {
		t.Run(string(tc.variant), func(t *testing.T) {
			service, renderer := newBetreuungsplanService(
				[]*Instance{noted}, nil, nil)
			params := careParams()
			params.Variant = tc.variant
			if _, err := service.ExportBetreuungsplan(context.Background(), params); err != nil {
				t.Fatalf("ExportBetreuungsplan: %v", err)
			}
			cell := cellFor(t, renderer.doc, "Mensa", listexport.ColumnPlanMonday)
			if got := strings.Contains(cell, "Zweite Gruppe isst später"); got != tc.wantNote {
				t.Fatalf("cell = %q, note present = %v, want %v", cell, got, tc.wantNote)
			}
		})
	}
}

// A cancelled block keeps its note too — the internal sheet is where anyone
// finds out what actually happened that day.
func TestBetreuungsplanCancelledBlockKeepsNoteInternally(t *testing.T) {
	t.Parallel()

	cancelled := instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)
	cancelled.Cancelled = true
	cancelled.Notes = ptr("Ersatz in Gruppenraum 1")

	service, renderer := newBetreuungsplanService(
		[]*Instance{cancelled}, nil, nil)
	params := careParams()
	params.Variant = VariantInternal
	if _, err := service.ExportBetreuungsplan(context.Background(), params); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}
	cell := cellFor(t, renderer.doc, "Mensa", listexport.ColumnPlanMonday)
	if !strings.Contains(cell, "Ersatz in Gruppenraum 1") {
		t.Fatalf("cell = %q, want the note kept on the cancelled block", cell)
	}
}

// The care plan accepts only its own template; asking for the staff layout
// is a client error, not a silently different document.
func TestBetreuungsplanRejectsForeignTemplate(t *testing.T) {
	t.Parallel()

	service, _ := newBetreuungsplanService(nil, nil, nil)
	params := careParams()
	params.Template = TemplateByPerson
	if _, err := service.ExportBetreuungsplan(context.Background(), params); err == nil {
		t.Fatal("expected the staff template to be refused")
	}
}
