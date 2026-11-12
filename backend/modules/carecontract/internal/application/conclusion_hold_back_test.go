package application

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
)

// TestSendHoldBackKeepsTheChildACandidateForEveryHeldMail: a mail that
// would stay back at the address holds back only this child (#4680), it
// never fails the conclusion run of the whole Träger. A send of the office
// that got there first is no hold-back: the run loses that race.
func TestSendHoldBackKeepsTheChildACandidateForEveryHeldMail(t *testing.T) {
	t.Parallel()
	for sent, reason := range map[error]string{
		carecontract.ErrRecipientMissing:    carecontract.SkipRecipientMissing,
		carecontract.ErrMailDelayed:         carecontract.SkipMailDelayed,
		carecontract.ErrMailBlocked:         carecontract.SkipMailBlocked,
		carecontract.ErrMailBlockUnreadable: carecontract.SkipMailBlockUnreadable,
		carecontract.ErrMailPending:         "",
		carecontract.ErrContractNotSendable: "",
		errors.New("database down"):         "",
	} {
		assert.Equal(t, reason, holdBackReason(sendHoldBack(fmt.Errorf("send: %w", sent))), sent.Error())
	}
}
