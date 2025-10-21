## ADDED Requirements

### Requirement: Unified Student Location Badge Helper
- The frontend MUST expose a single `getLocationBadge` (or equivalent ModernStatusBadge wrapper) that converts `student.current_location` plus optional room metadata into a standardized badge contract (label, background token, gradient token).
- The helper MUST interpret the following states: group room, other room, Schulhof, WC, Bus, Unterwegs, Zuhause, Unbekannt, `Anwesend - <Room>`, and `Anwesend in <Room>` prefixes.
- Styling tokens MUST be sourced from a centralized map so labels and gradients remain consistent across surfaces.

#### Scenario: Parse detailed present state with room metadata
- **GIVEN** a student whose `current_location` equals `"Anwesend"`
- **AND** room status indicates they are in their assigned group room named "Grüner Raum"
- **WHEN** the helper is invoked
- **THEN** it MUST return label "Grüner Raum"
- **AND** it MUST reference the green "present" token set shared by all surfaces.

#### Scenario: Fallback to suffix parsing when room metadata missing
- **GIVEN** a student whose `current_location` equals `"Anwesend - Werkraum"`
- **AND** no room-status payload is provided
- **WHEN** the helper is invoked
- **THEN** it MUST return label "Werkraum"
- **AND** it MUST mark the badge as "other room" styling.

#### Scenario: Parse "Anwesend in" prefix
- **GIVEN** a student whose `current_location` equals `"Anwesend in Bibliothek"`
- **AND** no additional room metadata is provided
- **WHEN** the helper is invoked
- **THEN** it MUST return label "Bibliothek"
- **AND** it MUST use the blue "other room" token set.

#### Scenario: Handle non-present states from raw string
- **GIVEN** a student whose `current_location` equals `"Bus"`
- **WHEN** the helper is invoked
- **THEN** it MUST return label "Bus"
- **AND** it MUST map to the purple "Unterwegs" token set.

#### Scenario: Handle direct "Unterwegs" status
- **GIVEN** a student whose `current_location` equals `"Unterwegs"`
- **WHEN** the helper is invoked
- **THEN** it MUST return label "Unterwegs"
- **AND** it MUST map to the purple "Unterwegs" token set.

#### Scenario: Handle "Zuhause" status
- **GIVEN** a student whose `current_location` equals `"Zuhause"`
- **WHEN** the helper is invoked
- **THEN** it MUST return label "Zuhause"
- **AND** it MUST map to the red "home" token set.

#### Scenario: Handle "WC" status
- **GIVEN** a student whose `current_location` equals `"WC"`
- **WHEN** the helper is invoked
- **THEN** it MUST return label "WC"
- **AND** it MUST map to the blue "WC" token set.

#### Scenario: Handle "School Yard" translation
- **GIVEN** a student whose `current_location` equals `"School Yard"`
- **WHEN** the helper is invoked
- **THEN** it MUST return label "Schulhof"
- **AND** it MUST map to the orange "school yard" token set.

#### Scenario: Handle unknown fallback
- **GIVEN** a student whose `current_location` equals `"SomethingElse"`
- **WHEN** the helper is invoked
- **THEN** it MUST return label "Unbekannt"
- **AND** it MUST map to the neutral "unknown" token set.

#### Scenario: Handle empty or missing current_location
- **GIVEN** a student whose `current_location` is `null` (or an empty string)
- **WHEN** the helper is invoked
- **THEN** it MUST return label "Unbekannt"
- **AND** it MUST map to the neutral "unknown" token set.

#### Scenario: Prefer room metadata over generic present label
- **GIVEN** a student whose `current_location` equals `"Anwesend"`
- **AND** room status provides `current_room_id = 42` resolved via the room map to "Kreativraum"
- **WHEN** the helper is invoked
- **THEN** it MUST return label "Kreativraum"
- **AND** it MUST map to the blue "other room" token set (indicating foreign room).

### Requirement: Surfaces Consume Unified Helper
- When the `NEXT_PUBLIC_ENABLE_UNIFIED_LOCATION_BADGE` flag is enabled, the OGS groups page, student search page, My Room, and student detail modal MUST render location badges using the shared helper/component.
- Under the enabled flag, each surface MUST display the same label and styling for identical student state.
- My Room and the student detail modal MUST display a location badge if data is available.

#### Scenario: My Room renders foreign-room badge
- **GIVEN** a student card in My Room whose visit data includes `current_room_id` 17 mapped to "Musikraum"
- **WHEN** the page renders
- **THEN** the badge MUST display "Musikraum"
- **AND** it MUST use the blue "other room" token set.

#### Scenario: Student detail modal shows real-time status
- **GIVEN** the modal loads a student whose `/current-location` endpoint reports status `"School Yard"`
- **WHEN** the modal header renders
- **THEN** it MUST show a badge labeled "Schulhof"
- **AND** it MUST use the orange token set aligned with other surfaces.

#### Scenario: Student detail modal refreshes every 30 seconds
- **GIVEN** the feature flag is enabled
- **AND** the student detail modal remains open for longer than 30 seconds
- **WHEN** more than 30 seconds elapse since the last `/current-location` fetch
- **THEN** the modal MUST trigger a refresh call to `/current-location` to update the badge state.

#### Scenario: OGS and search surfaces stay in sync
- **GIVEN** a student appears in both the OGS group list and the student search results
- **AND** the student's `current_location` equals `"Anwesend - Werkraum"`
- **WHEN** both surfaces are rendered after the helper integration
- **THEN** each surface MUST display label "Werkraum"
- **AND** both MUST use the blue "other room" token set.

#### Scenario: Feature flag disabled reverts to legacy behaviour
- **GIVEN** the runtime flag `NEXT_PUBLIC_ENABLE_UNIFIED_LOCATION_BADGE` is set to `false`
- **WHEN** the application renders the OGS group view
- **THEN** it MUST continue to use the legacy badge logic (pre-helper) so rollout can be safely toggled.
- **AND** when the student detail modal is opened, it MUST also fall back to its legacy state (no unified badge) while the flag remains disabled.

#### Scenario: Feature flag disabled keeps search/My Room on legacy behaviour
- **GIVEN** the runtime flag `NEXT_PUBLIC_ENABLE_UNIFIED_LOCATION_BADGE` is set to `false`
- **WHEN** the application renders the student search page and the My Room view
- **THEN** both MUST continue to use their existing (pre-helper) badge or label behavior while the flag remains disabled, ensuring the shared helper is gated behind the rollout flag for all surfaces.

### Requirement: Remove Legacy Boolean Dependencies
- Frontend mapping MUST stop populating deprecated boolean flags (`in_house`, `school_yard`, `wc`) except where required for backward compatibility tooling.
- UI logic MUST derive state from `current_location` (and room metadata) instead of those booleans.
- Automated tests MUST cover badge output for each supported state.

#### Scenario: Mapper no longer sets school yard boolean
- **GIVEN** the frontend maps a backend student payload
- **WHEN** `backendStudent.location` equals `"School Yard"`
- **THEN** the resulting student object MUST reflect `current_location = "School Yard"`
- **AND** it MUST NOT set `school_yard = true`.

#### Scenario: Mapper no longer sets in-house or WC booleans
- **GIVEN** the frontend maps a backend student payload
- **WHEN** `backendStudent.location` equals `"Anwesend"`
- **THEN** the resulting student object MUST reflect `current_location = "Anwesend"`
- **AND** it MUST NOT set `in_house = true` or `wc = true`.

#### Scenario: Tests enforce badge outputs
- **GIVEN** the badge helper/component test suite runs
- **WHEN** it evaluates the canonical state `"Zuhause"`
- **THEN** the test MUST assert the helper returns label "Zuhause" and references the red "home" token set.
- **AND** additional test cases MUST cover group room, other room, Schulhof, WC, Bus, Unterwegs, and Unbekannt labels with matching tokens.
