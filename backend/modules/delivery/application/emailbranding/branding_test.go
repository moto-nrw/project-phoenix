package emailbranding

import "testing"

func TestMotoLogoURL(t *testing.T) {
	t.Parallel()

	if got := MotoLogoURL("https://parents.localhost:3000"); got != "https://parents.localhost:3000/images/moto-logo-mit-schriftzug.png" {
		t.Fatalf("unexpected moto logo url: %q", got)
	}
	if got := MotoLogoURL("https://parents.localhost:3000/"); got != "https://parents.localhost:3000/images/moto-logo-mit-schriftzug.png" {
		t.Fatalf("trailing slash not normalized: %q", got)
	}
	if got := MotoLogoURL(""); got != "/images/moto-logo-mit-schriftzug.png" {
		t.Fatalf("empty base should yield bare path, got %q", got)
	}
}

func TestSchoolLogoURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		baseURL string
		rawURL  string
		want    string
	}{
		{"uploaded image rewritten to public endpoint", "https://parents.localhost:3000", "/uploads/login-images/2_abc.jpg", "https://parents.localhost:3000/api/public/login-image/2_abc.jpg"},
		{"nested upload path rejected", "https://parents.localhost:3000", "/uploads/login-images/nested/f.jpg", ""},
		{"absolute url passes through", "https://parents.localhost:3000", "https://cdn.example/logo.png", "https://cdn.example/logo.png"},
		{"empty raw yields empty", "https://parents.localhost:3000", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SchoolLogoURL(tc.baseURL, tc.rawURL); got != tc.want {
				t.Fatalf("SchoolLogoURL(%q, %q) = %q, want %q", tc.baseURL, tc.rawURL, got, tc.want)
			}
		})
	}
}

func TestAbsoluteURL(t *testing.T) {
	t.Parallel()

	cases := []struct{ base, raw, want string }{
		{"https://x.test", "", ""},
		{"https://x.test", "   ", ""},
		{"https://x.test", "https://cdn/logo.png", "https://cdn/logo.png"},
		{"https://x.test", "http://cdn/logo.png", "http://cdn/logo.png"},
		{"https://x.test", "/logo.png", "https://x.test/logo.png"},
		{"https://x.test", "logo.png", "https://x.test/logo.png"},
		{"https://x.test/", "/logo.png", "https://x.test/logo.png"},
		{"", "/logo.png", "/logo.png"},
		{"  https://x.test  ", "/logo.png", "https://x.test/logo.png"},
	}
	for _, tc := range cases {
		if got := AbsoluteURL(tc.base, tc.raw); got != tc.want {
			t.Fatalf("AbsoluteURL(%q, %q) = %q, want %q", tc.base, tc.raw, got, tc.want)
		}
	}
}
