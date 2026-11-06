package api

// fakeTraegerDocumentData answers the routes of the shared document step
// (#4692) for the seeder fakes: one school with one group, no documents yet,
// and an ID for every upload.
func fakeTraegerDocumentData(method, path string) (any, bool) {
	switch {
	case method == "GET" && path == "/traeger/documents/audiences":
		return []map[string]any{{"id": 1, "name": "OGS", "groups": []map[string]any{{"id": 5, "name": "Sonnenblumen"}}}}, true
	case method == "GET" && path == "/traeger/documents/":
		return []any{}, true
	case method == "POST" && path == "/traeger/documents/":
		return map[string]any{"id": 1}, true
	}
	return nil, false
}

// isFakeTraegerDocumentRoute reports whether fakeTraegerDocumentData answers
// the route.
func isFakeTraegerDocumentRoute(method, path string) bool {
	_, ok := fakeTraegerDocumentData(method, path)
	return ok
}
