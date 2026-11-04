package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// qualificationRecords is what the Nachweise (#4808) need of the record
// rows: the store and the calendar day. Every operation joins the tenant unit
// of its caller (the request transaction of the staff routes, the Worker's
// per-school transaction) and opens none of its own.
type qualificationRecords struct {
	store ports.Store
	clock ports.Clock
}

func newQualificationRecords(store ports.Store, clock ports.Clock) qualificationRecords {
	if store == nil || clock == nil {
		panic("qualification records: store and clock are required")
	}
	return qualificationRecords{store: store, clock: clock}
}

func (r qualificationRecords) ensureStandardTypes(ctx context.Context) error {
	_, err := r.store.EnsureStandardQualificationTypes(ctx, domain.StandardQualificationTypes())
	return err
}

// load reads the school's kinds with the live qualifications of one staff
// member, or of every staff member when staffID is zero.
func (r qualificationRecords) load(ctx context.Context, staffID int64) ([]domain.QualificationType, []domain.StaffQualification, error) {
	if err := r.ensureStandardTypes(ctx); err != nil {
		return nil, nil, err
	}
	types, _, err := r.store.ListQualificationTypes(ctx)
	if err != nil {
		return nil, nil, err
	}
	var qualifications []domain.StaffQualification
	if staffID > 0 {
		qualifications, _, err = r.store.ListStaffQualifications(ctx, staffID)
	} else {
		qualifications, _, err = r.store.ListLiveQualifications(ctx)
	}
	return types, qualifications, err
}

func (r qualificationRecords) today() calendar.Date {
	today, err := calendar.ParseDate(r.clock.Today())
	if err != nil {
		return calendar.TodayDate()
	}
	return today
}

// Qualifications is the Nachweise of #4808: the kinds, the list and the
// reminders over one store. The reminders need their ports; a root that
// serves only the routes leaves them zero.
type Qualifications struct {
	*QualificationTypes
	*QualificationList
	*QualificationReminders
}

// NewQualifications composes the Nachweise. staff may be nil for a root
// that needs only the kinds; RunDue then fails instead of guessing.
func NewQualifications(store ports.Store, clock ports.Clock, staff workforce.QualificationStaff, bound workforce.QualificationReminderPorts) (*Qualifications, error) {
	result := &Qualifications{QualificationTypes: NewQualificationTypes(store, clock)}
	if staff == nil {
		return result, nil
	}
	result.QualificationList = NewQualificationList(store, clock, staff)
	if bound.Staff == nil {
		bound.Staff = staff
	}
	if bound.Notify == nil {
		return result, nil
	}
	reminders, err := NewQualificationReminders(store, clock, bound)
	result.QualificationReminders = reminders
	return result, err
}
