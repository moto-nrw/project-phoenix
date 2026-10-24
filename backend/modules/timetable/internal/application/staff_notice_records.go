package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	internalports "github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// staffNoticeRecords owns the unit of work for a single-owner notice command.
// The SQL adapter requires the resulting transaction and never starts one.
type staffNoticeRecords struct {
	ports.StaffNoticeRecords
	tx internalports.Transaction
}

func NewStaffNoticeRecords(records ports.StaffNoticeRecords, tx internalports.Transaction) ports.StaffNoticeRecords {
	return staffNoticeRecords{records, tx}
}
func (r staffNoticeRecords) Create(ctx context.Context, n *schedulerecords.StaffNotice) (err error) {
	err = r.tx.RunWrite(ctx, false, func(txCtx context.Context) error { err = r.StaffNoticeRecords.Create(txCtx, n); return err })
	return
}
func (r staffNoticeRecords) Update(ctx context.Context, n *schedulerecords.StaffNotice) (err error) {
	err = r.tx.RunWrite(ctx, false, func(txCtx context.Context) error { err = r.StaffNoticeRecords.Update(txCtx, n); return err })
	return
}
func (r staffNoticeRecords) FindByID(ctx context.Context, id int64) (value0 *schedulerecords.StaffNotice, err error) {
	err = r.tx.RunWrite(ctx, false, func(txCtx context.Context) error { value0, err = r.StaffNoticeRecords.FindByID(txCtx, id); return err })
	return
}
func (r staffNoticeRecords) Delete(ctx context.Context, id int64) (err error) {
	err = r.tx.RunWrite(ctx, false, func(txCtx context.Context) error { err = r.StaffNoticeRecords.Delete(txCtx, id); return err })
	return
}
func (r staffNoticeRecords) List(ctx context.Context, includeInactive bool) (value0 []*schedulerecords.StaffNotice, err error) {
	err = r.tx.RunWrite(ctx, false, func(txCtx context.Context) error {
		value0, err = r.StaffNoticeRecords.List(txCtx, includeInactive)
		return err
	})
	return
}
func (r staffNoticeRecords) ListValidOn(ctx context.Context, date calendar.Date, reader string) (value0 []*schedulerecords.StaffNotice, err error) {
	err = r.tx.RunWrite(ctx, false, func(txCtx context.Context) error {
		value0, err = r.StaffNoticeRecords.ListValidOn(txCtx, date, reader)
		return err
	})
	return
}
func (r staffNoticeRecords) Acknowledge(ctx context.Context, noticeID, accountID int64) (err error) {
	err = r.tx.RunWrite(ctx, false, func(txCtx context.Context) error {
		err = r.StaffNoticeRecords.Acknowledge(txCtx, noticeID, accountID)
		return err
	})
	return
}
func (r staffNoticeRecords) AcknowledgedAtFor(ctx context.Context, accountID int64, noticeIDs []int64) (value0 map[int64]time.Time, err error) {
	err = r.tx.RunWrite(ctx, false, func(txCtx context.Context) error {
		value0, err = r.StaffNoticeRecords.AcknowledgedAtFor(txCtx, accountID, noticeIDs)
		return err
	})
	return
}
func (r staffNoticeRecords) AcknowledgedCounts(ctx context.Context, noticeIDs []int64) (value0 map[int64]int, err error) {
	err = r.tx.RunWrite(ctx, false, func(txCtx context.Context) error {
		value0, err = r.StaffNoticeRecords.AcknowledgedCounts(txCtx, noticeIDs)
		return err
	})
	return
}
func (r staffNoticeRecords) Acknowledgements(ctx context.Context, noticeID int64) (value0 []*schedulerecords.StaffNoticeAck, err error) {
	err = r.tx.RunWrite(ctx, false, func(txCtx context.Context) error {
		value0, err = r.StaffNoticeRecords.Acknowledgements(txCtx, noticeID)
		return err
	})
	return
}
