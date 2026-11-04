package openholidays

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const schoolHolidaysBody = `[
 {"id":"a","startDate":"2026-12-23","endDate":"2027-01-06","type":"School","name":[{"language":"DE","text":"Weihnachtsferien"}],"nationwide":false,"subdivisions":[{"code":"DE-NW","shortName":"NW"}]},
 {"id":"b","startDate":"2026-12-21","endDate":"2027-01-02","type":"School","name":[{"language":"EN","text":"Christmas"},{"language":"DE","text":"Weihnachtsferien"}],"nationwide":false,"subdivisions":[{"code":"DE-BY","shortName":"BY"}]},
 {"id":"c","startDate":"2027-05-18","endDate":"2027-05-18","type":"School","name":[{"language":"DE","text":"Bundesweiter Ferientag"}],"nationwide":true,"subdivisions":[]},
 {"id":"d","startDate":"2027-05-01","endDate":"2027-05-01","type":"Public","name":[{"language":"DE","text":"Tag der Arbeit"}],"nationwide":true}
]`

func TestSchoolVacations_ReadsEveryStateFromOneRequest(t *testing.T) {
	t.Parallel()
	var query map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/SchoolHolidays", r.URL.Path)
		query = map[string]string{}
		for key := range r.URL.Query() {
			query[key] = r.URL.Query().Get(key)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(schoolHolidaysBody))
	}))
	t.Cleanup(server.Close)

	vacations, err := New(server.Client(), server.URL, []string{"DE-BY", "DE-NW"}).SchoolVacations(context.Background(), "2026-08-01", "2028-10-31")
	require.NoError(t, err)

	assert.Equal(t, map[string]string{
		"countryIsoCode": "DE", "languageIsoCode": "DE", "validFrom": "2026-08-01", "validTo": "2028-10-31",
	}, query, "one request for every state, no subdivision filter")
	assert.Equal(t, []Vacation{
		{Region: "DE-NW", Name: "Weihnachtsferien", StartDate: "2026-12-23", EndDate: "2027-01-06"},
		{Region: "DE-BY", Name: "Weihnachtsferien", StartDate: "2026-12-21", EndDate: "2027-01-02"},
		{Region: "DE-BY", Name: "Bundesweiter Ferientag", StartDate: "2027-05-18", EndDate: "2027-05-18"},
		{Region: "DE-NW", Name: "Bundesweiter Ferientag", StartDate: "2027-05-18", EndDate: "2027-05-18"},
	}, vacations, "the German name, nationwide entries once per state, public holidays left out")
}

func TestSchoolVacations_FailsOnABadAnswer(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		status int
		body   string
	}{
		"server error":   {status: http.StatusInternalServerError, body: `{"title":"error"}`},
		"range too long": {status: http.StatusBadRequest, body: `{"detail":"The maximum date range is 1095 days."}`},
		"not json":       {status: http.StatusOK, body: `<html>`},
		"broken range":   {status: http.StatusOK, body: `[{"startDate":"2027-01-06","endDate":"2026-12-23","type":"School","name":[{"language":"DE","text":"X"}],"subdivisions":[{"code":"DE-NW"}]}]`},
		"no name":        {status: http.StatusOK, body: `[{"startDate":"2026-12-23","endDate":"2027-01-06","type":"School","name":[],"subdivisions":[{"code":"DE-NW"}]}]`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(server.Close)

			vacations, err := New(server.Client(), server.URL, []string{"DE-NW"}).SchoolVacations(context.Background(), "2026-08-01", "2027-07-31")
			require.Error(t, err)
			assert.Nil(t, vacations, "a failed read returns nothing to store")
		})
	}
}

func TestNew_DefaultsToThePublicAPI(t *testing.T) {
	t.Parallel()
	client := New(nil, "", nil)
	assert.Equal(t, BaseURL, client.baseURL)
	assert.Equal(t, requestTimeout, client.httpClient.Timeout)
}
