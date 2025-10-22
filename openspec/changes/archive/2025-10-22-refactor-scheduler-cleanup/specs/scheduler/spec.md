## ADDED Requirements
### Requirement: Typed Cleanup Job Registry
Scheduler cleanup orchestration MUST register cleanup jobs via typed interfaces and a shared job list instead of using reflection.

#### Scenario: Compile-Time Conformance
- **GIVEN** the scheduler is built with auth and invitation services
- **WHEN** a cleanup method is renamed or its signature changes
- **THEN** the build MUST fail because the service no longer satisfies the typed cleanup interface.

#### Scenario: Reusable Cleanup Jobs
- **GIVEN** another component needs to trigger the same cleanup routines
- **WHEN** it consumes the scheduler's cleanup job list
- **THEN** it MUST be able to execute those jobs directly without using reflection helpers.
