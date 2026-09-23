package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PickupBaselineRecords supplies the owner's records for baseline projection.
type PickupBaselineRecords interface {
	ListPickupSchedules(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupSchedule, error)
	ListCareOfferings(context.Context, careplan.CareOfferingFilter) ([]careplan.CareOffering, error)
}

func NewPickupBaselines(records PickupBaselineRecords, links careplan.ApprovedBookingReader, authoritative func(context.Context) (bool, error)) (careplan.PickupBaselineReader, error) {
	if records == nil || links == nil || authoritative == nil {
		return nil, errors.New("pickup baselines: records, approved bookings, and booking mode are required")
	}
	return application.NewPickupBaselineQueries(pickupBaselineSource{records, links, authoritative}), nil
}

type pickupBaselineSource struct {
	records       PickupBaselineRecords
	links         careplan.ApprovedBookingReader
	authoritative func(context.Context) (bool, error)
}

func (s pickupBaselineSource) StoredPickupRows(ctx context.Context, ids []int64) ([]*careplan.PickupSchedule, error) {
	if len(ids) == 0 {
		return []*careplan.PickupSchedule{}, nil
	}
	rows, err := s.records.ListPickupSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: ids})
	if err != nil {
		return nil, fmt.Errorf("project pickup baselines: load stored schedules: %w", err)
	}
	out := make([]*careplan.PickupSchedule, len(rows))
	for i := range rows {
		out[i] = &rows[i]
	}
	return out, nil
}

func (s pickupBaselineSource) BookingsAuthoritative(ctx context.Context) (bool, error) {
	result, err := s.authoritative(ctx)
	if err != nil {
		return false, fmt.Errorf("project pickup baselines: resolve booking mode: %w", err)
	}
	return result, nil
}

func (s pickupBaselineSource) BookingRows(ctx context.Context, ids []int64, from, to calendar.Date) ([]*careplan.ApprovedBooking, error) {
	return s.links.ListApprovedByStudentIDsInRange(ctx, ids, from, to)
}

func (s pickupBaselineSource) OfferingRows(ctx context.Context, links []*careplan.ApprovedBooking) (map[int64]*careplan.CareOffering, error) {
	rows, err := baselineOfferingRows(ctx, s.records, links)
	if err != nil {
		return nil, fmt.Errorf("project pickup baselines: load care offerings: %w", err)
	}
	return rows, nil
}

func baselineOfferingRows(ctx context.Context, records interface {
	ListCareOfferings(context.Context, careplan.CareOfferingFilter) ([]careplan.CareOffering, error)
}, links []*careplan.ApprovedBooking) (map[int64]*careplan.CareOffering, error) {
	ids := make([]int64, 0, len(links))
	seen := make(map[int64]bool, len(links))
	for _, entry := range links {
		if entry != nil && entry.Link != nil && entry.Link.CareOfferingID > 0 && !seen[entry.Link.CareOfferingID] {
			ids = append(ids, entry.Link.CareOfferingID)
			seen[entry.Link.CareOfferingID] = true
		}
	}
	if len(ids) == 0 {
		return map[int64]*careplan.CareOffering{}, nil
	}
	rows, err := records.ListCareOfferings(ctx, careplan.CareOfferingFilter{IDs: ids})
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*careplan.CareOffering, len(rows))
	for i := range rows {
		out[rows[i].ID] = &rows[i]
	}
	return out, nil
}
