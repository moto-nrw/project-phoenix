package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The seeder gives the first class a class-wide arrival day exception on the
// next school day (#2962) through the same endpoint the Klassen-Modal uses.
func TestFixedSeeder_SeedClassArrivalException(t *testing.T) {
	t.Parallel()

	var paths []string
	var bodies []map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		require.Equal(t, "PUT", r.Method)
		paths = append(paths, r.URL.Path)
		body := map[string]any{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		bodies = append(bodies, body)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"
	fs := NewFixedSeeder(client, true, "")
	result := &FixedResult{}

	require.NoError(t, fs.seedClassArrivalException(context.TODO(), result))

	assert.Equal(t, 1, result.ClassArrivalExceptionCount)
	require.Len(t, paths, 1)
	// r.URL.Path arrives decoded; the seeder escapes the class on the wire.
	assert.True(t, strings.HasPrefix(paths[0], "/api/students/class-arrival-exceptions/Klasse 1a/"), paths[0])
	assert.Equal(t, "11:00", bodies[0]["arrival_time"])
	assert.Equal(t, "Unterricht fällt aus", bodies[0]["reason"])
}
