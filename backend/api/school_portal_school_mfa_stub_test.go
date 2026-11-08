package api

import (
	"context"
	"errors"
	"net"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/schoolportal"
)

// The second factor reaches the school portal as the runtime the composition
// root binds, with the school scope already applied (#3364). The stub below
// fills the closures a test does not describe, so an unexpected call fails
// loudly instead of nil-dereferencing.
var errStubMFAUnexpectedCall = errors.New("stub: unexpected MFA call")

func stubSchoolMFA(runtime schoolportal.MFARuntime) schoolportal.MFARuntime {
	if runtime.VerifyChallenge == nil {
		runtime.VerifyChallenge = func(context.Context, string, string) (schoolportal.VerifiedChallenge, error) {
			return schoolportal.VerifiedChallenge{}, errStubMFAUnexpectedCall
		}
	}
	if runtime.VerifyChallengeForOwner == nil {
		runtime.VerifyChallengeForOwner = func(context.Context, string, string, int64, int64) (schoolportal.VerifiedChallenge, error) {
			return schoolportal.VerifiedChallenge{}, errStubMFAUnexpectedCall
		}
	}
	if runtime.ResendChallenge == nil {
		runtime.ResendChallenge = func(context.Context, string, net.IP) (string, error) {
			return "", errStubMFAUnexpectedCall
		}
	}
	if runtime.StartChallenge == nil {
		runtime.StartChallenge = func(context.Context, int64, int64, net.IP) (string, error) {
			return "", errStubMFAUnexpectedCall
		}
	}
	if runtime.Enroll == nil {
		runtime.Enroll = func(context.Context, int64) error { return nil }
	}
	if runtime.PrepareTrustedDevice == nil {
		runtime.PrepareTrustedDevice = func(context.Context, int64, int64, string, net.IP) (identityaccess.TrustedDeviceIssue, error) {
			return nil, nil
		}
	}
	return runtime
}
