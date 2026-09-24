package timetableplanning

// successorIncludesClosingDays resolves the closing-day opt-in a series split
// carries to its successor (#3594): an explicit request value wins, an
// omitted one inherits the source series' flag.
func successorIncludesClosingDays(requested *bool, inherited bool) bool {
	if requested != nil {
		return *requested
	}
	return inherited
}

func cloneOptionalInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
