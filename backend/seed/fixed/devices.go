package fixed

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/moto-nrw/project-phoenix/models/iot"
)

// Device placement data
type devicePlacement struct {
	deviceID   string
	deviceType string
	name       string
	roomName   string
	apiKey     string
}

// seedIoTDevices creates IoT devices with proper room assignments
func (s *Seeder) seedIoTDevices(ctx context.Context) error {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Device placements with room assignments
	placements := []devicePlacement{
		{"RFID-MAIN-001", "rfid_reader", "Haupteingang-Scanner", "Lehrerzimmer", generateAPIKey()},
		{"RFID-MENSA-001", "rfid_reader", "Mensa-Leser", "Mensa", generateAPIKey()},
		{"RFID-OGS-001", "rfid_reader", "OGS-Bereich-Scanner", "OGS-Raum 1", generateAPIKey()},
		{"RFID-SPORT-001", "rfid_reader", "Sporthalle-Terminal", "Sporthalle", generateAPIKey()},
		{"RFID-LIB-001", "rfid_reader", "Bibliothek-Terminal", "Bibliothek", generateAPIKey()},
		{"TEMP-CLASS-001", "temperature_sensor", "Temperatursensor Klassenzimmer", "Klassenzimmer 1A", generateAPIKey()},
		{"TEMP-MENSA-001", "temperature_sensor", "Temperatursensor Mensa", "Mensa", generateAPIKey()},
	}

	// Use the first staff member as device registrar
	registrarID := s.result.Persons[0].ID

	for _, placement := range placements {
		// Find the room
		var roomID *int64
		for _, room := range s.result.Rooms {
			if room.Name == placement.roomName {
				roomID = &room.ID
				break
			}
		}

		if roomID == nil {
			return fmt.Errorf("room %s not found for device %s", placement.roomName, placement.name)
		}

		// Create device
		lastSeen := time.Now().Add(-time.Duration(rng.Intn(60)) * time.Minute)
		device := &iot.Device{
			DeviceID:       placement.deviceID,
			DeviceType:     placement.deviceType,
			Name:           &placement.name,
			Status:         iot.DeviceStatusActive,
			LastSeen:       &lastSeen,
			RegisteredByID: &registrarID,
			APIKey:         &placement.apiKey,
		}
		device.CreatedAt = time.Now()
		device.UpdatedAt = time.Now()

		_, err := s.tx.NewInsert().Model(device).ModelTableExpr("iot.devices").Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create device %s: %w", placement.name, err)
		}

		s.result.Devices = append(s.result.Devices, device)
		s.result.DevicesByRoom[*roomID] = device
	}

	if s.verbose {
		log.Printf("Created %d IoT devices with room assignments", len(s.result.Devices))
	}

	return nil
}

// generateAPIKey creates a realistic API key for devices
func generateAPIKey() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}