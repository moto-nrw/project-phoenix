package compose

import "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"

func NewPersonWrites(directory application.PersonWriteCapability) *application.RetainedPersonWrites {
	return application.NewRetainedPersonWrites(directory)
}
