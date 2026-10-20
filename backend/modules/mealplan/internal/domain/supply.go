package domain

import "time"

// Supply is a school's supply by its caterer (ADR 0075). A school has at most
// one running supply; ending it takes the school from the caterer at once.
type Supply struct {
	CatererID       int64
	DeliveryAddress string
	StartedAt       time.Time
}
