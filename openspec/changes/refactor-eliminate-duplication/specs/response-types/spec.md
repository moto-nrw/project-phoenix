# Response Types Capability

## ADDED Requirements

### Requirement: Canonical Response Type Library
The system SHALL provide a shared response type library in `api/common/responses/` to consolidate 69 duplicate response type definitions into 15 canonical types.

#### Scenario: Student response consolidation
- **GIVEN** StudentResponse exists in 3 different versions (21, 16, and 4 fields)
- **WHEN** response library is implemented
- **THEN** SHALL provide `StudentDetail` type with all 21 fields
- **AND** SHALL provide `StudentSummary` type with essential 4 fields (id, firstName, lastName, groupId)
- **AND** all APIs SHALL use appropriate type based on context

#### Scenario: Teacher response consolidation
- **GIVEN** TeacherResponse exists in 2 different versions
- **WHEN** response library is implemented
- **THEN** SHALL provide single canonical `TeacherResponse` type
- **AND** SHALL include all fields from both versions
- **AND** SHALL support optional fields via pointers

#### Scenario: Response type hierarchy
- **GIVEN** entities have both summary and detail representations
- **WHEN** defining response types
- **THEN** SHALL follow naming convention: `EntitySummary` for list views, `EntityDetail` for single views
- **AND** Summary types SHALL be subsets of Detail types
- **AND** Summary types SHALL omit expensive-to-compute or rarely-needed fields

### Requirement: Generic Response Mapper
The system SHALL provide a reflection-based mapper to automate model-to-response transformations, eliminating 1,826 manual nil checks.

#### Scenario: Automatic struct mapping
- **GIVEN** domain model and response type with matching field names
- **WHEN** calling `mapper.Map(model, &response)`
- **THEN** mapper SHALL copy all matching fields automatically
- **AND** SHALL handle type conversions (int64 → string, time.Time → common.Time)
- **AND** SHALL not require manual field-by-field assignment

#### Scenario: Nil pointer handling
- **GIVEN** response type has optional fields (pointer types)
- **WHEN** mapping from model with nil pointer field
- **THEN** mapper SHALL set response field to nil
- **AND** SHALL not panic on nil dereference
- **AND** WHEN model field is non-nil
- **THEN** mapper SHALL dereference and copy value

#### Scenario: Nested struct mapping
- **GIVEN** response type contains nested structs (e.g., Student with Group)
- **WHEN** mapping model with nested relationships
- **THEN** mapper SHALL recursively map nested structs
- **AND** SHALL handle nil nested pointers gracefully
- **AND** SHALL support slice of nested structs

#### Scenario: Custom mapping rules via struct tags
- **GIVEN** field names differ between model and response
- **WHEN** response struct uses `map:"model_field_name"` tag
- **THEN** mapper SHALL use tag to match fields
- **AND** SHALL support `omitempty` tag to skip nil values
- **AND** SHALL support custom transformers via `transform:"funcName"` tag

### Requirement: Slice Mapping Helpers
The system SHALL provide helpers to map slices of models to slices of responses, eliminating repetitive loop code.

#### Scenario: Batch response building
- **GIVEN** handler has slice of domain models
- **WHEN** calling `mapper.MapSlice[Model, Response](models)`
- **THEN** helper SHALL return slice of responses
- **AND** SHALL apply mapper.Map to each element
- **AND** SHALL handle empty slices (return empty slice, not nil)

#### Scenario: Custom mapper function
- **GIVEN** response requires complex transformation logic
- **WHEN** calling `buildListResponse(items, customMapperFunc)`
- **THEN** helper SHALL apply custom function to each item
- **AND** SHALL collect results into slice
- **AND** SHALL maintain order of input slice

### Requirement: Pagination Response Wrapper
The system SHALL provide helpers to wrap paginated responses with metadata.

#### Scenario: Paginated list response
- **GIVEN** handler has paginated data
- **WHEN** calling `buildPaginatedResponse(items, page, pageSize, total)`
- **THEN** helper SHALL wrap data in paginated structure
- **AND** SHALL include pagination metadata (current_page, page_size, total_pages, total_records)
- **AND** SHALL calculate total_pages from total and page_size

#### Scenario: Consistent pagination structure
- **GIVEN** multiple list endpoints
- **WHEN** returning paginated responses
- **THEN** all endpoints SHALL use identical JSON structure:
  ```json
  {
    "status": "success",
    "data": [...],
    "pagination": {
      "current_page": 1,
      "page_size": 50,
      "total_pages": 3,
      "total_records": 142
    }
  }
  ```

### Requirement: Response Type Field Consistency
Shared response types SHALL maintain consistent field naming and typing across the API.

#### Scenario: ID field consistency
- **GIVEN** all response types include entity ID
- **WHEN** defining response struct
- **THEN** ID field SHALL be named `ID` (not `Id` or `EntityID`)
- **AND** SHALL use type `int64` (not string or int)
- **AND** SHALL have JSON tag `json:"id"`

#### Scenario: Timestamp field consistency
- **GIVEN** response types include timestamps
- **WHEN** defining timestamp fields
- **THEN** SHALL use `common.Time` type (not time.Time)
- **AND** SHALL use JSON tags `created_at` and `updated_at`
- **AND** SHALL format as RFC3339 in JSON

#### Scenario: Optional field consistency
- **GIVEN** response field is optional/nullable
- **WHEN** defining in struct
- **THEN** SHALL use pointer type (*string, *int64, etc.)
- **AND** SHALL include `omitempty` in JSON tag
- **AND** nil values SHALL be omitted from JSON output

### Requirement: Response Builder Migration
Existing API handlers SHALL be migrated to use shared response types incrementally.

#### Scenario: Student API migration
- **GIVEN** `api/students/api.go` uses custom StudentResponse
- **WHEN** migrating to shared types
- **THEN** SHALL import `"github.com/moto-nrw/project-phoenix/api/common/responses"`
- **AND** SHALL replace local type with `responses.StudentDetail`
- **AND** SHALL use mapper instead of manual field assignment
- **AND** SHALL verify JSON output structure unchanged

#### Scenario: Groups API migration
- **GIVEN** `api/groups/api.go` uses inline StudentResponse for list views
- **WHEN** migrating to shared types
- **THEN** SHALL use `responses.StudentSummary` for list endpoints
- **AND** SHALL use `responses.StudentDetail` for individual student in group context if needed

### Requirement: Response Type Documentation
The response type library SHALL be documented with clear usage guidelines.

#### Scenario: Type selection guide
- **GIVEN** developer needs to choose response type
- **WHEN** reviewing `api/common/responses/README.md`
- **THEN** documentation SHALL provide decision tree:
  - List endpoint → Use Summary type
  - Detail endpoint → Use Detail type
  - Nested entity → Use Summary type unless details required
- **AND** SHALL include examples of each usage pattern

#### Scenario: Mapper usage examples
- **GIVEN** developer wants to use response mapper
- **WHEN** reviewing mapper documentation
- **THEN** SHALL include examples of:
  - Simple mapping
  - Slice mapping
  - Nested struct mapping
  - Custom transformer usage
- **AND** SHALL document performance characteristics

## Migration Notes
- Shared types are additive - old response types can coexist during migration
- APIs migrated incrementally (one at a time) to reduce risk
- JSON output structure maintained for backward compatibility
- Mapper performance acceptable (<1ms per response) based on benchmarks
- Custom mappers still allowed for complex transformations not suited to reflection
