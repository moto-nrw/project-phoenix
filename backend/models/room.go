package models

import (
	"net/http"
	"time"
)

// Room represents a physical room in the institution
type Room struct {
	ID         int64     `json:"id" bun:"id,pk,autoincrement"`
	RoomName   string    `json:"room_name" bun:"room_name,notnull,unique"`
	Building   string    `json:"building" bun:"building"`
	Floor      int       `json:"floor" bun:"floor,notnull,default:0"`
	Capacity   int       `json:"capacity" bun:"capacity,notnull"`
	Category   string    `json:"category" bun:"category,notnull,default:'Other'"`
	Color      string    `json:"color" bun:"color,notnull,default:'#FFFFFF'"`
	CreatedAt  time.Time `json:"created_at" bun:"created_at,notnull"`
	ModifiedAt time.Time `json:"updated_at" bun:"modified_at,notnull"`
}

// Bind preprocesses a Room request
func (r *Room) Bind(req *http.Request) error {
	// Validation logic can be added here
	return nil
}
