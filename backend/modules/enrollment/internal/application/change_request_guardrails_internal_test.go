package application

import (
	"context"
	"errors"
	"testing"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubGuardrailProfiles struct {
	GuardianProfiles
	byEmail []*GuardianProfile
}

func (s stubGuardrailProfiles) GuardianProfileByEmail(context.Context, string) (*GuardianProfile, error) {
	return nil, nil
}

func (s stubGuardrailProfiles) GuardianProfilesByEmails(context.Context, []string) ([]*GuardianProfile, error) {
	return s.byEmail, nil
}

type stubGuardrailGuardians struct{ IntakeGuardians }

func (stubGuardrailGuardians) RequestGuardians(context.Context, []int64) ([]*enrollmentNative.RequestGuardian, error) {
	return nil, nil
}

// guardianPhonePort lets the stub embed the port although the port and its
// read method share a name; unstubbed methods panic.
type guardianPhonePort = GuardianPhones

type stubGuardrailPhones struct {
	guardianPhonePort
	rows []*GuardianPhone
	err  error
}

func (s stubGuardrailPhones) GuardianPhones(context.Context, int64) ([]*GuardianPhone, error) {
	return s.rows, s.err
}

func guardrailService(phones stubGuardrailPhones) *ChangeRequests {
	email := "co@example.test"
	return NewChangeRequests(ChangeRequestDependencies{
		Settings:   intakeSettingsFake{},
		Guardians:  stubGuardrailGuardians{},
		ParentsURL: "https://parents.example.test",
		People: PeopleDirectory{
			GuardianProfiles: stubGuardrailProfiles{byEmail: []*GuardianProfile{{
				ID: 9, FirstName: "Eva", LastName: "Kurz", Email: &email, HasAccount: true,
			}}},
			GuardianPhones: phones,
		},
	})
}

func guardrailEdit() SubmitRequest {
	email := "co@example.test"
	phone := "0170 1234"
	return SubmitRequest{AdditionalGuardians: []SubmitGuardian{{
		FirstName: "Eva", LastName: "Kurz", Email: &email, Phone: &phone,
	}}}
}

func TestAccountLinkedCoGuardianPhoneLookupFailureIsNotInvalidInput(t *testing.T) {
	t.Parallel()
	lookupErr := errors.New("phone store unavailable")
	svc := guardrailService(stubGuardrailPhones{err: lookupErr})

	err := svc.validateAccountLinkedGuardianEdits(context.Background(), &Request{}, guardrailEdit())

	require.ErrorIs(t, err, lookupErr)
	assert.NotErrorIs(t, err, enrollmentNative.ErrChangeRequestInvalidData)
}

func TestAccountLinkedCoGuardianPhoneMustMatchProfile(t *testing.T) {
	t.Parallel()
	matching := guardrailService(stubGuardrailPhones{rows: []*GuardianPhone{{PhoneNumber: " 0170 1234 "}}})
	require.NoError(t, matching.validateAccountLinkedGuardianEdits(context.Background(), &Request{}, guardrailEdit()))

	differing := guardrailService(stubGuardrailPhones{rows: []*GuardianPhone{{PhoneNumber: "0170 9999"}}})
	err := differing.validateAccountLinkedGuardianEdits(context.Background(), &Request{}, guardrailEdit())
	require.ErrorIs(t, err, enrollmentNative.ErrChangeRequestInvalidData)
}
