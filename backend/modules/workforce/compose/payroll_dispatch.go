package compose

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// The port vocabulary of the payroll dispatch (#4805) the root binds.
type (
	PayrollDispatchConfig   = application.PayrollDispatchConfig
	PayrollDispatchSettings = application.PayrollDispatchSettings
	PayrollExport           = application.PayrollExport
	PayrollTransfer         = application.PayrollTransfer
	PayrollTransferRequest  = application.PayrollTransferRequest
	PayrollTransferOutcome  = application.PayrollTransferOutcome
	PayrollEmailResult      = application.PayrollEmailResult
	PayrollMail             = application.PayrollMail
	PayrollMailMessage      = application.PayrollMailMessage
	PayrollSchools          = application.PayrollSchools
	PayrollSchool           = application.PayrollSchool
	PayrollManagers         = application.PayrollManagers
	PayrollContact          = application.PayrollContact
	PayrollTokens           = application.PayrollTokens
)

// Mail states the root's PayrollMail answers.
const (
	PayrollMailPending = application.PayrollMailPending
	PayrollMailSent    = application.PayrollMailSent
	PayrollMailFailed  = application.PayrollMailFailed
)

// PayrollDispatchDependencies wire the automatic send of the month file.
// Every dependency is required.
type PayrollDispatchDependencies struct {
	DB       *bun.DB
	Settings PayrollDispatchSettings
	Export   PayrollExport
	Transfer PayrollTransfer
	Mail     PayrollMail
	Schools  PayrollSchools
	Managers PayrollManagers
	Tokens   PayrollTokens
	// FrontendURL is the base of the download link.
	FrontendURL string
	Logger      *slog.Logger
	// Now is optional and pins the clock in tests.
	Now func() time.Time
}

// NewPayrollDispatch builds the payroll dispatch over its own table and the
// tenant runtime: every step of a pass commits in its own transaction.
func NewPayrollDispatch(deps PayrollDispatchDependencies) (workforce.PayrollDispatch, error) {
	if deps.DB == nil {
		return nil, errors.New("payroll dispatch: database is required")
	}
	return application.NewPayrollDispatch(application.PayrollDispatchDependencies{
		Store:    postgres.New(postgres.DatabaseRuntime(deps.DB)),
		Settings: deps.Settings, Export: deps.Export, Transfer: deps.Transfer,
		Mail: deps.Mail, Schools: deps.Schools, Managers: deps.Managers, Tokens: deps.Tokens,
		FrontendURL: deps.FrontendURL, Now: deps.Now, Logger: deps.Logger,
		Runtime: application.PayrollRuntime{
			TenantID: func(ctx context.Context) (int64, error) {
				id, err := tenant.TenantFromContext(ctx)
				return id.Int64(), err
			},
			Within: tenant.WithinCurrentTenant,
			WithinTenant: func(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
				id, err := tenant.NewTenantID(tenantID)
				if err != nil {
					return err
				}
				return tenant.WithinTenant(ctx, id, fn)
			},
			Detached: func(ctx context.Context) context.Context {
				return tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx))
			},
		},
	})
}
