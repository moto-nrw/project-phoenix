package scheduler

import (
	"context"
	"errors"
	"testing"
)

type fakeAuthCleanup struct {
	tokenCalls      int
	passwordCalls   int
	rateLimitCalls  int
	tokenResult     int
	passwordResult  int
	rateLimitResult int
	tokenErr        error
	passwordErr     error
	rateLimitErr    error
}

func (f *fakeAuthCleanup) CleanupExpiredTokens(ctx context.Context) (int, error) {
	f.tokenCalls++
	return f.tokenResult, f.tokenErr
}

func (f *fakeAuthCleanup) CleanupExpiredPasswordResetTokens(ctx context.Context) (int, error) {
	f.passwordCalls++
	return f.passwordResult, f.passwordErr
}

func (f *fakeAuthCleanup) CleanupExpiredRateLimits(ctx context.Context) (int, error) {
	f.rateLimitCalls++
	return f.rateLimitResult, f.rateLimitErr
}

type fakeInvitationCleanup struct {
	calls   int
	result  int
	callErr error
}

func (f *fakeInvitationCleanup) CleanupExpiredInvitations(ctx context.Context) (int, error) {
	f.calls++
	return f.result, f.callErr
}

func TestRunCleanupJobsExecutesAllJobs(t *testing.T) {
	auth := &fakeAuthCleanup{
		tokenResult:     1,
		passwordResult:  2,
		rateLimitResult: 3,
	}
	invitations := &fakeInvitationCleanup{result: 4}

	s := NewScheduler(nil, nil, auth, invitations)

	if err := s.RunCleanupJobs(); err != nil {
		t.Fatalf("RunCleanupJobs() returned error: %v", err)
	}

	if auth.tokenCalls != 1 || auth.passwordCalls != 1 || auth.rateLimitCalls != 1 {
		t.Fatalf("expected auth cleanup jobs to be called once each, got tokens=%d passwords=%d rate=%d",
			auth.tokenCalls, auth.passwordCalls, auth.rateLimitCalls)
	}

	if invitations.calls != 1 {
		t.Fatalf("expected invitation cleanup to be called once, got %d", invitations.calls)
	}
}

func TestRunCleanupJobsReturnsFirstErrorAndContinues(t *testing.T) {
	expectedErr := errors.New("rate limit cleanup failed")

	auth := &fakeAuthCleanup{
		rateLimitErr: expectedErr,
	}
	invitations := &fakeInvitationCleanup{}

	s := NewScheduler(nil, nil, auth, invitations)

	err := s.RunCleanupJobs()
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	if auth.tokenCalls != 1 || auth.passwordCalls != 1 || auth.rateLimitCalls != 1 {
		t.Fatalf("expected auth cleanup jobs to be called once each despite error, got tokens=%d passwords=%d rate=%d",
			auth.tokenCalls, auth.passwordCalls, auth.rateLimitCalls)
	}

	if invitations.calls != 1 {
		t.Fatalf("expected invitation cleanup to run after auth error, got %d", invitations.calls)
	}
}
