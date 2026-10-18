package presenceprojectionhttp

func derefOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
