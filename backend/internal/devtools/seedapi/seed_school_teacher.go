package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// seedSchoolTeacherStep drives moto schule with its own school-scoped session.
// Registration and class assignment stay on their public admin contracts;
// today's entries go through the same guards and owner commands as the UI.
type seedSchoolTeacherStep struct{ seeder *Seeder }

func (seedSchoolTeacherStep) Name() string { return "Seeding school teacher and today's class entries" }

func (s seedSchoolTeacherStep) Run(_ context.Context, rt *Runtime) error {
	return s.runOn(rt, todaySeedDate())
}

func (s seedSchoolTeacherStep) runOn(rt *Runtime, today seedDate) error {
	if !localSeedAccountsEnabled(s.seeder.client.baseURL, s.seeder.options) {
		return nil
	}
	fs := rt.FixedSeeder
	if fs == nil || rt.Bootstrap == nil || len(fs.staffCredentials) == 0 || len(DemoStudents) == 0 {
		return fmt.Errorf("school teacher requires bootstrap, staff credentials and students")
	}
	roleID := fs.roleIDs["lehrkraft"]
	studentID := fs.studentIDByIndex[0]
	if roleID <= 0 || studentID <= 0 {
		return fmt.Errorf("school teacher requires lehrkraft role and first OGS child")
	}
	// Reuse the seed's existing password source, including --staff-password.
	// The private seed state documents the login, never source or logs.
	password := fs.staffCredentials[0].Password
	email := scopedEmail("lehrkraft@example.test", fs.accountScope)
	username := "lehrkraft"
	if fs.accountScope != "" {
		username = "lk-" + fs.accountScope
	}
	account, err := registerSeedStaffAccount(rt, map[string]any{
		"email": email, "username": username, "password": password, "confirm_password": password,
		"first_name": "Lena", "last_name": "Weber", "role_id": roleID,
	})
	if err != nil {
		return fmt.Errorf("register school teacher: %w", err)
	}
	staffID := account.StaffID
	class := DemoStudents[0].Class
	if _, err := rt.Client.PutWithAuth(rt.TenantAuth, fmt.Sprintf("/api/staff/%d/school-classes", staffID), map[string]any{
		"school_classes": []string{class},
	}); err != nil {
		return fmt.Errorf("assign school teacher's class: %w", err)
	}
	s.seeder.schoolTeacherAccount = &AccountCredentials{
		Key: "lehrkraft", AccountID: account.AccountID, StaffID: staffID,
		Email: email, Password: password, Name: "Lena Weber",
	}
	auth, err := loginSchoolTeacher(rt, email, password)
	if err != nil {
		return err
	}
	return s.seedClassDay(rt, auth, class, studentID, today)
}

func loginSchoolTeacher(rt *Runtime, email, password string) (AuthRef, error) {
	raw, err := rt.Client.PostPublic("/school/auth/login", map[string]any{
		"email": email, "password": password, "tenant_slug": rt.Bootstrap.TenantSlug,
	})
	if err != nil {
		return AuthRef{}, fmt.Errorf("school teacher login: %w", err)
	}
	var session struct {
		Status      string `json:"status"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &session); err != nil {
		return AuthRef{}, fmt.Errorf("decode school teacher session: %w", err)
	}
	if session.Status != "authenticated" {
		return AuthRef{}, fmt.Errorf("school teacher login requires an authenticated session; MFA or enrollment must be completed first")
	}
	if session.AccessToken == "" {
		return AuthRef{}, fmt.Errorf("school teacher login returned no school session")
	}
	return AuthRef{Kind: AuthBearer, Token: session.AccessToken}, nil
}

func (s seedSchoolTeacherStep) seedClassDay(rt *Runtime, auth AuthRef, class string, studentID int64, today seedDate) error {
	// Screenshot schools disable the Friday-based weekend plan.
	if s.seeder.options.ProductScreenshots && (today.Weekday() == time.Saturday || today.Weekday() == time.Sunday) {
		fmt.Println("  School teacher assigned; today's class entries skipped on a screenshot weekend")
		return nil
	}
	dayPath := url.PathEscape(class) + "/" + today.String()
	for _, entry := range []struct {
		path string
		body any
	}{
		{"arrival-exceptions/" + dayPath, map[string]any{"arrival_time": "12:00", "reason": "Der Unterricht endet heute früher."}},
		{"day-hints/" + dayPath, map[string]any{"body": "Die Klasse hat heute viel gearbeitet. Bitte starten Sie ruhig in den Nachmittag."}},
		{"day-hints/" + dayPath + fmt.Sprintf("/students/%d", studentID), map[string]any{"body": "Bitte erinnern Sie das Kind heute ans Trinken."}},
		{"morning-attendance/" + dayPath, map[string]any{"marks": []map[string]any{{"student_id": studentID, "status": "present"}}}},
	} {
		if _, err := rt.Client.PutWithAuth(auth, "/school/class-day/"+entry.path, entry.body); err != nil {
			return fmt.Errorf("seed school class day %s: %w", entry.path, err)
		}
	}
	fmt.Println("  School teacher assigned; today's arrival exception, class/child hints and morning attendance created")
	return nil
}
