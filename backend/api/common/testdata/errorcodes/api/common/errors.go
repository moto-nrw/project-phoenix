package common

type ErrResponse struct{ Code string }

func ErrorConflictWithCode(err error, code string) *ErrResponse { return &ErrResponse{Code: code} }
