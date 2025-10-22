# iot-feedback Specification

## Purpose
TBD - created by archiving change fix-feedback-error-handling. Update Purpose after archive.
## Requirements
### Requirement: Type-Safe Error Detection
The feedback endpoint MUST use type-safe error detection instead of string matching to identify database errors.

#### Scenario: Student not found
- **WHEN** a feedback submission references a non-existent student ID
- **THEN** the system uses `errors.Is(err, sql.ErrNoRows)` to detect the condition
- **AND** returns a 404 Not Found error with message "student not found"

#### Scenario: Database error
- **WHEN** a database error occurs during student lookup
- **THEN** the system uses `errors.Is()` to distinguish between "not found" and other database errors
- **AND** returns a 500 Internal Server Error for non-"not found" errors

