package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWeeklyProfileRejectsBookingDrivenCareDays(t *testing.T) {
	t.Parallel()
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/care-offerings") {
			_, _ = fmt.Fprint(w, `{"data":{"offerings":[{"id":"42","weekdays":[1]}]}}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"data":{"weekdays":[{"weekday":1,"status":"scheduled","arrival":"12:00","pickup":"15:00"},{"weekday":2,"status":"not_scheduled"},{"weekday":3,"status":"not_scheduled"},{"weekday":4,"status":"not_scheduled"},{"weekday":5,"status":"not_scheduled"}]}}`)
	})
	defer srv.Close()
	err := verifyWeeklyProfileCare(&Runtime{Client: newTestClient(srv.URL, false)}, AuthRef{Token: "parent"}, 17, 42)
	require.ErrorContains(t, err, "care-day priority failed on weekday 2")
}

func TestBookingProfileRejectsLegacyWeeklyCareDays(t *testing.T) {
	t.Parallel()
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/care-offerings") {
			_, _ = fmt.Fprint(w, `{"data":{"offerings":[{"id":"42","weekdays":[1]}]}}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"data":{"weekdays":[{"weekday":1,"status":"scheduled","arrival":"12:00","pickup":"15:00"},{"weekday":2,"status":"scheduled","arrival":"12:00","pickup":"15:00"},{"weekday":3,"status":"not_scheduled"},{"weekday":4,"status":"scheduled","arrival":"12:00","pickup":"15:00"},{"weekday":5,"status":"not_scheduled"}]}}`)
	})
	defer srv.Close()
	err := verifyEnrollmentProfileCare(&Runtime{Client: newTestClient(srv.URL, false)}, AuthRef{Token: "parent"}, 17, 42, true)
	require.ErrorContains(t, err, "care-day priority failed on weekday 2")
}

// weeklyProfileAPIMock extends the full workflow's HTTP fixture with a second
// tenant. Writes determine subsequent reads, including decisions and schedules.
type weeklyProfileAPIMock struct {
	terminalScans        int
	withdrawalStudentIDs []string
	traces               []*fullSeedAPITrace
	bookings             bool
	organizations        int
	active               bool
	nextID               int64
	requests             map[string]map[string]any
	guardians            map[string]map[string]any
	schedules            map[string][]map[string]any
	settings             map[string]any
	offeringID           int64
	phase                map[string]any
}

func (m *weeklyProfileAPIMock) serve(t *testing.T, w seedHTTPResponseWriter, r *seedHTTPRequest) bool {
	t.Helper()
	if r.URL.Path == "/operator/organizations" {
		m.organizations++
		if m.organizations == 2 {
			m.active = true
			m.nextID = 10000
			m.requests = make(map[string]map[string]any)
			m.guardians = make(map[string]map[string]any)
			m.schedules = make(map[string][]map[string]any)
			m.settings = make(map[string]any)
		}
	}
	if !m.active {
		return false
	}
	m.nextID++
	w.Header().Set("Content-Type", "application/json")
	var body map[string]any
	if r.Method == "POST" || r.Method == "PUT" {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
	}
	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	data := any(map[string]any{"id": m.nextID})
	switch {
	case path == "/operator/organizations":
		data = map[string]any{"id": 2}
	case path == "/operator/schools":
		if body["slug"] == enrollmentBookingsProfileKey {
			m.bookings = true
			m.requests = make(map[string]map[string]any)
			m.guardians = make(map[string]map[string]any)
			m.schedules = make(map[string][]map[string]any)
			m.settings = make(map[string]any)
		}
		schoolID := 3
		if m.bookings {
			schoolID = 4
		}
		data = map[string]any{"id": schoolID, "subdomain": body["slug"]}
	case strings.HasSuffix(path, "/invite-admin"):
		data = map[string]any{"token": "weekly-admin"}
	case path == "/auth/login":
		token := "weekly-admin"
		if body["tenant_slug"] == "vollbetrieb" {
			token = "developer-primary"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": token, "refresh_token": "refresh"})
		return true
	case strings.Contains(path, "/settings/values/"):
		m.settings[parts[len(parts)-1]] = body["value"]
	case strings.HasSuffix(path, "/settings/schema"):
		items := []map[string]any{}
		for key, value := range m.settings {
			items = append(items, map[string]any{"key": key, "value": value})
		}
		data = map[string]any{"tabs": []map[string]any{{"categories": []map[string]any{{"items": items}}}}}
	case path == "/api/iot/checkin":
		m.terminalScans++
		action := "checked_in"
		if m.terminalScans%2 == 0 {
			action = "checked_out"
		}
		data = map[string]any{"action": action}
	case path == "/api/students/arrival-settings":
		data = map[string]any{"care_days_source": "bookings"}
	case path == "/api/iot/config":
		data = map[string]any{"presence_mode": m.settings[profileSettingPresenceMode]}
	case strings.HasSuffix(path, "/school-checkin"):
		status := "checked_in"
		if body["action"] == "out" {
			status = "checked_out"
		}
		data = map[string]any{"status": status, "changed": true}
	case path == "/api/active/visits" || strings.HasSuffix(path, "/visit-history"):
		data = []any{}
	case strings.Contains(path, "/care-withdrawals/") && strings.HasSuffix(path, "/preview"):
		for _, trace := range m.traces {
			trace.withdrawalPreviews++
		}
		data = map[string]any{"token": "preview"}
	case strings.Contains(path, "/care-withdrawals/") && strings.HasSuffix(path, "/care-end"):
		for _, trace := range m.traces {
			trace.withdrawalEnds++
		}
	case path == "/api/students/care-withdrawals":
		item := map[string]any{"id": "42"}
		if r.URL.Query().Get("state") != "" {
			studentID := r.URL.Query().Get("student_id")
			item["student_id"], item["state"], item["urgency"] = studentID, r.URL.Query().Get("state"), "planned"
			if item["state"] == "resolved" {
				item["outcome"] = "care_ended"
			}
			for _, trace := range m.traces {
				if len(trace.withdrawalRemovals) == 3 && studentID == m.withdrawalStudentIDs[1] {
					item["urgency"] = "overdue"
				}
			}
		}
		data = map[string]any{"items": []map[string]any{item}}
	case strings.HasSuffix(path, "/offerings") && r.Method == "PUT":
		m.withdrawalStudentIDs = append(m.withdrawalStudentIDs, parts[6])
		for _, trace := range m.traces {
			trace.withdrawalRemovals = append(trace.withdrawalRemovals, body)
			trace.withdrawalToday = todaySeedDate()
		}
		data = map[string]any{"created_student_id": parts[6]}
	case path == "/api/iot/" && r.Method == "POST":
		data = map[string]any{"id": m.nextID, "api_key": "synthetic-device-key"}
	case path == "/api/iot/":
		data = []map[string]any{}
		if m.bookings && r.URL.Query().Get("device_type") == "terminal" {
			data = []map[string]any{{"id": 42, "device_id": "BUCHUNGEN-NFC-001", "device_type": "terminal"}}
		}
		if r.URL.Query().Get("device_type") == "virtual" {
			data = []map[string]any{{"device_id": webManualDeviceID, "device_type": "virtual"}}
		}
	case path == "/api/students":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "pagination": map[string]any{"total_records": len(m.guardians)}})
		return true
	case path == "/api/enrollment/care-offerings":
		require.Equal(t, true, body["counts_as_care"])
		require.Len(t, body["pickup_times"], 5)
		m.offeringID = m.nextID
	case path == "/api/enrollment/phases":
		m.phase = body
	case strings.HasPrefix(path, "/api/enrollment/phases/"):
		data = m.phase
	case strings.HasSuffix(path, "/submit"):
		require.Contains(t, []string{enrollmentWeeklyProfileKey, enrollmentBookingsProfileKey}, parts[len(parts)-2])
		id := fmt.Sprint(m.nextID)
		childID := fmt.Sprint(m.nextID + 1000)
		m.requests[id] = map[string]any{"status_token": id, "children": []map[string]any{{"id": childID, "status": "submitted"}}}
		m.guardians[childID] = map[string]any{"id": m.nextID + 2000, "email": body["guardian_email"]}
		data = map[string]any{"request_id": id, "status_url": "http://demo.test/status/" + id}
	case strings.HasSuffix(path, "/decide"):
		request := m.requests[parts[4]]
		request["children"].([]map[string]any)[0]["status"] = body["status"]
		if body["status"] == "approved" {
			children := request["children"].([]map[string]any)
			children[0]["created_student_id"] = children[0]["id"]
		}
	case strings.HasSuffix(path, "/withdraw"):
		m.requests[parts[3]]["children"].([]map[string]any)[0]["status"] = "withdrawn"
	case strings.HasPrefix(path, "/api/enrollment/admin/requests/"):
		data = m.requests[parts[4]]
	case strings.HasSuffix(path, "/guardians"):
		data = []map[string]any{{"guardian": m.guardians[parts[3]]}}
	case strings.HasSuffix(path, "/invite"):
		data = map[string]any{"token": "weekly-guardian"}
	case strings.HasPrefix(path, "/auth/guardian-invitations/"):
		data = map[string]any{"account_id": m.nextID}
	case path == "/parent/auth/login":
		data = map[string]any{"access_token": "weekly-parent", "refresh_token": "refresh"}
	case strings.HasSuffix(path, "/arrival-schedules") || strings.HasSuffix(path, "/pickup-schedules"):
		rows := []map[string]any{}
		for _, row := range body["schedules"].([]any) {
			rows = append(rows, row.(map[string]any))
		}
		m.schedules[parts[2]+"/"+parts[3]] = rows
	case strings.HasSuffix(path, "/care-offerings"):
		data = map[string]any{"offerings": []map[string]any{{"id": fmt.Sprint(m.offeringID), "weekdays": []int{1}}}}
	case strings.HasSuffix(path, "/care-schedule"):
		rows := []map[string]any{}
		for day := 1; day <= 5; day++ {
			row := map[string]any{"weekday": day, "status": "not_scheduled"}
			for _, arrival := range m.schedules[parts[3]+"/arrival-schedules"] {
				if arrival["weekday"] == float64(day) {
					row["status"] = "scheduled"
					row["arrival"] = arrival["expected_arrival"]
				}
			}
			for _, pickup := range m.schedules[parts[3]+"/pickup-schedules"] {
				if pickup["weekday"] == float64(day) {
					row["pickup"] = pickup["pickup_time"]
				}
			}
			if m.settings[profileSettingBookingsAuthoritative] == true {
				row = map[string]any{"weekday": day, "status": "not_scheduled"}
				if day == 1 {
					row["status"], row["arrival"], row["pickup"] = "scheduled", "12:00", "15:00"
				}
			}
			rows = append(rows, row)
		}
		data = map[string]any{"weekdays": rows}
	case path == "/auth/roles":
		data = []map[string]any{{"id": 9000, "name": "admin"}}
	case path == "/auth/link-to-tenant":
		data = map[string]any{"school_identity": map[string]any{"staff_id": "9001"}}
	case path == "/auth/account/tenants":
		schoolID := 3
		if m.bookings {
			schoolID = 4
		}
		data = []map[string]any{{"tenant_id": schoolID}}
	case path == "/auth/switch-tenant":
		if body["tenant_slug"] == "vollbetrieb" {
			w.WriteHeader(401)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "tenant access denied"})
			return true
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "developer-secondary"})
		return true
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": data})
	return true
}
