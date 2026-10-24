package main

import (
	"context"
	"encoding/json"
	"fmt"
)

// This adapter exists only in the devtools binary. It completes the real
// enrollment/verification through local Mailpit, without a backend MFA bypass.
func (a *Adapter) completeTenantLoginMFA(ctx context.Context, body []byte, token, recipient string) (string, error) {
	type loginMFAState struct {
		Status             string `json:"status"`
		ChallengeToken     string `json:"challenge_token"`
		EnrollmentRequired bool   `json:"mfa_enrollment_required"`
	}
	var response struct {
		loginMFAState
		Data loginMFAState `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}
	state := response.Data
	if state.Status == "" {
		state = response.loginMFAState
	}
	switch {
	case state.EnrollmentRequired || state.Status == "mfa_enrollment_required":
		return a.completeMFAEnrollment(ctx, "/auth", token, recipient)
	case state.Status == "mfa_required":
		return a.completeMFAVerify(ctx, "/auth", state.ChallengeToken, recipient)
	default:
		if token == "" {
			return "", fmt.Errorf("tenant login returned no access token")
		}
		return token, nil
	}
}
