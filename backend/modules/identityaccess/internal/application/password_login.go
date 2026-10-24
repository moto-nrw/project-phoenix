package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// checkPasswordLogin runs under the identity row lock. A refusal returns false,
// not an error: the caller must commit its failure counter before answering.
// The verifier performs the normal password work even for missing/locked rows.
func checkPasswordLogin(ctx context.Context, store ports.PasswordLoginStore, passwords ports.PasswordVerifier, policy func() (int, time.Duration, error), id int64, hash, password string, operator bool) (bool, error) {
	threshold, duration, err := policy()
	if err != nil || threshold <= 0 || duration <= 0 {
		return false, errors.New("password login protection policy unavailable")
	}
	var state domain.PasswordLoginState
	if id != 0 {
		state, err = store.LockPasswordLoginState(ctx, id, operator)
		if err != nil {
			return false, err
		}
	}
	valid, verifyErr := passwords.VerifyPassword(password, hash)
	now := time.Now()
	if id == 0 || (state.PasswordLockedUntil != nil && state.PasswordLockedUntil.After(now)) {
		return false, nil
	}
	changed := state.PasswordAttempts != 0 || state.PasswordLockedUntil != nil
	if state.PasswordLockedUntil != nil {
		state = domain.PasswordLoginState{}
	}
	if verifyErr == nil && valid && hash != "" {
		if !changed {
			return true, nil
		}
		return true, store.SavePasswordLoginState(ctx, id, operator, domain.PasswordLoginState{})
	}
	state.PasswordAttempts++
	if state.PasswordAttempts >= threshold {
		until := now.Add(duration)
		state.PasswordLockedUntil = &until
	}
	return false, store.SavePasswordLoginState(ctx, id, operator, state)
}
