package compose

import (
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type DocumentPushAuthorizationDependencies struct {
	Runtime        care.Runtime
	TraegerRuntime care.TraegerRuntime
	Schools        care.SchoolTraeger
	Recipients     care.DocumentPushRecipients
	Objects        care.DocumentPushObjects
}

func NewDocumentPushAuthorization(deps DocumentPushAuthorizationDependencies) parentportal.DocumentPushAuthorization {
	if deps.Runtime == nil || deps.TraegerRuntime == nil || deps.Schools == nil || deps.Recipients == nil || deps.Objects == nil {
		panic("parent portal: document push authorization requires all dependencies")
	}
	return &care.DocumentPushAuthorization{Runtime: deps.Runtime, TraegerRuntime: deps.TraegerRuntime, Schools: deps.Schools, Recipients: deps.Recipients, Objects: deps.Objects}
}
