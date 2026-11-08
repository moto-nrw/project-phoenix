package carecontract

import "context"

// DocumentPushCandidate is an announcement of the first contract send to
// this address. That mailing fixes the version, even after later resends.
type DocumentPushCandidate struct {
	Key        int64   `json:"key"`
	ContractID int64   `json:"contract_id"`
	Email      string  `json:"email"`
	SchoolID   int64   `json:"school_id"`
	StudentIDs []int64 `json:"student_ids"`
}

type DocumentPushValidity interface {
	ValidContractPushes(context.Context, int64, []DocumentPushCandidate) (map[int64]bool, error)
}
