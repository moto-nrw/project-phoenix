package timetable

import (
	"testing"

	"github.com/stretchr/testify/assert"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// The Timetable owner may not name the Audit Platform, so it mirrors the
// vocabulary its trails store (#3424 slice S3). The ports pass the values
// through unchanged; this pins them to the Audit Platform's own constants.
func TestTimetableAuditVocabularyMatchesTheAuditPlatform(t *testing.T) {
	t.Parallel()

	assert.Equal(t, auditModels.DeviationEventAbsence, timetableCompose.DeviationEventAbsence)
	assert.Equal(t, auditModels.DeviationEventReturnToPresence, timetableCompose.DeviationEventReturnToPresence)
	assert.Equal(t, auditModels.DeviationEventSubstitution, timetableCompose.DeviationEventSubstitution)
	assert.Equal(t, auditModels.DeviationEventSubstituteRemoved, timetableCompose.DeviationEventSubstituteRemoved)
	assert.Equal(t, auditModels.DeviationEventSickReported, timetableCompose.DeviationEventSickReported)
	assert.Equal(t, auditModels.DeviationEventSickCleared, timetableCompose.DeviationEventSickCleared)

	assert.Equal(t, auditModels.AttendanceFieldStatus, timetableModule.AttendanceCorrectionFieldStatus)
	assert.Equal(t, auditModels.AttendanceFieldSubstatus, timetableModule.AttendanceCorrectionFieldSubstatus)
	assert.Equal(t, auditModels.AttendanceFieldNote, timetableModule.AttendanceCorrectionFieldNote)
	assert.Equal(t, auditModels.CorrectionReasonMaxLength, timetableModule.AttendanceCorrectionReasonMaxLength)
}
