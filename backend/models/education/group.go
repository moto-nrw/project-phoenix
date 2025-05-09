package education

import (
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// Group represents an educational group/class
type Group struct {
	base.Model
	Name   string `bun:"name,notnull,unique" json:"name"`
	RoomID *int64 `bun:"room_id" json:"room_id,omitempty"`

	// Relations not stored in the database
	Room *facilities.Room `bun:"-" json:"room,omitempty"`
	// Teachers are linked through the GroupTeacher model
	// Students will be a relationship from the Student model
}

// TableName returns the database table name
func (g *Group) TableName() string {
	return "education.groups"
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

// GetID returns the group's ID
func (g *Group) GetID() int64 {
	return g.ID
}

// GetName returns the group's name
func (g *Group) GetName() string {
	return g.Name
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

// GroupTeacher represents the many-to-many relationship between groups and teachers
type GroupTeacher struct {
	base.Model
	GroupID   int64 `bun:"group_id,notnull" json:"group_id"`
	TeacherID int64 `bun:"teacher_id,notnull" json:"teacher_id"`

	// Relations not stored in the database
	Group   *Group         `bun:"-" json:"group,omitempty"`
	Teacher *users.Teacher `bun:"-" json:"teacher,omitempty"`
}

// TableName returns the database table name
func (gt *GroupTeacher) TableName() string {
	return "education.group_teacher"
}

// Validate ensures group teacher data is valid
func (gt *GroupTeacher) Validate() error {
	if gt.GroupID <= 0 {
		return errors.New("group ID is required")
	}

	if gt.TeacherID <= 0 {
		return errors.New("teacher ID is required")
	}

	return nil
}
