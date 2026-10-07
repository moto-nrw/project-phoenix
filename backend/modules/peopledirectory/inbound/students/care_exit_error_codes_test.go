package students

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every care-lifecycle refusal answers with its own code, so the dialog words
// it from the catalog instead of a class sentence (ADR 0006, #2513).
func TestCareExitErrorRendererAnswersWithCodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		err    error
		status int
		code   string
	}{
		{careplan.ErrCareExitNoStudents, http.StatusBadRequest, common.CodeStudentsCareExitNoStudents},
		{careplan.ErrCareExitTooManyStudents, http.StatusBadRequest, common.CodeStudentsCareExitTooManyStudents},
		{careplan.ErrCareExitDayInPast, http.StatusBadRequest, common.CodeStudentsCareExitDayInPast},
		{careplan.ErrCareExitPreviewChanged, http.StatusConflict, common.CodeStudentsCareExitPreviewChanged},
		{careplan.ErrCareExitBlocked, http.StatusConflict, common.CodeStudentsCareExitBlocked},
		{careplan.ErrCareExitNotPlanned, http.StatusConflict, common.CodeStudentsCareExitNotPlanned},
		{careplan.ErrCareExitAlreadyEffective, http.StatusConflict, common.CodeStudentsCareExitAlreadyEffective},
		{careplan.ErrCareResumeNotEnded, http.StatusConflict, common.CodeStudentsCareResumeNotEnded},
		{careplan.ErrCareResumeMissing, http.StatusConflict, common.CodeStudentsCareResumeMissing},
		{careplan.ErrCareResumeStartInPast, http.StatusBadRequest, common.CodeStudentsCareResumeStartInPast},
		{careplan.ErrCareResumeNotChecked, http.StatusBadRequest, common.CodeStudentsCareResumeNotChecked},
		{careplan.ErrCareWithdrawalNotFound, http.StatusNotFound, common.CodeStudentsCareWithdrawalNotFound},
		{careplan.ErrCareWithdrawalAfterGap, http.StatusBadRequest, common.CodeStudentsCareWithdrawalAfterGap},
		{careplan.ErrCareWithdrawalAlreadyResolved, http.StatusConflict, common.CodeStudentsCareWithdrawalAlreadyResolved},
		{&careplan.CareWithdrawalDateError{Message: "vor dem Beginn"}, http.StatusBadRequest, common.CodeStudentsCareWithdrawalDateInvalid},
		{careplan.ErrCareExitInvalidReason, http.StatusBadRequest, common.CodeStudentsCareExitInvalidReason},
		{careplan.ErrCareExitNoteRequired, http.StatusBadRequest, common.CodeStudentsCareExitNoteRequired},
		{careplan.ErrCareExitNoteNotAllowed, http.StatusBadRequest, common.CodeStudentsCareExitNoteNotAllowed},
		{careplan.ErrCareExitNoteTooLong, http.StatusBadRequest, common.CodeStudentsCareExitNoteTooLong},
	}

	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			t.Parallel()

			// Wrapped like the service returns it: the rule still matches.
			resp, ok := careExitErrorRenderer(fmt.Errorf("care exit: %w", tc.err)).(*common.ErrResponse)
			require.True(t, ok)
			assert.Equal(t, tc.status, resp.HTTPStatusCode)
			assert.Equal(t, tc.code, resp.Code)
		})
	}
}
