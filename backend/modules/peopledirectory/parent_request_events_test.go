package peopledirectory_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/stretchr/testify/assert"
)

func TestValidateParentRequestEvent(t *testing.T) {
	t.Parallel()
	studentID, requestID := int64(71), int64(72)
	assert.NoError(t, peopledirectory.ValidateParentRequestEvent(studentID, "offering", requestID, "submitted"))
	for name, err := range map[string]error{
		"student":    peopledirectory.ValidateParentRequestEvent(0, "offering", requestID, "submitted"),
		"type":       peopledirectory.ValidateParentRequestEvent(studentID, "", requestID, "submitted"),
		"request":    peopledirectory.ValidateParentRequestEvent(studentID, "offering", 0, "submitted"),
		"event type": peopledirectory.ValidateParentRequestEvent(studentID, "offering", requestID, ""),
	} {
		assert.ErrorIs(t, err, peopledirectory.ErrParentRequestEventIncomplete, name)
	}
	assert.EqualError(t, peopledirectory.ErrParentRequestEventIncomplete, "parent request event: student, request and event type are required")
}
