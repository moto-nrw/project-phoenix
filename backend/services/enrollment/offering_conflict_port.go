package enrollment

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

// Angebote side of the conflict resolver (#2267, stories 6-10). Two open
// switch requests for the same offering always overlap — their validity ranges
// are open-ended — so they are resolved as one group.

var _ usersService.ParentRequestConflictPort = (*offeringChangeRequestService)(nil)

func (s *offeringChangeRequestService) ConflictCandidate(
	ctx context.Context, requestID int64,
) (*usersService.ParentRequestConflictCandidate, error) {
	row, err := s.ChangeRepo.FindByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if row.IsTerminal() {
		return nil, enrollmentModels.ErrOfferingChangeNotPending
	}
	return &usersService.ParentRequestConflictCandidate{
		StudentID: row.StudentID,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

func (s *offeringChangeRequestService) LockConflictRequest(ctx context.Context, requestID int64) error {
	row, err := s.ChangeRepo.FindByIDForUpdate(ctx, requestID)
	if err != nil {
		return err
	}
	if row.IsTerminal() {
		return enrollmentModels.ErrOfferingChangeNotPending
	}
	return nil
}

func (s *offeringChangeRequestService) DecideConflictRequest(
	ctx context.Context, decision usersService.ParentRequestConflictDecision,
) error {
	return s.Decide(ctx, DecideOfferingChangeInput{
		RequestID:       decision.RequestID,
		Approve:         decision.Approve,
		Reason:          decision.Reason,
		ReviewedBy:      decision.ReviewerID,
		ActorRole:       decision.ActorRole,
		ExpectedVersion: decision.ExpectedVersion,
	})
}

// WriteStaffValue books the offerings the staff member chose instead of any of
// the rejected wishes, through the same direct-correction path the office uses
// on the child's own screen — so it lands in the offering adjustment log with
// source "direct" and passes the live catalog validation.
//
// Value is {"value": {"effective_from": "YYYY-MM-DD", "selections":
// [{"offering_id": 12, "selected_days": ["mon","wed"]}]}}.
func (s *offeringChangeRequestService) WriteStaffValue(
	ctx context.Context, write usersService.ParentRequestStaffValueWrite,
) error {
	if s.DirectApplier == nil {
		return usersService.ErrStaffValueUnsupported
	}
	effectiveFrom, selections, err := parseStaffOfferingValue(write.Value)
	if err != nil {
		return err
	}
	return s.ApplyDirectOfferingAdjustment(ctx, DirectOfferingAdjustmentInput{
		StudentID:      write.StudentID,
		EffectiveFrom:  effectiveFrom,
		Selections:     selections,
		Reason:         write.Reason,
		ActorAccountID: write.ReviewerID,
		ActorRole:      write.ActorRole,
	})
}

func parseStaffOfferingValue(value map[string]any) (timezone.Date, []OfferingChangeSelection, error) {
	inner, ok := value["value"].(map[string]any)
	if !ok {
		return timezone.Date(""), nil, fmt.Errorf("%w: staff value is missing", ErrOfferingChangeInvalid)
	}
	rawDate, ok := inner["effective_from"].(string)
	if !ok {
		return timezone.Date(""), nil, fmt.Errorf("%w: effective_from is required", ErrOfferingChangeInvalid)
	}
	effectiveFrom, err := timezone.ParseDate(rawDate)
	if err != nil {
		return timezone.Date(""), nil, fmt.Errorf("%w: effective_from is not a date", ErrOfferingChangeInvalid)
	}
	rawSelections, ok := inner["selections"].([]any)
	if !ok {
		return timezone.Date(""), nil, fmt.Errorf("%w: selections are required", ErrOfferingChangeInvalid)
	}
	selections := make([]OfferingChangeSelection, 0, len(rawSelections))
	for _, entry := range rawSelections {
		selection, err := parseStaffOfferingSelection(entry)
		if err != nil {
			return timezone.Date(""), nil, err
		}
		selections = append(selections, selection)
	}
	return effectiveFrom, selections, nil
}

func parseStaffOfferingSelection(entry any) (OfferingChangeSelection, error) {
	raw, ok := entry.(map[string]any)
	if !ok {
		return OfferingChangeSelection{}, fmt.Errorf("%w: selection is malformed", ErrOfferingChangeInvalid)
	}
	// JSON numbers decode as float64; an offering id that is not a whole
	// number is a malformed request, not a rounding job.
	id, ok := raw["offering_id"].(float64)
	if !ok || id <= 0 || id != float64(int64(id)) {
		return OfferingChangeSelection{}, fmt.Errorf("%w: offering_id is invalid", ErrOfferingChangeInvalid)
	}
	rawDays, _ := raw["selected_days"].([]any)
	days := make([]string, 0, len(rawDays))
	for _, day := range rawDays {
		text, ok := day.(string)
		if !ok {
			return OfferingChangeSelection{}, fmt.Errorf("%w: selected_days are invalid", ErrOfferingChangeInvalid)
		}
		days = append(days, text)
	}
	return OfferingChangeSelection{OfferingID: int64(id), SelectedDays: days}, nil
}
