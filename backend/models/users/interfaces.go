package users

// GroupLike defines the minimum interface a Group must implement
// to be used as a Student's Group without creating import cycles
type GroupLike interface {
	// GetID returns the ID of the group
	GetID() int64

	// GetName returns the name of the group
	GetName() string
}
