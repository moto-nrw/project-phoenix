package domain

// AvatarCleanup is a durable deletion intent, independent of any school.
type AvatarCleanup struct {
	ID     int64
	Avatar string
}
