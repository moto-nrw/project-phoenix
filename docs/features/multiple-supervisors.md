# Multiple Supervisors Feature

## Overview

The Multiple Supervisors feature allows educational groups and rooms to have multiple staff members assigned as supervisors simultaneously. This enhances oversight capabilities and provides flexibility in staff management.

## Database Structure

### Table: `active.group_supervisors`
This table manages the many-to-many relationship between staff members and active groups.

```sql
CREATE TABLE active.group_supervisors (
    id BIGSERIAL PRIMARY KEY,
    staff_id BIGINT NOT NULL,             -- Reference to users.staff
    group_id BIGINT NOT NULL,             -- Reference to active.groups
    role VARCHAR(50) NOT NULL DEFAULT 'supervisor', -- Role in the group
    start_date DATE NOT NULL DEFAULT CURRENT_DATE,
    end_date DATE,                        -- Optional end date if temporary
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT unique_staff_group_role UNIQUE (staff_id, group_id, role),
    CONSTRAINT fk_supervision_staff FOREIGN KEY (staff_id) 
        REFERENCES users.staff(id) ON DELETE CASCADE,
    CONSTRAINT fk_supervision_group FOREIGN KEY (group_id) 
        REFERENCES active.groups(id) ON DELETE CASCADE
);
```

### Key Features:
- **Role-based assignments**: Staff can have different roles (supervisor, assistant, etc.)
- **Date tracking**: Start and optional end dates for temporary assignments
- **Unique constraint**: Prevents duplicate assignments with the same role
- **Cascade deletion**: Automatically removes assignments when staff or groups are deleted

## Frontend Implementation

### SupervisorMultiSelect Component

Located at: `frontend/src/components/groups/supervisor-multi-select.tsx`

This component provides a user-friendly interface for selecting multiple supervisors:

```typescript
interface SupervisorMultiSelectProps {
  selectedSupervisors: string[];
  onSelectionChange: (supervisorIds: string[]) => void;
  placeholder?: string;
  className?: string;
  onError?: (error: string) => void;
}
```

#### Features:
- **Search functionality**: Filter teachers by name or specialization
- **Visual feedback**: Selected supervisors shown as badges
- **Checkbox selection**: Clear indication of selected items
- **Real-time updates**: Changes reflected immediately
- **Error handling**: Graceful error display and recovery
- **Performance optimization**: Uses React memoization for large lists

#### Usage Example:
```typescript
const [selectedSupervisors, setSelectedSupervisors] = useState<string[]>([]);

<SupervisorMultiSelect
  selectedSupervisors={selectedSupervisors}
  onSelectionChange={setSelectedSupervisors}
  placeholder="Select supervisors..."
  onError={(error) => console.error(error)}
/>
```

## Backend Implementation

### Active Service Changes

The active service (`backend/services/active/active_service.go`) has been enhanced to support multiple supervisors:

1. **Conflict Detection**: New method `CheckActivityDeviceConflict` prevents the same activity from running on multiple devices simultaneously
2. **Force Start**: Enhanced `ForceStartActivitySession` properly ends existing sessions before starting new ones
3. **Conflict Information**: Improved conflict messages to clearly indicate activity vs device conflicts

### Repository Layer

The group repository (`backend/database/repositories/active/group.go`) includes:
- `CheckActivityDeviceConflict`: Validates that activities aren't already active on other devices
- Proper transaction handling for concurrent operations

## API Integration

### Endpoints Affected:
- `POST /api/active/sessions` - Create new active sessions with multiple supervisors
- `PUT /api/active/sessions/{id}` - Update supervisor assignments
- `GET /api/active/groups` - Returns supervisor information with groups

### Response Structure:
```json
{
  "id": 123,
  "name": "Group A",
  "room_id": 45,
  "supervisors": [
    {
      "id": 1,
      "staff_id": 10,
      "name": "John Doe",
      "role": "supervisor"
    },
    {
      "id": 2,
      "staff_id": 11,
      "name": "Jane Smith",
      "role": "assistant"
    }
  ]
}
```

## Security Considerations

1. **Permission Checks**: Only users with appropriate permissions can modify supervisor assignments
2. **GDPR Compliance**: Supervisor data follows the same privacy rules as other personal data
3. **Audit Trail**: All supervisor assignments are tracked with timestamps

## Migration Notes

- Migration `001004003_active_groups_supervisors.go` creates the necessary table structure
- No breaking changes to existing functionality
- Existing single supervisor assignments remain compatible

## Future Enhancements

1. **Notification System**: Alert supervisors when assigned to groups
2. **Scheduling Integration**: Automatic supervisor assignments based on schedules
3. **Reporting**: Analytics on supervisor workload and group coverage
4. **Mobile Support**: Enhanced mobile UI for supervisor management

## Testing

### Backend Tests
- Unit tests for conflict detection logic
- Integration tests for concurrent supervisor assignments
- Repository tests for data integrity

### Frontend Tests
- Component tests for SupervisorMultiSelect
- Integration tests with API
- Accessibility testing for multi-select interface

## Troubleshooting

### Common Issues:
1. **Duplicate assignments**: Check unique constraint on (staff_id, group_id, role)
2. **Performance with many teachers**: Ensure proper indexing on staff tables
3. **UI not updating**: Verify state management in parent components

### Debug Queries:
```sql
-- View all supervisor assignments for a group
SELECT gs.*, s.*, p.first_name, p.last_name 
FROM active.group_supervisors gs
JOIN users.staff s ON s.id = gs.staff_id
JOIN users.persons p ON p.id = s.person_id
WHERE gs.group_id = ?;

-- Find groups without supervisors
SELECT g.* FROM active.groups g
LEFT JOIN active.group_supervisors gs ON gs.group_id = g.id
WHERE gs.id IS NULL AND g.end_time IS NULL;
```