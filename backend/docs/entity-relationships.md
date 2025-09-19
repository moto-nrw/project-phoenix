# Entity Relationship Map

This document provides a comprehensive map of all entity relationships in the Project Phoenix system, including foreign key relationships, join tables, and the correct order for creating entities to respect foreign key constraints.

## Schema Organization

The database is organized into multiple PostgreSQL schemas:
- **auth**: Authentication and authorization
- **users**: User profiles and person management
- **education**: Educational groups and structures
- **facilities**: Physical rooms and locations
- **activities**: Student activities and enrollments
- **active**: Real-time session tracking
- **iot**: Device management
- **audit**: Audit logging
- **feedback**: User feedback
- **config**: System configuration
- **schedule**: Time and schedule management

## Core Entity Relationships

### 1. User Management Hierarchy

#### Person → Staff → Teacher
```
users.persons
    ├── account_id → auth.accounts (optional)
    └── tag_id → users.rfid_cards (optional)
         ↓
users.staff
    └── person_id → users.persons (required, unique)
         ↓
users.teachers
    └── staff_id → users.staff (required, unique)
```

**Creation Order:**
1. auth.accounts (if needed)
2. users.rfid_cards (if needed)
3. users.persons
4. users.staff
5. users.teachers

#### Person → Student
```
users.persons
    ├── account_id → auth.accounts (optional)
    └── tag_id → users.rfid_cards (optional)
         ↓
users.students
    ├── person_id → users.persons (required)
    └── group_id → education.groups (optional)
```

**Creation Order:**
1. auth.accounts (if needed)
2. users.rfid_cards (if needed)
3. users.persons
4. facilities.rooms (if group needs room)
5. education.groups (if needed)
6. users.students

### 2. Educational Structure

#### Groups and Teachers (Many-to-Many)
```
education.groups
    └── room_id → facilities.rooms (optional)
         ↓
education.group_teacher (join table)
    ├── group_id → education.groups (required)
    └── teacher_id → users.teachers (required)
```

**Creation Order:**
1. facilities.rooms (if needed)
2. education.groups
3. users.persons → users.staff → users.teachers
4. education.group_teacher

#### Group Substitutions
```
education.group_substitution
    ├── group_id → education.groups (required)
    ├── regular_staff_id → users.staff (optional)
    └── substitute_staff_id → users.staff (required)
```

### 3. Active Session Management

#### Active Groups
```
active.groups
    ├── group_id → activities.groups (required)
    ├── device_id → iot.devices (optional)
    └── room_id → facilities.rooms (required)
```

**Creation Order:**
1. facilities.rooms
2. activities.categories (for activities.groups)
3. activities.groups
4. iot.devices (if needed)
5. active.groups

#### Active Group Supervisors (Many-to-Many)
```
active.group_supervisors
    ├── staff_id → users.staff (required)
    └── group_id → active.groups (required)
```

#### Student Visits
```
active.visits
    ├── student_id → users.students (required)
    └── active_group_id → active.groups (required)
```

**Creation Order:**
1. active.groups (see above)
2. users.persons → users.students
3. active.visits

#### Combined Groups
```
active.combined_groups
         ↓
active.group_mappings (join table)
    ├── active_combined_group_id → active.combined_groups (required)
    └── active_group_id → active.groups (required)
```

### 4. Activity Management

#### Activity Groups
```
activities.groups
    ├── category_id → activities.categories (required)
    └── planned_room_id → facilities.rooms (optional)
```

#### Student Enrollments (Many-to-Many)
```
activities.student_enrollments
    ├── student_id → users.students (required)
    └── activity_group_id → activities.groups (required)
```

#### Activity Supervisors
```
activities.supervisor_planned
    ├── staff_id → users.staff (required)
    ├── group_id → activities.groups (required)
    └── schedule_id → activities.schedules (optional)
```

### 5. IoT Device Management

```
iot.devices
    └── registered_by_id → users.persons (optional)
```

### 6. Authentication and Authorization

#### Account Relationships
```
auth.accounts
         ↓
auth.account_roles (join table)
    ├── account_id → auth.accounts (required)
    └── role_id → auth.roles (required)
         ↓
auth.account_permissions (join table)
    ├── account_id → auth.accounts (required)
    └── permission_id → auth.permissions (required)
```

#### Role-Permission Mapping
```
auth.role_permissions (join table)
    ├── role_id → auth.roles (required)
    └── permission_id → auth.permissions (required)
```

### 7. Guardian Relationships

```
users.students_guardians (join table)
    ├── student_id → users.students (required)
    └── guardian_account_id → auth.account_parents (required)
```

## Key Relationship Patterns

### One-to-One Relationships
- users.persons ↔ auth.accounts (optional)
- users.persons ↔ users.rfid_cards (optional)
- users.staff → users.persons (unique)
- users.teachers → users.staff (unique)

### One-to-Many Relationships
- education.groups → facilities.rooms
- users.students → education.groups
- activities.groups → activities.categories
- active.groups → facilities.rooms
- active.groups → activities.groups
- active.groups → iot.devices (optional)

### Many-to-Many Relationships (via join tables)
- education.groups ↔ users.teachers (via education.group_teacher)
- users.students ↔ activities.groups (via activities.student_enrollments)
- active.groups ↔ users.staff (via active.group_supervisors)
- users.students ↔ auth.account_parents (via users.students_guardians)
- auth.accounts ↔ auth.roles (via auth.account_roles)
- auth.accounts ↔ auth.permissions (via auth.account_permissions)
- auth.roles ↔ auth.permissions (via auth.role_permissions)
- active.combined_groups ↔ active.groups (via active.group_mappings)

## Required vs Optional Relationships

### Always Required
- users.staff → users.persons
- users.teachers → users.staff
- users.students → users.persons
- education.group_teacher → education.groups, users.teachers
- active.groups → activities.groups, facilities.rooms
- active.visits → users.students, active.groups
- activities.groups → activities.categories

### Optional Relationships
- users.persons → auth.accounts
- users.persons → users.rfid_cards
- users.students → education.groups
- education.groups → facilities.rooms
- active.groups → iot.devices
- activities.groups → facilities.rooms (planned_room_id)
- education.group_substitution → users.staff (regular_staff_id)

## Cascade Delete Patterns

Most foreign key relationships are set up with `ON DELETE CASCADE` to ensure referential integrity:

1. Deleting a person cascades to:
   - users.staff
   - users.students
   - Any associated records

2. Deleting a staff member cascades to:
   - users.teachers
   - active.group_supervisors
   - education.group_substitution

3. Deleting an active group cascades to:
   - active.visits
   - active.group_supervisors
   - active.group_mappings

## Data Creation Best Practices

### Order of Operations for Common Scenarios

#### Creating a Teacher
1. Create auth.account (if login needed)
2. Create users.rfid_card (if RFID access needed)
3. Create users.person with account_id and/or tag_id
4. Create users.staff with person_id
5. Create users.teacher with staff_id

#### Creating a Student with Guardian
1. Create guardian auth.account_parent
2. Create student users.rfid_card (if needed)
3. Create student users.person
4. Create education.group (if needed)
5. Create users.student with group_id
6. Create users.students_guardians relationship

#### Starting an Active Session
1. Ensure facilities.room exists
2. Ensure activities.group exists
3. Ensure iot.device exists (if RFID system)
4. Create active.group
5. Assign supervisors via active.group_supervisors
6. Track student visits via active.visits

### Important Notes

1. **Schema-Qualified Tables**: Always use schema-qualified table names in queries (e.g., `users.persons`, not just `persons`)

2. **Unique Constraints**: Some relationships have unique constraints:
   - users.staff.person_id is unique (one staff record per person)
   - users.teachers.staff_id is unique (one teacher record per staff)
   - auth.accounts.email is unique

3. **Device Authentication**: IoT devices use a two-layer authentication:
   - Device API key (stored in iot.devices)
   - Staff PIN (stored in auth.accounts.pin_hash)

4. **Student Location Tracking**: 
   - Real tracking: active.visits and active.attendance tables
   - Deprecated: Boolean flags in users.students (in_house, wc, school_yard)
   - Bus flag: Administrative permission only, not location

5. **Group Types**:
   - education.groups: Regular educational groups/classes
   - activities.groups: Activity groups for extracurricular activities
   - active.groups: Currently active sessions
   - active.combined_groups: Multiple groups combined into one session