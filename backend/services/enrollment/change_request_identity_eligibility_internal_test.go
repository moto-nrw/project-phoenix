package enrollment

import (
	"context"
	"log/slog"
	"testing"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// newIdentityEligibilityService wires a change-request service with only the
// student repository the enrolled-status probe needs.
func newIdentityEligibilityService(exists bool) *changeRequestService {
	svc := &changeRequestService{ChangeRequestServiceConfig: ChangeRequestServiceConfig{
		StudentRepo: &stubEligibilityStudentRepo{exists: exists},
	}}
	svc.Logger = slog.Default()
	return svc
}

func identityEligibilityChild(id int64, firstName string) *enrollmentModels.RequestChild {
	child := &enrollmentModels.RequestChild{
		FirstName:   firstName,
		LastName:    "Müller",
		DateOfBirth: "2019-04-12",
	}
	child.ID = id
	return child
}

func identityEligibilitySubmit(id int64, firstName string) SubmitChild {
	return SubmitChild{
		ID:          id,
		FirstName:   firstName,
		LastName:    "Müller",
		DateOfBirth: timezone.NewDate(2019, 4, 12),
	}
}

// A change request that retargets a child onto an already-enrolled student's
// name+birthday must be rejected on a new_students phase: approval would take
// the fresh-create branch and duplicate a child the school already has (#1663).
func TestValidateChangedChildIdentityEligibility_RejectsRetargetToEnrolledIdentity(t *testing.T) {
	t.Parallel()

	svc := newIdentityEligibilityService(true)
	phase := &capability.Phase{Audience: capability.PhaseAudienceNewStudents}
	children := []*enrollmentModels.RequestChild{
		identityEligibilityChild(11, "Anna"),
		identityEligibilityChild(12, "Ben"),
	}
	editReq := SubmitRequest{
		TenantID: 77,
		Children: []SubmitChild{
			identityEligibilitySubmit(11, "Anna"),
			identityEligibilitySubmit(12, "Clara"), // identity rewritten
		},
	}

	err := svc.validateChangedChildIdentityEligibility(context.Background(), phase, editReq, children)
	require.ErrorIs(t, err, ErrChildAlreadyEnrolled)
	require.ErrorIs(t, err, ErrInvalidSubmission, "already-enrolled must keep the 400 category")
	assert.Contains(t, err.Error(), "child 1", "the error must point at the edited row, not the filtered index")
}

// The gate must NOT fire on children whose identity the edit leaves alone. A
// change request filed after approval sits on a child whose own approval created
// the matching student; probing it would reject every later change request.
func TestValidateChangedChildIdentityEligibility_SkipsUnchangedIdentities(t *testing.T) {
	t.Parallel()

	svc := newIdentityEligibilityService(true)
	phase := &capability.Phase{Audience: capability.PhaseAudienceNewStudents}
	children := []*enrollmentModels.RequestChild{identityEligibilityChild(11, "Anna")}
	editReq := SubmitRequest{
		TenantID: 77,
		Children: []SubmitChild{identityEligibilitySubmit(11, "Anna")},
	}

	require.NoError(t, svc.validateChangedChildIdentityEligibility(context.Background(), phase, editReq, children))
}

// The inverse audience is covered by the same probe: an existing_students child
// edited into an identity nobody is enrolled under is rejected.
func TestValidateChangedChildIdentityEligibility_ExistingStudentsRejectsUnknownIdentity(t *testing.T) {
	t.Parallel()

	svc := newIdentityEligibilityService(false)
	phase := &capability.Phase{Audience: capability.PhaseAudienceExistingStudents}
	children := []*enrollmentModels.RequestChild{identityEligibilityChild(11, "Anna")}
	editReq := SubmitRequest{
		TenantID: 77,
		Children: []SubmitChild{identityEligibilitySubmit(11, "Ben")},
	}

	err := svc.validateChangedChildIdentityEligibility(context.Background(), phase, editReq, children)
	require.ErrorIs(t, err, ErrChildNotEnrolled)
}

// Audiences without an enrolled-status rule never probe, even on a rewrite.
func TestValidateChangedChildIdentityEligibility_OtherAudiencesPass(t *testing.T) {
	t.Parallel()

	svc := newIdentityEligibilityService(true)
	phase := &capability.Phase{Audience: capability.PhaseAudienceOpen}
	children := []*enrollmentModels.RequestChild{identityEligibilityChild(11, "Anna")}
	editReq := SubmitRequest{
		TenantID: 77,
		Children: []SubmitChild{identityEligibilitySubmit(11, "Ben")},
	}

	require.NoError(t, svc.validateChangedChildIdentityEligibility(context.Background(), phase, editReq, children))
}
