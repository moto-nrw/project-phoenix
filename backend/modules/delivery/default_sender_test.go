package delivery

import "testing"

func TestSenderOrDefaultFallsBackWithoutAnAddress(t *testing.T) {
	t.Parallel()
	if name, address := SenderOrDefault("Schule", "info@schule.test"); name != "Schule" || address != "info@schule.test" {
		t.Errorf("configured sender = %q <%s>", name, address)
	}
	if name, address := SenderOrDefault("Schule", ""); name != "moto" || address != "no-reply@moto.local" {
		t.Errorf("default sender = %q <%s>", name, address)
	}
}
