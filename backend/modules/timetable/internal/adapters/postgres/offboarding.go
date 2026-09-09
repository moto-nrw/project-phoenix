package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

type offboardingInstanceRow struct {
	instanceStaffRow
	Date              string    `bun:"offboarding_date"`
	Status            string    `bun:"offboarding_status"`
	InstanceUpdatedAt time.Time `bun:"instance_updated_at"`
}

// Lock all assigned instances, including history: a date/status edit must
// not move an unlocked row into the delete set.
func (s *Store) PreviewStaffOffboarding(ctx context.Context, staffID int64, from string) (domain.OffboardingSnapshot, domain.OperationStats, error) {
	snapshot := domain.OffboardingSnapshot{StaffID: staffID, From: from}
	stats := domain.OperationStats{}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return snapshot, stats, err
	}
	rows := []offboardingInstanceRow{}
	query := instanceStaffSelect(db, &rows, tenantID).
		ColumnExpr(`"activity_instance".date::text AS offboarding_date, "activity_instance".status AS offboarding_status, "activity_instance".updated_at AS instance_updated_at`).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_staff".instance_id AND "activity_instance".tenant_id = "instance_staff".tenant_id`).
		Where(`"instance_staff".staff_id = ?`, staffID).
		OrderExpr(`"activity_instance".id ASC, "instance_staff".id ASC`).For("UPDATE OF activity_instance, instance_staff")
	readStats, err := scanAll(ctx, query, "preview offboarding instance assignments")
	stats.Add(readStats)
	if err != nil {
		return snapshot, stats, err
	}
	for _, row := range rows {
		if row.Date > from || (row.Date == from && row.Status == "planned") {
			snapshot.Instances = append(snapshot.Instances, domain.OffboardingInstance{Assignment: instanceStaffToDomain(row.instanceStaffRow), Date: row.Date, Status: row.Status, InstanceUpdatedAt: row.InstanceUpdatedAt})
		}
	}
	supervisors := []plannedSupervisorRow{}
	query = plannedSupervisorSelect(db, &supervisors, tenantID).
		Where(`"supervisor".staff_id = ?`, staffID).OrderExpr(`"supervisor".id ASC`).For("UPDATE")
	readStats, err = scanAll(ctx, query, "preview offboarding planned supervisors")
	stats.Add(readStats)
	if err != nil {
		return snapshot, stats, err
	}
	for _, row := range supervisors {
		snapshot.Supervisors = append(snapshot.Supervisors, plannedSupervisorToDomain(row))
	}
	return snapshot, stats, nil
}
