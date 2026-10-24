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

// The OGS views (Gruppe, Kindersuche, Aufsicht) show which children the
// Lehrkraft marked "nicht da" when she handed her class over (#4794). Each
// view owns its port and wire type; the root reads Student Presence's
// absences once and names the staff members in one People Directory read.

// classHandoverAbsenceSource is Student Presence's read of the handovers.
type classHandoverAbsenceSource interface {
	ClassHandoversOn(ctx context.Context, date calendar.Date) ([]studentpresence.ClassHandover, error)
}

// classHandoverStaffNamer maps staff ids to display names in one read.
type classHandoverStaffNamer interface {
	StaffNames(ctx context.Context, staffIDs []int64) (map[int64]string, error)
}

// classHandoverAbsence is one named absence before it takes a view's type.
type classHandoverAbsence struct {
	schoolClass  string
	handedOverAt time.Time
	handedOverBy string
}

// classHandoverAbsenceReads are the handovers and the staff names the
// absences are joined from.
type classHandoverAbsenceReads struct {
	absences classHandoverAbsenceSource
	staff    classHandoverStaffNamer
}

// loadClassHandoverAbsences maps each of studentIDs a handover on day marked
// "nicht da" to its class, moment and staff name. The name read runs only
// when a handover marked one of the children.
func loadClassHandoverAbsences(ctx context.Context, r classHandoverAbsenceReads, studentIDs []int64, day calendar.Date) (map[int64]classHandoverAbsence, error) {
	result := map[int64]classHandoverAbsence{}
	if len(studentIDs) == 0 {
		return result, nil
	}
	handovers, err := r.absences.ClassHandoversOn(ctx, day)
	if err != nil || len(handovers) == 0 {
		return result, err
	}
	marked, staffIDs := markedClassHandovers(handovers, studentIDs)
	names := map[int64]string{}
	if len(staffIDs) > 0 && r.staff != nil {
		if names, err = r.staff.StaffNames(ctx, staffIDs); err != nil {
			return nil, err
		}
	}
	for id, handover := range marked {
		result[id] = classHandoverAbsence{
			schoolClass: handover.SchoolClass, handedOverAt: handover.HandedOverAt.In(calendar.Berlin), handedOverBy: names[handover.HandedOverByStaffID],
		}
	}
	return result, nil
}

// markedClassHandovers maps each of studentIDs a handover marked "nicht da"
// to that handover and lists the staff members of those handovers.
func markedClassHandovers(handovers []studentpresence.ClassHandover, studentIDs []int64) (map[int64]studentpresence.ClassHandover, []int64) {
	wanted := make(map[int64]bool, len(studentIDs))
	for _, id := range studentIDs {
		wanted[id] = true
	}
	marked := map[int64]studentpresence.ClassHandover{}
	named := map[int64]bool{}
	var staffIDs []int64
	for _, handover := range handovers {
		for _, id := range handover.AbsentStudentIDs {
			if !wanted[id] {
				continue
			}
			marked[id] = handover
			if staff := handover.HandedOverByStaffID; staff > 0 && !named[staff] {
				named[staff] = true
				staffIDs = append(staffIDs, staff)
			}
		}
	}
	return marked, staffIDs
}

// groupLiveHandovers serves the live group view's handover port.
type groupLiveHandovers struct{ reads classHandoverAbsenceReads }

func (h groupLiveHandovers) HandoverAbsences(ctx context.Context, studentIDs []int64, day grouplive.Date) (map[int64]grouplive.HandoverAbsence, error) {
	loaded, err := loadClassHandoverAbsences(ctx, h.reads, studentIDs, calendar.Date(day))
	if err != nil {
		return nil, err
	}
	result := make(map[int64]grouplive.HandoverAbsence, len(loaded))
	for id, absence := range loaded {
		result[id] = grouplive.HandoverAbsence{SchoolClass: absence.schoolClass, HandedOverAt: absence.handedOverAt, HandedOverBy: absence.handedOverBy}
	}
	return result, nil
}

// studentListHandovers serves the Kindersuche's handover port.
type studentListHandovers struct{ reads classHandoverAbsenceReads }

func (h studentListHandovers) HandoverAbsences(ctx context.Context, studentIDs []int64, day calendar.Date) (map[int64]presenceprojectionhttp.ClassHandoverAbsence, error) {
	loaded, err := loadClassHandoverAbsences(ctx, h.reads, studentIDs, day)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]presenceprojectionhttp.ClassHandoverAbsence, len(loaded))
	for id, absence := range loaded {
		result[id] = presenceprojectionhttp.ClassHandoverAbsence{SchoolClass: absence.schoolClass, HandedOverAt: absence.handedOverAt, HandedOverBy: absence.handedOverBy}
	}
	return result, nil
}

// supervisionHandovers serves the supervision dashboard's handover port.
type supervisionHandovers struct{ reads classHandoverAbsenceReads }

func (h supervisionHandovers) HandoverAbsences(ctx context.Context, studentIDs []int64, day supervisiondashboard.Date) (map[int64]supervisiondashboard.HandoverAbsence, error) {
	loaded, err := loadClassHandoverAbsences(ctx, h.reads, studentIDs, calendar.Date(day))
	if err != nil {
		return nil, err
	}
	result := make(map[int64]supervisiondashboard.HandoverAbsence, len(loaded))
	for id, absence := range loaded {
		result[id] = supervisiondashboard.HandoverAbsence{SchoolClass: absence.schoolClass, HandedOverAt: absence.handedOverAt, HandedOverBy: absence.handedOverBy}
	}
	return result, nil
}
