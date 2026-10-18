package facilities

import "context"

type SystemActivity struct {
	ID              int64
	Name            string
	MaxParticipants int
	IsOpen          bool
	CategoryID      int64
	PlannedRoomID   *int64
	IsSystem        bool
	CreatedBy       *int64
}

type SystemCategory struct {
	ID          int64
	Name        string
	Description string
	Color       string
	IsSystem    bool
}

type ActivityCatalog interface {
	ListActivities(context.Context, string) ([]SystemActivity, error)
	CreateActivity(context.Context, SystemActivity) (SystemActivity, error)
	ListCategories(context.Context) ([]SystemCategory, error)
	CreateCategory(context.Context, SystemCategory) (SystemCategory, error)
}
