package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnrollmentRejectsMissingSettingsAtConstruction(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { NewIntake(IntakeDependencies{}) })
	assert.Panics(t, func() { NewDecisions(DecisionDependencies{}) })
	assert.Panics(t, func() { NewCaptcha(nil, &captchaProviderStub{}, nil) })
}
