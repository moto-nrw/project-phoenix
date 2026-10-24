package workforce_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

func TestNewOffboardingAbsenceDeletionKeepsTheDeletedRowAsPayload(t *testing.T) {
	t.Parallel()
	staffID, absenceID, actorID := int64(41), int64(73), int64(29)
	absence := workforce.StaffAbsence{ID: absenceID, StaffID: staffID, AbsenceType: "vacation", DateStart: "2026-10-12", DateEnd: "2026-10-16"}

	deletion, err := workforce.NewOffboardingAbsenceDeletion(absence, actorID)

	require.NoError(t, err)
	payload, err := json.Marshal(absence)
	require.NoError(t, err)
	assert.Equal(t, workforce.OffboardingAbsenceDeletion{
		StaffID: staffID, AbsenceID: absenceID, DeletedBy: actorID, Payload: payload, Note: "Personal-Offboarding",
	}, deletion)
}
