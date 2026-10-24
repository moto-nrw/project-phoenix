package api

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	presenceprojectionhttp "github.com/moto-nrw/project-phoenix/modules/presenceprojection/http"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The OGS views (Gruppe, Kindersuche, Aufsicht) show today's handover of a
// child (#4794): when and by whom the Lehrkraft handed its class over, and
// whether she marked the child "da" or "nicht da". Each view owns its port
// and wire type; the root reads Student Presence's handovers once and names
// the staff members in one People Directory read.

// classHandoverSource is Student Presence's read of the handovers.
type classHandoverSource interface {
	ClassHandoversOn(ctx context.Context, date calendar.Date) ([]studentpresence.ClassHandover, error)
}

// classHandoverStaffNamer maps staff ids to display names in one read.
type classHandoverStaffNamer interface {
	StaffNames(ctx context.Context, staffIDs []int64) (map[int64]string, error)
}

// childClassHandover is one child's handover before it takes a view's type.
type childClassHandover struct {
	schoolClass  string
	handedOverAt time.Time
	handedOverBy string
	absent       bool
}

// classHandoverReads are the handovers and the staff names the views' rows
// are joined from.
type classHandoverReads struct {
	handovers classHandoverSource
	staff     classHandoverStaffNamer
}

// loadClassHandovers maps each of studentIDs a handover on day named "da"
// or "nicht da" to its class, moment, staff name and mark. The name read
// runs only when a handover named one of the children.
func loadClassHandovers(ctx context.Context, r classHandoverReads, studentIDs []int64, day calendar.Date) (map[int64]childClassHandover, error) {
	result := map[int64]childClassHandover{}
	if len(studentIDs) == 0 {
		return result, nil
	}
	handovers, err := r.handovers.ClassHandoversOn(ctx, day)
	if err != nil || len(handovers) == 0 {
		return result, err
	}
	matched, staffIDs := matchClassHandovers(handovers, studentIDs)
	names := map[int64]string{}
	if len(staffIDs) > 0 && r.staff != nil {
		if names, err = r.staff.StaffNames(ctx, staffIDs); err != nil {
			return nil, err
		}
	}
	for id, m := range matched {
		result[id] = childClassHandover{
			schoolClass: m.handover.SchoolClass, handedOverAt: m.handover.HandedOverAt.In(calendar.Berlin),
			handedOverBy: names[m.handover.HandedOverByStaffID], absent: m.absent,
		}
	}
	return result, nil
}

// matchedClassHandover is the handover that named a child, and how.
type matchedClassHandover struct {
	handover studentpresence.ClassHandover
	absent   bool
}

// matchClassHandovers maps each of studentIDs a handover named to that
// handover and lists the staff members of those handovers once each.
func matchClassHandovers(handovers []studentpresence.ClassHandover, studentIDs []int64) (map[int64]matchedClassHandover, []int64) {
	named := map[int64]matchedClassHandover{}
	for _, handover := range handovers {
		for _, id := range handover.PresentStudentIDs {
			named[id] = matchedClassHandover{handover: handover}
		}
		for _, id := range handover.AbsentStudentIDs {
			named[id] = matchedClassHandover{handover: handover, absent: true}
		}
	}
	matched := map[int64]matchedClassHandover{}
	seenStaff := map[int64]bool{}
	var staffIDs []int64
	for _, id := range studentIDs {
		m, ok := named[id]
		if !ok {
			continue
		}
		matched[id] = m
		if staff := m.handover.HandedOverByStaffID; staff > 0 && !seenStaff[staff] {
			seenStaff[staff] = true
			staffIDs = append(staffIDs, staff)
		}
	}
	return matched, staffIDs
}

// groupLiveHandovers serves the live group view's handover port.
type groupLiveHandovers struct{ reads classHandoverReads }

func (h groupLiveHandovers) ClassHandovers(ctx context.Context, studentIDs []int64, day grouplive.Date) (map[int64]grouplive.ClassHandover, error) {
	loaded, err := loadClassHandovers(ctx, h.reads, studentIDs, calendar.Date(day))
	if err != nil {
		return nil, err
	}
	result := make(map[int64]grouplive.ClassHandover, len(loaded))
	for id, c := range loaded {
		result[id] = grouplive.ClassHandover{SchoolClass: c.schoolClass, HandedOverAt: c.handedOverAt, HandedOverBy: c.handedOverBy, Absent: c.absent}
	}
	return result, nil
}

// studentListHandovers serves the Kindersuche's handover port.
type studentListHandovers struct{ reads classHandoverReads }

func (h studentListHandovers) ClassHandovers(ctx context.Context, studentIDs []int64, day calendar.Date) (map[int64]presenceprojectionhttp.ClassHandover, error) {
	loaded, err := loadClassHandovers(ctx, h.reads, studentIDs, day)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]presenceprojectionhttp.ClassHandover, len(loaded))
	for id, c := range loaded {
		result[id] = presenceprojectionhttp.ClassHandover{SchoolClass: c.schoolClass, HandedOverAt: c.handedOverAt, HandedOverBy: c.handedOverBy, Absent: c.absent}
	}
	return result, nil
}

// supervisionHandovers serves the supervision dashboard's handover port.
type supervisionHandovers struct{ reads classHandoverReads }

func (h supervisionHandovers) ClassHandovers(ctx context.Context, studentIDs []int64, day supervisiondashboard.Date) (map[int64]supervisiondashboard.ClassHandover, error) {
	loaded, err := loadClassHandovers(ctx, h.reads, studentIDs, calendar.Date(day))
	if err != nil {
		return nil, err
	}
	result := make(map[int64]supervisiondashboard.ClassHandover, len(loaded))
	for id, c := range loaded {
		result[id] = supervisiondashboard.ClassHandover{SchoolClass: c.schoolClass, HandedOverAt: c.handedOverAt, HandedOverBy: c.handedOverBy, Absent: c.absent}
	}
	return result, nil
}
