package importapi

import (
	"context"
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/dataimport"
)

type Middleware = func(http.Handler) http.Handler

// Failure preserves the response envelope without coupling import handlers to
// the common renderer or its implementation types.
type Failure struct {
	Status  int
	Cause   error
	Message string
	Code    string
	// Details are the values a coded refusal names (#2517); Result marks a
	// batch failure and travels in details.result instead.
	Details map[string]any
	Result  any
}

// ChildQuota is the school's Kinderkontingent (Booked) next to its
// Kontingentzahl (Occupied) of today (#3571). Free is what it still takes;
// Admit returns the owner's refusal when requested new children do not fit,
// nil otherwise.
type ChildQuota struct {
	Booked   int
	Occupied int
	Free     int
	Admit    func(requested int) error
}

// Runtime binds transport authentication, actor lookup, transaction ownership,
// and response rendering at composition time. Handlers never open a database
// connection or interpret JWT implementation types themselves.
type Runtime struct {
	Middleware           []Middleware
	RequireAnyPermission func(...string) Middleware
	TemplateTransaction  Middleware
	WithinTenant         func(context.Context, func(context.Context) error) error
	TenantID             func(context.Context) int64
	AccountID            func(context.Context) (int64, error)
	Permissions          func(context.Context) []string
	StaffID              func(context.Context) (int64, error)
	OpeningDecider       func(context.Context, int64) (int64, error)
	ValidateOpeningDate  func(string) (string, error)
	OpeningImport        func(string, string, int64) (dataimport.RowImporter[dataimport.OpeningBalanceImportRow], error)
	// ChildQuota reads the Kinderkontingent of the tenant in context; limited
	// is false for a school without one.
	ChildQuota func(context.Context) (quota ChildQuota, limited bool, err error)
	Success    func(http.ResponseWriter, *http.Request, int, any, string)
	Failure    func(http.ResponseWriter, *http.Request, Failure)
}

type Dependencies struct {
	Students  dataimport.RowImporter[dataimport.StudentImportRow]
	Staff     dataimport.RowImporter[dataimport.StaffImportRow]
	ClassList dataimport.RowImporter[dataimport.ClassListEntryImportRow]
	Files     dataimport.FileDecoder
	Runtime   Runtime
}
