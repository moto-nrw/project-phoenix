package localization

import "testing"

// These functions are the contract everything else trusts: IsSupported gates
// what the parent profile API will persist, and NormalizeLocale is what coerces
// stored/incoming values before they reach a message catalog. de is the embedded
// fallback (locales.json).

func TestDefaultLocale(t *testing.T) {
	t.Parallel()

	if got := DefaultLocale(); got != "de" {
		t.Fatalf("DefaultLocale() = %q, want %q (fallback in locales.json)", got, "de")
	}
}

func TestIsSupported(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"registered de", "de", true},
		{"registered en", "en", true},
		{"registered ru", "ru", true},
		{"registered sq", "sq", true},
		{"region subtag stripped", "en-US", true},
		{"underscore subtag stripped", "de_DE", true},
		{"uppercase normalized", "EN", true},
		{"surrounding whitespace trimmed", "  en  ", true},
		{"unknown locale", "xx", false},
		{"unknown with region", "xx-YY", false},
		{"empty string", "", false},
		{"whitespace only", "   ", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsSupported(tc.raw); got != tc.want {
				t.Fatalf("IsSupported(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestNormalizeLocale(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"registered passes through", "en", "en"},
		{"region subtag stripped", "en-US", "en"},
		{"underscore subtag stripped", "ru_RU", "ru"},
		{"uppercase normalized", "SQ", "sq"},
		{"surrounding whitespace trimmed", "  de  ", "de"},
		// Unlike IsSupported, NormalizeLocale must NEVER reject — an unknown or
		// empty value coerces to the default so a catalog always resolves.
		{"unknown coerces to default", "xx", "de"},
		{"unknown with region coerces to default", "xx-YY", "de"},
		{"empty coerces to default", "", "de"},
		{"whitespace only coerces to default", "   ", "de"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeLocale(tc.raw); got != tc.want {
				t.Fatalf("NormalizeLocale(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// IsSupported and NormalizeLocale must agree on which raw values are real
// locales: anything IsSupported accepts must normalize to itself, and anything
// it rejects must fall back to the default rather than pass through.
func TestIsSupportedAndNormalizeAgree(t *testing.T) {
	t.Parallel()

	raws := []string{"de", "en", "ru", "sq", "en-US", "DE", "xx", "", "  "}
	for _, raw := range raws {
		if IsSupported(raw) {
			if got := NormalizeLocale(raw); !IsSupported(got) {
				t.Fatalf("IsSupported(%q) is true but NormalizeLocale(%q) = %q is not supported", raw, raw, got)
			}
		} else if got := NormalizeLocale(raw); got != DefaultLocale() {
			t.Fatalf("IsSupported(%q) is false but NormalizeLocale(%q) = %q, want default %q", raw, raw, got, DefaultLocale())
		}
	}
}

// --- loader invariants -----------------------------------------------------
//
// indexLocales and findFallbackLocale enforce the contract the embedded
// locales.json must satisfy. They panic on a malformed list, so a future edit
// that breaks the contract fails loudly at startup instead of silently
// shipping a broken locale table.

func TestIndexLocales_PanicsOnDuplicateCode(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("indexLocales must panic on a duplicate locale code")
		}
	}()
	indexLocales([]Locale{
		{Code: "de", Label: "Deutsch"},
		{Code: "DE", Label: "Duplicate"}, // same code after lowercasing
	})
}

func TestIndexLocales_PanicsOnEmptyCode(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("indexLocales must panic on an empty locale code")
		}
	}()
	indexLocales([]Locale{{Code: "  ", Label: "Blank"}})
}

func TestIndexLocales_LowercasesCode(t *testing.T) {
	t.Parallel()

	out := indexLocales([]Locale{{Code: "EN", Label: "English"}})
	if _, ok := out["en"]; !ok {
		t.Fatalf("indexLocales must key entries by lowercased code, got keys %v", out)
	}
}

func TestFindFallbackLocale_PrefersExplicitFallback(t *testing.T) {
	t.Parallel()

	got := findFallbackLocale([]Locale{
		{Code: "en", Label: "English"},
		{Code: "de", Label: "Deutsch", Fallback: true},
	})
	if got.Code != "de" {
		t.Fatalf("findFallbackLocale = %q, want the entry flagged Fallback (de)", got.Code)
	}
}

func TestFindFallbackLocale_DefaultsToFirstWhenNoneFlagged(t *testing.T) {
	t.Parallel()

	got := findFallbackLocale([]Locale{
		{Code: "en", Label: "English"},
		{Code: "ru", Label: "Русский"},
	})
	if got.Code != "en" {
		t.Fatalf("findFallbackLocale = %q, want the first entry when none is flagged", got.Code)
	}
}
