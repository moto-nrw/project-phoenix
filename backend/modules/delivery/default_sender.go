package delivery

// SenderOrDefault is the From identity of the platform's mails: the
// configured sender, or "moto" <no-reply@moto.local> when no address is
// configured.
func SenderOrDefault(name, address string) (string, string) {
	if address == "" {
		return "moto", "no-reply@moto.local"
	}
	return name, address
}
