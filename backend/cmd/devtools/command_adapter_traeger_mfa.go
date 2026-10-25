package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// LoginTraeger signs in to the Träger-Portal. Every Träger session needs the
// second factor (#4651): the adapter completes the real enrollment or
// verification with the mailed code, like the operator and school logins,
// without a backend bypass. The Träger is named on every step.
func (a *Adapter) LoginTraeger(ctx context.Context, email, password string, organizationID int64) (AuthRef, error) {
	watermark := time.Now()
	respBody, status, err := a.Raw(ctx, AuthRef{}, http.MethodPost, "/traeger/auth/login",
		map[string]any{"email": email, "password": password, "organization_id": organizationID}, nil)
	if err != nil {
		return AuthRef{}, fmt.Errorf("traeger login request failed: %w", err)
	}
	var login struct {
		Status         string `json:"status"`
		AccessToken    string `json:"access_token"`
		ChallengeToken string `json:"challenge_token"`
		OrganizationID int64  `json:"organization_id"`
	}
	if err := json.Unmarshal(respBody, &login); err != nil {
		return AuthRef{}, fmt.Errorf("parse traeger login response with status %d: %w", status, err)
	}
	token := login.AccessToken
	switch login.Status {
	case "mfa_enrollment_required":
		enrollAuth := AuthRef{Kind: AuthBearer, Label: "mfa-enroll", Token: login.AccessToken}
		startBody, _, err := a.Raw(ctx, enrollAuth, http.MethodPost, "/traeger/auth/mfa/enroll/start", nil, nil)
		if err != nil {
			return AuthRef{}, fmt.Errorf("traeger enroll start: %w", err)
		}
		var start struct {
			ChallengeToken string `json:"challenge_token"`
		}
		if err := json.Unmarshal(startBody, &start); err != nil {
			return AuthRef{}, fmt.Errorf("parse traeger enroll start response: %w", err)
		}
		token, err = a.redeemTraegerCode(ctx, enrollAuth, "/traeger/auth/mfa/enroll/confirm", start.ChallengeToken, email, login.OrganizationID, watermark)
		if err != nil {
			return AuthRef{}, err
		}
	case "mfa_required":
		token, err = a.redeemTraegerCode(ctx, AuthRef{}, "/traeger/auth/mfa/verify", login.ChallengeToken, email, login.OrganizationID, watermark)
		if err != nil {
			return AuthRef{}, err
		}
	}
	if token == "" {
		return AuthRef{}, fmt.Errorf("POST /traeger/auth/login response with status %d has no access token", status)
	}
	return AuthRef{Kind: AuthBearer, Label: "traeger", Token: token}, nil
}

// redeemTraegerCode reads the mailed code and exchanges it with the
// challenge for the Träger session.
func (a *Adapter) redeemTraegerCode(ctx context.Context, auth AuthRef, path, challengeToken, recipient string, organizationID int64, watermark time.Time) (string, error) {
	code, err := a.fetchLatestMFACode(ctx, recipient, watermark)
	if err != nil {
		return "", err
	}
	respBody, status, err := a.Raw(ctx, auth, http.MethodPost, path,
		map[string]any{"challenge_token": challengeToken, "code": code, "organization_id": organizationID}, nil)
	if err != nil {
		return "", fmt.Errorf("POST %s: %w", path, err)
	}
	token, err := parseLoginToken(respBody)
	if err != nil {
		return "", fmt.Errorf("decode POST %s response with status %d: %w", path, status, err)
	}
	if token == "" {
		return "", fmt.Errorf("POST %s response with status %d has no access token", path, status)
	}
	return token, nil
}
