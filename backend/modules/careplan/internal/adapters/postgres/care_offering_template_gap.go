package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// SetCareOfferingTemplateGapAck writes the school's decision on an offering
// without a matching Regeltermin (#5140); an empty list stores NULL, no
// decision. ErrCareOfferingNotFound when the tenant has no such offering.
func (s *Store) SetCareOfferingTemplateGapAck(ctx context.Context, offeringID int64, schoolClasses []string) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "decide a care offering without a Regeltermin")
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := db.NewUpdate().Model((*careOfferingRow)(nil)).
		ModelTableExpr(`enrollment.care_offerings AS "care_offering"`).
		Set("updated_at = NOW()").
		Where(`"care_offering".id = ?`, offeringID)
	if len(schoolClasses) == 0 {
		query = query.Set("template_gap_ack_classes = NULL")
	} else {
		query = query.Set("template_gap_ack_classes = ?::text[]", pgdialect.Array(schoolClasses))
	}
	return execGuarded(ctx, withTenant(query, "care_offering", tenantID), "set care offering template gap ack", careplan.ErrCareOfferingNotFound)
}
