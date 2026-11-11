import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import type { HolidayCareDay } from "~/lib/holiday-care";
import type { StaffScheduleStaff } from "~/lib/shift-helpers";

const mocks = vi.hoisted(() => ({ useMediaQuery: vi.fn(() => false) }));

vi.mock("~/lib/hooks/use-media-query", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/hooks/use-media-query")>()),
  useMediaQuery: mocks.useMediaQuery,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn() }),
}));

vi.mock("~/lib/swr", () => ({
  useTenantMutateMatching: () => vi.fn(),
}));

import { DienstplanPersonWeekGrid } from "./dienstplan-person-week-grid";
import { DienstplanResourceGrid } from "./dienstplan-resource-grid";

/**
 * Ferienbetreuung für den Träger (ADR 0116) im Dienstplan: der Kopf jedes
 * Tages nennt sie mit Kindern und Betreuungszeit, damit das Team die Woche
 * besetzen kann. Ein Schließtag schlägt sie.
 */

const member: StaffScheduleStaff = {
  id: "7",
  firstName: "Ada",
  lastName: "Lovelace",
};

const WEEK_DAYS = [
  "2027-01-04",
  "2027-01-05",
  "2027-01-06",
  "2027-01-07",
  "2027-01-08",
];

const CARE: HolidayCareDay = {
  weekId: "3",
  children: 0,
  startsAt: "07:45",
  endsAt: "13:00",
};
const HOLIDAY_CARE = new Map<string, HolidayCareDay>(
  ["2027-01-04", "2027-01-05", "2027-01-07", "2027-01-08"].map((iso) => [
    iso,
    CARE,
  ]),
);
const CLOSING = new Map([["2027-01-06", "Pädagogischer Tag"]]);

describe("DienstplanResourceGrid holiday care", () => {
  beforeEach(() => mocks.useMediaQuery.mockReturnValue(false));

  function renderGrid(holidayCareHref?: string) {
    render(
      <DienstplanResourceGrid
        staff={[member]}
        shiftsByStaff={new Map()}
        assignmentsByStaff={new Map()}
        summaryByStaff={new Map()}
        weekDays={WEEK_DAYS}
        todayIso="2027-01-04"
        closingDays={CLOSING}
        holidayCare={HOLIDAY_CARE}
        holidayCareHref={holidayCareHref}
        typesById={new Map()}
        shiftTypes={[]}
        onCellClick={vi.fn()}
      />,
    );
  }

  it("names the holiday care in the day head, also a week without confirmed children", () => {
    renderGrid();

    const monday = screen.getByRole("columnheader", { name: /Mo 04\.01\./ });
    expect(within(monday).getByText("Ferienbetreuung")).toBeInTheDocument();
    expect(
      within(monday).getByText("0 Kinder · 07:45–13:00"),
    ).toBeInTheDocument();
    expect(
      within(monday).getByText(
        "Ferienbetreuung für den Träger: noch kein Kind bestätigt, 07:45 bis 13:00 Uhr.",
      ),
    ).toHaveClass("sr-only");
    expect(within(monday).queryByRole("link")).not.toBeInTheDocument();
  });

  it("shows only the closing day on a closing day of the week", () => {
    renderGrid();

    const wednesday = screen.getByRole("columnheader", { name: /Mi 06\.01\./ });
    expect(
      within(wednesday).getByTitle("Schließtag: Pädagogischer Tag"),
    ).toBeInTheDocument();
    expect(
      within(wednesday).queryByText("Ferienbetreuung"),
    ).not.toBeInTheDocument();
  });

  it("links to the holiday weeks when the person may open them", () => {
    renderGrid("/ferienwochen");

    const monday = screen.getByRole("columnheader", { name: /Mo 04\.01\./ });
    const links = within(monday).getAllByRole("link", {
      name: "Ferienbetreuung · 0 Kinder · 07:45–13:00",
    });
    expect(links.length).toBeGreaterThan(0);
    expect(links[0]).toHaveAttribute("href", "/ferienwochen");
  });
});

describe("DienstplanResourceGrid holiday care, compact day strip", () => {
  it("shows the symbol without a link: each day of the strip is a button", () => {
    mocks.useMediaQuery.mockReturnValue(true);
    render(
      <DienstplanResourceGrid
        staff={[member]}
        shiftsByStaff={new Map()}
        assignmentsByStaff={new Map()}
        summaryByStaff={new Map()}
        weekDays={WEEK_DAYS}
        todayIso="2027-01-04"
        closingDays={CLOSING}
        holidayCare={HOLIDAY_CARE}
        holidayCareHref="/ferienwochen"
        typesById={new Map()}
        shiftTypes={[]}
        onCellClick={vi.fn()}
      />,
    );

    const sentences = screen.getAllByText(
      "Ferienbetreuung für den Träger: noch kein Kind bestätigt, 07:45 bis 13:00 Uhr.",
    );
    expect(sentences).toHaveLength(4);
    expect(sentences[0]?.closest("button")).not.toBeNull();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});

describe("DienstplanPersonWeekGrid holiday care", () => {
  it("names the holiday care in the day heads", () => {
    render(
      <DienstplanPersonWeekGrid
        member={member}
        shiftsByDate={new Map()}
        weekDays={WEEK_DAYS}
        todayIso="2027-01-04"
        closingDays={CLOSING}
        holidayCare={HOLIDAY_CARE}
        typesById={new Map()}
        shiftTypes={[]}
        onCreate={vi.fn()}
        onEdit={vi.fn()}
      />,
    );

    // Je Tag die volle Fassung; die schmale unter `sm` zeigt nur das Symbol.
    expect(screen.getAllByText("0 Kinder · 07:45–13:00")).toHaveLength(4);
    expect(
      screen.getByTitle("Schließtag: Pädagogischer Tag"),
    ).toBeInTheDocument();
  });
});
