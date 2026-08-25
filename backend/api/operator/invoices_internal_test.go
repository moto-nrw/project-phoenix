package operator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	platformModel "github.com/moto-nrw/project-phoenix/models/platform"
	billingSvc "github.com/moto-nrw/project-phoenix/services/billing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fake billing service ---------------------------------------------------

type fakeBillingService struct {
	listResult []billingSvc.InvoiceView
	view       *billingSvc.InvoiceView
	err        error

	gotTenantID  int64
	gotInvoiceID int64
	gotInput     billingSvc.InvoiceInput
	deleteCalls  int
}

func (f *fakeBillingService) GetOverview(context.Context) (*billingSvc.ContractOverview, error) {
	return nil, errors.New("not used by the operator surface")
}

func (f *fakeBillingService) ListInvoices(_ context.Context, tenantID int64) ([]billingSvc.InvoiceView, error) {
	f.gotTenantID = tenantID
	return f.listResult, f.err
}

func (f *fakeBillingService) CreateInvoice(_ context.Context, tenantID int64, input billingSvc.InvoiceInput) (*billingSvc.InvoiceView, error) {
	f.gotTenantID = tenantID
	f.gotInput = input
	return f.view, f.err
}

func (f *fakeBillingService) UpdateInvoice(_ context.Context, tenantID, invoiceID int64, input billingSvc.InvoiceInput) (*billingSvc.InvoiceView, error) {
	f.gotTenantID = tenantID
	f.gotInvoiceID = invoiceID
	f.gotInput = input
	return f.view, f.err
}

func (f *fakeBillingService) DeleteInvoice(_ context.Context, tenantID, invoiceID int64) error {
	f.gotTenantID = tenantID
	f.gotInvoiceID = invoiceID
	f.deleteCalls++
	return f.err
}

// invoiceRouter wires the resource exactly as mountSchoolRoutes does, so the
// tests exercise the real URL parameters instead of a hand-stuffed context.
func invoiceRouter(service billingSvc.Service) chi.Router {
	resource := NewInvoicesResource(service)
	r := chi.NewRouter()
	r.Route("/schools/{id}/invoices", func(r chi.Router) {
		r.Get("/", resource.ListSchoolInvoices)
		r.Post("/", resource.CreateSchoolInvoice)
		r.Put("/{invoiceId}", resource.UpdateSchoolInvoice)
		r.Delete("/{invoiceId}", resource.DeleteSchoolInvoice)
	})
	return r
}

func doInvoiceRequest(t *testing.T, service billingSvc.Service, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	invoiceRouter(service).ServeHTTP(rec, req)
	return rec
}

func validInvoiceBody() map[string]any {
	return map[string]any{
		"period_label":   "Januar 2026",
		"invoice_number": "R-2026-001",
		"amount_cents":   19900,
		"due_date":       "2026-01-31",
		"status":         "offen",
		"note":           "",
	}
}

func sampleView() *billingSvc.InvoiceView {
	return &billingSvc.InvoiceView{
		ID:          7,
		PeriodLabel: "Januar 2026",
		AmountCents: 19900,
		DueDate:     timezone.NewDate(2026, time.January, 31),
		Status:      platformModel.InvoiceStatusOpen,
	}
}

// --- list -------------------------------------------------------------------

func TestListSchoolInvoices_ReturnsSchedule(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{listResult: []billingSvc.InvoiceView{*sampleView()}}

	rec := doInvoiceRequest(t, service, http.MethodGet, "/schools/42/invoices/", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int64(42), service.gotTenantID)
	assert.Contains(t, rec.Body.String(), "Januar 2026")
	assert.Contains(t, rec.Body.String(), "2026-01-31", "dates travel as calendar days")
}

func TestListSchoolInvoices_RejectsNonNumericSchoolID(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{}

	rec := doInvoiceRequest(t, service, http.MethodGet, "/schools/abc/invoices/", nil)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Zero(t, service.gotTenantID)
}

func TestListSchoolInvoices_ServiceFailureIs500(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{err: errors.New("db down")}

	rec := doInvoiceRequest(t, service, http.MethodGet, "/schools/42/invoices/", nil)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- create -----------------------------------------------------------------

func TestCreateSchoolInvoice_Returns201AndParsesPayload(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{view: sampleView()}

	rec := doInvoiceRequest(t, service, http.MethodPost, "/schools/42/invoices/", validInvoiceBody())

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, int64(42), service.gotTenantID)
	assert.Equal(t, "Januar 2026", service.gotInput.PeriodLabel)
	assert.Equal(t, "R-2026-001", service.gotInput.InvoiceNumber)
	assert.Equal(t, int64(19900), service.gotInput.AmountCents)
	assert.Equal(t, "2026-01-31", service.gotInput.DueDate.String())
	assert.Nil(t, service.gotInput.PaidOn)
}

func TestCreateSchoolInvoice_ParsesPaidOn(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{view: sampleView()}
	body := validInvoiceBody()
	body["status"] = "bezahlt"
	body["paid_on"] = "2026-02-03"

	rec := doInvoiceRequest(t, service, http.MethodPost, "/schools/42/invoices/", body)

	assert.Equal(t, http.StatusCreated, rec.Code)
	require.NotNil(t, service.gotInput.PaidOn)
	assert.Equal(t, "2026-02-03", service.gotInput.PaidOn.String())
}

func TestCreateSchoolInvoice_RejectsMalformedJSON(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{}
	req := httptest.NewRequest(http.MethodPost, "/schools/42/invoices/", bytes.NewReader([]byte("{oops")))
	rec := httptest.NewRecorder()
	invoiceRouter(service).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateSchoolInvoice_RejectsMissingDueDate(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{}
	body := validInvoiceBody()
	body["due_date"] = ""

	rec := doInvoiceRequest(t, service, http.MethodPost, "/schools/42/invoices/", body)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Fälligkeitsdatum")
}

// A German date must be rejected, not silently reinterpreted: "01.02.2026"
// parsed as a US date would bill the school a month early.
func TestCreateSchoolInvoice_RejectsNonISODueDate(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{}
	body := validInvoiceBody()
	body["due_date"] = "31.01.2026"

	rec := doInvoiceRequest(t, service, http.MethodPost, "/schools/42/invoices/", body)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "JJJJ-MM-TT")
}

func TestCreateSchoolInvoice_RejectsNonISOPaidOn(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{}
	body := validInvoiceBody()
	body["paid_on"] = "gestern"

	rec := doInvoiceRequest(t, service, http.MethodPost, "/schools/42/invoices/", body)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Zahlungsdatum")
}

func TestCreateSchoolInvoice_ValidationErrorIs400(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{err: platformModel.ErrInvoicePeriodLabelRequired}

	rec := doInvoiceRequest(t, service, http.MethodPost, "/schools/42/invoices/", validInvoiceBody())

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateSchoolInvoice_DuplicateNumberIs409(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{err: billingSvc.ErrInvoiceNumberTaken}

	rec := doInvoiceRequest(t, service, http.MethodPost, "/schools/42/invoices/", validInvoiceBody())

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestCreateSchoolInvoice_RejectsNonNumericSchoolID(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{}

	rec := doInvoiceRequest(t, service, http.MethodPost, "/schools/abc/invoices/", validInvoiceBody())

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- update -----------------------------------------------------------------

func TestUpdateSchoolInvoice_PassesBothIDs(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{view: sampleView()}

	rec := doInvoiceRequest(t, service, http.MethodPut, "/schools/42/invoices/7", validInvoiceBody())

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int64(42), service.gotTenantID)
	assert.Equal(t, int64(7), service.gotInvoiceID)
}

func TestUpdateSchoolInvoice_RejectsBadSchoolIDBeforeReadingTheBody(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{}

	rec := doInvoiceRequest(t, service, http.MethodPut, "/schools/abc/invoices/7", validInvoiceBody())

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Zero(t, service.gotInvoiceID)
}

func TestUpdateSchoolInvoice_RejectsMalformedJSON(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{}
	req := httptest.NewRequest(http.MethodPut, "/schools/42/invoices/7", bytes.NewReader([]byte("{oops")))
	rec := httptest.NewRecorder()
	invoiceRouter(service).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Zero(t, service.gotInvoiceID)
}

func TestUpdateSchoolInvoice_UnknownInvoiceIs404(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{err: billingSvc.ErrInvoiceNotFound}

	rec := doInvoiceRequest(t, service, http.MethodPut, "/schools/42/invoices/7", validInvoiceBody())

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUpdateSchoolInvoice_RejectsNonNumericInvoiceID(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{}

	rec := doInvoiceRequest(t, service, http.MethodPut, "/schools/42/invoices/abc", validInvoiceBody())

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Zero(t, service.gotInvoiceID)
}

// --- delete -----------------------------------------------------------------

func TestDeleteSchoolInvoice_Returns204(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{}

	rec := doInvoiceRequest(t, service, http.MethodDelete, "/schools/42/invoices/7", nil)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, 1, service.deleteCalls)
	assert.Equal(t, int64(42), service.gotTenantID)
	assert.Equal(t, int64(7), service.gotInvoiceID)
}

func TestDeleteSchoolInvoice_UnknownInvoiceIs404(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{err: billingSvc.ErrInvoiceNotFound}

	rec := doInvoiceRequest(t, service, http.MethodDelete, "/schools/42/invoices/7", nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDeleteSchoolInvoice_RejectsNonNumericIDs(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{}

	assert.Equal(t, http.StatusBadRequest,
		doInvoiceRequest(t, service, http.MethodDelete, "/schools/abc/invoices/7", nil).Code)
	assert.Equal(t, http.StatusBadRequest,
		doInvoiceRequest(t, service, http.MethodDelete, "/schools/42/invoices/xyz", nil).Code)
	assert.Zero(t, service.deleteCalls)
}

// --- helpers ----------------------------------------------------------------

func TestParseInvoiceDate(t *testing.T) {
	t.Parallel()

	parsed, err := parseInvoiceDate("2026-01-31")
	require.NoError(t, err)
	require.NotNil(t, parsed)
	assert.Equal(t, "2026-01-31", parsed.String())

	trimmed, err := parseInvoiceDate("  2026-01-31  ")
	require.NoError(t, err)
	require.NotNil(t, trimmed)
	assert.Equal(t, "2026-01-31", trimmed.String())

	empty, err := parseInvoiceDate("")
	require.NoError(t, err)
	assert.Nil(t, empty, "an empty value clears the date instead of failing")

	blank, err := parseInvoiceDate("   ")
	require.NoError(t, err)
	assert.Nil(t, blank)

	_, err = parseInvoiceDate("31.01.2026")
	assert.Error(t, err)
}

func TestIsInvoiceValidationError(t *testing.T) {
	t.Parallel()

	for _, sentinel := range []error{
		platformModel.ErrInvoicePeriodLabelRequired,
		platformModel.ErrInvoiceAmountNegative,
		platformModel.ErrInvoiceDueDateRequired,
		platformModel.ErrInvoiceUnknownStatus,
		platformModel.ErrInvoicePaidOnRequired,
		platformModel.ErrInvoicePaidOnNotAllowed,
	} {
		assert.Truef(t, isInvoiceValidationError(sentinel), "%v should be a 400", sentinel)
	}

	assert.False(t, isInvoiceValidationError(errors.New("db down")))
	assert.False(t, isInvoiceValidationError(billingSvc.ErrInvoiceNotFound))
}

func TestNewInvoicesResource(t *testing.T) {
	t.Parallel()

	service := &fakeBillingService{}
	resource := NewInvoicesResource(service)

	require.NotNil(t, resource)
	assert.Equal(t, service, resource.service)
}
