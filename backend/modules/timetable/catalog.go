package timetable

import "context"

// Catalog publishes the established activity catalog without exposing retained rows.
// Provisioning preserves the activity service's validation and error contract.
type Catalog interface {
	FindCatalogGroup(context.Context, int64) (*Group, error)
	ListCatalogGroups(context.Context, string) ([]Group, error)
	ListCatalogCategories(context.Context) ([]Category, error)
	CreateCatalogCategory(context.Context, CreateCategory) (Category, error)
	CreateCatalogGroup(context.Context, GroupInput) (Group, error)
	OccupancyQuery
}

// OccupiedGroup adds the current session occupancy to the owner's group value.
type OccupiedGroup struct {
	Group
	IsOccupied bool
}

// OccupancyQuery reads catalog groups with their live occupancy in one batch.
type OccupancyQuery interface {
	ListOccupiedGroups(context.Context) ([]OccupiedGroup, error)
}
