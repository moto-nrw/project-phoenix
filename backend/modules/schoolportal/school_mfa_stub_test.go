package schoolportal_test

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolportal"
)

// The second factor reaches the school portal as the runtime the composition
// root binds, with the school scope already applied (#3364). The stub below
// fills the closures a test does not describe, so an unexpected call fails
// loudly instead of nil-dereferencing, and the classifiers answer on the
// stub's own sentinels.
var (
	errStubMFAUnavailable       = errors.New("stub: mfa status unavailable")
	errStubMFABlocked           = errors.New("stub: mfa blocked")
	errStubMFAChallengeUnusable = errors.New("stub: mfa challenge unusable")
	errStubMFAAlreadyEnrolled   = errors.New("stub: mfa already enrolled")
	errStubMFAUnexpectedCall    = errors.New("stub: unexpected MFA call")
)

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
	if runtime.IssueTrustedDevice == nil {
		runtime.IssueTrustedDevice = func(context.Context, int64, int64, string, net.IP) (string, time.Time, error) {
			return "", time.Time{}, nil
		}
	}
	runtime.ChallengeUnusable = func(err error) bool { return errors.Is(err, errStubMFAChallengeUnusable) }
	runtime.Blocked = func(err error) bool { return errors.Is(err, errStubMFABlocked) }
	runtime.Unavailable = func(err error) bool { return errors.Is(err, errStubMFAUnavailable) }
	runtime.AlreadyEnrolled = func(err error) bool { return errors.Is(err, errStubMFAAlreadyEnrolled) }
	return runtime
}
