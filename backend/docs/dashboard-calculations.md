# Dashboard Analytics Calculations

This document explains how each value on the dashboard is calculated in the `GetDashboardAnalytics` method in `/backend/services/active/active_service.go`.

## Student Overview Section

### Students Present (Anwesend heute)
- **Calculation**: Count of all active visits in the `active.visits` table
- **Logic**: Iterates through all visits where `exit_time IS NULL` (student is still checked in)
- **Source**: `active.visits` table with `IsActive()` check

### Students Enrolled (Gesamt eingeschrieben)
- **Calculation**: Total count of all students in the system
- **Logic**: Counts all records in `users.students` table
- **Source**: `users.students` table via `studentRepo.List()`

### Students on Playground (Schulhof)
- **Calculation**: Count of students in rooms categorized as playground
- **Logic**: Sums visit counts for rooms where:
  - `room.Category` is "Schulhof", "Playground", or "school_yard"
- **Source**: Active visits grouped by room, filtered by room category

### Students in Transit (Unterwegs)
- **Calculation**: Students who are in OGS (in_house = true) but not currently in any room
- **Logic**: 
  1. Identifies all students with `in_house = true` (enrolled in OGS)
  2. Checks which of these OGS students do NOT have an active visit
  3. These are students who belong to OGS but are not currently in any room, WC, or schoolyard
- **Data Source**: 
  - `users.students` table for `in_house` field
  - `active.visits` table to check current location
- **Note**: This represents OGS students who are in the building but temporarily between locations

## Activities & Rooms Section

### Active Activities (Aktuelle Aktivitäten)
- **Calculation**: Count of active groups (both educational and activity groups)
- **Logic**: Counts all records in `active.groups` where `end_time IS NULL`
- **Source**: `active.groups` table with `IsActive()` check

### Free Rooms (Freie Räume)
- **Calculation**: Total rooms minus occupied rooms
- **Formula**: `Total Rooms - Count of Unique Room IDs with Active Groups`
- **Logic**: 
  - Gets total room count from `facilities.rooms`
  - Subtracts count of unique room IDs that have active groups
- **Source**: `facilities.rooms` and `active.groups` tables

### Capacity Utilization (Kapazität genutzt)
- **Calculation**: Percentage of total room capacity being used
- **Formula**: `(Students in Rooms / Total Room Capacity) * 100`
- **Logic**:
  - Numerator: Total students currently in rooms (from active visits)
  - Denominator: Sum of all room capacities
- **Source**: Room capacities from `facilities.rooms`, student counts from active visits

### Activity Categories (Kategorien)
- **Calculation**: Total count of activity categories
- **Logic**: Counts all records in `activities.categories` table
- **Source**: `activities.categories` table

## OGS Groups Overview Section

### Active OGS Groups (Aktive Gruppen)
- **Calculation**: Count of active educational groups
- **Logic**: 
  - For each active group, checks if it's an educational group
  - Educational groups are identified by finding the group in `education.groups` table
- **Note**: ALL educational groups are considered OGS groups in this system
- **Source**: `active.groups` joined with `education.groups`

### Students in Group Rooms (In Gruppenräumen)
- **Calculation**: Count of students in rooms assigned to educational groups
- **Logic**:
  - Identifies all rooms assigned to educational groups
  - Sums active visit counts for those specific rooms
- **Source**: Room assignments from `education.groups`, visit counts from active visits

### Supervisors Today (Betreuer heute)
- **Calculation**: Count of unique staff members supervising today
- **Logic**:
  - Counts unique staff IDs from active group supervisors
  - Includes supervisors who started today or are currently active
- **Source**: `active.group_supervisors` table

### Students in Home Room (In Heimatraum)
- **Calculation**: Currently same as "Students in Group Rooms"
- **Logic**: All students in educational group rooms are considered to be in their home room
- **Note**: This may need refinement based on specific business rules

## Recent Activity Section

### Recent Activity List
- **Calculation**: Last 3 group activities within the past 30 minutes
- **Logic**:
  - Filters active groups started within last 30 minutes
  - Includes group name, room name, and student count
  - Limited to 3 most recent entries
- **Privacy**: No individual student data is exposed
- **Source**: `active.groups` with related group and room names

## Current Activities Section

### Current Activities List
- **Calculation**: Up to 2 currently active activity groups (not educational groups)
- **Logic**:
  - Checks which activity groups have active sessions
  - Includes participant count and capacity
  - Status determined by fill rate:
    - "full": participants >= max capacity
    - "ending_soon": participants > 80% of capacity
    - "active": otherwise
- **Source**: `activities.groups` matched with `active.groups`

## Active Groups Summary Section

### Active Groups Summary
- **Calculation**: Up to 2 currently active groups (any type)
- **Logic**:
  - Lists active groups with their type (OGS group or activity)
  - Includes location and student count
  - Group type determined by checking if group exists in `education.groups`
- **Source**: `active.groups` with type determination

## Key Data Relationships

1. **Active Visits**: Central to most calculations
   - Links students to active groups
   - Tracks check-in/check-out times
   - `student_id` and `active_group_id` are key fields

2. **Active Groups**: Represents current sessions
   - Links to either educational or activity groups via `group_id`
   - Tracks room assignment via `room_id`
   - Active when `end_time IS NULL`

3. **Room Categories**: Used for location-based calculations
   - "Schulhof"/"Playground"/"school_yard": Outdoor areas
   - "WC"/"Toilette"/"Restroom"/"wc": Restroom facilities
   - Educational group rooms: Identified by room assignments in `education.groups`

4. **Time-based Filters**:
   - Active sessions: `end_time IS NULL`
   - Today's activities: Compare with `time.Now().Truncate(24 * time.Hour)`
   - Recent activities: Within last 30 minutes

## Performance Considerations

- Multiple database queries are executed to gather all data
- Room and group lookups are cached in maps for efficiency
- Limits are applied to lists (3 for recent activities, 2 for current activities)
- All calculations are done in-memory after data retrieval