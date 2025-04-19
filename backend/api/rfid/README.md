# RFID Integration System

This package implements an RFID-based student tracking system for a school environment. It provides functionality for tracking student locations, room occupancy, and visit history.

## Features

- RFID tag reading and tracking
- Student location tracking (in-house, WC, school yard)
- Room occupancy monitoring
- Student visit records with timestamps
- Room entry and exit tracking
- Tauri desktop app integration

## API Endpoints

### RFID Tag Handling
- `POST /tag` - Logs a new RFID tag read
- `GET /tags` - Lists all RFID tag reads

### Student Tracking
- `POST /track-student` - Tracks student location based on RFID tag
- `GET /student/{id}/visits` - Gets visit history for a specific student

### Room Management
- `POST /room-entry` - Records a student entering a room
- `POST /room-exit` - Records a student exiting a room 
- `GET /room-occupancy` - Returns current room occupancy data
- `GET /room/{id}/visits` - Gets visit history for a specific room
- `GET /visits/today` - Gets all visits for the current day

### Tauri App Integration
- `POST /app/sync` - Syncs data from Tauri desktop app
- `GET /app/status` - Returns system status for Tauri app

## Testing

The system includes comprehensive tests for all components including:

### Unit Tests
- Basic functionality testing of individual handler functions
- Mocking of all dependencies using testify/mock
- Tests for tag reading, student tracking, and basic endpoints

### Integration Tests
- Complete flow testing from RFID tag read to student location updates
- Multi-step processes like entrance, room entry, room exit
- Testing of complex data relationships like visits and timespans
- Multiple student scenarios with concurrent processing

### Failure and Edge Case Tests
- Error handling for unknown tags
- Database failures during tag registration
- Timespan creation errors
- Student location update issues
- Malformed request handling
- Missing student record handling
- Missing dependency handling
- Concurrent tag read handling

### Test Coverage
- The tests cover all main code paths and critical logic
- Error paths and edge cases are thoroughly tested
- Multi-user scenarios are simulated

## Dependencies

- Chi router for HTTP routing
- Bun ORM for database operations
- Logrus for structured logging
- Testify for testing utilities and mocking

## Implementation Details

- Implemented with a repository pattern (Store interfaces)
- Uses dependency injection for testability
- Follows RESTful API design principles
- Uses structured logging for operations and errors
- Implements proper error handling with graceful degradation