package parentmessaging

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// DeclarationRules retains the staff adapter's construction contract while
// Communication owns the shared, public value policy.
type DeclarationRules communication.DeclarationRules

func ValidateDeclarationSettings(kind, signers string) error {
	return communication.ValidateDeclarationSettings(kind, signers)
}
func DeclarationChildState(rules DeclarationRules, eligible []int64, latest map[int64]string, now time.Time) string {
	return communication.DeclarationChildState(communication.DeclarationRules(rules), eligible, latest, now)
}
func DeclarationOwesAction(rules DeclarationRules, canSubmit bool, myLatest, childState string, now time.Time) bool {
	return communication.DeclarationOwesAction(communication.DeclarationRules(rules), canSubmit, myLatest, childState, now)
}
