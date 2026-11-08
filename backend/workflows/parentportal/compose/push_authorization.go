package compose

import (
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type DocumentPushAuthorizationDependencies struct {
	Runtime                      care.Runtime
	TraegerRuntime               care.TraegerRuntime
	Schools                      care.SchoolTraeger
	Accounts                     care.DocumentPushAccounts
	Relationships                care.DocumentPushRelationships
	Students                     care.DocumentPushStudents
	Billing, Contracts, Releases care.DocumentPushObjects
}

func NewDocumentPushAuthorization(deps DocumentPushAuthorizationDependencies) parentportal.DocumentPushAuthorization {
	if deps.Runtime == nil || deps.TraegerRuntime == nil || deps.Schools == nil || deps.Accounts == nil || deps.Relationships == nil || deps.Students == nil || deps.Billing == nil || deps.Contracts == nil || deps.Releases == nil {
		panic("parent portal: document push authorization requires all dependencies")
	}
	return &care.DocumentPushAuthorization{Runtime: deps.Runtime, TraegerRuntime: deps.TraegerRuntime, Schools: deps.Schools, Accounts: deps.Accounts, Relationships: deps.Relationships, Students: deps.Students, Billing: deps.Billing, Contracts: deps.Contracts, Releases: deps.Releases}
}
