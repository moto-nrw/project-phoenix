package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// familyAccounts builds n family app accounts; account i has child 100+i.
func familyAccounts(n int) []ParentCredentials {
	accounts := make([]ParentCredentials, n)
	for i := range accounts {
		accounts[i] = ParentCredentials{
			Email: fmt.Sprintf("family%d@example.test", i), Password: "Parent1234%",
			Name: fmt.Sprintf("Familie %d", i), StudentIDs: []int64{int64(100 + i)},
		}
	}
	return accounts
}

// familyFeedServer answers the parent logins with a token per e-mail and
// each account's feed with the given announcements for that account's child
// (child 1 and 2 for second@example.test). It records every write.
type familyFeedServer struct {
	mu     sync.Mutex
	logins []string
	writes []familyWrite
}

type familyWrite struct {
	email, path string
	body        map[string]any
}

func (f *familyFeedServer) handler(t *testing.T, feed func(children []string) string) func(seedHTTPResponseWriter, *seedHTTPRequest) {
	return func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		email := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer token-")
		switch {
		case r.URL.Path == "/parent/auth/login":
			var login struct {
				Email string `json:"email"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&login))
			f.logins = append(f.logins, login.Email)
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"access_token":"token-%s"}}`, login.Email)
		case r.URL.Path == "/parent/me/news":
			children := []string{"1"}
			switch {
			case email == "second@example.test":
				children = []string{"1", "2"}
			case strings.HasPrefix(email, "family"):
				var i int
				_, err := fmt.Sscanf(email, "family%d@", &i)
				require.NoError(t, err)
				children = []string{fmt.Sprint(100 + i)}
			}
			_, _ = fmt.Fprint(w, feed(children))
		case strings.HasPrefix(r.URL.Path, "/parent/me/news/"):
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			f.writes = append(f.writes, familyWrite{email: email, path: r.URL.Path, body: body})
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		case strings.HasSuffix(r.URL.Path, "/publish"), r.URL.Path == "/api/parent-announcements/",
			strings.HasPrefix(r.URL.Path, "/api/announcement-attachments/"):
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":"81"}}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	}
}

func familyRuntime(srvURL string, accounts int) *Runtime {
	client := newTestClient(srvURL, false)
	return &Runtime{
		Client: client, Adapter: client.adapter, TenantAuth: AuthRef{Token: "staff"},
		Parents: []ParentCredentials{
			{Email: "first@example.test", Password: "Parent1234%", Name: "Sabine Schneider", StudentIDs: []int64{1}},
			{Email: "second@example.test", Password: "Parent1234%", Name: "Klaus Schneider", StudentIDs: []int64{1}},
		},
		FamilyAccounts: familyAccounts(accounts),
	}
}

// Three of four family app accounts answer the poll as well, so the results
// fill up; the first family's child stays open.
func TestSeedParentPollFamilyAccountsAnswerThreeOfFour(t *testing.T) {
	t.Parallel()

	fake := &familyFeedServer{}
	srv := newSeedHTTPTestServer(fake.handler(t, func(children []string) string {
		items := make([]string, 0, len(children))
		for _, id := range children {
			items = append(items, fmt.Sprintf(`{"student_id":%q,"selected_options":[]}`, id))
		}
		return `{"status":"success","data":[{"id":"73","title":"Termine für den Elternsprechtag","published_at":"2026-10-20T10:00:00Z",` +
			`"options":[{"id":"o1"},{"id":"o2"},{"id":"o3"}],"children":[` + strings.Join(items, ",") + `]}]}`
	}))
	defer srv.Close()

	rt := familyRuntime(srv.URL, 8)
	require.NoError(t, (seedParentPollStep{}).Run(t.Context(), rt))

	assert.NotContains(t, fake.logins, "first@example.test")
	assert.NotContains(t, fake.logins, "family3@example.test", "every fourth family leaves the poll unanswered")
	assert.NotContains(t, fake.logins, "family7@example.test")
	answered := map[any]bool{}
	for _, write := range fake.writes {
		assert.Equal(t, "/parent/me/news/73/respond", write.path)
		answered[write.body["student_id"]] = true
	}
	assert.False(t, answered["1"], "the first family's child stays open")
	assert.True(t, answered["2"])
	for _, i := range []int{0, 1, 2, 4, 5, 6} {
		assert.True(t, answered[fmt.Sprint(100+i)], "family %d answers", i)
	}
	assert.Len(t, fake.writes, 7)
}

// Most families agree to the zoo outing with a drawn signature, a few
// decline, one revokes; the first family's child stays with the first
// parent's own answer. The swimming course gets fewer answers.
func TestSeedDeclarationAnswersSpreadOverFamilies(t *testing.T) {
	t.Parallel()

	fake := &familyFeedServer{}
	srv := newSeedHTTPTestServer(fake.handler(t, func(children []string) string {
		items := make([]string, 0, len(children))
		for _, id := range children {
			items = append(items, fmt.Sprintf(`{"student_id":%q,"can_submit":true}`, id))
		}
		declaration := `"declaration":{"version":{"id":"5"},"children":[` + strings.Join(items, ",") + `]}`
		return fmt.Sprintf(`{"status":"success","data":[{"id":"81","title":%q,"published_at":"2026-10-20T10:00:00Z",%s},`+
			`{"id":"82","title":%q,"published_at":"2026-10-20T10:00:00Z",%s}]}`,
			seedDeclarationTitle, declaration, seedSwimDeclarationTitle, declaration)
	}))
	defer srv.Close()

	rt := familyRuntime(srv.URL, 20)
	require.NoError(t, seedParentDeclaration(rt))

	actions := map[string]map[string][]string{} // path -> student -> actions
	for _, write := range fake.writes {
		if !strings.HasSuffix(write.path, "/declaration") {
			continue
		}
		if actions[write.path] == nil {
			actions[write.path] = map[string][]string{}
		}
		student := write.body["student_id"].(string)
		action := write.body["action"].(string)
		actions[write.path][student] = append(actions[write.path][student], action)
		signature, signed := write.body["signature"].(map[string]any)
		assert.Equal(t, action == "agreed", signed, "only an agreement carries a signature")
		if signed {
			assert.Equal(t, "drawn", signature["method"])
		}
	}
	zoo, swim := actions["/parent/me/news/81/declaration"], actions["/parent/me/news/82/declaration"]
	assert.Equal(t, []string{"agreed"}, zoo["1"], "only the first parent answers for the first family's child")
	assert.Empty(t, swim["1"], "the first family leaves the swimming course open")

	counts := map[string]int{}
	for _, list := range zoo {
		counts[strings.Join(list, ",")]++
	}
	assert.Equal(t, 1, counts["agreed,revoked"], "one family revokes")
	assert.GreaterOrEqual(t, counts["declined"], 1, "a few families decline")
	assert.Greater(t, counts["agreed"], counts["declined"]*5, "most families agree")
	assert.Less(t, len(swim), len(zoo), "the newer Erklärung is still filling up")
	assert.NotEmpty(t, swim)
}

func TestRespondingFamiliesLeaveOutTheFirstParentAndEveryFourthFamily(t *testing.T) {
	t.Parallel()

	rt := &Runtime{
		Parents:        []ParentCredentials{{Email: "first"}, {Email: "second"}},
		FamilyAccounts: familyAccounts(5),
	}
	emails := []string{}
	for _, parent := range respondingFamilies(rt) {
		emails = append(emails, parent.Email)
	}
	assert.Equal(t, []string{"second", "family0@example.test", "family1@example.test", "family2@example.test", "family4@example.test"}, emails)
	assert.Empty(t, respondingFamilies(&Runtime{Parents: []ParentCredentials{{Email: "first"}}}))
}

// The signature is a real drawing within the limits the server checks:
// a PNG of at most 1600x800 pixels and 200 KiB with visible ink.
func TestDemoSignaturePNGIsAValidDeterministicDrawing(t *testing.T) {
	t.Parallel()

	encoded := demoSignaturePNG("Sabine Schneider")
	assert.Equal(t, encoded, demoSignaturePNG("Sabine Schneider"), "the same name gives the same image")
	assert.NotEqual(t, encoded, demoSignaturePNG("Klaus-Dieter Menzel"), "names give different signatures")
	raw, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(raw), 200<<10)
	img, err := png.Decode(bytes.NewReader(raw))
	require.NoError(t, err)
	assert.LessOrEqual(t, img.Bounds().Dx(), 1600)
	assert.LessOrEqual(t, img.Bounds().Dy(), 800)
	inked, clipped := 0, false
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a > 0x1000 {
				inked++
				clipped = clipped || x == 0 || y == 0 || x == img.Bounds().Max.X-1 || y == img.Bounds().Max.Y-1
			}
		}
	}
	assert.Greater(t, inked, 2000, "a signature, not a dot")
	assert.False(t, clipped, "the drawing stays inside the canvas")
}

func TestRelatedAccountInviteNamesTheGrandmother(t *testing.T) {
	t.Parallel()

	assert.Equal(t, map[string]any{
		"email": "renate.franke@mail.example", "first_name": "Renate", "last_name": "Franke",
	}, relatedAccountInvite(ParentCredentials{Name: "Elfriede Franke"}))
	assert.Equal(t, "renate.krueger@mail.example", relatedAccountInvite(ParentCredentials{Name: "Martin Krüger"})["email"])
}

func TestFamilyAccountLeavesOutLimitedRoles(t *testing.T) {
	t.Parallel()

	rt := &Runtime{FixedSeeder: &FixedSeeder{studentIDByIndex: map[int]int64{}}}
	for index := range DemoStudents {
		rt.FixedSeeder.studentIDByIndex[index] = int64(500 + index)
	}
	account, ok := familyAccount(rt, 2, 1002, 9002, "Parent1234%")
	require.True(t, ok)
	assert.Equal(t, ParentCredentials{
		Email: DemoGuardians[2].Email, Password: "Parent1234%", Name: "Petra Meyer",
		AccountID: 9002, GuardianID: 1002, StudentIDs: []int64{501},
	}, account)
	require.Equal(t, "Katja Albrecht", demoGuardianKey(DemoGuardians[52]))
	_, ok = familyAccount(rt, 52, 1052, 9052, "Parent1234%")
	assert.False(t, ok, "Katja Albrecht is pickup only")
	require.Equal(t, "Milly Nikoliin", demoGuardianKey(DemoGuardians[42]))
	_, ok = familyAccount(rt, 42, 1042, 9042, "Parent1234%")
	assert.False(t, ok, "Milly Nikoliin has a custom role")
}

func TestFridayDepartureChangeChangesOnlyFriday(t *testing.T) {
	t.Parallel()

	pickup := map[string][]string{"mon": {"pickup"}, "tue": {"bus", "alone"}, "wed": {"pickup"}, "thu": {"pickup"}, "fri": {"pickup"}}
	assert.Equal(t, map[string][]string{
		"mon": {"pickup"}, "tue": {"bus", "alone"}, "wed": {"pickup"}, "thu": {"pickup"}, "fri": {"alone"},
	}, fridayDepartureChange(pickup))
	assert.Equal(t, []string{"pickup"}, pickup["fri"], "the current modes stay untouched")
	assert.Equal(t, map[string][]string{"fri": {"pickup"}}, fridayDepartureChange(nil),
		"without modes the child walks alone, so the change is a Friday pickup")
	assert.Equal(t, map[string][]string{"mon": {"pickup"}, "fri": {"pickup"}}, fridayDepartureChange(map[string][]string{"mon": {"pickup"}, "fri": {"alone"}}))
}

// Two families ask to pick up their child earlier today, which makes the
// requests urgent; on a weekend there is no care day.
func TestSeedUrgentPickupChangesOnlyOnACareDay(t *testing.T) {
	t.Parallel()

	var bodies []map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/parent/auth/login" {
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"access_token":"parent-token"}}`)
			return
		}
		require.True(t, strings.HasSuffix(r.URL.Path, "/care-exception"), r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		bodies = append(bodies, body)
		_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":"1"}}`)
	})
	defer srv.Close()

	rt := familyRuntime(srv.URL, 25)
	filed, err := seedUrgentPickupChangesOn(t.Context(), rt, mustParseSeedDate(t, "2026-10-24")) // Saturday
	require.NoError(t, err)
	assert.Zero(t, filed)
	filed, err = seedUrgentPickupChangesOn(t.Context(), rt, mustParseSeedDate(t, "2026-10-20")) // Tuesday
	require.NoError(t, err)
	assert.Equal(t, 2, filed)
	require.Len(t, bodies, 2)
	for _, body := range bodies {
		assert.Equal(t, "2026-10-20", body["date"])
		assert.NotEmpty(t, body["pickup_time"])
		assert.NotEmpty(t, body["reason"])
	}
}

func TestSeedFamilyMealParticipationRegistersAboutTwentyChildren(t *testing.T) {
	t.Parallel()

	var registered []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/parent/auth/login" {
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"access_token":"parent-token"}}`)
			return
		}
		require.True(t, strings.HasSuffix(r.URL.Path, "/meal-participation"), r.URL.Path)
		registered = append(registered, r.URL.Path)
		_, _ = fmt.Fprint(w, `{"status":"success","data":{"effective_from":"2026-10-21"}}`)
	})
	defer srv.Close()

	rt := familyRuntime(srv.URL, 75)
	count, err := seedFamilyMealParticipation(t.Context(), rt)
	require.NoError(t, err)
	assert.Equal(t, 20, count)
	assert.Len(t, registered, 20)
	assert.Equal(t, "/parent/me/children/100/meal-participation", registered[0])
}

func mustParseSeedDate(t *testing.T, value string) seedDate {
	t.Helper()
	date, err := parseSeedDate(value)
	require.NoError(t, err)
	return date
}
