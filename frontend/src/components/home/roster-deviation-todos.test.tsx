import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { RosterDeviation } from "~/lib/roster-deviation";

const state = vi.hoisted(() => ({
  deviations: [] as unknown[],
  keys: [] as Array<string | null>,
}));

vi.mock("~/lib/timetable-api", () => ({ timetableService: {} }));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => {
    state.keys.push(key);
    if (key === null) return {};
    if (key.startsWith("roster-deviations-")) return { data: state.deviations };
    return {
      data: [
        {
          id: "4",
          name: "Schuljahr",
          isActive: true,
          startDate: "2026-08-01",
          endDate: "2027-07-31",
        },
      ],
    };
  },
}));

vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => `/test-tenant${path}`,
}));

import { RosterDeviationTodos } from "./roster-deviation-todos";

function deviation(templateId: string): RosterDeviation {
  return {
    templateId,
    offerings: [{ id: "3", name: "Randstunde" }],
    gradeLevels: [],
    schoolClasses: [],
    unbookedCount: 4,
    missingCount: 0,
    candidates: [],
    combinations: [],
    ambiguous: false,
  };
}

describe("RosterDeviationTodos (#5103)", () => {
  beforeEach(() => {
    state.deviations = [];
    state.keys = [];
  });

  it("names how many Regeltermine to check and links to „Zu prüfen“", () => {
    state.deviations = [deviation("1"), deviation("2")];
    render(<RosterDeviationTodos enabled />);

    expect(state.keys).toContain("roster-deviations-4");
    expect(
      screen.getByText(
        "2 Regeltermine weichen von den Anmeldungen ab. Prüfen Sie die Kinderlisten.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Regeltermine prüfen" }),
    ).toHaveAttribute(
      "href",
      "/test-tenant/betreuungsplan?view=serien&pruefen=1",
    );
  });

  it("stays away without deviations or without planning rights", () => {
    const { container, rerender } = render(<RosterDeviationTodos enabled />);
    expect(container).toBeEmptyDOMElement();

    state.deviations = [deviation("1")];
    rerender(<RosterDeviationTodos enabled={false} />);
    expect(container).toBeEmptyDOMElement();
  });
});
