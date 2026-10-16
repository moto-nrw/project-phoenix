package care

// CareOffering is the catalog information needed to render one booked
// offering. FixedDays chooses the catalog days over the booking's selection.
type CareOffering struct {
	ID                  int64
	Name                string
	Description         *string
	FixedDays           bool
	AvailableDays       []string
	IncludesHolidayCare bool
	IncludesLunch       bool
	PriceCents          *int
	SortOrder           int
}
