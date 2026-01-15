package auth

const maxEmailErrorLength = 1024

func truncateError(msg string) string {
	if msg == "" {
		return ""
	}
	if len(msg) <= maxEmailErrorLength {
		return msg
	}
	return msg[:maxEmailErrorLength]
}
