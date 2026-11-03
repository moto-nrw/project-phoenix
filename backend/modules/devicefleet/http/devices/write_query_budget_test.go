package devices

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Independent wire contract, not a process facade DTO or an owner composition.
type deviceWriteBudgetWire struct {
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
	APIKey         *string `json:"api_key,omitempty"`
}

func TestDeviceWriteQueryBudgets(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"create-no-room", "update-no-room", "update-with-room"} {
		for _, n := range []int{1, 3, 8} {
			t.Run(fmt.Sprintf("%s/N%d", scenario, n), func(t *testing.T) {
				testpkg.OwnTenant(t)
				db, fleet := ComposeDeviceFleet(t)
				var transactionStamp time.Time
				runtime := testRuntime()
				success := runtime.Success
				runtime.Success = func(w http.ResponseWriter, r *http.Request, status int, data any, message string) {
					raw, present := testpkg.TransactionFromContext(r.Context())
					require.True(t, present)
					tx, ok := raw.(bun.Tx)
					require.True(t, ok)
					var role, tenant string
					// Test-only metadata on the actual transaction, explicitly
					// unmarked. No production statement is filtered from counts.
					require.NoError(t, tx.NewRaw("SELECT current_user, current_setting('app.current_tenant_id'), CURRENT_TIMESTAMP").Scan(testpkg.Ctx(t), &role, &tenant, &transactionStamp))
					require.Equal(t, "phoenix_tenant", role)
					require.Equal(t, fmt.Sprint(testpkg.Tenant(t)), tenant)
					success(w, r, status, data, message)
				}
				router := testutil.NewTenantRouter(db)
				router.Mount("/devices", NewResource(devicefleet.NewAdministration(fleet), runtime).Router())
				var lastID int64
				for i := 0; i < n; i++ {
					id := fmt.Sprintf("write-budget-%s-%d-%d", scenario, n, i)
					person := testpkg.CreateTestPerson(t, db, "Budget", fmt.Sprintf("Registrar%d", i))
					var roomID *int64
					var roomName *string
					if scenario == "update-with-room" {
						room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Write budget room %d", i))
						roomID, roomName = &room.ID, &room.Name
					}
					var before devicefleet.Device
					method, path, permission, status := http.MethodPost, "/devices/", "iot:manage", http.StatusCreated
					if scenario != "create-no-room" {
						seen := time.Now().Truncate(time.Microsecond)
						require.NoError(t, testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context) error {
							var err error
							before, err = fleet.CreateDevice(ctx, devicefleet.CreateDevice{DeviceID: id, DeviceType: "sensor", Name: new("Before"), Status: devicefleet.DeviceStatusOffline, LastSeen: &seen, RoomID: roomID})
							return err
						}))
						method, path, permission, status = http.MethodPut, fmt.Sprintf("/devices/%d", before.ID), "iot:update", http.StatusOK
					}
					name := "After"
					body := map[string]any{"device_id": id, "device_type": "terminal", "name": name, "status": "active", "registered_by_id": person.ID}
					request := testutil.NewAuthenticatedRequest(t, method, path, body, testutil.WithClaims(t, testutil.DefaultTestClaims()), testutil.WithPermissions(permission))
					ctx, cancel := context.WithTimeout(request.Context(), 10*time.Second)
					var dbStart, dbEnd time.Time
					require.NoError(t, db.NewRaw("SELECT clock_timestamp()").Scan(testpkg.Ctx(t), &dbStart))
					hostStart := time.Now().Truncate(time.Microsecond)
					counter := testpkg.CaptureQueriesForContext(t, db)
					counter.Reset()
					response := testutil.ExecuteRequest(router, request.WithContext(counter.Context(ctx)))
					counter.Stop()
					cancel()
					hostEnd := time.Now().Add(time.Microsecond)
					require.NoError(t, db.NewRaw("SELECT clock_timestamp()").Scan(testpkg.Ctx(t), &dbEnd))
					require.Equal(t, status, response.Code)
					wire, key := decodeDeviceWriteBudgetWire(t, response.Body.Bytes(), scenario == "create-no-room")
					var row devicefleet.Device
					require.NoError(t, testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context) error {
						var err error
						row, err = fleet.FindDevice(ctx, wire.ID)
						return err
					}))
					require.Positive(t, row.ID)
					require.True(t, row.UpdatedAt.Equal(transactionStamp), "exact trigger/default transaction time")
					require.Nil(t, row.APIKey, "stored reads must not disclose a raw credential")
					require.NotNil(t, row.APIKeyHash)
					require.False(t, row.UpdatedAt.Before(dbStart))
					require.False(t, row.UpdatedAt.After(dbEnd))
					want := before
					want.DeviceID, want.DeviceType, want.Name, want.Status, want.RegisteredByID = id, "terminal", &name, devicefleet.DeviceStatusActive, &person.ID
					want.UpdatedAt, want.RoomName = row.UpdatedAt, roomName
					if scenario == "create-no-room" {
						require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
						require.True(t, row.CreatedAt.Equal(row.UpdatedAt))
						require.NotNil(t, row.LastSeen)
						require.False(t, row.LastSeen.Before(hostStart))
						require.False(t, row.LastSeen.After(hostEnd))
						hash := testpkg.TokenHash(t, db, key)
						require.True(t, *row.APIKeyHash == hash, "one-time credential fingerprint mismatch")
						require.True(t, row.MaskedAPIKey == "..."+key[len(key)-4:], "one-time credential mask mismatch")
						want.ID, want.TenantID, want.CreatedAt, want.LastSeen = row.ID, testpkg.Tenant(t), row.CreatedAt, row.LastSeen
					} else {
						require.Empty(t, response.Header().Get("Cache-Control"))
						require.True(t, before.APIKeyHash != nil && *before.APIKeyHash == *row.APIKeyHash, "update changed credential fingerprint")
						require.True(t, before.MaskedAPIKey == row.MaskedAPIKey, "update changed credential mask")
					}
					require.Equal(t, testpkg.Tenant(t), row.TenantID)
					assertDeviceWriteBudgetRow(t, want, row)
					lastSeen := row.LastSeen.Format(time.RFC3339)
					require.Equal(t, deviceWriteBudgetWire{ID: row.ID, DeviceID: id, DeviceType: "terminal", Name: &name, Status: "active", LastSeen: &lastSeen, RegisteredByID: &person.ID, RoomName: roomName, IsOnline: true, CreatedAt: row.CreatedAt.Format(time.RFC3339), UpdatedAt: row.UpdatedAt.Format(time.RFC3339)}, wire)
					require.Equal(t, 1, len(counter.Operation("INSERT"))+len(counter.Operation("UPDATE")))
					inserts := 0
					if scenario == "create-no-room" {
						inserts = 1
					}
					require.Len(t, counter.Operation("INSERT"), inserts)
					require.Empty(t, counter.Operation("DELETE"))
					require.Empty(t, counter.Operation("MERGE"))
					require.EqualValues(t, 1, counter.WriteRows())
					// Redact only the exact stored credential from diagnostics. All
					// production statements, including BEGIN/role+GUC/COMMIT, remain.
					queries := redactDeviceWriteBudgetSQL(counter.Queries(), *row.APIKeyHash, row.MaskedAPIKey)
					require.Equal(t, 1, countDeviceWriteBudgetSelects(queries, "config.setting_values"), "fresh LastSeen must exercise the online-window read")
					require.Zero(t, countDeviceWriteBudgetSelects(queries, "users.persons"), "valid registrar uses PostgreSQL FK checks, not an extra client read")
					rooms := 0
					if roomID != nil {
						rooms = 3
					}
					require.Equal(t, rooms, countDeviceWriteBudgetSelects(queries, "facilities.rooms"))
					switch scenario {
					case "create-no-room":
						querybudgets.AssertQueryBudget(t, "api.iot.devices.create.no_room", queries)
					case "update-no-room":
						querybudgets.AssertQueryBudget(t, "api.iot.devices.update.no_room", queries)
					case "update-with-room":
						querybudgets.AssertQueryBudget(t, "api.iot.devices.update.with_room", queries)
					}
					lastID = row.ID
				}
				foreign := testpkg.UniqueTestTenantID(t)
				testpkg.EnsureTestTenant(t, db, foreign)
				require.NoError(t, testpkg.WithinTenantContext(t, testpkg.ContextForTenant(testpkg.Ctx(t), foreign), db, foreign, func(ctx context.Context) error {
					raw, ok := testpkg.TransactionFromContext(ctx)
					require.True(t, ok)
					tx, ok := raw.(bun.Tx)
					require.True(t, ok)
					var role, tenant string
					var visible int
					require.NoError(t, tx.NewRaw("SELECT current_user, current_setting('app.current_tenant_id'), (SELECT count(*) FROM iot.devices WHERE id=?)", lastID).Scan(ctx, &role, &tenant, &visible))
					require.Equal(t, "phoenix_tenant", role)
					require.Equal(t, fmt.Sprint(foreign), tenant)
					require.Zero(t, visible, "actual RLS must hide the written row from another school")
					return nil
				}))
			})
		}
	}
}

func decodeDeviceWriteBudgetWire(t *testing.T, body []byte, create bool) (deviceWriteBudgetWire, string) {
	t.Helper()
	var envelope struct {
		Status  string          `json:"status"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	require.True(t, decoder.Decode(&envelope) == nil, "invalid success envelope")
	require.Equal(t, "success", envelope.Status)
	message := "Device updated successfully"
	if create {
		message = "Device created successfully"
	}
	require.Equal(t, message, envelope.Message)
	var wire deviceWriteBudgetWire
	decoder = json.NewDecoder(bytes.NewReader(envelope.Data))
	decoder.DisallowUnknownFields()
	require.True(t, decoder.Decode(&wire) == nil, "invalid or secret-bearing wire shape")
	require.Equal(t, create, wire.APIKey != nil, "raw key is create-only")
	key := ""
	if wire.APIKey != nil {
		key = *wire.APIKey
		require.True(t, regexp.MustCompile(`^dev_[0-9a-f]{64}$`).MatchString(key), "invalid generated key format")
	}
	wire.APIKey = nil
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(envelope.Data, &fields))
	delete(fields, "api_key")
	actual, err := json.Marshal(fields)
	require.NoError(t, err)
	want, err := json.Marshal(wire)
	require.NoError(t, err)
	require.JSONEq(t, string(want), string(actual), "exact default/omitempty field presence")
	return wire, key
}

func assertDeviceWriteBudgetRow(t *testing.T, want, got devicefleet.Device) {
	t.Helper()
	// Credential relationships are asserted separately without secret output.
	want.APIKey, got.APIKey, want.APIKeyHash, got.APIKeyHash = nil, nil, nil, nil
	want.MaskedAPIKey, got.MaskedAPIKey = "", ""
	require.Equal(t, want, got, "all persisted fields, tenant and room projection")
}

func redactDeviceWriteBudgetSQL(queries []string, hash, mask string) []string {
	redact := strings.NewReplacer(hash, "<device-key-hash>", mask, "<device-key-mask>")
	result := make([]string, len(queries))
	for i, query := range queries {
		result[i] = redact.Replace(query)
	}
	return result
}

func countDeviceWriteBudgetSelects(queries []string, table string) int {
	count := 0
	for _, query := range queries {
		query = strings.ToLower(strings.ReplaceAll(query, `"`, ""))
		if strings.HasPrefix(strings.TrimSpace(query), "select") && strings.Contains(query, table) {
			count++
		}
	}
	return count
}
