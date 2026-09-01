package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type seedCareWithdrawalsStep struct{ seeder *Seeder }

func (seedCareWithdrawalsStep) Name() string { return "Seeding care withdrawal tasks" }

func (s seedCareWithdrawalsStep) Run(ctx context.Context, rt *Runtime) error {
	primaryAuth := rt.TenantAuth
	demo, auth, err := s.provisionDemoSchool(ctx, rt)
	if err != nil {
		return err
	}
	rt.SetTenantAuth(auth)
	defer rt.SetTenantAuth(primaryAuth)
	requests, err := s.seedDemoBookings(rt, auth, demo.SchoolID, demo.TenantSlug)
	if err != nil {
		return err
	}
	if err := enableSeedBookingAuthority(rt, demo.SchoolID); err != nil {
		return err
	}
	if err := seedCareWithdrawalStates(rt, requests); err != nil {
		return err
	}
	rt.CareWithdrawals = demo
	return nil
}

func (s seedCareWithdrawalsStep) provisionDemoSchool(
	ctx context.Context, rt *Runtime,
) (*SeedCareWithdrawalDemo, AuthRef, error) {
	if rt.Bootstrap == nil {
		return nil, AuthRef{}, fmt.Errorf("bootstrap state not available")
	}
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	name, slug := "Demo Betreuung abschließen", truncateSeedSubdomain("abschluss-demo-"+suffix)
	schoolID, err := createWithdrawalDemoSchool(rt, name, slug)
	if err != nil {
		return nil, AuthRef{}, err
	}
	admin, err := s.inviteWithdrawalDemoAdmin(rt, schoolID, suffix)
	if err != nil {
		return nil, AuthRef{}, err
	}
	auth, err := rt.Adapter.LoginTenant(ctx, admin.Email, admin.Password, slug)
	if err != nil {
		return nil, AuthRef{}, fmt.Errorf("login withdrawal demo admin: %w", err)
	}
	demo := &SeedCareWithdrawalDemo{SchoolID: schoolID, SchoolName: name, TenantSlug: slug, SchoolAdmin: admin}
	return demo, auth, nil
}

func createWithdrawalDemoSchool(rt *Runtime, name, slug string) (int64, error) {
	raw, err := rt.Client.PostWithAuth(rt.OperatorAuth, "/operator/schools", map[string]any{
		"organization_id": rt.Bootstrap.OrganizationID,
		"name":            name, "slug": slug, "subdomain": slug,
	})
	if err != nil {
		return 0, fmt.Errorf("create withdrawal demo school: %w", err)
	}
	var response struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &response); err != nil || response.Data.ID <= 0 {
		return 0, fmt.Errorf("parse withdrawal demo school response")
	}
	return response.Data.ID, nil
}

func (s seedCareWithdrawalsStep) inviteWithdrawalDemoAdmin(
	rt *Runtime, schoolID int64, suffix string,
) (BootstrapAdminCredentials, error) {
	password, err := s.withdrawalDemoPassword()
	if err != nil {
		return BootstrapAdminCredentials{}, err
	}
	admin := BootstrapAdminCredentials{Email: "abschluss-demo-" + suffix + "@example.test", Password: password, Name: "Abschluss Demo", Position: "OGS-Büro"}
	raw, err := rt.Client.PostWithAuthAndHeaders(rt.OperatorAuth, fmt.Sprintf("/operator/schools/%d/invite-admin", schoolID), map[string]any{
		"email": admin.Email, "first_name": "Abschluss", "last_name": "Demo", "position": admin.Position,
	}, map[string]string{seedTokenHeader: "true"})
	if err != nil {
		return BootstrapAdminCredentials{}, fmt.Errorf("invite withdrawal demo admin: %w", err)
	}
	token, err := s.seeder.extractBootstrapInvitationToken(raw)
	if err != nil {
		return BootstrapAdminCredentials{}, err
	}
	_, err = rt.Client.PostPublic("/auth/invitations/"+token+"/accept", map[string]any{"password": password, "confirm_password": password})
	return admin, err
}

func (s seedCareWithdrawalsStep) withdrawalDemoPassword() (string, error) {
	if s.seeder.options.StaffPassword != "" {
		return s.seeder.options.StaffPassword, nil
	}
	return generateSeedPassword(s.seeder.random)
}

func (s seedCareWithdrawalsStep) seedDemoBookings(
	rt *Runtime, auth AuthRef, schoolID int64, tenantSlug string,
) ([]SeedEnrollmentRequest, error) {
	for key, value := range map[string]any{
		"enrollment.enabled":         true,
		"enrollment.require_captcha": false,
		"operations.group_mode":      "open_care",
		"checkout.schulhof_enabled":  false,
		"checkout.wc_enabled":        true,
	} {
		if _, err := rt.Client.PutWithAuth(auth, "/api/settings/values/"+key, map[string]any{"value": value}); err != nil {
			return nil, fmt.Errorf("configure withdrawal demo: %w", err)
		}
	}
	if _, err := rt.Client.DeleteWithAuth(auth, "/api/settings/values/checkout.wc_enabled"); err != nil {
		return nil, fmt.Errorf("reset withdrawal demo WC setting: %w", err)
	}
	if _, err := rt.Client.PutWithAuth(auth, "/api/settings/values/checkout.wc_enabled", map[string]any{"value": false}); err != nil {
		return nil, fmt.Errorf("disable withdrawal demo WC: %w", err)
	}
	presencePath := fmt.Sprintf("/operator/schools/%d/settings/values/operations.presence_mode", schoolID)
	if _, err := rt.Client.PutWithAuth(rt.OperatorAuth, presencePath, map[string]any{"value": "binary"}); err != nil {
		return nil, fmt.Errorf("configure withdrawal demo presence mode: %w", err)
	}
	parentStep := parentEnrollmentSeedStep(s)
	schemaID, err := parentStep.createEnrollmentSchema(rt, auth)
	if err != nil {
		return nil, err
	}
	phaseID, err := parentStep.createEnrollmentPhase(rt, auth, schemaID)
	if err != nil {
		return nil, err
	}
	offerings, err := parentStep.createCareOfferings(rt, auth, phaseID)
	if err != nil {
		return nil, err
	}
	return s.submitWithdrawalDemoChildren(rt, auth, tenantSlug, phaseID, offerings)
}

func (s seedCareWithdrawalsStep) submitWithdrawalDemoChildren(
	rt *Runtime, auth AuthRef, tenantSlug string, phaseID int64, offerings map[string]int64,
) ([]SeedEnrollmentRequest, error) {
	parentStep := parentEnrollmentSeedStep(s)
	requests := make([]SeedEnrollmentRequest, 0, 3)
	for index, firstName := range []string{"Plan", "Fällig", "Erledigt"} {
		body := withdrawalDemoSubmission(parentStep, phaseID, offerings, firstName, index)
		raw, err := rt.Client.PostPublicWithHeaders("/api/enrollment/"+tenantSlug+"/submit", body, publicEnrollmentSeedHeaders(40+index))
		if err != nil {
			return nil, err
		}
		request, err := approveWithdrawalDemoRequest(parentStep, rt, auth, raw)
		if err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	return requests, nil
}

func withdrawalDemoSubmission(
	step parentEnrollmentSeedStep, phaseID int64, offerings map[string]int64, firstName string, index int,
) map[string]any {
	careID := offerings["ogs-ganztag"]
	email := fmt.Sprintf("abschluss-%d-%d@example.test", index, time.Now().UnixNano())
	return step.enrollmentSubmissionWithDays(phaseID, offerings, firstName, "Abschluss", "2018-04-18", 2,
		"Demo", "Eltern", email, "care-withdrawal-demo", []int64{careID}, map[int64][]string{careID: {"mon", "wed"}})
}

func approveWithdrawalDemoRequest(
	step parentEnrollmentSeedStep, rt *Runtime, auth AuthRef, raw []byte,
) (SeedEnrollmentRequest, error) {
	request, err := parseEnrollmentSubmitResponse(raw, "public")
	if err != nil {
		return request, err
	}
	detail, err := step.loadEnrollmentRequestDetail(rt, auth, request.RequestID)
	if err != nil {
		return request, err
	}
	if len(detail.ChildIDs) != 1 {
		return request, fmt.Errorf("withdrawal demo request %d has %d children", request.RequestID, len(detail.ChildIDs))
	}
	request.ChildIDs, request.Status = detail.ChildIDs, "approved"
	err = step.decideEnrollmentChild(rt, auth, request.RequestID, detail.ChildIDs[0], request.Status, "Demo-Zusage für den Betreuungsabschluss")
	return request, err
}

func enableSeedBookingAuthority(rt *Runtime, schoolID int64) error {
	path := fmt.Sprintf("/operator/schools/%d/settings/values/%s", schoolID, "enrollment.bookings_authoritative")
	_, err := rt.Client.PutWithAuth(rt.OperatorAuth, path, map[string]any{"value": true})
	if err != nil {
		return fmt.Errorf("enable booking authority for withdrawal demo: %w", err)
	}
	return nil
}

func seedCareWithdrawalStates(rt *Runtime, approved []SeedEnrollmentRequest) error {
	if len(approved) != 3 {
		return fmt.Errorf("care withdrawal seed requires three approved enrollment requests")
	}
	today := todaySeedDate()
	dates := []seedDate{today.AddDays(7), today, today.AddDays(1)}
	studentIDs := make([]int64, len(dates))
	for index, effectiveFrom := range dates {
		studentID, err := removeSeedCareBooking(rt, approved[index], effectiveFrom)
		if err != nil {
			return err
		}
		studentIDs[index] = studentID
	}
	return resolveSeedCareWithdrawal(rt, studentIDs[2], today)
}

func removeSeedCareBooking(
	rt *Runtime, request SeedEnrollmentRequest, effectiveFrom seedDate,
) (int64, error) {
	path := fmt.Sprintf("/api/enrollment/admin/requests/%d/children/%d/offerings", request.RequestID, request.ChildIDs[0])
	raw, err := rt.Client.PutWithAuth(rt.TenantAuth, path, map[string]any{
		"offerings": []any{}, "reason": "Demo-Abschluss für die lokale Prüfung",
		"effective_from": effectiveFrom.String(), "complete_withdrawal_confirmed": true,
	})
	if err != nil {
		return 0, err
	}
	var response struct {
		Data struct {
			StudentID string `json:"created_student_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return 0, fmt.Errorf("parse seeded withdrawal student: %w", err)
	}
	studentID, err := strconv.ParseInt(response.Data.StudentID, 10, 64)
	if err != nil || studentID <= 0 {
		return 0, fmt.Errorf("seeded withdrawal returned invalid student id %q", response.Data.StudentID)
	}
	return studentID, nil
}

func resolveSeedCareWithdrawal(rt *Runtime, studentID int64, lastCareDay seedDate) error {
	completionID, err := findSeedCareWithdrawal(rt, studentID)
	if err != nil {
		return err
	}
	body := map[string]any{"last_care_day": lastCareDay.String(), "reason": "other", "reason_note": "Demo-Abschluss wurde erledigt"}
	previewPath := fmt.Sprintf("/api/students/care-withdrawals/%d/care-end/preview", completionID)
	raw, err := rt.Client.PostWithAuth(rt.TenantAuth, previewPath, body)
	if err != nil {
		return err
	}
	var preview struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &preview); err != nil || preview.Data.Token == "" {
		return fmt.Errorf("seeded withdrawal preview returned no token")
	}
	body["token"] = preview.Data.Token
	_, err = rt.Client.PostWithAuth(rt.TenantAuth, fmt.Sprintf("/api/students/care-withdrawals/%d/care-end", completionID), body)
	return err
}

func findSeedCareWithdrawal(rt *Runtime, studentID int64) (int64, error) {
	raw, err := rt.Client.GetWithAuth(rt.TenantAuth, fmt.Sprintf("/api/students/care-withdrawals?student_id=%d", studentID))
	if err != nil {
		return 0, err
	}
	var response struct {
		Data struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil || len(response.Data.Items) != 1 {
		return 0, fmt.Errorf("seeded withdrawal task was not found")
	}
	id, err := strconv.ParseInt(response.Data.Items[0].ID, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("seeded withdrawal returned invalid completion id")
	}
	return id, nil
}
