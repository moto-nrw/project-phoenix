package enrollment

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"

	owner "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

type OfferingHistoryReader interface {
	RequestChildOfferingHistory(context.Context, int64) ([]*owner.RequestChildOffering, error)
}

type OfferingSelectionReader interface {
	RequestChildOfferingsAtDate(context.Context, int64, owner.Date) ([]*owner.RequestChildOffering, error)
	RequestChildOfferingHistory(context.Context, int64) ([]*owner.RequestChildOffering, error)
}

func readOwnerOfferingSelections(ctx context.Context, reader OfferingSelectionReader, childID int64, onDate timezone.Date) ([]*RequestChildOffering, error) {
	values, err := reader.RequestChildOfferingsAtDate(ctx, childID, owner.Date(onDate))
	if err != nil {
		return nil, err
	}
	return legacyOfferingSelections(values), nil
}

func ReadOfferingHistory(ctx context.Context, reader OfferingHistoryReader, childID int64) ([]*RequestChildOffering, error) {
	values, err := reader.RequestChildOfferingHistory(ctx, childID)
	if err != nil {
		return nil, err
	}
	return legacyOfferingSelections(values), nil
}

func legacyOfferingSelections(values []*owner.RequestChildOffering) []*RequestChildOffering {
	if values == nil {
		return nil
	}
	selections := make([]*RequestChildOffering, len(values))
	for index, value := range values {
		if value == nil {
			continue
		}
		selection := &RequestChildOffering{
			RequestChildID: value.RequestChildID, CareOfferingID: value.CareOfferingID,
			SelectedDays: value.SelectedDays, ManualSelectedDays: value.ManualSelectedDays,
			AutomaticSelectedDays: value.AutomaticSelectedDays, Notes: value.Notes,
		}
		selection.ID, selection.TenantID = value.ID, value.TenantID
		selection.CreatedAt, selection.UpdatedAt = value.CreatedAt, value.UpdatedAt
		if value.ValidFrom != nil {
			date := timezone.Date(*value.ValidFrom)
			selection.ValidFrom = &date
		}
		if value.ValidUntil != nil {
			date := timezone.Date(*value.ValidUntil)
			selection.ValidUntil = &date
		}
		selections[index] = selection
	}
	return selections
}

type OfferingSelectionBatchReader interface {
	RequestChildOfferingHistoryForChildren(context.Context, []int64) ([]*owner.RequestChildOffering, error)
	RequestChildOfferingsForChildrenAtDate(context.Context, []int64, owner.Date) ([]*owner.RequestChildOffering, error)
}

func readOwnerOfferingBatchHistory(ctx context.Context, reader OfferingSelectionBatchReader, childIDs []int64) ([]*RequestChildOffering, error) {
	values, err := reader.RequestChildOfferingHistoryForChildren(ctx, childIDs)
	if err != nil {
		return nil, err
	}
	return legacyOfferingSelections(values), nil
}

func readOwnerOfferingBatchSelections(ctx context.Context, reader OfferingSelectionBatchReader, childIDs []int64, onDate timezone.Date) ([]*RequestChildOffering, error) {
	values, err := reader.RequestChildOfferingsForChildrenAtDate(ctx, childIDs, owner.Date(onDate))
	if err != nil {
		return nil, err
	}
	return legacyOfferingSelections(values), nil
}

type OfferingCapacityReader interface {
	OfferingCapacityPeak(context.Context, int64, []int64, owner.Date, owner.Date) (int, error)
}
