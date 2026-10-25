package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedAnnouncementsMarksOperatorAnnouncementSeenByStaff(t *testing.T) {
	t.Parallel()

	var paths []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"id":%d}}`, len(paths))
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false), OperatorAuth: AuthRef{Token: "operator"}, TenantAuth: AuthRef{Token: "staff"}}
	require.NoError(t, (seedAnnouncementsStep{}).Run(t.Context(), rt))
	assert.Equal(t, []string{
		"/operator/announcements", "/operator/announcements", "/operator/announcements",
		"/api/platform/announcements/1/seen",
	}, paths)
}

func TestSeedParentLetterMarksLetterReadByParent(t *testing.T) {
	t.Parallel()

	var paths []string
	var created []map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/parent-announcements/":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			created = append(created, body)
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"id":"%d"}}`, 70+len(created))
		case r.URL.Path == "/parent/auth/login":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"access_token":"parent-token"}}`)
		case strings.HasSuffix(r.URL.Path, "/publish"):
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"published_at":"2026-08-31T00:00:00Z"}}`)
		default:
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		}
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	rt := &Runtime{
		Client: client, Adapter: client.adapter, TenantAuth: AuthRef{Token: "staff"},
		Parents: []ParentCredentials{{Email: "parent@example.test", Password: "Parent1234%"}},
	}
	require.NoError(t, seedParentLetter(rt))
	// Three earlier notices come first, each read by the first parent, so the
	// portal lists read mail below the letter. The attachment upload sits
	// between create and publish on purpose (#2890): a published
	// announcement is immutable, so an upload after the publish call would be
	// refused with 409 and the demo letter would ship without its file.
	// Then the announcement with a scheduled reminder (#3162): created and
	// published, nothing else — the reminder is a field of the announcement,
	// not a further call. The draft is created and never published.
	assert.Equal(t, []string{
		"/parent/auth/login",
		"/api/parent-announcements/", "/api/parent-announcements/71/publish", "/parent/me/news/71/read",
		"/api/parent-announcements/", "/api/parent-announcements/72/publish", "/parent/me/news/72/read",
		"/api/parent-announcements/", "/api/parent-announcements/73/publish", "/parent/me/news/73/read",
		"/api/parent-announcements/", "/api/announcement-attachments/74",
		"/api/parent-announcements/74/publish", "/parent/me/news/74/read",
		"/api/parent-announcements/", "/api/parent-announcements/75/publish",
		"/api/parent-announcements/",
	}, paths)
	require.Len(t, created, 6)
	assert.Equal(t, seedParentLetterTitle, created[3]["title"])
	assert.Equal(t, "letter", created[3]["delivery_mode"])
	assert.Contains(t, created[3]["body"], "Laternenfest")
	assert.NotContains(t, created[3]["body"], "Elternportal", "the letter is read inside the portal")
	assert.Equal(t, "Weihnachtsbasteln: Wer hilft mit?", created[5]["title"])
}

func TestSeedParentLetterSharesTheCalendarDates(t *testing.T) {
	t.Parallel()

	tuesday, err := parseSeedDate("2026-10-20")
	require.NoError(t, err)
	festival := seedLanternFestivalDate(tuesday)
	assert.Equal(t, "2026-11-20", festival.String())
	assert.Contains(t, seedLanternLetterBody(festival), "am Freitag, 20. November feiern wir unser Laternenfest")
	assert.Equal(t, "Wann: Freitag, 20. November, 17:00 bis 19:00 Uhr", seedLanternFlyerLines(festival)[0])
	assert.Equal(t, "2026-10-26", seedParentsEveningDate(tuesday).String())
}

func TestDemoIllustratedPDFHasTheSizeOfAFlyer(t *testing.T) {
	t.Parallel()

	pdf := demoIllustratedPDF("Unser Laternenfest", lanternPicture(), []string{"Treffpunkt: Schulhof"})
	assert.True(t, bytes.HasPrefix(pdf, []byte("%PDF-1.4\n")))
	assert.True(t, bytes.HasSuffix(pdf, []byte("%%EOF\n")))
	assert.Contains(t, string(pdf), "/Subtype /Image")
	assert.Greater(t, len(pdf), 20<<10, "tens of kilobytes, not a few hundred bytes")
	assert.Less(t, len(pdf), 200<<10)
	assert.Equal(t, pdf, demoIllustratedPDF("Unser Laternenfest", lanternPicture(), []string{"Treffpunkt: Schulhof"}),
		"the same input gives the same file")
}

// The Elternsprechtag poll offers 24 appointments on three afternoons: a
// believable choice for a school and still a long poll (#3861).
func TestSeedParentPollCreatesTwentyFourAppointmentOptions(t *testing.T) {
	t.Parallel()

	var poll struct {
		ResponseType string   `json:"response_type"`
		Options      []string `json:"options"`
	}
	var paths []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/parent-announcements/":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&poll))
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":"73"}}`)
		case "/api/parent-announcements/73/publish":
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		default:
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false), TenantAuth: AuthRef{Token: "staff"}}
	require.NoError(t, (seedParentPollStep{}).Run(t.Context(), rt))

	assert.Equal(t, []string{"/api/parent-announcements/", "/api/parent-announcements/73/publish"}, paths)
	assert.Equal(t, "multi_choice", poll.ResponseType)
	require.Len(t, poll.Options, 24)
	assert.Greater(t, len(poll.Options), 10, "from 11 options on the parent portal shows the long-poll layout")
	option := regexp.MustCompile(`^(Dienstag|Mittwoch|Donnerstag), \d{1,2}\. \p{L}+, \d{2}:\d{2} Uhr$`)
	for _, o := range poll.Options {
		assert.Regexp(t, option, o)
	}
	assert.True(t, strings.HasPrefix(poll.Options[0], "Dienstag, "), poll.Options[0])
	assert.True(t, strings.HasSuffix(poll.Options[0], ", 14:00 Uhr"), poll.Options[0])
	distinct := make(map[string]struct{}, len(poll.Options))
	for _, option := range poll.Options {
		distinct[option] = struct{}{}
	}
	assert.Len(t, distinct, len(poll.Options), "each appointment option must be distinct")
}

// The appointments lie in the coming week on every seed day, never in the
// past, and the weekday matches the date.
func TestSeedParentPollOptionsFollowNextWeek(t *testing.T) {
	t.Parallel()

	cases := []struct{ today, first, last string }{
		{"2026-10-20", "Dienstag, 27. Oktober, 14:00 Uhr", "Donnerstag, 29. Oktober, 17:30 Uhr"}, // Tuesday
		{"2026-10-25", "Dienstag, 27. Oktober, 14:00 Uhr", "Donnerstag, 29. Oktober, 17:30 Uhr"}, // Sunday
		{"2026-10-26", "Dienstag, 3. November, 14:00 Uhr", "Donnerstag, 5. November, 17:30 Uhr"}, // Monday
	}
	for _, tc := range cases {
		today, err := parseSeedDate(tc.today)
		require.NoError(t, err)
		options := seedParentPollOptions(today)
		require.Len(t, options, 24, tc.today)
		assert.Equal(t, tc.first, options[0], tc.today)
		assert.True(t, strings.HasSuffix(options[1], ", 14:30 Uhr"), options[1])
		assert.True(t, strings.HasPrefix(options[8], "Mittwoch, "), options[8])
		assert.True(t, strings.HasSuffix(options[8], ", 14:00 Uhr"), options[8])
		assert.Equal(t, tc.last, options[23], tc.today)
	}
}

// The answers spread over every appointment of all three afternoons with a
// believable weighting, one to three adjacent appointments on one afternoon
// each, the same on every run. Before, they only reached the first twelve.
func TestSeedParentPollChoiceSpreadsOverEveryAppointment(t *testing.T) {
	t.Parallel()

	options := make([]seedParentPollOption, 24)
	index := make(map[string]int, len(options))
	for i := range options {
		options[i].ID = fmt.Sprintf("o%d", i)
		index[options[i].ID] = i
	}
	chosen := map[int]int{}
	firstSlots := map[int]int{}
	for answer := range 60 {
		choice := seedParentPollChoice(options, answer)
		require.NotEmpty(t, choice)
		require.LessOrEqual(t, len(choice), 3)
		assert.Equal(t, choice, seedParentPollChoice(options, answer), "deterministic")
		day := index[choice[0]] / seedParentPollSlotsPerDay
		for k, id := range choice {
			assert.Equal(t, index[choice[0]]+k, index[id], "adjacent appointments")
			assert.Equal(t, day, index[id]/seedParentPollSlotsPerDay, "on one afternoon")
			chosen[index[id]]++
		}
		firstSlots[index[choice[0]]%seedParentPollSlotsPerDay]++
	}
	for slot := range options {
		assert.Positive(t, chosen[slot], "appointment %d gets at least one answer", slot)
	}
	assert.Greater(t, firstSlots[4]+firstSlots[5], firstSlots[0]+firstSlots[1], "after work is more popular than 14:00")
	for answer := range 10 {
		short := seedParentPollChoice(options[:3], answer)
		require.NotEmpty(t, short, "fewer options still give an answer")
		for _, id := range short {
			assert.Less(t, index[id], 3)
		}
	}
}

func TestSeedParentDeclarationDeclaresThroughTheParentAPI(t *testing.T) {
	t.Parallel()

	var paths []string
	var declared, read map[string]any
	var created []map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/parent-announcements/":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			created = append(created, body)
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"id":"%d"}}`, 80+len(created))
		case "/parent/auth/login":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"access_token":"parent-token"}}`)
		case "/parent/me/news":
			_, _ = fmt.Fprintf(w, `{"status":"success","data":[{"id":"81","title":%q,"published_at":"2026-10-20T10:00:00Z","declaration":{"version":{"id":"5"},"children":[{"student_id":"9","can_submit":false},{"student_id":"10","can_submit":true}]}}]}`, seedDeclarationTitle)
		case "/parent/me/news/81/declaration":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&declared))
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"created":true}}`)
		case "/parent/me/news/81/read":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&read))
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		default:
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		}
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	rt := &Runtime{
		Client: client, Adapter: client.adapter, TenantAuth: AuthRef{Token: "staff"},
		Parents: []ParentCredentials{{Email: "parent@example.test", Password: "Parent1234%"}},
	}
	require.NoError(t, (seedParentDeclarationStep{}).Run(t.Context(), rt))
	// The document is attached before the first publication: after it the
	// attachments of an Erklärung are fixed. The parent reads the Erklärung
	// after consenting, so the portal does not list it as unread.
	assert.Equal(t, []string{
		"/api/parent-announcements/", "/api/announcement-attachments/81",
		"/api/parent-announcements/81/publish",
		"/api/parent-announcements/", "/api/parent-announcements/82/publish",
		"/parent/auth/login", "/parent/me/news", "/parent/me/news/81/declaration",
		"/parent/me/news/81/read",
	}, paths)
	require.Len(t, created, 2)
	assert.Equal(t, seedDeclarationTitle, created[0]["title"])
	assert.Equal(t, seedSwimDeclarationTitle, created[1]["title"])
	for _, c := range created {
		assert.Equal(t, "declaration", c["delivery_mode"])
		assert.Equal(t, true, c["declaration_requires_signature"])
	}
	assert.Equal(t, map[string]any{"published_at": "2026-10-20T10:00:00Z"}, read)
	signature, ok := declared["signature"].(map[string]any)
	require.True(t, ok, "the seeded consent is signed (#4785)")
	delete(declared, "signature")
	assert.Equal(t, map[string]any{"student_id": "10", "action": "agreed", "version_id": "5"}, declared,
		"the parent declares only for a child they may declare for")
	assert.Equal(t, "drawn", signature["method"])
	assert.NotEmpty(t, signature["image_png"])
}

func TestSeedParentPollAnswersForOtherFamiliesOnly(t *testing.T) {
	t.Parallel()

	// The first parent keeps the poll open: the parent portal screenshot shows
	// the open answer form (#3759). The second parent shares the first
	// parent's child and answers only for the child that is theirs alone.
	pollFeed := func(studentIDs ...string) string {
		children := make([]string, 0, len(studentIDs))
		for _, id := range studentIDs {
			children = append(children, fmt.Sprintf(`{"student_id":%q}`, id))
		}
		return `{"status":"success","data":[{"id":"73","title":"Termine für den Elternsprechtag","published_at":"2026-10-20T10:00:00Z",` +
			`"options":[{"id":"o1"},{"id":"o2"},{"id":"o3"},{"id":"o4"},{"id":"o5"},{"id":"o6"},{"id":"o7"},{"id":"o8"}],` +
			`"children":[` + strings.Join(children, ",") + `]}]}`
	}
	feeds := map[string]string{
		"Bearer token-second": pollFeed("1", "2"),
		"Bearer token-third":  pollFeed("3"),
	}
	tokens := map[string]string{
		"second@example.test": "token-second",
		"third@example.test":  "token-third",
	}
	type answer struct {
		auth string
		body map[string]any
	}
	var answers []answer
	var logins []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/parent-announcements/":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":"73"}}`)
		case "/api/parent-announcements/73/publish":
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		case "/parent/auth/login":
			var login struct {
				Email string `json:"email"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&login))
			logins = append(logins, login.Email)
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"access_token":%q}}`, tokens[login.Email])
		case "/parent/me/news":
			_, _ = fmt.Fprint(w, feeds[r.Header.Get("Authorization")])
		case "/parent/me/news/73/respond":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			answers = append(answers, answer{auth: r.Header.Get("Authorization"), body: body})
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"answered":true}}`)
		default:
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	rt := &Runtime{
		Client: client, Adapter: client.adapter, TenantAuth: AuthRef{Token: "staff"},
		Parents: []ParentCredentials{
			{Email: "first@example.test", Password: "Parent1234%", StudentIDs: []int64{1}},
			{Email: "second@example.test", Password: "Parent1234%"},
			{Email: "third@example.test", Password: "Parent1234%"},
		},
	}
	require.NoError(t, (seedParentPollStep{}).Run(t.Context(), rt))

	assert.Equal(t, []string{"second@example.test", "third@example.test"}, logins,
		"the first parent never signs in to answer")
	require.Len(t, answers, 2, "one answer per child outside the first family")
	assert.Equal(t, "Bearer token-second", answers[0].auth)
	assert.Equal(t, "2", answers[0].body["student_id"])
	assert.Equal(t, "Bearer token-third", answers[1].auth)
	assert.Equal(t, "3", answers[1].body["student_id"])
	for _, a := range answers {
		assert.Equal(t, "2026-10-20T10:00:00Z", a.body["published_at"])
		options, ok := a.body["option_ids"].([]any)
		require.True(t, ok)
		assert.NotEmpty(t, options)
	}
	assert.NotEqual(t, answers[0].body["option_ids"], answers[1].body["option_ids"],
		"families pick different appointments, so the results show a spread")
}

func TestSeedParentPollRefusesWithoutTheFirstFamilysChildren(t *testing.T) {
	t.Parallel()

	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/parent-announcements/":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":"73"}}`)
		case "/api/parent-announcements/73/publish":
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		default:
			t.Errorf("no parent may answer without the first family known: %s", r.URL.Path)
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	rt := &Runtime{
		Client: client, Adapter: client.adapter, TenantAuth: AuthRef{Token: "staff"},
		Parents: []ParentCredentials{
			{Email: "first@example.test", Password: "Parent1234%"},
			{Email: "second@example.test", Password: "Parent1234%"},
		},
	}
	require.ErrorContains(t, (seedParentPollStep{}).Run(t.Context(), rt), "first demo parent has no children")
}
