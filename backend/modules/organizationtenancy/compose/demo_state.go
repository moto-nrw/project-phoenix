package compose

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

func NewDemoSchools(db *bun.DB) (*organizationtenancy.DemoSchools, error) {
	if db == nil {
		return nil, fmt.Errorf("demo schools require a database")
	}
	return organizationtenancy.NewDemoSchools(postgres.DemoStateEngine{Store: postgres.NewDemoStateStore(db)}), nil
}
