# Guardian System Migration Guide

## Overview

This guide documents the migration from the legacy guardian system (data stored in `users.students` table) to the new dedicated guardian system with `users.guardians` and proper many-to-many relationships.

## Migration Architecture

### New Database Structure

```
┌─────────────────────────────┐
│  auth.accounts_parents      │  Authentication (optional)
│  (email, password)          │
└──────────┬──────────────────┘
           │ 1-to-0/1
           ▼
┌─────────────────────────────┐
│  users.guardians            │  Profile Information (NEW)
│  (name, phone, email, etc.) │
└──────────┬──────────────────┘
           │ many-to-many
           ▼
┌─────────────────────────────┐
│  users.students_guardians   │  Relationship Link
│  (student_id, guardian_id)  │
└──────────┬──────────────────┘
           │
           ▼
┌─────────────────────────────┐
│  users.students             │  Legacy fields removed
└─────────────────────────────┘
```

## Migration Steps

### 1. Pre-Migration Checklist

- [ ] **BACKUP DATABASE**: `pg_dump project_phoenix > backup_before_guardian_migration.sql`
- [ ] Ensure no active transactions on `users.students` table
- [ ] Review current guardian data quality
- [ ] Test migrations on a staging environment first

### 2. Run Migrations (Automated)

```bash
cd backend

# Check migration status
go run main.go migrate status

# Run the guardian migrations
go run main.go migrate

# Verify migrations
go run main.go migrate status
```

### Migration Sequence

**Migration 1.4.1: Create Guardians Table**
- Creates `users.guardians` table with all necessary fields
- Adds indexes for email, phone, last_name, active status
- Sets up update triggers

**Migration 1.4.2: Migrate Legacy Data**
- Reads all students with `guardian_name`, `guardian_contact`, `guardian_email`, `guardian_phone`
- Parses full names into first/last name
- Deduplicates guardians by email/phone
- Creates `users.guardians` records
- Creates `users.students_guardians` relationships
- Maintains referential integrity

**Migration 1.4.3: Update Structure**
- Renames `guardian_account_id` → `guardian_id` in `users.students_guardians`
- Updates foreign key to reference `users.guardians` instead of `auth.accounts_parents`
- Drops `users.persons_guardians` table (obsolete)
- Removes legacy columns from `users.students`:
  - `guardian_name`
  - `guardian_contact`
  - `guardian_email`
  - `guardian_phone`

### 3. Verify Migration

```sql
-- Check guardians were created
SELECT COUNT(*) FROM users.guardians;

-- Check relationships were created
SELECT COUNT(*) FROM users.students_guardians;

-- Verify students no longer have legacy columns
SELECT column_name FROM information_schema.columns
WHERE table_schema = 'users' AND table_name = 'students'
AND column_name LIKE 'guardian%';
-- Should return no rows

-- Sample guardian data
SELECT id, first_name, last_name, email, phone FROM users.guardians LIMIT 10;

-- Sample relationships
SELECT sg.*, g.first_name, g.last_name
FROM users.students_guardians sg
JOIN users.guardians g ON g.id = sg.guardian_id
LIMIT 10;
```

### 4. Post-Migration Testing

```bash
# Test guardian CRUD endpoints
cd bruno
bru run --env Local 0*.bru

# Manual API tests
curl -X GET http://localhost:8080/api/guardians \
  -H "Authorization: Bearer $TOKEN"

curl -X GET http://localhost:8080/api/students/1/guardians \
  -H "Authorization: Bearer $TOKEN"
```

## Data Migration Logic

### Name Parsing

```
Input: "Maria Schmidt" → First: "Maria", Last: "Schmidt"
Input: "Schmidt" → First: "Schmidt", Last: "Guardian"
Input: "Dr. Hans Peter Müller" → First: "Dr.", Last: "Hans Peter Müller"
```

### Contact Parsing

1. **If `guardian_email` exists**: Use as primary email
2. **If `guardian_phone` exists**: Use as primary phone
3. **If `guardian_contact` contains '@'**: Treat as email, generate placeholder phone
4. **Otherwise**: Treat as phone, generate placeholder email

### Deduplication

Guardians are deduplicated by **email OR phone**:
- If guardian with same email exists → reuse
- If guardian with same phone exists → reuse
- Otherwise → create new guardian

## API Endpoints

### Guardian CRUD

```
GET    /api/guardians              - List all guardians (paginated)
GET    /api/guardians/search?q=... - Search guardians
GET    /api/guardians/{id}         - Get guardian details
POST   /api/guardians              - Create guardian
PUT    /api/guardians/{id}         - Update guardian
DELETE /api/guardians/{id}         - Delete guardian (if no students)
GET    /api/guardians/{id}/students - Get guardian's students
```

### Student-Guardian Management

```
GET    /api/students/{id}/guardians                - Get student's guardians
POST   /api/students/{id}/guardians                - Add guardian to student
DELETE /api/students/{id}/guardians/{guardianId}  - Remove guardian
PUT    /api/students/{id}/guardians/{guardianId}  - Update relationship
```

## Common Issues & Solutions

### Issue: Migration fails with duplicate key error

**Cause**: Guardians with same email/phone already exist

**Solution**:
```sql
-- Find duplicates
SELECT email, phone, COUNT(*)
FROM users.guardians
GROUP BY email, phone
HAVING COUNT(*) > 1;

-- Manually merge duplicates before migration
```

### Issue: Students have no guardians after migration

**Cause**: Legacy `guardian_name` was empty or NULL

**Solution**: Manually add guardians via API or seed data

### Issue: Foreign key constraint violation

**Cause**: Migration 1.4.3 ran before 1.4.2 completed

**Solution**:
```bash
# Check migration order
go run main.go migrate status

# Rollback if needed
go run main.go migrate reset  # WARNING: Deletes all data!
go run main.go migrate
```

## Rollback Procedure

### If Migration Fails During 1.4.2 or 1.4.3

```bash
# Stop the application
docker compose down

# Restore from backup
psql project_phoenix < backup_before_guardian_migration.sql

# Restart application
docker compose up -d
```

### Partial Rollback

Migration 1.4.3 has a rollback function that:
1. Re-adds legacy columns to `users.students`
2. Renames `guardian_id` back to `guardian_account_id`
3. Recreates `users.persons_guardians` table
4. **Does NOT restore legacy data** (manual step required)

## Frontend Integration

### Required Frontend Changes

1. **Type Definitions** (`frontend/src/lib/guardian-helpers.ts`)
   - Guardian interface
   - StudentGuardian interface
   - Response mapping functions

2. **API Client** (`frontend/src/lib/guardian-api.ts`)
   - guardianService methods
   - Student-guardian relationship methods

3. **Next.js Routes** (`frontend/src/app/api/guardians/`)
   - GET/POST `/api/guardians`
   - GET/PUT/DELETE `/api/guardians/[id]`

4. **UI Components**
   - GuardianForm (create/edit)
   - GuardianList (display, search)
   - Update student detail page

### Frontend Testing Checklist

- [ ] Guardian list page displays correctly
- [ ] Search guardians works
- [ ] Create guardian form validates properly
- [ ] Edit guardian updates data
- [ ] Student detail page shows all guardians
- [ ] Add guardian to student works
- [ ] Remove guardian from student works
- [ ] Multiple guardians per student supported
- [ ] Primary guardian indicator displayed

## Performance Considerations

### Indexes

All critical columns are indexed:
- `users.guardians(email)` - UNIQUE for lookups
- `users.guardians(phone)` - For phone searches
- `users.guardians(last_name)` - For name sorting
- `users.guardians(active)` - For filtering
- `users.students_guardians(student_id)` - For student queries
- `users.students_guardians(guardian_id)` - For guardian queries

### Query Optimization

Use `FindByStudentIDWithGuardians` instead of separate queries:
```go
// ✅ GOOD - Single query with join
relationships := studentGuardianRepo.FindByStudentIDWithGuardians(ctx, studentID)

// ❌ BAD - N+1 queries
relationships := studentGuardianRepo.FindByStudentID(ctx, studentID)
for _, rel := range relationships {
    guardian := guardianRepo.FindByID(ctx, rel.GuardianID)
}
```

## Security Considerations

1. **Permission Requirements**:
   - `users:read` - View guardians
   - `users:create` - Create guardians
   - `users:update` - Update guardians
   - `users:delete` - Delete guardians

2. **Data Validation**:
   - Email format validated
   - Phone format validated (international support)
   - Required fields enforced
   - Duplicate prevention

3. **Cascade Deletion**:
   - Deleting guardian: Blocked if students exist
   - Deleting student: Cascades to relationships
   - Soft delete option via `active` flag

## Monitoring

### Health Checks

```sql
-- Orphaned relationships (should be 0)
SELECT COUNT(*) FROM users.students_guardians sg
LEFT JOIN users.guardians g ON g.id = sg.guardian_id
WHERE g.id IS NULL;

-- Students without guardians
SELECT COUNT(*) FROM users.students s
LEFT JOIN users.students_guardians sg ON sg.student_id = s.id
WHERE sg.id IS NULL;

-- Guardians without students
SELECT COUNT(*) FROM users.guardians g
LEFT JOIN users.students_guardians sg ON sg.guardian_id = g.id
WHERE sg.id IS NULL;
```

## Next Steps

1. ✅ Backend migrations complete
2. ⏳ Frontend implementation (in progress)
3. ⏳ Update seed data for testing
4. ⏳ End-to-end testing
5. ⏳ Production deployment

## Support

For issues or questions:
1. Check migration logs: `docker compose logs server`
2. Verify database state with SQL queries above
3. Review migration files: `backend/database/migrations/001004*`
4. Check API documentation: `go run main.go gendoc`
