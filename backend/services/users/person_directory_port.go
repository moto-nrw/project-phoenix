package users

import (
	"context"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// The person write path moved to its owner in #3349. What stays here is the
// contract the retained handlers call; the validation, the tag and account
// conflicts and the soft delete belong to modules/peopledirectory, and the
// composition root binds an implementation
// (database/repositories.NewPersonDirectory).

// PersonWriter is the owner's person write path. Each call reflects the stored
// row back onto the caller's struct, so a handler that renders the person it
// just wrote renders what was actually persisted.
type PersonWriter interface {
	CreatePerson(ctx context.Context, person *userModels.Person) error
	UpdatePerson(ctx context.Context, person *userModels.Person) error
	// DeletePerson soft-deletes the row; the person keeps existing for every
	// record that references them.
	DeletePerson(ctx context.Context, id int64) error
}
