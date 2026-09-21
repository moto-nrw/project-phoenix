package care

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func (s *Service) ListPickupChangeRequests(ctx context.Context, accountID, studentID int64) ([]carerequests.Request, error) {
	child, err := s.ResolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	if s.CareRequests == nil {
		return nil, errors.New("parent: pickup change request service not configured")
	}
	var rows []carerequests.Request
	err = tenant.WithTenantTx(ctx, s.DB, child.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var listErr error
		rows, listErr = s.CareRequests.ListPickupChangeRequests(txCtx, studentID, time.Now().AddDate(0, -2, 0))
		if listErr != nil {
			return listErr
		}
		visibility, visibilityErr := s.RequestSharing.LoadRequestShareVisibility(txCtx, studentID)
		if visibilityErr != nil {
			return visibilityErr
		}
		visible := rows[:0]
		for _, row := range rows {
			if visibility.Allows("pickup_change", row.ID, accountID, row.SubmittedBy) {
				visible = append(visible, row)
			}
		}
		rows = visible
		s.enrichLegacyPickupChangeRequests(txCtx, studentID, rows)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("parent: list pickup change requests: %w", err)
	}
	return rows, nil
}

func (s *Service) enrichLegacyPickupChangeRequests(ctx context.Context, studentID int64, rows []carerequests.Request) {
	if s.PickupSchedules == nil {
		return
	}
	for i := range rows {
		s.enrichLegacyPickupChangeRequest(ctx, studentID, &rows[i])
	}
}

func (s *Service) enrichLegacyPickupChangeRequest(ctx context.Context, studentID int64, row *carerequests.Request) {
	if row.Status != "pending" {
		return
	}
	var payload map[string]any
	if json.Unmarshal(row.Payload, &payload) != nil {
		return
	}
	if previous, _ := payload["previous_pickup_time"].(string); previous != "" {
		return
	}
	dateRaw, _ := payload["date"].(string)
	date, err := timezone.ParseDate(dateRaw)
	if err != nil {
		s.Logger.Warn("parent: legacy pickup request has invalid date",
			"request_id", row.ID,
			"student_id", studentID,
		)
		return
	}
	effective, err := s.PickupSchedules.GetEffectivePickupTimeForDate(ctx, studentID, date)
	if err != nil {
		s.Logger.Warn("parent: resolve legacy pickup request baseline failed",
			"request_id", row.ID,
			"student_id", studentID,
			"error", err,
		)
		return
	}
	if effective == nil || effective.PickupTime == nil {
		return
	}
	payload["previous_pickup_time"] = effective.PickupTime.Format("15:04")
	encoded, err := json.Marshal(payload)
	if err != nil {
		return
	}
	row.Payload = encoded
}
