# Seed Data Documentation

This document describes the seed data functionality for the Project Phoenix backend.

## Overview

The `seed` command populates the database with dummy data for testing and development purposes. It creates realistic test data for all major entities in the system.

## Usage

### Basic Usage

```bash
go run main.go seed
```

This will add dummy data to your existing database.

### Reset and Seed

```bash
go run main.go seed --reset
```

This will first delete all existing data from the seeded tables and then populate them with fresh dummy data. You will be prompted for confirmation before any data is deleted.

## What Gets Created

The seed command creates the following entities:

### Rooms (24 total)
- **Classrooms**: 6 rooms (Main Building)
- **Science Labs**: 4 labs (Science Building)
- **Sports Facilities**: 2 gyms (Sports Complex)
- **Art/Music Rooms**: 2 rooms (Creative Wing)
- **Computer Labs**: 2 labs (Tech Center)
- **Library**: 2 rooms (Library Building)
- **Special Purpose**: Cafeteria, Auditorium, Nurse's Office
- **Offices**: Principal's Office, Teachers' Lounge, Conference Room

### Groups (25 total)
- **Grade Classes**: 10 classes (1A, 1B, 2A, 2B, ... 5A, 5B)
- **Activity Groups**: 15 groups (Science Club, Art Club, Sports Teams, etc.)

### People (150 total)
- **Staff/Teachers**: 30 people
  - 20 are teachers with various specializations
  - 10 are other staff members
- **Students**: 120 people
  - Distributed across grade classes
  - Each with guardian information
  - Some assigned to use the bus (30%)

## Data Characteristics

### Rooms
- Each room has:
  - Name and building location
  - Floor number
  - Capacity
  - Category (Classroom, Laboratory, etc.)
  - Color code for UI display

### Groups
- Grade classes are assigned to classrooms
- Activity groups may or may not have assigned rooms

### Teachers
- Specializations include: Mathematics, Science, English, History, etc.
- Each teacher has:
  - Role/position
  - Qualifications
  - Associated staff record

### Students
- Each student has:
  - School class assignment (1A-5B)
  - Guardian contact information
  - Location status (bus, in-house, etc.)
  - RFID tag ID

## Implementation Details

The seed command is implemented in `/backend/cmd/seed.go` with the following features:

1. **Direct SQL Queries**: Uses raw SQL queries for efficient insertion
2. **Transaction Safety**: All operations are wrapped in a database transaction
3. **Error Handling**: Comprehensive error handling with rollback on failure
4. **Realistic Data**: Generates realistic names, contacts, and relationships
5. **Reset Option**: Optional `--reset` flag to clear existing data
6. **Confirmation Prompt**: Asks for confirmation before deleting data

The seed command:
- Creates data in the correct order to respect foreign key constraints
- Generates IDs that are returned and used for relationships
- Uses proper timestamp fields (`created_at`, `updated_at`)
- Follows the database schema exactly
- Is efficient with batch operations in a single transaction

## Custom Random Data

The seed command uses randomized but realistic data:
- Names are randomly selected from pools of common first and last names
- Phone numbers follow a consistent format
- Email addresses are generated based on names
- RFID tags are unique sequential identifiers

## Extending the Seed Data

To add more seed data or modify existing patterns, edit the `/backend/cmd/seed.go` file. The main functions to modify are:

- `seedRooms()` - Add or modify room data
- `seedGroups()` - Add or modify group data
- `seedPersons()` - Modify person generation
- `seedStaff()` - Modify staff creation
- `seedTeachers()` - Modify teacher data
- `seedStudents()` - Modify student generation

## Error Handling

The seed command includes comprehensive error handling:
- Database connection failures are caught early
- Each entity creation is checked for errors
- Transaction rollback on any failure
- Clear error messages indicate which operation failed

## Performance

The seed operation typically completes in under 5 seconds, creating:
- ~24 rooms
- ~25 groups
- ~150 persons
- ~30 staff records
- ~20 teacher records
- ~120 student records

All operations are batched in a single transaction for optimal performance.