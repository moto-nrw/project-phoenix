package api

import (
	"encoding/json"
	"fmt"
	"strconv"
)

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
