package middleware

import (
	"bytes"
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	slogchi "github.com/samber/slog-chi"
)

func TestRedactFeedToken(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"/public/calendar/secret-token":                     "/public/calendar/[REDACTED]",
		"/public/request-feed/rss-secret":                   "/public/request-feed/[REDACTED]",
		"/public/calendar/tok123?refresh=1":                 "/public/calendar/[REDACTED]?refresh=1",
		"GET /public/calendar/abc 200 12ms":                 "GET /public/calendar/[REDACTED] 200 12ms",
		"/public/calendar/one /public/request-feed/two":     "/public/calendar/[REDACTED] /public/request-feed/[REDACTED]",
		"/public/request-feed/one /public/request-feed/two": "/public/request-feed/[REDACTED] /public/request-feed/[REDACTED]",
		"/public/calendar/":                                 "/public/calendar/", // no token segment
		"/api/calendar/appointments/5/ics":                  "/api/calendar/appointments/5/ics",
		"nothing sensitive here":                            "nothing sensitive here",
		"POST /caterer/invitations/42/secret/accept":        "POST /caterer/invitations/42/[REDACTED]/accept",
		"/caterer/einladung/42/secret":                      "/caterer/einladung/42/[REDACTED]",
		"/caterer/invitations/42":                           "/caterer/invitations/42",
		"PUT /api/enrollment/requests/st4tus-token 200":     "PUT /api/enrollment/requests/[REDACTED] 200",
		"/auth/invitations/inv1te-token/accept":             "/auth/invitations/[REDACTED]/accept",
		"/auth/invitations/42/resend":                       "/auth/invitations/42/resend",
		"/auth/invitations/former-employment":               "/auth/invitations/former-employment",
		"/auth/invitations/":                                "/auth/invitations/",
		"/auth/guardian-invitations/gu4rdian?x=1":           "/auth/guardian-invitations/[REDACTED]?x=1",
		"/traeger/holiday-registrations/f0rm/documents/7":   "/traeger/holiday-registrations/[REDACTED]/documents/7",
		"/public/payroll-download/l1nk":                     "/public/payroll-download/[REDACTED]",
	}
	for in, want := range cases {
		if got := RedactFeedToken(in); got != want {
			t.Errorf("RedactFeedToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSigningTokenRedaction(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"/public/care-contract-signing/42.signing-credential/sign",
		"/api/public/care-contract-signing/42.signing-credential/document",
		"https://parents.example/vertrag-unterschreiben/42.signing-credential",
		"/parents/vertrag-unterschreiben/42.signing-credential?from=mail",
		"/public/care%2Dcontract-signing/42.signing-credential",
		"/%76ertrag-unterschreiben/42.signing-credential",
	} {
		got := RedactFeedToken(path)
		if strings.Contains(got, "signing-credential") || !strings.Contains(got, "[REDACTED]") {
			t.Errorf("signing credential survived: %s", got)
		}
	}
	if got := RedactFeedToken("/api/public/care-contract-signing/42.secret/document?view=1"); got != "/api/public/care-contract-signing/[REDACTED]/document?view=1" {
		t.Errorf("lost route diagnostics: %s", got)
	}
}

func TestSigningCredentialsStayOutOfAccessLogs(t *testing.T) {
	t.Parallel()
	for _, suffix := range []string{"", "/document", "/sign"} {
		var out bytes.Buffer
		logger := slog.New(NewFeedTokenRedactor(slog.NewJSONHandler(&out, nil)))
		router := chi.NewRouter()
		router.Use(slogchi.NewWithConfig(logger, slogchi.Config{WithRequestHeader: true}))
		router.HandleFunc("/public/care-contract-signing/{link}"+suffix, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
		method := http.MethodGet
		if suffix == "/sign" {
			method = http.MethodPost
		}
		request := httptest.NewRequest(method, "/public/care-contract-signing/42.access-credential"+suffix, nil)
		request.Header.Set("Referer", "https://parents.example/vertrag-unterschreiben/42.access-credential")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("route no longer works: %d", response.Code)
		}
		if strings.Contains(out.String(), "access-credential") {
			t.Errorf("signing credential leaked: %s", out.String())
		}
		if !strings.Contains(out.String(), "[REDACTED]") {
			t.Errorf("missing access log: %s", out.String())
		}
	}
}

func TestFeedTokenRedactorHandler(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{})
	logger := slog.New(NewFeedTokenRedactor(base))

	// A grouped "request" attribute (as the request logger emits) must be redacted.
	logger.Info("request",
		slog.Group("request",
			slog.String("method", "GET"),
			slog.String("path", "/public/calendar/leaky-token"),
			slog.Any("params", map[string]string{"token": "leaky-param-token"}),
		),
	)
	// A top-level path attribute (as the security logger emits) must be redacted.
	logger.Info("security event", slog.String("path", "/public/calendar/another-token"))

	out := buf.String()
	if strings.Contains(out, "leaky-token") || strings.Contains(out, "leaky-param-token") || strings.Contains(out, "another-token") {
		t.Errorf("feed token leaked into logs:\n%s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Errorf("expected [REDACTED] in logs:\n%s", out)
	}
	// A non-feed path is untouched.
	buf.Reset()
	logger.Info("other", slog.String("path", "/api/staff"))
	if !strings.Contains(buf.String(), "/api/staff") {
		t.Errorf("non-feed path should be preserved:\n%s", buf.String())
	}
}

// The security logger derived from NewSecurityLogger must redact feed tokens.
func TestSecurityLoggerRedactsFeedToken(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	sl := &SecurityLogger{logger: slog.New(NewFeedTokenRedactor(slog.NewTextHandler(&buf, nil)))}

	req := httptest.NewRequest("GET", "/public/calendar/rate-limited-token", nil)
	sl.LogRateLimitExceeded(req)

	out := buf.String()
	if strings.Contains(out, "rate-limited-token") {
		t.Errorf("security logger leaked feed token:\n%s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Errorf("expected redacted path in security log:\n%s", out)
	}
}

func TestRedactQueryValues(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"search=Mustermann":               "search",
		"search=Mustermann&page=2":        "search&page",
		"first_name=Max&last_name=Muster": "first_name&last_name",
		"email=max%40example.com":         "email",
		"flag":                            "flag", // valueless parameter stays
		"":                                "",
		"a=1&b&c=x=y":                     "a&b&c", // value containing '=' is dropped entirely
	}
	for in, want := range cases {
		if got := RedactQueryValues(in); got != want {
			t.Errorf("RedactQueryValues(%q) = %q, want %q", in, got, want)
		}
	}
}

// Issue #2105: the access-log "query" attribute (nested in slog-chi's
// "request" group) and the "referer" attribute must lose their query values
// while other attributes stay intact.
func TestQueryValueRedactorHandler(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(NewQueryValueRedactor(slog.NewTextHandler(&buf, nil)))

	logger.Info("request",
		slog.Group("request",
			slog.String("method", "GET"),
			slog.String("path", "/api/students"),
			slog.String("query", "search=Mustermann&page=2"),
			slog.String("referer", "https://schule.example.com/students/search?search=Mustermann"),
		),
	)

	out := buf.String()
	if strings.Contains(out, "Mustermann") {
		t.Errorf("query value leaked into logs:\n%s", out)
	}
	for _, want := range []string{"query=search&page", "/api/students", "method=GET", "referer=https://schule.example.com/students/search?search"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in logs:\n%s", want, out)
		}
	}
	// Non-query attributes containing '=' or '&' are untouched.
	buf.Reset()
	logger.Info("other", slog.String("detail", "a=1&b=2"))
	if !strings.Contains(buf.String(), "a=1&b=2") {
		t.Errorf("non-query attribute should be preserved:\n%s", buf.String())
	}
}

var _ slog.Handler = (*attrRedactor)(nil)

func TestFeedTokenRedactorEnabled(t *testing.T) {
	t.Parallel()

	h := NewFeedTokenRedactor(slog.NewTextHandler(&bytes.Buffer{}, nil))
	if !h.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected Info to be enabled")
	}
}

// credentialRouteParam matches the chi parameters whose value is the only
// credential of its route.
var credentialRouteParam = regexp.MustCompile(`(?i)\{[a-z]*token[a-z]*\}|\{link\}`)

var anyRouteParam = regexp.MustCompile(`\{[^}]+\}`)

// TestRouteCredentialsStayOutOfLogs walks the production route table. A route
// that carries its credential in the path must lose it in the access log and
// in Sentry events; every other route must stay readable, IDs included.
func TestRouteCredentialsStayOutOfLogs(t *testing.T) {
	t.Parallel()
	const secret = "s3cret-Credential_value"
	root := filepath.Join("..", "api", "testdata", "routes")
	credentialRoutes := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".golden" {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
			_, pattern, found := strings.Cut(line, " ")
			if !found {
				t.Errorf("%s: unexpected route line %q", path, line)
				continue
			}
			carriesCredential := credentialRouteParam.MatchString(pattern)
			request := credentialRouteParam.ReplaceAllString(pattern, secret)
			request = anyRouteParam.ReplaceAllString(request, "17")
			redacted := RedactFeedToken(request)
			switch {
			case carriesCredential:
				credentialRoutes++
				if strings.Contains(redacted, secret) {
					t.Errorf("%s leaves its credential in logs: %s", line, redacted)
				}
			case redacted != request:
				t.Errorf("%s has no credential but is logged as %s", line, redacted)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk route goldens: %v", err)
	}
	if credentialRoutes == 0 {
		t.Fatal("no route with a credential parameter found; the route goldens moved")
	}
}

func TestHolidayParentSetupCredentialsStayOutOfLogs(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		var out bytes.Buffer
		logger := slog.New(NewFeedTokenRedactor(slog.NewJSONHandler(&out, nil)))
		router := chi.NewRouter()
		router.Use(slogchi.NewWithConfig(logger, slogchi.Config{WithRequestHeader: true}))
		router.HandleFunc("/auth/holiday-parent-setups/{token}", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
		request := httptest.NewRequest(method, "/auth/holiday-parent-setups/setup-credential", nil)
		request.Header.Set("Referer", "https://parents.example/ferienkonto/setup-credential")
		router.ServeHTTP(httptest.NewRecorder(), request)
		if strings.Contains(out.String(), "setup-credential") || !strings.Contains(out.String(), "[REDACTED]") {
			t.Errorf("setup credential not redacted: %s", out.String())
		}
	}
	for _, path := range []string{
		"/auth/holiday%2Dparent-setups/setup-credential",
		"/parents/%66erienkonto/setup-credential?from=mail",
	} {
		if got := RedactFeedToken(path); strings.Contains(got, "setup-credential") {
			t.Errorf("encoded setup credential survived: %s", got)
		}
	}
}
