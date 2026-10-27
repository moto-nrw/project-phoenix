package studentpresencehttp

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// SchoolAbsenceReports records the report the Schulverwaltung enters in "moto
// schule" afterwards for a child the Lehrkraft marked absent without one
// (#4791), with the status-day write of the OGS: the same locks, the same
// refusal of a day the child already has a status or a partial absence on,
// the same live flags, the same absence notice and the same after-commit
// fan-out. The row carries the school source, so only the school takes it
// back. The caller proved its class reach; the locked child must still be in
// that class (inClass), else ErrStudentStatusDayReassigned.
type SchoolAbsenceReports struct {
	rs      *Resource
	morning studentpresence.MorningAbsenceGuard
}

// NewSchoolAbsenceReports builds the write over the collaborators the
// student routes use.
func NewSchoolAbsenceReports(cfg ResourceConfig, morning studentpresence.MorningAbsenceGuard) SchoolAbsenceReports {
	return SchoolAbsenceReports{rs: NewResource(cfg), morning: morning}
}

// RecordSchoolAbsenceReport writes the sick or excused day of one child on
// date inside the caller's transaction. actorAccountID is the reporting
// account; the absence notice spares it.
func (t SchoolAbsenceReports) RecordSchoolAbsenceReport(ctx context.Context, studentID int64, status string, date calendar.Date, actorAccountID int64, inClass func(schoolClass string) bool) error {
	if err := t.configured(inClass); err != nil {
		return err
	}
	if status != absencerecords.StudentStatusDaySick && status != absencerecords.StudentStatusDayExcused {
		return errors.New("school absence report: status must be sick or excused")
	}
	tenantID := tenant.FromContext(ctx)
	wc := t.writeContext(tenantID, status, inClass)
	if t.morning == nil {
		return errors.New("school absence reports: morning absence guard not configured")
	}
	wc.ValidateLocked = func(ctx context.Context) error {
		return t.morning.RequireAbsentMorningAttendance(ctx, studentID, date)
	}
	dates := []calendar.Date{date}
	wc.AfterCreate = func(ctx context.Context, studentIDs []int64) error {
		return t.rs.NotifyAbsenceReported(ctx, tenantID, studentIDs, status, dates, false, actorAccountID)
	}
	return t.rs.StudentStatusDayService.CreateForDates(ctx, wc, studentID, status, "", dates)
}

// WithdrawSchoolAbsenceReport clears the child's active status day of the
// school's source on date and returns its status. A day the OGS or the
// parents entered stays: studentpresence.ErrSchoolAbsenceReportNotFound.
func (t SchoolAbsenceReports) WithdrawSchoolAbsenceReport(ctx context.Context, studentID int64, date calendar.Date, inClass func(schoolClass string) bool) (string, error) {
	if err := t.configured(inClass); err != nil {
		return "", err
	}
	rows, err := t.rs.StudentStatusDayService.FindActiveByStudentAndDateRange(ctx, studentID, date, date)
	if err != nil {
		return "", err
	}
	for _, row := range rows {
		if row == nil || row.Source != absencerecords.StudentStatusSourceSchool {
			continue
		}
		wc := t.writeContext(tenant.FromContext(ctx), row.Status, inClass)
		if err := t.rs.StudentStatusDayService.DeleteStatusDay(ctx, wc, row.ID, studentID); err != nil {
			return "", err
		}
		return row.Status, nil
	}
	return "", studentpresence.ErrSchoolAbsenceReportNotFound
}

func (t SchoolAbsenceReports) configured(inClass func(string) bool) error {
	if t.rs == nil || t.rs.StudentStatusDayService == nil || t.rs.PeopleDirectory == nil {
		return errors.New("school absence reports: status days not configured")
	}
	if inClass == nil {
		return errors.New("school absence reports: class check is required")
	}
	return nil
}

// writeContext is the class-scoped write of moto schule with the school's
// source.
func (t SchoolAbsenceReports) writeContext(tenantID int64, status string, inClass func(string) bool) studentpresence.StatusDayWriteContext {
	wc := t.rs.schoolClassWriteContext(tenantID, status, inClass)
	wc.Source = absencerecords.StudentStatusSourceSchool
	return wc
}
