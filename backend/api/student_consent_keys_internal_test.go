package api

import (
	"testing"

	"github.com/stretchr/testify/assert"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// The trail stores the People Directory's consent keys as its discriminators.
func TestStudentConsentKeysAreTheTrailDiscriminators(t *testing.T) {
	t.Parallel()
	assert.Equal(t, auditModels.StudentConsentAGB, peopleModule.StudentConsentAGB)
	assert.Equal(t, auditModels.StudentConsentDataProcessing, peopleModule.StudentConsentDataProcessing)
	assert.Equal(t, auditModels.StudentConsentEmailContact, peopleModule.StudentConsentEmailContact)
	assert.Equal(t, auditModels.StudentConsentPhoto, peopleModule.StudentConsentPhoto)
}
