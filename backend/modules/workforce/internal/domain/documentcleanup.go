package domain

type DocumentCleanupClaim struct {
	StaffID  int64
	Token    string
	Attempts int
}

type DocumentCleanupBacklog struct {
	Pending          int
	OldestAgeSeconds float64
}
