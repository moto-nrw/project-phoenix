package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/domain"
	"github.com/uptrace/bun"
)

type appointmentRecipientReadRow struct {
	appointmentRecipientRow
	StudentIDs []int64 `bun:"student_ids,array"`
}

func (s *Store) FindAppointmentRecipients(ctx context.Context, appointmentIDs []int64) ([]*appointments.AppointmentRecipient, domain.OperationStats, error) {
	if len(appointmentIDs) == 0 {
		return []*appointments.AppointmentRecipient{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.databaseForTenant(ctx, "find appointment recipients")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := make([]appointmentRecipientReadRow, 0)
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(`calendar.appointment_recipients AS "appointment_recipient"`).
		ColumnExpr(`"appointment_recipient".*`).
		ColumnExpr(`ARRAY(SELECT ars.student_id FROM calendar.appointment_recipient_students AS ars
			WHERE ars.recipient_id = "appointment_recipient".id
			  AND ars.tenant_id = "appointment_recipient".tenant_id
			ORDER BY ars.student_id) AS student_ids`).
		Where(`"appointment_recipient".appointment_id IN (?)`, bun.List(appointmentIDs)).
		OrderExpr(`"appointment_recipient".appointment_id ASC, "appointment_recipient".recipient_type ASC, "appointment_recipient".id ASC`), "appointment_recipient", tenantID)
	stats, err := scanAll(ctx, query, "find appointment recipients")
	stats.Rows = int64(len(rows))
	result := make([]*appointments.AppointmentRecipient, 0, len(rows))
	for _, row := range rows {
		recipient := appointmentRecipientToPublic(row.appointmentRecipientRow)
		if len(row.StudentIDs) > 0 {
			recipient.StudentIDs = row.StudentIDs
		}
		result = append(result, recipient)
	}
	return result, stats, err
}
