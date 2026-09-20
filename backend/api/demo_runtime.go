package api

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationcompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presencecompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/uptrace/bun"
)

type DemoSchoolRecord struct {
	SchoolID int64
	SeedJSON []byte
}

type DemoVisit struct {
	StudentID   int64
	Active, Web bool
	ChangedAt   time.Time
}

// DemoRuntime composes only the two owner capabilities needed by the sidecar.
// It deliberately does not construct the retained API/service factory graph.
type DemoRuntime struct {
	schools      *organizationtenancy.DemoSchools
	presence     *studentpresence.Module
	transactions services.TenantRuntime
}

func NewDemoRuntime(db *bun.DB) (*DemoRuntime, error) {
	schools, err := organizationcompose.NewDemoSchools(db)
	if err != nil {
		return nil, err
	}
	presence, err := presencecompose.New(presencecompose.Dependencies{DB: db, Observe: func(presencecompose.Observation) {}})
	if err != nil {
		return nil, err
	}
	pgRuntime, err := database.NewDemoPostgresUnitOfWork(db, func(context.Context, time.Duration) {})
	if err != nil {
		return nil, err
	}
	transactions, err := services.BindTenantRuntime(pgRuntime.WithinTenant, pgRuntime.WithinAdmin, pgRuntime, func(error) bool { return false })
	if err != nil {
		return nil, err
	}
	return &DemoRuntime{schools: schools, presence: presence, transactions: transactions}, nil
}

func (d *DemoRuntime) LoadDemoSchool(ctx context.Context, name string) (*DemoSchoolRecord, error) {
	state, err := d.schools.LoadDemoSchool(ctx, name)
	if state == nil || err != nil {
		return nil, err
	}
	return &DemoSchoolRecord{SchoolID: state.SchoolID, SeedJSON: state.SeedJSON}, nil
}

func (d *DemoRuntime) RememberDemoSchool(ctx context.Context, name string, state DemoSchoolRecord) error {
	return d.schools.RememberDemoSchool(ctx, name, organizationtenancy.DemoSchoolState{SchoolID: state.SchoolID, SeedJSON: state.SeedJSON})
}

func (d *DemoRuntime) WithDemoLease(ctx context.Context, name string, run func(context.Context) error) error {
	return d.schools.WithDemoLease(ctx, name, run)
}

func (d *DemoRuntime) TenantRuntime() services.TenantRuntime { return d.transactions }

// LatestDemoVisits translates the owner snapshot for the demo CLI.
// The caller must provide its tenant-scoped transaction context.
func (d *DemoRuntime) LatestDemoVisits(ctx context.Context, webDeviceID int64, fromDate, untilDate string) ([]DemoVisit, error) {
	rows, err := d.presence.LatestDemoVisits(ctx, webDeviceID, fromDate, untilDate)
	if err != nil {
		return nil, err
	}
	result := make([]DemoVisit, 0, len(rows))
	for _, row := range rows {
		result = append(result, DemoVisit{StudentID: row.StudentID, Active: row.Active, Web: row.Web, ChangedAt: row.ChangedAt})
	}
	return result, nil
}
