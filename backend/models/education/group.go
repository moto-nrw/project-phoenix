package education

import (
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/uptrace/bun"
)

// Group represents an educational group/class
type Group struct {
	base.Model `bun:"schema:education,table:groups"`
	base.TenantModel
	Name   string `bun:"name,notnull" json:"name"`
	RoomID *int64 `bun:"room_id" json:"room_id,omitempty"`

	// Relations not stored in the database
	Room *facilities.Room `bun:"rel:belongs-to,join:room_id=id" json:"room,omitempty"`
	// Teachers are linked through the GroupTeacher model
	// Students will be a relationship from the Student model
}

// BeforeAppendModel lets us modify query before it's executed
func (g *Group) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`education.groups AS "group"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`education.groups AS "group"`)
	}
	return nil
}

// Validate ensures group data is valid
func (g *Group) Validate() error {
	if g.Name == "" {
		return errors.New("group name is required")
	}

	// Trim spaces from name
	g.Name = strings.TrimSpace(g.Name)

	return nil
}

// SetRoom assigns this group to a room
func (g *Group) SetRoom(room *facilities.Room) {
	g.Room = room
	if room != nil {
		g.RoomID = &room.ID
	} else {
		g.RoomID = nil
	}
}

// HasRoom checks if the group has a room assigned
func (g *Group) HasRoom() bool {
	return g.RoomID != nil && *g.RoomID > 0
}
