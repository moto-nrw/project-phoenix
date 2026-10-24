//go:build testhooks

package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// nativeFixtureGroup writes fixture groups through the owner's group
// capability with the legacy row checks.
type nativeFixtureGroup struct {
	groups ports.GroupRows
}

func NativeGroupRecords(groups ports.GroupRows) ports.NativeGroupRecords {
	return nativeFixtureGroup{groups: groups}
}

func (r nativeFixtureGroup) Create(ctx context.Context, value *timetable.Group) error {
	if value == nil {
		return createLegacyGroup(ctx, r.groups, nil)
	}
	row := legacyGroup(*value)
	if err := row.Validate(); err != nil {
		*value = CatalogGroup(row)
		return err
	}
	created, err := r.groups.CreateGroup(ctx, publicGroupInput(row))
	if err != nil {
		*value = CatalogGroup(row)
		return fmt.Errorf("database error during create: %w", err)
	}
	*value = created
	return nil
}

func (r nativeFixtureGroup) Update(ctx context.Context, value *timetable.Group) error {
	if value == nil {
		return updateLegacyGroup(ctx, r.groups, nil)
	}
	row := legacyGroup(*value)
	defer func() { *value = CatalogGroup(row) }()
	if err := row.Validate(); err != nil {
		return err
	}
	if _, err := r.groups.UpdateGroup(ctx, row.ID, publicGroupInput(row)); err != nil {
		return legacyGroupReadError("update", err)
	}
	return nil
}

func (r nativeFixtureGroup) FindByID(ctx context.Context, id any) (*timetable.Group, error) {
	groupID, ok := LegacyGroupID(id)
	if !ok {
		return nil, legacyDatabaseError("find by id", fmt.Errorf("invalid activity record id %T", id))
	}
	group, err := r.groups.FindGroup(ctx, groupID)
	if err != nil {
		return nil, legacyGroupReadError("find by id", err)
	}
	return &group, nil
}
