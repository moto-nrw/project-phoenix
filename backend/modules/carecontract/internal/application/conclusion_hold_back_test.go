package application

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// TestSendHoldBackKeepsTheChildACandidateForEveryHeldMail: a mail that
// would stay back at the address holds back only this child (#4680), it
// never fails the conclusion run of the whole Träger.
func TestSendHoldBackKeepsTheChildACandidateForEveryHeldMail(t *testing.T) {
	t.Parallel()
	for sent, reason := range map[error]string{
		carecontract.ErrRecipientMissing:    carecontract.SkipRecipientMissing,
		carecontract.ErrMailDelayed:         carecontract.SkipMailDelayed,
		carecontract.ErrMailBlocked:         carecontract.SkipMailBlocked,
		carecontract.ErrMailBlockUnreadable: carecontract.SkipMailBlockUnreadable,
	} {
		assert.Equal(t, reason, holdBackReason(sendHoldBack(fmt.Errorf("send: %w", sent))), sent.Error())
	}
	other := errors.New("database down")
	assert.Equal(t, "", holdBackReason(sendHoldBack(other)), "any other failure stays an error of the run")
}
