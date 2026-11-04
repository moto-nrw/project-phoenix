package communicationtest

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// Leaders is a fixed leadership directory (#5200): the persons who read the
// leadership conversations of the school in context. An empty value means
// nobody holds the leadership permission.
type Leaders []communication.ParentMessageLeader

// ParentMessageLeaders returns the fixed persons.
func (l Leaders) ParentMessageLeaders(context.Context) ([]communication.ParentMessageLeader, error) {
	return l, nil
}
