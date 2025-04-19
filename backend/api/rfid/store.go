package rfid

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

// RFIDStore defines database operations for RFID tag management
type RFIDStore interface {
	SaveTag(ctx context.Context, tagID, readerID string) (*Tag, error)
	GetAllTags(ctx context.Context) ([]Tag, error)
	GetTagStats(ctx context.Context) (int, error)
	SaveTauriTags(ctx context.Context, deviceID string, tags []SyncTag) error

	// Room occupancy tracking operations
	RecordRoomEntry(ctx context.Context, studentID, roomID int64) error
	RecordRoomExit(ctx context.Context, studentID, roomID int64) error
	GetRoomOccupancy(ctx context.Context, roomID int64) (*RoomOccupancyData, error)
	GetCurrentRooms(ctx context.Context) ([]RoomOccupancyData, error)

	// Tauri device management operations
	RegisterDevice(ctx context.Context, deviceID, name, description string) (*TauriDevice, string, error)
	GetDevice(ctx context.Context, deviceID string) (*TauriDevice, error)
	GetDeviceByAPIKey(ctx context.Context, apiKey string) (*TauriDevice, error)
	UpdateDevice(ctx context.Context, deviceID string, updates map[string]interface{}) error
	ListDevices(ctx context.Context) ([]TauriDevice, error)
	RecordDeviceSync(ctx context.Context, deviceID, ipAddress, appVersion string, tagsCount int) error
	GetDeviceSyncHistory(ctx context.Context, deviceID string, limit int) ([]DeviceSyncHistory, error)
}

type rfidStore struct {
	db *bun.DB
}

// NewRFIDStore returns a new RFIDStore implementation
func NewRFIDStore(db *bun.DB) RFIDStore {
	return &rfidStore{db: db}
}

// SaveTag saves an RFID tag read to the database
func (s *rfidStore) SaveTag(ctx context.Context, tagID, readerID string) (*Tag, error) {
	now := time.Now()
	tag := &Tag{
		TagID:     tagID,
		ReaderID:  readerID,
		ReadAt:    now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err := s.db.NewInsert().
		Model(tag).
		Exec(ctx)

	if err != nil {
		return nil, err
	}

	return tag, nil
}

// GetAllTags returns all stored RFID tags
func (s *rfidStore) GetAllTags(ctx context.Context) ([]Tag, error) {
	var tags []Tag
	err := s.db.NewSelect().
		Model(&tags).
		Order("read_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return tags, nil
}

// GetTagStats returns statistics about RFID tags
func (s *rfidStore) GetTagStats(ctx context.Context) (int, error) {
	count, err := s.db.NewSelect().
		Model((*Tag)(nil)).
		Count(ctx)

	if err != nil {
		return 0, err
	}

	return count, nil
}

// SaveTauriTags saves a batch of tags from the Tauri app
func (s *rfidStore) SaveTauriTags(ctx context.Context, deviceID string, syncTags []SyncTag) error {
	if len(syncTags) == 0 {
		return nil
	}

	tags := make([]Tag, len(syncTags))
	now := time.Now()

	for i, syncTag := range syncTags {
		tags[i] = Tag{
			TagID:     syncTag.TagID,
			ReaderID:  syncTag.ReaderID,
			ReadAt:    syncTag.LocalReadAt,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}

	_, err := s.db.NewInsert().
		Model(&tags).
		Exec(ctx)

	return err
}

// RecordRoomEntry records a student entering a room
func (s *rfidStore) RecordRoomEntry(ctx context.Context, studentID, roomID int64) error {
	// This is a simplified implementation
	// In a real application, you would:
	// 1. Check if the student is already in a room
	// 2. If yes, record an exit from the previous room
	// 3. Record an entry to the new room

	// For now, we'll just create a visit record in the database
	// This assumes the database has a table for student visits with roomID, studentID, and entryTime fields
	now := time.Now()
	visit := struct {
		RoomID    int64     `bun:"room_id"`
		StudentID int64     `bun:"student_id"`
		EntryTime time.Time `bun:"entry_time"`
		ExitTime  time.Time `bun:"exit_time,nullzero"`
		CreatedAt time.Time `bun:"created_at"`
	}{
		RoomID:    roomID,
		StudentID: studentID,
		EntryTime: now,
		CreatedAt: now,
	}

	_, err := s.db.NewInsert().
		Model(&visit).
		Exec(ctx)

	return err
}

// RecordRoomExit records a student exiting a room
func (s *rfidStore) RecordRoomExit(ctx context.Context, studentID, roomID int64) error {
	// Find the current visit for this student and room where exit time is null
	// and update the exit time
	now := time.Now()

	_, err := s.db.NewUpdate().
		Table("student_room_visits").
		Set("exit_time = ?", now).
		Where("student_id = ? AND room_id = ? AND exit_time IS NULL", studentID, roomID).
		Exec(ctx)

	return err
}

// GetRoomOccupancy gets the current occupancy for a specific room
func (s *rfidStore) GetRoomOccupancy(ctx context.Context, roomID int64) (*RoomOccupancyData, error) {
	// This is a simplified implementation
	// In a real application, you would join with the rooms and students tables
	// to get all the details needed

	// For now, we'll return a mocked response
	result := &RoomOccupancyData{
		RoomID:       roomID,
		RoomName:     "Room " + fmt.Sprintf("%d", roomID), // You would get this from the DB
		Capacity:     30,                                  // You would get this from the DB
		StudentCount: 0,
		Students:     []RoomOccupancyStudent{},
	}

	// Query the students currently in the room (entry_time IS NOT NULL AND exit_time IS NULL)
	type queryResult struct {
		StudentID int64     `bun:"student_id"`
		Name      string    `bun:"name"` // This assumes you have a way to join to get the name
		EntryTime time.Time `bun:"entry_time"`
	}

	var results []queryResult

	// This is a placeholder query - in a real app, you would join with student/user tables
	err := s.db.NewSelect().
		TableExpr("student_room_visits v").
		ColumnExpr("v.student_id, 'Student Name' AS name, v.entry_time"). // Placeholder name
		Where("v.room_id = ? AND v.exit_time IS NULL", roomID).
		Scan(ctx, &results)

	if err != nil {
		return result, err
	}

	// Convert the results to RoomOccupancyStudent objects
	students := make([]RoomOccupancyStudent, len(results))
	for i, r := range results {
		students[i] = RoomOccupancyStudent{
			ID:        r.StudentID,
			Name:      r.Name,
			EnteredAt: r.EntryTime,
		}
	}

	result.Students = students
	result.StudentCount = len(students)

	return result, nil
}

// GetCurrentRooms gets all rooms with their current occupancy
func (s *rfidStore) GetCurrentRooms(ctx context.Context) ([]RoomOccupancyData, error) {
	// This is a simplified implementation
	// In a real application, you would get all rooms with active occupancy

	// For demonstration, let's get a list of room IDs with active visits
	// and then call GetRoomOccupancy for each
	var roomIDs []int64

	err := s.db.NewSelect().
		TableExpr("student_room_visits").
		ColumnExpr("DISTINCT room_id").
		Where("exit_time IS NULL").
		Scan(ctx, &roomIDs)

	if err != nil {
		return nil, err
	}

	// Get occupancy for each room
	result := make([]RoomOccupancyData, len(roomIDs))
	for i, roomID := range roomIDs {
		occupancy, err := s.GetRoomOccupancy(ctx, roomID)
		if err != nil {
			continue // Skip rooms with errors
		}
		result[i] = *occupancy
	}

	return result, nil
}

// generateAPIKey creates a secure random API key
func generateAPIKey() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "="), nil
}

// RegisterDevice creates a new Tauri device registration
func (s *rfidStore) RegisterDevice(ctx context.Context, deviceID, name, description string) (*TauriDevice, string, error) {
	// Check if device already exists
	var existingDevice TauriDevice
	err := s.db.NewSelect().
		Model(&existingDevice).
		Where("device_id = ?", deviceID).
		Scan(ctx)

	if err == nil {
		return nil, "", fmt.Errorf("device with ID %s already exists", deviceID)
	}

	// Generate a new API key
	apiKey, err := generateAPIKey()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate API key: %w", err)
	}

	// Create the new device
	now := time.Now()
	device := &TauriDevice{
		DeviceID:    deviceID,
		Name:        name,
		Description: description,
		Status:      "active",
		APIKey:      apiKey,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_, err = s.db.NewInsert().
		Model(device).
		Exec(ctx)

	if err != nil {
		return nil, "", err
	}

	return device, apiKey, nil
}

// GetDevice retrieves a device by its ID
func (s *rfidStore) GetDevice(ctx context.Context, deviceID string) (*TauriDevice, error) {
	device := new(TauriDevice)
	err := s.db.NewSelect().
		Model(device).
		Where("device_id = ?", deviceID).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return device, nil
}

// GetDeviceByAPIKey retrieves a device by its API key
func (s *rfidStore) GetDeviceByAPIKey(ctx context.Context, apiKey string) (*TauriDevice, error) {
	device := new(TauriDevice)
	err := s.db.NewSelect().
		Model(device).
		Where("api_key = ?", apiKey).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return device, nil
}

// UpdateDevice updates a device's information
func (s *rfidStore) UpdateDevice(ctx context.Context, deviceID string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()

	query := s.db.NewUpdate().Table("tauri_devices")

	// Add each update to the query
	for key, value := range updates {
		query = query.Set(fmt.Sprintf("%s = ?", key), value)
	}

	// Execute the query
	res, err := query.Where("device_id = ?", deviceID).Exec(ctx)

	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("device with ID %s not found", deviceID)
	}

	return nil
}

// ListDevices returns all registered devices
func (s *rfidStore) ListDevices(ctx context.Context) ([]TauriDevice, error) {
	var devices []TauriDevice
	err := s.db.NewSelect().
		Model(&devices).
		Order("created_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return devices, nil
}

// RecordDeviceSync logs a sync from a device
func (s *rfidStore) RecordDeviceSync(ctx context.Context, deviceID, ipAddress, appVersion string, tagsCount int) error {
	now := time.Now()

	// Update the device's last sync info
	updateErr := s.UpdateDevice(ctx, deviceID, map[string]interface{}{
		"last_sync_at": now,
		"last_ip":      ipAddress,
	})

	if updateErr != nil {
		// Just log the error, don't fail the whole sync
		fmt.Printf("Failed to update device last sync info: %v\n", updateErr)
	}

	// Record the sync history
	syncHistory := &DeviceSyncHistory{
		DeviceID:   deviceID,
		SyncAt:     now,
		IPAddress:  ipAddress,
		TagsCount:  tagsCount,
		AppVersion: appVersion,
		CreatedAt:  now,
	}

	_, err := s.db.NewInsert().
		Model(syncHistory).
		Exec(ctx)

	return err
}

// GetDeviceSyncHistory retrieves sync history for a device
func (s *rfidStore) GetDeviceSyncHistory(ctx context.Context, deviceID string, limit int) ([]DeviceSyncHistory, error) {
	if limit <= 0 {
		limit = 50 // Default limit
	}

	var history []DeviceSyncHistory
	err := s.db.NewSelect().
		Model(&history).
		Where("device_id = ?", deviceID).
		Order("sync_at DESC").
		Limit(limit).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return history, nil
}
