package checkin

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// Issue #2062 covers the binary check-in path too: it already logged only
// technical IDs, and this test is the guard that keeps it that way. The kiosk
// greeting ("Willkommen, Max!") and the student_name response field are built
// right next to the log call, so a future edit could easily slide one of them
// into the log line.

// TestProcessBinaryModeCheckin_LogsOmitStudentNameAndGreeting drives the binary
// attendance toggle with a capturing logger at Info level and asserts that the
// name reaches the HTTP response but not the log.
func TestProcessBinaryModeCheckin_LogsOmitStudentNameAndGreeting(t *testing.T) {
	const (
		firstName = "Loggingprobe"
		lastName  = "Namenspruefung"
	)

	var logs bytes.Buffer
	activeService := &binaryModeActiveServiceStub{}
	resource := &Resource{
		ActiveService: activeService,
		logger:        slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}
	student := &users.Student{
		Model:  base.Model{ID: 42},
		Person: &users.Person{FirstName: firstName, LastName: lastName},
	}
	kiosk := &iot.Device{Model: base.Model{ID: 7}}

	response := httptest.NewRecorder()
	resource.processBinaryModeCheckin(
		response,
		httptest.NewRequest(http.MethodPost, "/checkin", nil),
		student,
		kiosk,
		time.Now(),
	)

	require.Equal(t, http.StatusOK, response.Code)

	// The kiosk still receives the personalized greeting and the full name.
	body := response.Body.String()
	assert.Contains(t, body, "Willkommen, "+firstName+"!")
	assert.Contains(t, body, firstName+" "+lastName)

	// The log must not.
	output := logs.String()
	require.NotEmpty(t, output, "expected the binary check-in to emit an Info record")
	assert.NotContains(t, output, firstName)
	assert.NotContains(t, output, lastName)
	assert.NotContains(t, output, "Willkommen")
	assert.NotContains(t, output, "Tschüss")

	// Diagnostic fields stay available.
	var record map[string]any
	line := strings.TrimSpace(output)
	require.NoErrorf(t, json.Unmarshal([]byte(line), &record), "log line is not JSON: %s", line)
	assert.Equal(t, "binary mode checkin complete", record["msg"])
	assert.Equal(t, "checked_in", record["action"])
	assert.EqualValues(t, student.ID, record["student_id"])
	_, hasMessageAttr := record["message"]
	assert.False(t, hasMessageAttr, "binary check-in must not log a \"message\" attribute")
}
