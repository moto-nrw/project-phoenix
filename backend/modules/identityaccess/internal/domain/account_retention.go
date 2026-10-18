package domain

// ErasedAccount is an account the hourly cleanup tombstoned (#4043) with the
// avatar files it held; the files are removed after the rows.
type ErasedAccount struct {
	ID          int64
	AvatarFiles []string
}
