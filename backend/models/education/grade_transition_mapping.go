package education

// GradeTransitionMapping is the persistence shape of one class rename of a
// draft; a NULL target graduates the class. The owner capability lives in
// modules/schoolstructure (#2711); this struct remains for row fixtures.
type GradeTransitionMapping struct {
	Model
	TenantModel
	TransitionID int64   `bun:"transition_id,notnull" json:"transition_id"`
	FromClass    string  `bun:"from_class,notnull" json:"from_class"`
	ToClass      *string `bun:"to_class" json:"to_class,omitempty"` // NULL = graduate/delete
}
