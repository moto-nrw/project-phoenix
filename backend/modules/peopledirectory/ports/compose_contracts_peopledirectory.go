package ports

// StaffMessageRole is the school role class of one account.
type StaffMessageRole struct {
	AccountID   int64
	IsAdmin     bool
	IsLehrkraft bool
}
