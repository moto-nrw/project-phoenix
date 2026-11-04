package api

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyDeviceAdministrationLocked_ExpectsModuleBookingRefusal(t *testing.T) {
	t.Parallel()

	var paths []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/api/iot/":
			assert.Equal(t, "Bearer tenant-token", r.Header.Get("Authorization"))
			assert.Equal(t, "terminal", r.URL.Query().Get("device_type"))
			w.WriteHeader(seedHTTPStatusForbidden)
			_, _ = fmt.Fprint(w, `{"status":403,"detail":"module nfc_anwesenheit is not booked","code":"module.not_booked"}`)
		case "/operator/schools/42/devices":
			assert.Equal(t, "Bearer operator-token", r.Header.Get("Authorization"))
			_, _ = fmt.Fprint(w, `{"data":[{"id":9000,"device_id":"WEB-MANUAL-001","device_type":"virtual","name":"Web-Portal (Manuell)","status":"active"}]}`)
		case "/operator/devices/9000/transfer-status":
			assert.Equal(t, "Bearer operator-token", r.Header.Get("Authorization"))
			_, _ = fmt.Fprint(w, `{"data":{"is_protected":true}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.BindAuth(AuthRef{Kind: AuthBearer, Token: "tenant-token"})
	device, err := verifyDeviceAdministrationLocked(&Runtime{
		Client: client, TenantAuth: AuthRef{Kind: AuthBearer, Token: "tenant-token"},
		OperatorAuth: AuthRef{Kind: AuthBearer, Token: "operator-token"}, Bootstrap: &bootstrapSeedState{SchoolID: 42},
	})
	require.NoError(t, err)
	assert.Equal(t, SeedDevice{ID: 9000, DeviceID: webManualDeviceID, DeviceType: "virtual", Name: webManualDeviceName, Protected: true}, device)
	assert.Equal(t, []string{"/api/iot/", "/operator/schools/42/devices", "/operator/devices/9000/transfer-status"}, paths)
}

func TestVerifyDeviceAdministrationLocked_RejectsInvalidDeviceState(t *testing.T) {
	t.Parallel()

	validWebDevice := map[string]any{
		"id": 9000, "device_id": webManualDeviceID, "device_type": "virtual", "name": webManualDeviceName, "status": "active",
	}
	for _, tc := range []struct {
		name        string
		devices     []map[string]any
		isProtected bool
		want        string
	}{
		{name: "missing web device", want: "virtual web device WEB-MANUAL-001 missing"},
		{name: "physical terminal", devices: []map[string]any{validWebDevice, {"id": 9001, "device_id": "NFC-001", "device_type": "terminal"}}, want: "unexpected physical terminal NFC-001"},
		{name: "malformed web device", devices: []map[string]any{{"id": 0, "device_id": webManualDeviceID, "device_type": "virtual", "name": webManualDeviceName, "status": "active"}}, want: "invalid virtual web device WEB-MANUAL-001"},
		{name: "unprotected web device", devices: []map[string]any{validWebDevice}, want: "virtual web device WEB-MANUAL-001 is not protected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/iot/":
					w.WriteHeader(seedHTTPStatusForbidden)
					_, _ = fmt.Fprint(w, `{"status":403,"detail":"module nfc_anwesenheit is not booked","code":"module.not_booked"}`)
				case "/operator/schools/42/devices":
					_ = json.NewEncoder(w).Encode(map[string]any{"data": tc.devices})
				case "/operator/devices/9000/transfer-status":
					_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]bool{"is_protected": tc.isProtected}})
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
				}
			})
			defer srv.Close()

			client := newTestClient(srv.URL, false)
			client.BindAuth(AuthRef{Kind: AuthBearer, Token: "tenant-token"})
			_, err := verifyDeviceAdministrationLocked(&Runtime{
				Client: client, TenantAuth: AuthRef{Kind: AuthBearer, Token: "tenant-token"},
				OperatorAuth: AuthRef{Kind: AuthBearer, Token: "operator-token"}, Bootstrap: &bootstrapSeedState{SchoolID: 42},
			})
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestVerifyManualStudentRowsAcceptsAtSchoolBeforeCheckIn(t *testing.T) {
	t.Parallel()
	pickup := "2026-09-17T15:00:00+02:00"
	raw := []byte(`[
		{"id": 1, "school_class": "1a", "group_id": 7, "current_location": "Anwesend"},
		{"id": 2, "school_class": "1a", "group_id": 7, "current_location": "Abwesend", "actual_pickup_time": "` + pickup + `"},
		{"id": 3, "school_class": "1a", "group_id": 7, "current_location": "Schule"},
		{"id": 4, "school_class": "1a", "group_id": 7, "current_location": "Abwesend"}
	]`)
	var rows []struct {
		ID             int64   `json:"id"`
		SchoolClass    string  `json:"school_class"`
		GroupID        int64   `json:"group_id"`
		Location       string  `json:"current_location"`
		ActualPickupAt *string `json:"actual_pickup_time"`
	}
	require.NoError(t, json.Unmarshal(raw, &rows))

	err := verifyManualStudentRows(rows, SeedExpectedState{PresentStudents: 1, CheckedOutStudents: 1}, false)

	assert.NoError(t, err)
}

func TestVerifyManualStudentRowsRejectsRoomLocation(t *testing.T) {
	t.Parallel()
	raw := []byte(`[{"id": 5, "school_class": "1a", "group_id": 7, "current_location": "Raum 101"}]`)
	var rows []struct {
		ID             int64   `json:"id"`
		SchoolClass    string  `json:"school_class"`
		GroupID        int64   `json:"group_id"`
		Location       string  `json:"current_location"`
		ActualPickupAt *string `json:"actual_pickup_time"`
	}
	require.NoError(t, json.Unmarshal(raw, &rows))

	err := verifyManualStudentRows(rows, SeedExpectedState{}, false)

	assert.ErrorContains(t, err, `room-tracking location "Raum 101"`)
}

// With room tracking (presence mode "detailed") a child in
// a running supervision counts as present; a binary profile still rejects it.
func TestVerifyManualStudentRowsCountsRoomsOnlyWithRoomTracking(t *testing.T) {
	t.Parallel()
	raw := []byte(`[{"id": 6, "school_class": "1a", "group_id": 7, "current_location": "Anwesend - Bauraum"}]`)
	var rows []struct {
		ID             int64   `json:"id"`
		SchoolClass    string  `json:"school_class"`
		GroupID        int64   `json:"group_id"`
		Location       string  `json:"current_location"`
		ActualPickupAt *string `json:"actual_pickup_time"`
	}
	require.NoError(t, json.Unmarshal(raw, &rows))

	require.NoError(t, verifyManualStudentRows(rows, SeedExpectedState{PresentStudents: 1}, true))
	assert.ErrorContains(t, verifyManualStudentRows(rows, SeedExpectedState{PresentStudents: 1}, false), `room-tracking location "Anwesend - Bauraum"`)
}

func TestVerifyManualStudentRowsCountsTransitOnlyWithRoomTracking(t *testing.T) {
	t.Parallel()
	raw := []byte(`[{"id": 7, "school_class": "1a", "group_id": 7, "current_location": "Unterwegs"}]`)
	var rows []struct {
		ID             int64   `json:"id"`
		SchoolClass    string  `json:"school_class"`
		GroupID        int64   `json:"group_id"`
		Location       string  `json:"current_location"`
		ActualPickupAt *string `json:"actual_pickup_time"`
	}
	require.NoError(t, json.Unmarshal(raw, &rows))

	require.NoError(t, verifyManualStudentRows(rows, SeedExpectedState{PresentStudents: 1}, true))
	assert.Error(t, verifyManualStudentRows(rows, SeedExpectedState{PresentStudents: 1}, false))
}
