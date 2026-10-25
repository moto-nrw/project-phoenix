package planexport

import (
	"context"
	"errors"
	"slices"
	"testing"
)

// stubInstanceFilter keeps the listed blocks and records what it was asked.
type stubInstanceFilter struct {
	keep      []int64
	groupName string
	asked     *BlockFilter
	staffSeen *int
}

func (f stubInstanceFilter) MatchingInstances(_ context.Context, filter BlockFilter, ids []int64, staff []*InstanceStaff) (FilteredInstances, error) {
	*f.asked = filter
	*f.staffSeen = len(staff)
	matches := make(map[int64]bool, len(ids))
	for _, id := range ids {
		matches[id] = slices.Contains(f.keep, id)
	}
	return FilteredInstances{Matches: matches, GroupName: f.groupName}, nil
}

// #4766: a filtered Betreuungsplan prints only the blocks the filter keeps
// and names the filter, so the sheet cannot pass for the whole plan.
func TestBetreuungsplanPrintsOnlyFilteredBlocksAndNamesTheFilter(t *testing.T) {
	t.Parallel()

	var asked BlockFilter
	var staffSeen int
	renderer := &captureRenderer{}
	service := NewService(Dependencies{
		Instances: stubInstances{instances: []*Instance{
			instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3),
			instance(12, monday, clock(14, 0), clock(15, 0), "Lernzeit", 4),
		}},
		InstanceStaff: stubInstanceStaff{rows: []*InstanceStaff{instanceStaff(11, 7), instanceStaff(12, 8)}},
		Students:      stubStudentCounts{counts: map[int64]int{11: 5, 12: 9}},
		Rooms:         stubRooms{rooms: []*Room{room(3, "Speisesaal"), room(4, "Gruppenraum 1")}},
		Staff:         stubStaffDirectory{members: []*StaffMember{staffMember(7, "Franziska", "Kessener"), staffMember(8, "Anna", "Müller")}},
		Filter:        stubInstanceFilter{keep: []int64{12}, groupName: "Sonnen", asked: &asked, staffSeen: &staffSeen},
		Renderer:      renderer,
	}, nil)
	group, staff := int64(51), int64(8)
	params := careParams()
	params.Filter = BlockFilter{EducationGroupID: &group, StaffID: &staff}

	if _, err := service.ExportBetreuungsplan(context.Background(), params); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}
	if labels := rowLabels(renderer.doc); !slices.Equal(labels, []string{"Lernzeit"}) {
		t.Fatalf("rows = %v, want only the filtered block", labels)
	}
	if asked.EducationGroupID == nil || *asked.EducationGroupID != group || asked.StaffID == nil || *asked.StaffID != staff {
		t.Fatalf("filter asked = %+v, want the request's group and person", asked)
	}
	if staffSeen != 2 {
		t.Fatalf("filter saw %d staff rows, want the loaded rows of every block", staffSeen)
	}
	if !slices.Contains(renderer.doc.Filters, "Gruppe: Sonnen") || !slices.Contains(renderer.doc.Filters, "Person: Müller, A.") {
		t.Fatalf("filters = %v, want the group and person named", renderer.doc.Filters)
	}
}

func TestBetreuungsplanRejectsNonPositiveFilterIDs(t *testing.T) {
	t.Parallel()

	service, _ := newBetreuungsplanService(nil, nil, nil)
	zero := int64(0)
	params := careParams()
	params.Filter = BlockFilter{StaffID: &zero}

	if _, err := service.ExportBetreuungsplan(context.Background(), params); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("err = %v, want ErrInvalidParams", err)
	}
}

func TestBetreuungsplanFilterNeedsItsPort(t *testing.T) {
	t.Parallel()

	service, _ := newBetreuungsplanService(
		[]*Instance{instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)}, nil, nil,
	)
	staff := int64(7)
	params := careParams()
	params.Filter = BlockFilter{StaffID: &staff}

	if _, err := service.ExportBetreuungsplan(context.Background(), params); err == nil {
		t.Fatal("a filtered export without its filter port must fail, not print the whole plan")
	}
}
