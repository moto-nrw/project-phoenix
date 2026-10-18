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

// SchoolSender is the From identity of a school's mail: the school's name on
// the default address, or the default sender when the mail names no school.
func SchoolSender(schoolName, defaultName, defaultAddress string) (string, string) {
	if schoolName == "" {
		return defaultName, defaultAddress
	}
	return schoolName, defaultAddress
}
