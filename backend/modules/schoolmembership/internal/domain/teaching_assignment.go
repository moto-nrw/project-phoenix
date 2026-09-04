package domain

import (
	"errors"
	"time"
)

var (
	ErrClassAssignmentNotFound = errors.New("class assignment not found")
	ErrGroupAssignmentNotFound = errors.New("group assignment not found")
	ErrClassAssignmentConflict = errors.New("class assignment already exists")
	ErrGroupAssignmentConflict = errors.New("group assignment already exists")
)

type ClassAssignment struct {
	ID          int64
	TenantID    int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	StaffID     int64
	SchoolClass string
}

type ClassAssignmentFilter struct {
	IDs       []int64
	StaffIDs  []int64
	ClassKeys []string
}

type GroupAssignment struct {
	ID        int64
	TenantID  int64
	CreatedAt time.Time
	UpdatedAt time.Time
	GroupID   int64
	TeacherID int64
}

type GroupAssignmentFilter struct {
	IDs             []int64
	GroupIDs        []int64
	TeacherIDs      []int64
	TeacherStaffIDs []int64
}
