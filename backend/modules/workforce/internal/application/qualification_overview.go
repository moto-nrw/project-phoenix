package application

import (
	"context"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// QualificationList reads the Nachweise (#4808): every qualification of
// the live staff with its status and every mandatory kind a person lacks.
type QualificationList struct {
	records qualificationRecords
	staff   workforce.QualificationStaff
}

// NewQualificationList builds the Nachweise read over the records and
// the staff directory.
func NewQualificationList(store ports.Store, clock ports.Clock, staff workforce.QualificationStaff) *QualificationList {
	if staff == nil {
		panic("qualification overview: staff is required")
	}
	return &QualificationList{records: newQualificationRecords(store, clock), staff: staff}
}

// QualificationOverview lists the live staff by name, each with their
// qualifications in their order and the missing mandatory kinds after them.
func (o *QualificationList) QualificationOverview(ctx context.Context) ([]workforce.QualificationOverviewRow, error) {
	members, err := o.staff.ListStaffMembers(ctx)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(members, func(a, b workforce.QualificationStaffMember) int {
		if order := strings.Compare(a.LastName, b.LastName); order != 0 {
			return order
		}
		if order := strings.Compare(a.FirstName, b.FirstName); order != 0 {
			return order
		}
		switch {
		case a.StaffID < b.StaffID:
			return -1
		case a.StaffID > b.StaffID:
			return 1
		default:
			return 0
		}
	})
	types, qualifications, err := o.records.load(ctx, 0)
	if err != nil {
		return nil, mapStaffRecordError(err)
	}
	byStaff := make(map[int64][]domain.StaffQualification, len(members))
	for _, q := range qualifications {
		byStaff[q.StaffID] = append(byStaff[q.StaffID], q)
	}
	today := o.records.today()
	rows := make([]workforce.QualificationOverviewRow, 0, len(qualifications))
	for _, member := range members {
		for _, entry := range qualificationEntries(member.StaffID, byStaff[member.StaffID], types, today) {
			rows = append(rows, workforce.QualificationOverviewRow{QualificationEntry: entry, FirstName: member.FirstName, LastName: member.LastName})
		}
	}
	return rows, nil
}

// StaffQualificationStatus lists one person's qualifications with status and
// their missing mandatory kinds; the person's own view and the profile use it.
func (o *QualificationList) StaffQualificationStatus(ctx context.Context, staffID int64) ([]workforce.QualificationEntry, error) {
	if staffID <= 0 {
		return nil, invalidStaffRecord("staff ID is required")
	}
	types, qualifications, err := o.records.load(ctx, staffID)
	if err != nil {
		return nil, mapStaffRecordError(err)
	}
	return qualificationEntries(staffID, qualifications, types, o.records.today()), nil
}

// qualificationEntries gives each qualification its status and adds one
// missing entry per live mandatory kind the person has no qualification of.
func qualificationEntries(staffID int64, qualifications []domain.StaffQualification, types []domain.QualificationType, today calendar.Date) []workforce.QualificationEntry {
	byID := make(map[int64]domain.QualificationType, len(types))
	for _, kind := range types {
		byID[kind.ID] = kind
	}
	held := map[int64]bool{}
	entries := make([]workforce.QualificationEntry, 0, len(qualifications))
	for _, q := range qualifications {
		lead := domain.DefaultReminderLeadWeeks
		name := q.Name
		mandatory := false
		if q.TypeID != nil {
			held[*q.TypeID] = true
			if kind, ok := byID[*q.TypeID]; ok {
				lead, name = kind.ReminderLeadWeeks, kind.Name
				mandatory = kind.Mandatory && !kind.Archived()
			}
		}
		entries = append(entries, workforce.QualificationEntry{
			StaffID: staffID, QualificationID: new(q.ID), TypeID: q.TypeID, Name: name,
			AcquiredOn: q.AcquiredOn, ExpiresOn: q.ExpiresOn, Mandatory: mandatory,
			Status: domain.QualificationStatus(q.ExpiresOn, lead, today),
		})
	}
	for _, kind := range types {
		if kind.Mandatory && !kind.Archived() && !held[kind.ID] {
			entries = append(entries, workforce.QualificationEntry{
				StaffID: staffID, TypeID: new(kind.ID), Name: kind.Name, Mandatory: true,
				Status: domain.QualificationStatusMissing,
			})
		}
	}
	return entries
}
