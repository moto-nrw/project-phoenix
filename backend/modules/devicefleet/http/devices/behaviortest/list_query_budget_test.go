package devices_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/devicefleet"
	devices "github.com/moto-ogs/project-phoenix/modules/devicefleet/http/devices"
	"github.com/moto-ogs/project-phoenix/modules/settings"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

// Deliberately independent of the production response and process-facade DTOs.
type deviceListBudgetWire struct {
	ID             int64   `json:"id"`
	DeviceID       string  `json:"device_id"`
	DeviceType     string  `json:"device_type"`
	Name           *string `json:"name,omitempty"`
	Status         string  `json:"status"`
	LastSeen       *string `json:"last_seen,omitempty"`
	RegisteredByID *int64  `json:"registered_by_id,omitempty"`
	RoomName       *string `json:"room_name,omitempty"`
	IsOnline       bool    `json:"is_online"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

func TestDeviceListQueryBudgets(t *testing.T) {
	t.Parallel()
	// Initialize the top-level tenant before subtests, including -count reruns.
	_ = testpkg.Tenant(t)
	for _, route := range []string{"root", "type", "status", "registered-by", "active", "maintenance", "offline"} {
		t.Run(route, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db, fleet := composeDeviceFleet(t)
			tenant := testpkg.Tenant(t)
			foreign := testpkg.UniqueTestTenantID(t)
			testpkg.EnsureTestTenant(t, db, foreign)
			testpkg.SetTestTenantSetting(t, db, tenant, settings.KeyDeviceOnlineWindowMinutes, "60")
			// Distinguish the 60-minute override from the five-minute registry default.
			recent, old := time.Now().Add(-20*time.Minute), time.Now().Add(-48*time.Hour)
			registrar := testpkg.CreateTestPerson(t, db, "List", "Target")
			otherRegistrar := testpkg.CreateTestPerson(t, db, "List", "Other")
			status := devicefleet.DeviceStatusActive
			if route == "maintenance" {
				status = devicefleet.DeviceStatusMaintenance
			}
			create := func(school int64, input devicefleet.CreateDevice, backdate bool) devicefleet.Device {
				input.DeviceID = fmt.Sprintf("list-budget-%d-%s", school, input.DeviceID)
				var row devicefleet.Device
				err := testpkg.WithinTenantContext(t, testpkg.ContextForTenant(testpkg.Ctx(t), school), db, school, func(ctx context.Context) error {
					var err error
					row, err = fleet.CreateDevice(ctx, input)
					if err != nil {
						return err
					}
					row.APIKey, row.APIKeyHash, row.MaskedAPIKey = nil, nil, ""
					if backdate {
						// Fixture-only timestamp arrangement, outside request capture.
						raw, present := testpkg.TransactionFromContext(ctx)
						require.True(t, present)
						tx, ok := raw.(bun.Tx)
						require.True(t, ok)
						err = tx.NewRaw("UPDATE iot.devices SET created_at = ? WHERE id = ? RETURNING created_at, updated_at", old, row.ID).Scan(ctx, &row.CreatedAt, &row.UpdatedAt)
					}
					return err
				})
				require.NoError(t, err)
				return row
			}
			foreignPerson := testpkg.CreateTestPersonForTenant(t, db, foreign, "Foreign", "Registrar")
			foreignRoom := testpkg.CreateTestRoomForTenant(t, db, foreign, "Foreign room")
			foreignRow := create(foreign, devicefleet.CreateDevice{DeviceID: "foreign", DeviceType: "terminal", Status: status, LastSeen: &old, RegisteredByID: &foreignPerson.ID, RoomID: &foreignRoom.ID}, false)
			archived := create(tenant, devicefleet.CreateDevice{DeviceID: "archived", DeviceType: "terminal", Status: status, LastSeen: &old, RegisteredByID: &registrar.ID}, false)
			require.NoError(t, testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, tenant, func(ctx context.Context) error {
				rows, err := fleet.UpdateDeviceColumns(ctx, devicefleet.UpdateDevice{ID: archived.ID, ArchivedAt: &old}, []string{"archived_at"})
				require.EqualValues(t, 1, rows)
				return err
			}))
			// All list requests run under the real tenant role and RLS. This
			// test-only metadata query uses an unmarked context on the SAME tx.
			// No production SQL is removed from the captured request.
			runtime := testRuntime()
			success := runtime.Success
			runtime.Success = func(w http.ResponseWriter, r *http.Request, code int, data any, message string) {
				raw, present := testpkg.TransactionFromContext(r.Context())
				require.True(t, present)
				tx, ok := raw.(bun.Tx)
				require.True(t, ok)
				var role, school string
				var visible int
				require.NoError(t, tx.NewRaw("SELECT current_user, current_setting('app.current_tenant_id'), (SELECT count(*) FROM iot.devices WHERE id = ?)", foreignRow.ID).Scan(testpkg.Ctx(t), &role, &school, &visible))
				require.Equal(t, "phoenix_tenant", role)
				require.Equal(t, fmt.Sprint(tenant), school)
				require.Zero(t, visible, "RLS must hide a real other-school row")
				success(w, r, code, data, message)
			}
			router := testutil.NewTenantRouter(db)
			router.Mount("/", devices.NewResource(devicefleet.NewAdministration(fleet), runtime).Router())
			message, path := "Devices retrieved successfully", "/"
			switch route {
			case "type":
				path = "/type/terminal"
			case "status":
				path = "/status/active"
			case "registered-by":
				path = fmt.Sprintf("/registered-by/%d", registrar.ID)
			case "active":
				path, message = "/active", "Active devices retrieved successfully"
			case "maintenance":
				path, message = "/maintenance", "Devices requiring maintenance retrieved successfully"
			case "offline":
				path, message = "/offline?duration=24h", "Offline devices retrieved successfully"
			}
			want := make(map[int64]deviceListBudgetWire)
			controlStatus := devicefleet.DeviceStatusInactive
			if route == "maintenance" {
				controlStatus = devicefleet.DeviceStatusActive
			}
			control := create(tenant, devicefleet.CreateDevice{DeviceID: "control", DeviceType: "sensor", Status: controlStatus, LastSeen: &recent, RegisteredByID: &otherRegistrar.ID}, false)
			if route == "root" {
				want[control.ID] = expectedDeviceListWire(control, nil, true)
			}
			// A never-seen newly created device is NOT offline. Other routes
			// use their normal nonmatching controls above.
			if route == "offline" {
				create(tenant, devicefleet.CreateDevice{DeviceID: "never-seen-new", DeviceType: "terminal", Status: devicefleet.DeviceStatusOffline}, false)
			}
			var manual devicefleet.Device
			if route == "root" {
				// The default root hides this reserved device; explicit type
				// filters do not. Use the owner seam, not a synthetic ID.
				manual = create(tenant, devicefleet.CreateDevice{DeviceID: "manual-placeholder", DeviceType: "virtual", Status: devicefleet.DeviceStatusActive}, false)
				require.NoError(t, testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, tenant, func(ctx context.Context) error {
					_, err := fleet.UpdateDeviceColumns(ctx, devicefleet.UpdateDevice{ID: manual.ID, DeviceID: devicefleet.WebManualDeviceID}, []string{"device_id"})
					if err == nil {
						manual, err = fleet.FindDevice(ctx, manual.ID)
						manual.APIKey, manual.APIKeyHash, manual.MaskedAPIKey = nil, nil, ""
					}
					return err
				}))
			}
			request := func(t *testing.T, target string, expected map[int64]deviceListBudgetWire) *testpkg.QueryCounter {
				t.Helper()
				req := testutil.NewAuthenticatedRequest(t, http.MethodGet, target, nil, testutil.WithClaims(t, testutil.DefaultTestClaims()), testutil.WithPermissions("iot:read"))
				ctx, cancel := context.WithTimeout(settings.WithSettingsRequestCache(req.Context()), 10*time.Second)
				defer cancel()
				counter := testpkg.CaptureQueriesForContext(t, db)
				counter.Reset()
				response := testutil.ExecuteRequest(testpkg.MountedQueryBudgetRouter("/api/iot", router), req.WithContext(counter.Context(ctx)))
				counter.Stop()
				require.Equal(t, http.StatusOK, response.Code)
				require.Contains(t, response.Header().Get("Content-Type"), "application/json")
				require.Empty(t, response.Header().Get("Cache-Control"))
				assertDeviceListBudgetWire(t, response.Body.Bytes(), message, expected)
				for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
					require.Zero(t, len(counter.Operation(operation)), "GET must not write")
				}
				require.Zero(t, counter.WriteRows())
				return counter
			}
			grown := 0
			for _, n := range []int{1, 3, 8} {
				for grown < n {
					person := registrar
					if route != "registered-by" {
						person = testpkg.CreateTestPerson(t, db, "List", fmt.Sprintf("Registrar%d", grown))
					}
					room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("List room %d", grown))
					seen := &recent
					if grown%3 == 1 || route == "offline" {
						seen = &old
					}
					if grown%3 == 2 {
						seen = nil
					}
					rowStatus := status
					if route == "offline" && grown%2 == 1 {
						rowStatus = devicefleet.DeviceStatusInactive
					}
					var name *string
					if grown%2 == 0 {
						value := fmt.Sprintf("Device %d", grown)
						name = &value
					}
					row := create(tenant, devicefleet.CreateDevice{DeviceID: fmt.Sprintf("target-%d", grown), DeviceType: "terminal", Name: name, Status: rowStatus, LastSeen: seen, RegisteredByID: &person.ID, RoomID: &room.ID}, route == "offline" && seen == nil)
					var roomName *string
					if route != "offline" {
						roomName = &room.Name
					}
					want[row.ID] = expectedDeviceListWire(row, roomName, seen == &recent)
					grown++
				}
				t.Run(fmt.Sprintf("N%d", n), func(t *testing.T) {
					counter := request(t, path, want)
					require.Len(t, counter.Selects("iot.devices"), 1, "one live-device set read")
					require.NotEmpty(t, counter.Selects("config.setting_values"), "nonnil LastSeen exercises the online-window setting")
					rooms := 1
					if route == "offline" {
						rooms = 0
					}
					require.Len(t, counter.Selects("facilities.rooms"), rooms, "room ID-set read, never one read per device")
					queries := counter.Measurement(counter.Queries())
					switch route {
					case "root":
						querybudgets.AssertQueryBudget(t, "api.iot.devices.list", queries)
					case "type":
						querybudgets.AssertQueryBudget(t, "api.iot.devices.list.type", queries)
					case "status":
						querybudgets.AssertQueryBudget(t, "api.iot.devices.list.status", queries)
					case "registered-by":
						querybudgets.AssertQueryBudget(t, "api.iot.devices.list.registered_by", queries)
					case "active":
						querybudgets.AssertQueryBudget(t, "api.iot.devices.list.active", queries)
					case "maintenance":
						querybudgets.AssertQueryBudget(t, "api.iot.devices.list.maintenance", queries)
					case "offline":
						querybudgets.AssertQueryBudget(t, "api.iot.devices.list.offline", queries)
					}
				})
			}
			if route == "root" {
				// Semantic controls, not extra measured samples. The root query
				// must combine all four filters; search is case-insensitive.
				for id, row := range want {
					if row.DeviceType != "terminal" {
						continue
					}
					query := url.Values{"device_type": {row.DeviceType}, "status": {row.Status}, "registered_by_id": {fmt.Sprint(*row.RegisteredByID)}, "search": {"TARGET-"}}
					request(t, "/?"+query.Encode(), map[int64]deviceListBudgetWire{id: row})
					break
				}
				request(t, "/?device_type=virtual", map[int64]deviceListBudgetWire{manual.ID: expectedDeviceListWire(manual, nil, false)})
			}
		})
	}
}

func expectedDeviceListWire(row devicefleet.Device, roomName *string, online bool) deviceListBudgetWire {
	wire := deviceListBudgetWire{ID: row.ID, DeviceID: row.DeviceID, DeviceType: row.DeviceType, Name: row.Name, Status: string(row.Status), RegisteredByID: row.RegisteredByID, RoomName: roomName, IsOnline: online, CreatedAt: row.CreatedAt.Format(time.RFC3339), UpdatedAt: row.UpdatedAt.Format(time.RFC3339)}
	if row.LastSeen != nil {
		value := row.LastSeen.Format(time.RFC3339)
		wire.LastSeen = &value
	}
	return wire
}

func assertDeviceListBudgetWire(t *testing.T, body []byte, message string, want map[int64]deviceListBudgetWire) {
	t.Helper()
	var envelope struct {
		Status  string            `json:"status"`
		Message string            `json:"message"`
		Data    []json.RawMessage `json:"data"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	require.True(t, decoder.Decode(&envelope) == nil, "invalid or secret-bearing envelope")
	require.Equal(t, "success", envelope.Status)
	require.Equal(t, message, envelope.Message)
	require.NotNil(t, envelope.Data, "lists encode arrays, not null")
	got := make(map[int64]deviceListBudgetWire, len(envelope.Data))
	for _, raw := range envelope.Data {
		var row deviceListBudgetWire
		decoder = json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		// Do not print body/decoder errors: an unexpected field could be a key.
		require.True(t, decoder.Decode(&row) == nil, "invalid or secret-bearing list DTO")
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &fields))
		encoded, err := json.Marshal(row)
		require.NoError(t, err)
		actual, err := json.Marshal(fields)
		require.NoError(t, err)
		require.JSONEq(t, string(encoded), string(actual), "exact optional-field presence")
		_, duplicate := got[row.ID]
		require.False(t, duplicate, "duplicate list device")
		got[row.ID] = row
	}
	// SQL has no ORDER BY. Prove set identity, not an invented array order.
	require.Equal(t, want, got)
}
