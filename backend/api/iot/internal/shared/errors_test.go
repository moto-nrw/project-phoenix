package shared_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	shared "github.com/moto-nrw/project-phoenix/api/iot/internal/shared"
)

func TestErrorMessageConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "person is not a student", shared.ErrMsgPersonNotStudent)
	assert.Equal(t, "RFID tag not found", shared.ErrMsgRFIDTagNotFound)
	assert.Equal(t, "rfid_tag_not_found", shared.ErrCodeRFIDTagNotFound)
}
