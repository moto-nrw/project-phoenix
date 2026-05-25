package enrollment

import "testing"

func TestSchoolEmailLogoURLUsesPublicLoginImageProxy(t *testing.T) {
	t.Parallel()

	got := schoolEmailLogoURL("http://parents.localhost:3000", "/uploads/login-images/2_V3EjlEtM.jpg")
	want := "http://parents.localhost:3000/api/public/login-image/2_V3EjlEtM.jpg"

	if got != want {
		t.Fatalf("schoolEmailLogoURL() = %q, want %q", got, want)
	}
}

func TestSchoolEmailLogoURLRejectsNestedUploadPath(t *testing.T) {
	t.Parallel()

	got := schoolEmailLogoURL("http://parents.localhost:3000", "/uploads/login-images/nested/file.jpg")

	if got != "" {
		t.Fatalf("schoolEmailLogoURL() = %q, want empty string", got)
	}
}

func TestSchoolEmailLogoURLKeepsAbsoluteURL(t *testing.T) {
	t.Parallel()

	got := schoolEmailLogoURL("http://parents.localhost:3000", "https://cdn.example.test/logo.png")
	want := "https://cdn.example.test/logo.png"

	if got != want {
		t.Fatalf("schoolEmailLogoURL() = %q, want %q", got, want)
	}
}
