package planexport

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

// In-memory readers; every id below is a struct field, not a database row.

type stubInstances struct {
	instances []*scheduleModel.ActivityInstance
}

func (s stubInstances) FindByTenantAndDateRange(_ context.Context, from, to timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	kept := make([]*scheduleModel.ActivityInstance, 0, len(s.instances))
	for _, instance := range s.instances {
		if instance.Date.Before(from) || instance.Date.After(to) {
			continue
		}
		kept = append(kept, instance)
	}
	return kept, nil
}

type stubInstanceStaff struct {
	rows []*scheduleModel.InstanceStaff
}

func (s stubInstanceStaff) FindByInstanceIDs(_ context.Context, ids []int64) ([]*scheduleModel.InstanceStaff, error) {
	wanted := make(map[int64]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	kept := make([]*scheduleModel.InstanceStaff, 0, len(s.rows))
	for _, row := range s.rows {
		if wanted[row.InstanceID] {
			kept = append(kept, row)
		}
	}
	return kept, nil
}

type stubRooms struct{ rooms []*facilitiesModel.Room }

func (s stubRooms) FindByIDs(context.Context, []int64) ([]*facilitiesModel.Room, error) {
	return s.rooms, nil
}

type stubStaffDirectory struct{ members []*usersModel.Staff }

func (s stubStaffDirectory) ListAllWithPerson(context.Context) ([]*usersModel.Staff, error) {
	return s.members, nil
}

func (s stubStaffDirectory) FindWithPersonByIDs(context.Context, []int64) (map[int64]*usersModel.Staff, error) {
	out := make(map[int64]*usersModel.Staff, len(s.members))
	for _, member := range s.members {
		out[member.ID] = member
	}
	return out, nil
}

type stubStudentCounts struct{ counts map[int64]int }

func (s stubStudentCounts) CountNonAbsentByInstanceIDs(context.Context, []int64) (map[int64]int, error) {
	return s.counts, nil
}

func instance(id int64, date timezone.Date, from, to time.Time, title string, roomID int64) *scheduleModel.ActivityInstance {
	i := &scheduleModel.ActivityInstance{
		Date:      date,
		Title:     title,
		StartTime: from,
		EndTime:   to,
		RoomID:    roomID,
		Status:    scheduleModel.InstanceStatusPlanned,
	}
	i.ID = id
	return i
}

func room(id int64, name string) *facilitiesModel.Room {
	r := &facilitiesModel.Room{Name: name}
	r.ID = id
	return r
}

func instanceStaff(instanceID, staffID int64) *scheduleModel.InstanceStaff {
	return &scheduleModel.InstanceStaff{InstanceID: instanceID, StaffID: staffID}
}

func newBetreuungsplanService(
	instances []*scheduleModel.ActivityInstance,
	staffRows []*scheduleModel.InstanceStaff,
	counts map[int64]int,
) (Service, *captureRenderer) {
	renderer := &captureRenderer{}
	service := NewService(Dependencies{
		Instances:     stubInstances{instances: instances},
		InstanceStaff: stubInstanceStaff{rows: staffRows},
		Students:      stubStudentCounts{counts: counts},
		Rooms:         stubRooms{rooms: []*facilitiesModel.Room{room(3, "Speisesaal"), room(4, "Gruppenraum 1")}},
		Staff: stubStaffDirectory{members: []*usersModel.Staff{
			staffMember(7, "Franziska", "Kessener"),
			staffMember(8, "Anna", "Müller"),
		}},
		Renderer: renderer,
	}, nil)
	return service, renderer
}

func careParams() Params {
	params := defaultParams()
	params.Template = TemplateByOffering
	return params
}

func TestBetreuungsplanRendersBlockRoomStaffAndChildren(t *testing.T) {
	service, renderer := newBetreuungsplanService(
		[]*scheduleModel.ActivityInstance{instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)},
		[]*scheduleModel.InstanceStaff{instanceStaff(11, 7), instanceStaff(11, 8)},
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
	overridden := instanceStaff(11, 8)
	overridden.RoomID = ptr(int64(4))
	service, renderer := newBetreuungsplanService(
		[]*scheduleModel.ActivityInstance{instance(11, monday, clock(12, 0), clock(13, 0), "Lernzeit", 3)},
		[]*scheduleModel.InstanceStaff{instanceStaff(11, 7), overridden}, nil,
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
	service, renderer := newBetreuungsplanService(
		[]*scheduleModel.ActivityInstance{instance(11, monday, clock(14, 30), clock(15, 30), "Lernzeit", 4)},
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
	cancelled := instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)
	cancelled.Status = scheduleModel.InstanceStatusCancelled
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
				[]*scheduleModel.ActivityInstance{cancelled}, nil, nil)
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
	absent := instanceStaff(11, 8)
	absent.IsAbsent = true

	service, renderer := newBetreuungsplanService(
		[]*scheduleModel.ActivityInstance{instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)},
		[]*scheduleModel.InstanceStaff{instanceStaff(11, 7), absent},
		nil,
	)
	if _, err := service.ExportBetreuungsplan(context.Background(), careParams()); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}
	if cell := cellFor(t, renderer.doc, "Mensa", listexport.ColumnPlanMonday); strings.Contains(cell, "Müller") {
		t.Fatalf("notice cell = %q, want the absent staff member left out", cell)
	}

	service, renderer = newBetreuungsplanService(
		[]*scheduleModel.ActivityInstance{instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)},
		[]*scheduleModel.InstanceStaff{instanceStaff(11, 7), absent},
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
	service, renderer := newBetreuungsplanService(
		[]*scheduleModel.ActivityInstance{
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

// The care plan accepts only its own template; asking for the staff layout
// is a client error, not a silently different document.
func TestBetreuungsplanRejectsForeignTemplate(t *testing.T) {
	service, _ := newBetreuungsplanService(nil, nil, nil)
	params := careParams()
	params.Template = TemplateByPerson
	if _, err := service.ExportBetreuungsplan(context.Background(), params); err == nil {
		t.Fatal("expected the staff template to be refused")
	}
}
