package domain

import (
	"time"
)

const (
	SchulhofRoomName = "Schulhof"
	WCRoomName       = "WC"
	WCRoomAliasName  = "Toilette"

	RoomNameUniqueConstraint    = "idx_rooms_tenant_name"
	RoomColorUniqueConstraint   = "uniq_facilities_rooms_tenant_color"
	RoomWCAliasUniqueConstraint = "uniq_facilities_rooms_tenant_wc_alias"
)

type OperationStats struct {
	Queries           int64
	Rows              int64
	StatementDuration time.Duration
}

func (s *OperationStats) Add(other OperationStats) {
	s.Queries += other.Queries
	s.Rows += other.Rows
	s.StatementDuration += other.StatementDuration
}
