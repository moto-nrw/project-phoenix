# API Helpers Capability

## ADDED Requirements

### Requirement: Error Rendering Helper
The system SHALL provide a centralized error rendering helper to eliminate 341 duplicate error handling blocks across all API files.

#### Scenario: Standard error rendering
- **GIVEN** an API handler encounters an error
- **WHEN** the handler calls `renderError(w, r, errRenderer)`
- **THEN** the error SHALL be rendered as JSON with appropriate HTTP status
- **AND** SHALL log the error with context (request path, method, error details)
- **AND** SHALL handle render failures gracefully (fallback to http.Error)

#### Scenario: Consistent error logging
- **GIVEN** multiple API handlers use `renderError` helper
- **WHEN** errors occur in different handlers
- **THEN** all error logs SHALL have consistent format
- **AND** SHALL include request ID, path, method, and error message
- **AND** SHALL not leak sensitive information in responses

### Requirement: ID Parameter Parsing Helper
The system SHALL provide helpers to parse and validate ID parameters from URL paths, eliminating 86 duplicate ID parsing blocks.

#### Scenario: Valid ID extraction
- **GIVEN** an API route with `{id}` URL parameter
- **WHEN** handler calls `parseIDParam(r, "id")`
- **THEN** the helper SHALL extract the ID from URL
- **AND** SHALL parse it as int64
- **AND** SHALL return the parsed ID and nil error for valid IDs

#### Scenario: Invalid ID handling
- **GIVEN** a URL parameter contains non-numeric value
- **WHEN** handler calls `parseIDParam(r, "id")`
- **THEN** the helper SHALL return zero and a descriptive error
- **AND** error message SHALL indicate which parameter was invalid

#### Scenario: Required ID with auto-response
- **GIVEN** handler requires an ID parameter
- **WHEN** handler calls `requireIDParam(w, r, "id")`
- **THEN** helper SHALL parse the ID
- **AND** on success SHALL return ID and true
- **AND** on failure SHALL render error response and return 0, false
- **AND** handler can short-circuit on false return

### Requirement: Request Binding Helper
The system SHALL provide generic request binding helpers to eliminate 69 duplicate request binding blocks.

#### Scenario: Type-safe request binding
- **GIVEN** an API handler expects JSON request body of type T
- **WHEN** handler calls `bindRequest[T](r)`
- **THEN** helper SHALL decode JSON body into type T
- **AND** SHALL call the type's Bind validation method if it exists
- **AND** SHALL return pointer to T and nil error on success
- **AND** SHALL return nil and descriptive error on failure

#### Scenario: Validation error handling
- **GIVEN** request body fails validation in Bind method
- **WHEN** `bindRequest[T](r)` is called
- **THEN** helper SHALL return the validation error
- **AND** error message SHALL indicate which field(s) failed validation
- **AND** caller can render appropriate error response

### Requirement: Pagination Parsing Helper
The system SHALL provide pagination helpers for consistent list endpoint behavior, eliminating 12+ duplicate pagination parsing blocks.

#### Scenario: Default pagination values
- **GIVEN** a list endpoint receives request without pagination params
- **WHEN** handler calls `parsePagination(r)`
- **THEN** helper SHALL return default page=1, pageSize=50
- **AND** SHALL use constants DefaultPage and DefaultPageSize

#### Scenario: Custom pagination values
- **GIVEN** request includes `?page=3&page_size=25` query params
- **WHEN** handler calls `parsePagination(r)`
- **THEN** helper SHALL return page=3, pageSize=25

#### Scenario: Pagination limits enforcement
- **GIVEN** request includes `?page_size=500`
- **WHEN** handler calls `parsePagination(r)`
- **THEN** helper SHALL enforce MaxPageSize limit (100)
- **AND** SHALL return pageSize=100 instead of 500

#### Scenario: Invalid pagination handling
- **GIVEN** request includes `?page=-1` or `?page=0`
- **WHEN** handler calls `parsePagination(r)`
- **THEN** helper SHALL return page=1 (default)
- **AND** SHALL silently correct invalid values

### Requirement: Query Parameter Helpers
The system SHALL provide helpers for parsing optional query parameters with default values.

#### Scenario: Optional integer parameter
- **GIVEN** endpoint accepts optional integer query parameter
- **WHEN** handler calls `parseIntQuery(r, "limit", 50)`
- **THEN** helper SHALL return the provided value if present and valid
- **AND** SHALL return default value (50) if not present
- **AND** SHALL return default value if invalid

#### Scenario: Optional boolean parameter
- **GIVEN** endpoint accepts optional boolean flag
- **WHEN** handler calls `parseBoolQuery(r, "include_deleted", false)`
- **THEN** helper SHALL parse "true"/"false" string values
- **AND** SHALL return default value if not present

### Requirement: Middleware for Common Patterns
The system SHALL provide optional middleware for automatic parameter extraction.

#### Scenario: ID extraction middleware
- **GIVEN** route uses `WithIDParam` middleware
- **WHEN** request matches route with `{id}` parameter
- **THEN** middleware SHALL extract and validate ID
- **AND** SHALL store parsed ID in request context
- **AND** handler can retrieve ID from context without parsing

#### Scenario: Pagination middleware
- **GIVEN** route uses `WithPagination` middleware
- **WHEN** list endpoint is called
- **THEN** middleware SHALL parse pagination from query params
- **AND** SHALL store page and pageSize in request context
- **AND** handler can retrieve values from context

### Requirement: Helper Function Documentation
The API helpers SHALL be comprehensively documented for developer adoption.

#### Scenario: Usage examples provided
- **GIVEN** developer reviews helper functions
- **WHEN** reading function documentation
- **THEN** each helper SHALL have clear docstring with:
  - Purpose and use case
  - Parameter descriptions
  - Return value descriptions
  - Usage example code
- **AND** README SHALL include before/after migration examples

#### Scenario: Migration guide available
- **GIVEN** developer needs to adopt helpers in existing API
- **WHEN** reviewing `pkg/api/README.md`
- **THEN** guide SHALL show step-by-step migration process
- **AND** SHALL include common patterns and anti-patterns
- **AND** SHALL reference specific code examples in codebase

## Migration Notes
- Helpers are optional initially - existing code continues working
- Gradual adoption recommended (migrate high-duplication files first)
- Middleware optional - helpers can be called directly
- All helpers maintain backward compatible behavior with existing patterns
