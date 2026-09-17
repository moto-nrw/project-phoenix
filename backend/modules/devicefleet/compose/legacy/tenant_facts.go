package legacy

import (
	"context"
	"fmt"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	configService "github.com/moto-nrw/project-phoenix/services/config"
)

// SchoolDirectory reads the display facts of one school. Organisation &
// Tenancy owns the row; the root binds this port to its capability.
type SchoolDirectory interface {
	// FindDisplaySchool reports the school's name and lifecycle state; found
	// is false when the school does not exist.
	FindDisplaySchool(ctx context.Context, tenantID int64) (school compose.School, found bool, err error)
}

// TenantFactDependencies are the readers that answer the pre-tenant questions
// a display token raises.
type TenantFactDependencies struct {
	Schools  SchoolDirectory
	Settings configService.SettingsService
}

type tenantFacts struct{ deps TenantFactDependencies }

// NewTenantFacts adapts the school and settings readers to the owner's port.
func NewTenantFacts(deps TenantFactDependencies) compose.TenantFacts {
	if deps.Schools == nil || deps.Settings == nil {
		panic("devicefleet tenant facts: all dependencies are required")
	}
	return tenantFacts{deps: deps}
}

// School reports the tenant's name and lifecycle state. A school that cannot
// be found is reported as an unknown display, so a dead link never reveals
// whether the tenant ever existed.
func (f tenantFacts) School(ctx context.Context, tenantID int64) (compose.School, error) {
	school, found, err := f.deps.Schools.FindDisplaySchool(ctx, tenantID)
	if err != nil {
		return compose.School{}, fmt.Errorf("failed to load display school: %w", err)
	}
	if !found {
		return compose.School{}, compose.ErrDisplayNotFound
	}
	return school, nil
}

// DisplayEnabled resolves the opt-in display.enabled toggle for one tenant.
func (f tenantFacts) DisplayEnabled(ctx context.Context, tenantID int64) (bool, error) {
	return f.deps.Settings.ResolveBoolForTenant(ctx, tenantID, configModel.KeyDisplayEnabled)
}
