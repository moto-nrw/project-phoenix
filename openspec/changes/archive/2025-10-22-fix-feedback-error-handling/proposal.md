# Fix Feedback Endpoint Error Handling

## Why

The POST /api/iot/feedback endpoint uses fragile string matching to detect "not found" errors (`strings.Contains(err.Error(), "sql: no rows in result set")`) and has redundant nil checks. This creates maintenance risk if error messages change in future Go/SQL driver versions.

## What Changes

- Replace string matching with type-safe error checking using `errors.Is(err, sql.ErrNoRows)`
- Remove redundant `student == nil` check (when err is non-nil after first check, student must be nil)
- Add required import: `database/sql`

## Impact

- **Affected code**: `backend/api/iot/api.go:1537-1551` (POST /api/iot/feedback handler)
- **Breaking changes**: None - behavior remains identical
- **Testing**: Existing Bruno API tests cover all error paths
- **Performance**: No change
