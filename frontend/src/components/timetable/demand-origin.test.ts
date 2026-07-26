import { describe, expect, it } from "vitest";

import { resolveDemandOrigin } from "./demand-origin";
import type { Phase } from "~/lib/enrollment-phase-api";

function makePhase(overrides: Partial<Phase>): Phase {
  return {
    id: "1",
    name: "Phase",
    kind: "school_year",
    service_start_date: "2026-08-01",
    service_end_date: "2027-07-31",
    calendar_period_id: null,
    show_status_reason_to_parent: false,
    care_overflow_mode: "waitlist",
    care_offering_selection_mode: "optional",
    is_active: true,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("resolveDemandOrigin", () => {
  it("(a) names the single phase linked to the visible period", () => {
    const phases = [
      makePhase({ id: "1", name: "Anmeldung HJ1", calendar_period_id: "5" }),
      makePhase({ id: "2", name: "Anderes", calendar_period_id: "6" }),
    ];

    expect(resolveDemandOrigin(phases, "5", "2026-09-14")).toEqual({
      label: "Bedarf: Anmeldung HJ1",
    });
  });

  it("(b) falls back to the phase whose service window contains the date", () => {
    const phases = [
      makePhase({
        id: "1",
        name: "Schuljahr 2026/27",
        calendar_period_id: null,
        service_start_date: "2026-08-01",
        service_end_date: "2027-07-31",
      }),
      makePhase({
        id: "2",
        name: "Ferien",
        calendar_period_id: null,
        service_start_date: "2026-07-01",
        service_end_date: "2026-07-31",
      }),
    ];

    expect(resolveDemandOrigin(phases, "5", "2026-09-14")).toEqual({
      label: "Bedarf: Schuljahr 2026/27",
    });
  });

  it("(b) matches inclusively on both service window ends", () => {
    const phases = [
      makePhase({
        id: "1",
        name: "Rand",
        service_start_date: "2026-08-01",
        service_end_date: "2027-07-31",
      }),
    ];

    expect(resolveDemandOrigin(phases, null, "2026-08-01").label).toBe(
      "Bedarf: Rand",
    );
    expect(resolveDemandOrigin(phases, null, "2027-07-31").label).toBe(
      "Bedarf: Rand",
    );
  });

  it("(c) reports ambiguity when several phases link to the period", () => {
    const phases = [
      makePhase({ id: "1", name: "A", calendar_period_id: "5" }),
      makePhase({ id: "2", name: "B", calendar_period_id: "5" }),
    ];

    expect(resolveDemandOrigin(phases, "5", "2026-09-14")).toEqual({
      label: "Bedarf: nicht eindeutig zugeordnet",
      href: "/enrollment-phases",
    });
  });

  it("(c) reports ambiguity when the date-fallback is not unique", () => {
    const phases = [
      makePhase({
        id: "1",
        name: "A",
        calendar_period_id: null,
        service_start_date: "2026-08-01",
        service_end_date: "2027-07-31",
      }),
      makePhase({
        id: "2",
        name: "B",
        calendar_period_id: null,
        service_start_date: "2026-01-01",
        service_end_date: "2026-12-31",
      }),
    ];

    expect(resolveDemandOrigin(phases, null, "2026-09-14")).toEqual({
      label: "Bedarf: nicht eindeutig zugeordnet",
      href: "/enrollment-phases",
    });
  });

  it("(d) reports no linkage when nothing matches", () => {
    const phases = [
      makePhase({
        id: "1",
        name: "Vergangen",
        calendar_period_id: "9",
        service_start_date: "2025-08-01",
        service_end_date: "2026-07-31",
      }),
    ];

    expect(resolveDemandOrigin(phases, "5", "2026-09-14")).toEqual({
      label: "Bedarf: keine Anmeldung verknüpft",
      href: "/enrollment-phases",
    });
  });

  it("(d) reports no linkage for an empty phase list", () => {
    expect(resolveDemandOrigin([], "5", "2026-09-14")).toEqual({
      label: "Bedarf: keine Anmeldung verknüpft",
      href: "/enrollment-phases",
    });
  });

  it("prefers the direct period link over the date fallback", () => {
    const phases = [
      makePhase({
        id: "1",
        name: "Direkt",
        calendar_period_id: "5",
        // service window does NOT contain the date — irrelevant, direct wins
        service_start_date: "2020-01-01",
        service_end_date: "2020-12-31",
      }),
      makePhase({
        id: "2",
        name: "PerDatum",
        calendar_period_id: null,
        service_start_date: "2026-08-01",
        service_end_date: "2027-07-31",
      }),
    ];

    expect(resolveDemandOrigin(phases, "5", "2026-09-14").label).toBe(
      "Bedarf: Direkt",
    );
  });
});
