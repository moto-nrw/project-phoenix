package compose

import (
	applicationBindings "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	internalports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

// StudentPhotoConsentSnapshot is the photo slice of a consent change the
// composition root records in the Audit Platform trail.
type StudentPhotoConsentSnapshot = internalports.StudentPhotoConsentSnapshot

// StudentPhotoRuntime is everything the photo lifecycle needs beyond the
// directory's own rows: the tenant feature flag (Settings Platform), the
// acting account (Identity), the stored-file cleanup and live refresh
// (Delivery), and the consent trail (Audit Platform). The composition root
// binds them, so the owner keeps its rules without importing four other
// owners.
type StudentPhotoRuntime interface {
	applicationBindings.StudentPhotoRuntime
}
