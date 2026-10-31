import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import type { SchoolVacations } from "~/lib/school-vacations-api";

const state = vi.hoisted(() => ({
  data: undefined as unknown,
  keys: [] as Array<string | null>,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => {
    state.keys.push(key);
    return key === null
      ? { isLoading: false, mutate: vi.fn() }
      : { data: state.data, isLoading: false, mutate: vi.fn() };
  },
}));

vi.mock("~/lib/school-vacations-api", () => ({ getSchoolVacations: vi.fn() }));

vi.mock("~/lib/hooks/use-swr-load-error", () => ({
  useSwrLoadError: () => null,
}));

import { SchoolVacationsList } from "./school-vacations-list";

const schoolYear = {
  id: "4",
  name: "Schuljahr 2026/2027",
  periodType: "school_year",
  isActive: true,
  startDate: "2026-08-01",
  endDate: "2027-07-31",
} as CalendarPeriod;

function vacations(overrides: Partial<SchoolVacations> = {}): SchoolVacations {
  return {
    federalState: "DE-NW",
    federalStateName: "Nordrhein-Westfalen",
    syncedAt: "2026-09-09T03:00:00Z",
    vacations: [
      {
        name: "Herbstferien",
        startDate: "2026-10-17",
        endDate: "2026-10-31",
        acknowledged: false,
      },
      {
        name: "Weihnachtsferien",
        startDate: "2026-12-23",
        endDate: "2027-01-06",
        acknowledged: true,
      },
    ],
    ...overrides,
  };
}

describe("SchoolVacationsList (#5345)", () => {
  beforeEach(() => {
    state.data = undefined;
    state.keys = [];
  });

  it("lists the vacations of the school year, read-only, with source and who changes the state", () => {
    state.data = vacations();
    render(
      <SchoolVacationsList periods={[schoolYear]} periodsLoading={false} />,
    );

    expect(state.keys).toContain("school-vacations-2026-08-01-2027-07-31");
    expect(
      screen.getByRole("heading", { name: "Schulferien" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Nur zur Information: die Schulferien in Nordrhein-Westfalen für „Schuljahr 2026/2027“. moto holt sie jeden Tag aus dem öffentlichen Ferienkalender OpenHolidays. Ob die OGS schließt, tragen Sie als Schließtag ein. Stimmt das Bundesland nicht, wenden Sie sich an das moto-Team.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("17.10.2026 bis 31.10.2026")).toBeInTheDocument();
    expect(screen.getByText("Weihnachtsferien")).toBeInTheDocument();
    expect(screen.getByText("Betreuung geplant")).toBeInTheDocument();
  });

  it("says honestly when moto has not read the vacations yet", () => {
    state.data = vacations({ syncedAt: null, vacations: [] });
    render(
      <SchoolVacationsList periods={[schoolYear]} periodsLoading={false} />,
    );

    expect(screen.getByText("Noch keine Schulferien")).toBeInTheDocument();
    expect(
      screen.getByText(
        "moto hat die Ferien für Ihr Bundesland noch nicht geholt. Das passiert einmal am Tag. Schauen Sie später noch einmal nach.",
      ),
    ).toBeInTheDocument();
  });

  it("tells an empty school year apart from a vacation calendar not read yet", () => {
    state.data = vacations({ vacations: [] });
    render(
      <SchoolVacationsList periods={[schoolYear]} periodsLoading={false} />,
    );

    expect(screen.getByText("Keine Schulferien")).toBeInTheDocument();
    expect(
      screen.queryByText("Noch keine Schulferien"),
    ).not.toBeInTheDocument();
  });

  it("waits for the periods before it asks for the vacations", () => {
    render(<SchoolVacationsList periods={[]} periodsLoading />);

    // Anker: die Karte steht, nur die Abfrage wartet.
    expect(
      screen.getByRole("heading", { name: "Schulferien" }),
    ).toBeInTheDocument();
    expect(state.keys).toEqual([null]);
  });
});
