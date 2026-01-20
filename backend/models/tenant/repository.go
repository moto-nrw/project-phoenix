package tenant

import (
	"context"
)

// TraegerRepository defines the interface for traeger repository operations
type TraegerRepository interface {
	// Create inserts a new traeger into the database
	Create(ctx context.Context, traeger *Traeger) error

	// FindByID retrieves a traeger by its ID
	FindByID(ctx context.Context, id string) (*Traeger, error)

	// FindByName retrieves a traeger by its name
	FindByName(ctx context.Context, name string) (*Traeger, error)

	// Update updates an existing traeger
	Update(ctx context.Context, traeger *Traeger) error

	// Delete removes a traeger
	Delete(ctx context.Context, id string) error

	// List retrieves all traegers
	List(ctx context.Context) ([]*Traeger, error)

	// FindWithBueros retrieves a traeger with all its bueros loaded
	FindWithBueros(ctx context.Context, id string) (*Traeger, error)
}

// BueroRepository defines the interface for buero repository operations
type BueroRepository interface {
	// Create inserts a new buero into the database
	Create(ctx context.Context, buero *Buero) error

	// FindByID retrieves a buero by its ID
	FindByID(ctx context.Context, id string) (*Buero, error)

	// FindByTraegerID retrieves all bueros for a traeger
	FindByTraegerID(ctx context.Context, traegerID string) ([]*Buero, error)

	// Update updates an existing buero
	Update(ctx context.Context, buero *Buero) error

	// Delete removes a buero
	Delete(ctx context.Context, id string) error

	// List retrieves all bueros
	List(ctx context.Context) ([]*Buero, error)
}
