package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

// SetCareOfferingTemplateGapAck writes the school's decision on an offering
// without a matching Regeltermin (#5140) for one planning period; an empty
// list removes that period's entry, and an empty map stores NULL.
// ErrCareOfferingNotFound when the tenant has no such offering.
func (s *Store) SetCareOfferingTemplateGapAck(ctx context.Context, offeringID, calendarPeriodID int64, schoolClasses []string) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "decide a care offering without a Regeltermin")
	if err != nil {
		return domain.OperationStats{}, err
	}
	key := strconv.FormatInt(calendarPeriodID, 10)
	query := db.NewUpdate().Model((*careOfferingRow)(nil)).
		ModelTableExpr(`enrollment.care_offerings AS "care_offering"`).
		Set("updated_at = NOW()").
		Where(`"care_offering".id = ?`, offeringID)
	if len(schoolClasses) == 0 {
		query = query.Set(`template_gap_acks = NULLIF(COALESCE(template_gap_acks, '{}'::jsonb) - ?::text, '{}'::jsonb)`, key)
	} else {
		classes, marshalErr := json.Marshal(schoolClasses)
		if marshalErr != nil {
			return domain.OperationStats{}, fmt.Errorf("care plan postgres: encode template gap classes: %w", marshalErr)
		}
		query = query.Set(`template_gap_acks = jsonb_set(COALESCE(template_gap_acks, '{}'::jsonb), ARRAY[?::text], ?::jsonb)`, key, string(classes))
	}
	return execGuarded(ctx, withTenant(query, "care_offering", tenantID), "set care offering template gap ack", careplan.ErrCareOfferingNotFound)
}

// templateGapAcksToDomain keys the stored decisions by planning period id;
// a key that is no id cannot match a period and is dropped.
func templateGapAcksToDomain(stored map[string][]string) map[int64][]string {
	if len(stored) == 0 {
		return nil
	}
	acks := make(map[int64][]string, len(stored))
	for key, classes := range stored {
		if periodID, err := strconv.ParseInt(key, 10, 64); err == nil && periodID > 0 {
			acks[periodID] = classes
		}
	}
	return acks
}
