package checkin

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// The binary check-in path already logged IDs only (#2062). Its greeting and
// student_name response field are built right next to the log call, so this is
// the guard that keeps them from sliding into it.
func TestProcessBinaryModeCheckin_LogsOmitStudentNameAndGreeting(t *testing.T) {
	const (
		firstName = "Loggingprobe"
		lastName  = "Namenspruefung"
	)

	var logs bytes.Buffer
	resource := &Resource{
		ActiveService: &binaryModeActiveServiceStub{},
		logger:        slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}
	student := &users.Student{
		Model:  base.Model{ID: 42},
		Person: &users.Person{FirstName: firstName, LastName: lastName},
	}

	response := httptest.NewRecorder()
	resource.processBinaryModeCheckin(
		response,
		httptest.NewRequest(http.MethodPost, "/checkin", nil),
		student,
		&iot.Device{Model: base.Model{ID: 7}},
		time.Now(),
	)
	require.Equal(t, http.StatusOK, response.Code)

	// The kiosk keeps the personalized greeting and the full name.
	assert.Contains(t, response.Body.String(), "Willkommen, "+firstName+"!")
	assert.Contains(t, response.Body.String(), firstName+" "+lastName)

	// The log keeps only technical identifiers.
	out := logs.String()
	require.Contains(t, out, `"msg":"binary mode checkin complete"`)
	assert.Contains(t, out, `"action":"checked_in"`)
	for _, forbidden := range []string{firstName, lastName, "Willkommen", "Tschüss", `"message"`} {
		assert.NotContainsf(t, out, forbidden, "Info logs must not carry %q:\n%s", forbidden, out)
	}
}
