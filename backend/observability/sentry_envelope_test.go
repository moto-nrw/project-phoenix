package observability_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testReporter = observability.SentryReporter{DeviceID: "gkt-flur", SchoolID: 42}

const testDSN = "https://public@o1.ingest.de.sentry.io/7"

func TestParseSentryDSN(t *testing.T) {
	t.Parallel()
	target, err := observability.ParseSentryDSN(testDSN)
	require.NoError(t, err)
	assert.Equal(t, observability.SentryTarget{ProjectID: "7", EnvelopeURL: "https://o1.ingest.de.sentry.io/api/7/envelope/"}, target)

	target, err = observability.ParseSentryDSN("http://public@localhost:9000/sentry/8")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9000/sentry/api/8/envelope/", target.EnvelopeURL, "a path prefix stays")
}

func TestParseSentryDSNRejectsMalformedDSN(t *testing.T) {
	t.Parallel()
	for _, dsn := range []string{
		"",
		"not a url",
		"ftp://key@sentry.example/1",
		"https://sentry.example/1",
		"https://key@sentry.example/",
		"https://key@sentry.example/project",
	} {
		_, err := observability.ParseSentryDSN(dsn)
		assert.Error(t, err, dsn)
	}
}

func TestTagSentryEnvelopeTagsEveryEventShape(t *testing.T) {
	t.Parallel()
	session := `{"sid":"s","status":"ok"}`
	envelope := `{"dsn":"` + testDSN + `"}` + "\n" +
		// Event without a length and with list tags.
		`{"type":"event"}` + "\n" +
		`{"message":"a","tags":[["device_id","forged"],["platform","gkt"]],"extra":{"big":12345678901234567890}}` + "\n" +
		// Other items pass unchanged.
		`{"type":"session","length":` + strconv.Itoa(len(session)) + `}` + "\n" + session + "\n" +
		// Event without tags, trailing blank line.
		`{"type":"event"}` + "\n" + `{"message":"b"}` + "\n\n"

	tagged, err := observability.TagSentryEnvelope([]byte(envelope), "7", testReporter)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSuffix(string(tagged), "\n"), "\n")
	require.Len(t, lines, 7)
	assert.Equal(t, `{"dsn":"`+testDSN+`"}`, lines[0])

	var first map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[2]), &first))
	assert.Equal(t, []any{
		[]any{"platform", "gkt"},
		[]any{"device_id", "gkt-flur"},
		[]any{"school_id", "42"},
	}, first["tags"])
	assert.Contains(t, lines[2], "12345678901234567890", "numbers keep their precision")
	assert.JSONEq(t, `{"type":"event","length":`+strconv.Itoa(len(lines[2]))+`}`, lines[1])

	assert.Equal(t, `{"type":"session","length":`+strconv.Itoa(len(session))+`}`, lines[3])
	assert.Equal(t, session, lines[4])

	assert.JSONEq(t, `{"message":"b","tags":{"device_id":"gkt-flur","school_id":"42"}}`, lines[6])
}

func TestTagSentryEnvelopeRefusesEnvelopes(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		envelope string
		want     error
	}{
		"no header":             {"garbage", observability.ErrSentryEnvelopeInvalid},
		"no dsn":                {`{"event_id":"x"}` + "\n", observability.ErrSentryProjectNotAllowed},
		"other project":         {`{"dsn":"https://public@o1.ingest.de.sentry.io/8"}` + "\n", observability.ErrSentryProjectNotAllowed},
		"bad item header":       {`{"dsn":"` + testDSN + `"}` + "\n" + "garbage\n", observability.ErrSentryEnvelopeInvalid},
		"length beyond the end": {`{"dsn":"` + testDSN + `"}` + "\n" + `{"type":"event","length":99}` + "\n{}\n", observability.ErrSentryEnvelopeInvalid},
		"event is no object":    {`{"dsn":"` + testDSN + `"}` + "\n" + `{"type":"event"}` + "\n[1]\n", observability.ErrSentryEnvelopeInvalid},
	}
	for name, tc := range cases {
		_, err := observability.TagSentryEnvelope([]byte(tc.envelope), "7", testReporter)
		assert.ErrorIs(t, err, tc.want, name)
	}
}
