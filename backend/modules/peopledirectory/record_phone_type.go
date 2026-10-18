package peopledirectory

func ValidPhoneType(value string) bool {
	switch value {
	case PhoneTypeMobile, PhoneTypeHome, PhoneTypeWork, PhoneTypeOther:
		return true
	}
	return false
}
