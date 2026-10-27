package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDemoOrderFailureNamesStepAndCodeWithoutTheErrorText(t *testing.T) {
	t.Parallel()
	// The error of the failed orders on the demo server on 2026-10-27.
	err := errors.New(`demo school profile "vollbetrieb", workflow step Seeding a Träger holiday period failed: ` +
		`register for the holiday period: POST /traeger/holiday-registrations/3aa132a17c034ea6910aedae72e32e9176a17a2c69b146c3884b1d057f58b099 ` +
		`failed: 400 (enrollment.captcha_failed) - captcha verification failed`)

	step, code := demoOrderFailure(err)

	assert.Equal(t, "Seeding a Träger holiday period", step)
	assert.Equal(t, "enrollment.captcha_failed", code)
	assert.NotContains(t, step+code, "3aa132a1", "the link token stays out of the report")
}

func TestDemoOrderFailureWithoutStepOrCodeReportsEmpty(t *testing.T) {
	t.Parallel()
	step, code := demoOrderFailure(errors.New("sign in to the operator portal: connection refused"))
	assert.Empty(t, step)
	assert.Empty(t, code)
}

func TestInitDemoSentryWithoutDSNReportsNothing(t *testing.T) {
	t.Parallel()
	flush, err := initDemoSentry("  ", "development")
	assert.NoError(t, err)
	flush()
}
