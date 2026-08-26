package billing

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// settingsResolver is the slice of config.SettingsService this package needs.
// Narrow on purpose: a fake in a unit test implements three methods, not forty.
type settingsResolver interface {
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// studentCounter is the slice of the student repository this package needs:
// how many children the school currently has on its books.
type studentCounter interface {
	CountWithOptions(ctx context.Context, options *modelBase.QueryOptions) (int, error)
}

// Service is the contract surface: one tenant read, four operator writes.
type Service interface {
	// GetOverview builds the complete /vertrag payload for the tenant in the
	// ambient tenant transaction.
	GetOverview(ctx context.Context) (*ContractOverview, error)

	// ListInvoices returns one school's payment schedule for the operator.
	ListInvoices(ctx context.Context, tenantID int64) ([]InvoiceView, error)

	// CreateInvoice appends one billing period to a school's schedule.
	CreateInvoice(ctx context.Context, tenantID int64, input InvoiceInput) (*InvoiceView, error)

	// UpdateInvoice replaces every editable field of one invoice. This is how
	// an operator marks a payment received.
	UpdateInvoice(ctx context.Context, tenantID, invoiceID int64, input InvoiceInput) (*InvoiceView, error)

	// DeleteInvoice removes an invoice that should never have been there.
	// A withdrawn-but-real invoice belongs in status "storniert" instead.
	DeleteInvoice(ctx context.Context, tenantID, invoiceID int64) error
}

type service struct {
	invoices platform.SchoolInvoiceRepository
	students studentCounter
	settings settingsResolver
	db       *bun.DB
	logger   *slog.Logger
	// now is the reference-day source, injectable so tests can pin "today"
	// instead of racing the clock across midnight.
	now func() timezone.Date
}

// Config carries the service dependencies.
type Config struct {
	Invoices platform.SchoolInvoiceRepository
	Students studentCounter
	Settings settingsResolver
	DB       *bun.DB
	Logger   *slog.Logger
	// Now overrides the reference day. Nil means timezone.TodayDate.
	Now func() timezone.Date
}

// NewService builds the billing service.
func NewService(cfg Config) Service {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := cfg.Now
	if now == nil {
		now = timezone.TodayDate
	}
	return &service{
		invoices: cfg.Invoices,
		students: cfg.Students,
		settings: cfg.Settings,
		db:       cfg.DB,
		logger:   logger,
		now:      now,
	}
}

// GetOverview assembles the read surface. Every piece degrades independently:
// a missing settings row yields the registry default, and a failing child
// count must not hide the payment schedule — the number the school actually
// came for. Only a failing invoice read is fatal, because an overview that
// silently omits invoices would read as "nothing to pay".
func (s *service) GetOverview(ctx context.Context) (*ContractOverview, error) {
	if tenant.FromContext(ctx) == 0 {
		return nil, ErrNoTenantContext
	}

	today := s.now()

	invoices, err := s.invoices.ListForTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("billing: list invoices: %w", err)
	}
	views := ToSchoolViews(invoices, today)
	openCents, nextDue := summarizeInvoices(views)

	overview := &ContractOverview{
		ReferenceDate:   today,
		Invoices:        views,
		OpenAmountCents: openCents,
		NextDue:         nextDue,
	}

	s.fillContractFacts(ctx, overview)
	overview.ActiveChildren = s.countActiveChildren(ctx)
	overview.TierLabel = TierLabel(overview.Tier)
	overview.BillingCycleLabel = BillingCycleLabel(overview.BillingCycle)
	overview.Configured = isConfigured(overview)

	return overview, nil
}

// isConfigured reports whether the moto team has entered anything at all.
// Invoices alone count: a school can have received a bill before its tier was
// recorded, and showing "kein Vertrag hinterlegt" above a list of invoices
// would be the exact contradiction the Verständlichkeit rule exists to stop.
func isConfigured(o *ContractOverview) bool {
	return o.Tier != configModel.ContractTierUnset ||
		o.BookedChildren > 0 ||
		o.PricePerChildCents > 0 ||
		o.BillingCycle != configModel.ContractCycleUnset ||
		o.TermStart != nil ||
		o.TermEnd != nil ||
		o.InvoiceRecipient != "" ||
		o.CustomerNumber != "" ||
		o.SupportEmail != "" ||
		o.Note != "" ||
		len(o.Invoices) > 0
}

// fillContractFacts resolves the vertrag.* settings onto the overview.
// A resolve failure is logged and left at the zero value rather than failing
// the request: the payment schedule stays readable either way.
func (s *service) fillContractFacts(ctx context.Context, o *ContractOverview) {
	if s.settings == nil {
		return
	}

	o.Tier = s.resolveString(ctx, configModel.KeyContractTier)
	o.BillingCycle = s.resolveString(ctx, configModel.KeyContractBillingCycle)
	o.InvoiceRecipient = s.resolveString(ctx, configModel.KeyContractInvoiceRecipient)
	o.CustomerNumber = s.resolveString(ctx, configModel.KeyContractCustomerNumber)
	o.SupportEmail = s.resolveString(ctx, configModel.KeyContractSupportEmail)
	o.Note = s.resolveString(ctx, configModel.KeyContractNote)
	o.BookedChildren = s.resolveInt(ctx, configModel.KeyContractBookedChildren)
	o.PricePerChildCents = s.resolveInt(ctx, configModel.KeyContractPricePerChildCents)
	o.TermStart = s.resolveDate(ctx, configModel.KeyContractTermStart)
	o.TermEnd = s.resolveDate(ctx, configModel.KeyContractTermEnd)
}

func (s *service) resolveString(ctx context.Context, key string) string {
	value, err := s.settings.ResolveString(ctx, key)
	if err != nil {
		s.logger.Warn("billing: contract setting unreadable",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		return ""
	}
	return value
}

func (s *service) resolveInt(ctx context.Context, key string) int {
	value, err := s.settings.ResolveInt(ctx, key)
	if err != nil {
		s.logger.Warn("billing: contract setting unreadable",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		return 0
	}
	if value < 0 {
		return 0
	}
	return value
}

// resolveDate parses a FieldDate setting, which is stored as a "YYYY-MM-DD"
// string. An unparseable value is treated as unset and logged — showing a
// wrong contract date is worse than showing none.
func (s *service) resolveDate(ctx context.Context, key string) *timezone.Date {
	raw := s.resolveString(ctx, key)
	if raw == "" {
		return nil
	}
	parsed, err := timezone.ParseDate(raw)
	if err != nil {
		s.logger.Warn("billing: contract date setting is not a calendar date",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return &parsed
}

// countActiveChildren returns how many children the school currently has in
// status "active". Pending, inactive and alumni are excluded: the contingent
// is about children being cared for today, and counting a graduate against it
// would make the number impossible to reconcile with the Kinderliste.
//
// A failure yields 0 and a log line; the page labels the number as a snapshot
// and the rest of the overview is unaffected.
func (s *service) countActiveChildren(ctx context.Context) int {
	if s.students == nil {
		return 0
	}
	options := modelBase.NewQueryOptions()
	options.Filter.Equal("status", string(userModels.StudentStatusActive))

	count, err := s.students.CountWithOptions(ctx, options)
	if err != nil {
		s.logger.Warn("billing: active child count failed",
			slog.String("error", err.Error()),
		)
		return 0
	}
	return count
}

// ListInvoices reads one school's schedule from the operator side. The tenant
// transaction is opened here because operator requests carry no tenant
// context of their own — the same mechanism the operator settings editor uses.
func (s *service) ListInvoices(ctx context.Context, tenantID int64) ([]InvoiceView, error) {
	today := s.now()
	var views []InvoiceView

	err := s.inTenantTx(ctx, tenantID, func(ctx context.Context) error {
		invoices, err := s.invoices.ListForTenant(ctx)
		if err != nil {
			return fmt.Errorf("billing: list invoices: %w", err)
		}
		views = ToViews(invoices, today)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return views, nil
}

// CreateInvoice appends a billing period.
func (s *service) CreateInvoice(ctx context.Context, tenantID int64, input InvoiceInput) (*InvoiceView, error) {
	today := s.now()
	normalized := normalizeInput(input, today)

	invoice := &platform.SchoolInvoice{}
	invoice.SetTenantID(tenantID)
	applyInput(invoice, normalized)

	if err := invoice.Validate(); err != nil {
		return nil, err
	}

	err := s.inTenantTx(ctx, tenantID, func(ctx context.Context) error {
		return wrapWriteError("create invoice", s.invoices.Create(ctx, invoice))
	})
	if err != nil {
		return nil, err
	}

	view := toView(invoice, today)
	return &view, nil
}

// UpdateInvoice replaces the editable fields of one invoice. The row is read
// back inside the same transaction first so an id from another school reads as
// "not found" rather than writing across the tenant boundary.
func (s *service) UpdateInvoice(ctx context.Context, tenantID, invoiceID int64, input InvoiceInput) (*InvoiceView, error) {
	today := s.now()
	normalized := normalizeInput(input, today)

	var stored *platform.SchoolInvoice

	err := s.inTenantTx(ctx, tenantID, func(ctx context.Context) error {
		existing, err := s.invoices.FindByIDOrNil(ctx, invoiceID)
		if err != nil {
			return fmt.Errorf("billing: load invoice: %w", err)
		}
		if existing == nil {
			return ErrInvoiceNotFound
		}
		applyInput(existing, normalized)
		if validationErr := existing.Validate(); validationErr != nil {
			return validationErr
		}
		if updateErr := s.invoices.Update(ctx, existing); updateErr != nil {
			return wrapWriteError("update invoice", updateErr)
		}
		stored = existing
		return nil
	})
	if err != nil {
		return nil, err
	}

	view := toView(stored, today)
	return &view, nil
}

// DeleteInvoice removes an invoice row.
func (s *service) DeleteInvoice(ctx context.Context, tenantID, invoiceID int64) error {
	return s.inTenantTx(ctx, tenantID, func(ctx context.Context) error {
		existing, err := s.invoices.FindByIDOrNil(ctx, invoiceID)
		if err != nil {
			return fmt.Errorf("billing: load invoice: %w", err)
		}
		if existing == nil {
			return ErrInvoiceNotFound
		}
		if delErr := s.invoices.Delete(ctx, invoiceID); delErr != nil {
			return fmt.Errorf("billing: delete invoice: %w", delErr)
		}
		return nil
	})
}

// inTenantTx runs fn inside a tenant transaction for tenantID. When no *bun.DB
// was wired (unit tests with an in-memory repository) it runs fn against a
// context that merely carries the tenant id, so the repository fake still sees
// the right tenant.
func (s *service) inTenantTx(ctx context.Context, tenantID int64, fn func(ctx context.Context) error) error {
	if tenantID <= 0 {
		return ErrNoTenantContext
	}
	if s.db == nil {
		return fn(tenant.WithTenantID(ctx, tenantID))
	}
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return fn(ctx)
	})
}
