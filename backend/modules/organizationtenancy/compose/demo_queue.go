package compose

import (
	"fmt"
	"io"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

// NewDemoSchoolQueue composes the demo process's side of the queue on its
// privileged connection, like NewDemoSchools.
func NewDemoSchoolQueue(db *bun.DB) (*organizationtenancy.DemoSchoolQueue, error) {
	if db == nil {
		return nil, fmt.Errorf("demo school queue requires a database")
	}
	return organizationtenancy.NewDemoSchoolQueue(postgres.DemoQueueEngine{Store: postgres.NewDemoStateStore(db)}), nil
}

// NewDemoSchoolOrders composes the serving backend's side. Every call needs
// the caller's administrative transaction in ctx. random is the root's secure
// source; it makes the slug suffix unguessable. maxActive is the number of
// active demo schools beyond which no order is queued (#3466).
func NewDemoSchoolOrders(random io.Reader, maxActive int) *organizationtenancy.DemoSchoolOrders {
	if random == nil || maxActive < 1 {
		panic("demo school orders require a random source and a positive capacity")
	}
	store := postgres.NewDemoOrderStore(postgres.NewDemoOrderDatabase())
	return organizationtenancy.NewDemoSchoolOrders(postgres.DemoOrderEngine{Store: store, Random: random, MaxActive: maxActive})
}
