package enrollment

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	legacy "github.com/moto-nrw/project-phoenix/models/enrollment"
	owner "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

type OfferingSelectionWriter interface {
	ReplaceRequestChildOfferings(context.Context, int64, []*owner.RequestChildOffering) error
	ScheduleRequestChildOfferings(context.Context, int64, owner.Date, []*owner.RequestChildOffering) error
}

func writeOwnerOfferingSelections(ctx context.Context, writer OfferingSelectionWriter, childID int64, effectiveFrom *timezone.Date, selections []*legacy.RequestChildOffering) error {
	values := make([]*owner.RequestChildOffering, len(selections))
	for index, selection := range selections {
		if selection == nil {
			continue
		}
		value := &owner.RequestChildOffering{
			ID: selection.ID, TenantID: selection.TenantID, CreatedAt: selection.CreatedAt, UpdatedAt: selection.UpdatedAt,
			RequestChildID: selection.RequestChildID, CareOfferingID: selection.CareOfferingID,
			SelectedDays: selection.SelectedDays, ManualSelectedDays: selection.ManualSelectedDays, AutomaticSelectedDays: selection.AutomaticSelectedDays, Notes: selection.Notes,
		}
		if selection.ValidFrom != nil {
			date := owner.Date(*selection.ValidFrom)
			value.ValidFrom = &date
		}
		if selection.ValidUntil != nil {
			date := owner.Date(*selection.ValidUntil)
			value.ValidUntil = &date
		}
		values[index] = value
	}
	var err error
	if effectiveFrom == nil {
		err = writer.ReplaceRequestChildOfferings(ctx, childID, values)
	} else {
		err = writer.ScheduleRequestChildOfferings(ctx, childID, owner.Date(*effectiveFrom), values)
	}
	if err != nil {
		return err
	}
	for index, value := range values {
		selection := selections[index]
		selection.ID, selection.TenantID = value.ID, value.TenantID
		selection.CreatedAt, selection.UpdatedAt = value.CreatedAt, value.UpdatedAt
		selection.RequestChildID = value.RequestChildID
		if value.ValidFrom != nil {
			date := timezone.Date(*value.ValidFrom)
			selection.ValidFrom = &date
		}
		if value.ValidUntil != nil {
			date := timezone.Date(*value.ValidUntil)
			selection.ValidUntil = &date
		}
	}
	return nil
}

type OfferingHistoryReader interface {
	RequestChildOfferingHistory(context.Context, int64) ([]*owner.RequestChildOffering, error)
}

type OfferingSelectionReader interface {
	RequestChildOfferingsAtDate(context.Context, int64, owner.Date) ([]*owner.RequestChildOffering, error)
	RequestChildOfferingHistory(context.Context, int64) ([]*owner.RequestChildOffering, error)
}

func readOwnerOfferingSelections(ctx context.Context, reader OfferingSelectionReader, childID int64, onDate timezone.Date) ([]*legacy.RequestChildOffering, error) {
	values, err := reader.RequestChildOfferingsAtDate(ctx, childID, owner.Date(onDate))
	if err != nil {
		return nil, err
	}
	return legacyOfferingSelections(values), nil
}

func ReadOfferingHistory(ctx context.Context, reader OfferingHistoryReader, childID int64) ([]*legacy.RequestChildOffering, error) {
	values, err := reader.RequestChildOfferingHistory(ctx, childID)
	if err != nil {
		return nil, err
	}
	return legacyOfferingSelections(values), nil
}

func legacyOfferingSelections(values []*owner.RequestChildOffering) []*legacy.RequestChildOffering {
	if values == nil {
		return nil
	}
	selections := make([]*legacy.RequestChildOffering, len(values))
	for index, value := range values {
		if value == nil {
			continue
		}
		selection := &legacy.RequestChildOffering{
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

func readOwnerOfferingBatchHistory(ctx context.Context, reader OfferingSelectionBatchReader, childIDs []int64) ([]*legacy.RequestChildOffering, error) {
	values, err := reader.RequestChildOfferingHistoryForChildren(ctx, childIDs)
	if err != nil {
		return nil, err
	}
	return legacyOfferingSelections(values), nil
}

func readOwnerOfferingBatchSelections(ctx context.Context, reader OfferingSelectionBatchReader, childIDs []int64, onDate timezone.Date) ([]*legacy.RequestChildOffering, error) {
	values, err := reader.RequestChildOfferingsForChildrenAtDate(ctx, childIDs, owner.Date(onDate))
	if err != nil {
		return nil, err
	}
	return legacyOfferingSelections(values), nil
}

type OfferingCapacityReader interface {
	OfferingCapacityPeak(context.Context, int64, []int64, owner.Date, owner.Date) (int, error)
}
