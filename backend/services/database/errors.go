package database

import "errors"

var (
	// ErrNoPermissions is returned when user has no permissions to view any stats
	ErrNoPermissions = errors.New("no permissions to view database statistics")
	
	// ErrServiceUnavailable is returned when the database service is temporarily unavailable
	ErrServiceUnavailable = errors.New("database statistics service temporarily unavailable")
)