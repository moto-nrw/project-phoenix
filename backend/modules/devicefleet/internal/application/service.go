// Package application holds the Device Fleet owner's decisions: device
// lifecycle, info-point display lifecycle, and the public dashboard
// aggregate. It reads and writes iot.devices and display.displays only
// through its own ports.
package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/ports"
)

// Dependencies are the collaborators the composition root supplies.
type Dependencies struct {
	Devices   ports.DeviceStore
	Displays  ports.DisplayStore
	Rooms     ports.RoomDirectory
	Presence  ports.PresenceReader
	Dashboard ports.DashboardSources
	Tenants   ports.TenantFacts
	Tx        ports.Transaction
	Tokens    ports.TokenMinter
	APIKeys   ports.APIKeyMinter
	Now       ports.Clock
	Window    ports.WindowResolver
	Observe   ports.Observer
}

// Service is the Device Fleet application service.
type Service struct{ deps Dependencies }

// New validates and freezes the owner's dependency graph.
func New(deps Dependencies) *Service {
	if deps.Devices == nil || deps.Displays == nil || deps.Rooms == nil || deps.Presence == nil ||
		deps.Dashboard == nil || deps.Tenants == nil || deps.Tx == nil || deps.Tokens == nil ||
		deps.APIKeys == nil || deps.Now == nil || deps.Observe == nil {
		panic("devicefleet application: all dependencies are required")
	}
	return &Service{deps: deps}
}

// run records one operation's duration, statement counters, and outcome.
func (s *Service) run(ctx context.Context, operation string, body func(context.Context, *domain.OperationStats) error) error {
	stats := domain.OperationStats{}
	started := time.Now()
	err := body(ctx, &stats)
	s.deps.Observe(ports.Observation{
		Operation: operation, Duration: time.Since(started), Stats: stats, Err: err,
	})
	return err
}

// runWrite is run inside the caller's ambient tenant transaction, opening one
// when the caller has none.
func (s *Service) runWrite(ctx context.Context, operation string, body func(context.Context, *domain.OperationStats) error) error {
	return s.run(ctx, operation, func(ctx context.Context, stats *domain.OperationStats) error {
		return s.deps.Tx.RunWrite(ctx, func(txCtx context.Context) error {
			return body(txCtx, stats)
		})
	})
}

// Reject records a rejection the public facade made before any statement ran.
func (s *Service) Reject(operation string, err error) {
	s.deps.Observe(ports.Observation{Operation: operation, Err: err})
}
