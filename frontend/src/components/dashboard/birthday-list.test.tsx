import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";

import { BirthdayList } from "./birthday-list";
import type { BirthdayCelebration } from "~/lib/birthdays-api";

// Die Testuhr steht auf Mittwoch, 09.09.2026.
const TODAY = "2026-09-09";

function child(
  overrides: Partial<BirthdayCelebration> = {},
): BirthdayCelebration {
  return {
    kind: "student",
    id: "1",
    name: "Lina Adler",
    groupName: "Delfine",
    schoolClass: "1a",
    date: TODAY,
    age: 8,
    isToday: true,
    ...overrides,
  };
}

const staff: BirthdayCelebration = {
  kind: "staff",
  id: "7",
  name: "Anna Berg",
  date: TODAY,
  isToday: true,
};

function renderList(celebrations: BirthdayCelebration[]) {
  return render(
    <BirthdayList
      celebrations={celebrations}
      today={TODAY}
      isLoading={false}
      emptyTitle="Keine Geburtstage in dieser Woche"
    />,
  );
}

describe("BirthdayList", () => {
  it("renders names, not a bare count — congratulating someone needs the name", () => {
    renderList([child()]);

    expect(screen.getByText("Lina Adler")).toBeInTheDocument();
    // The class is printed verbatim: schools store it as "1a" or as
    // "Klasse 1a", and prefixing a label produced "Klasse Klasse 1a".
    expect(screen.getByText("Delfine · 1a · wird 8")).toBeInTheDocument();
    expect(screen.getByText("Heute")).toBeInTheDocument();
  });

  it("says so when nobody is celebrating instead of rendering an empty card", () => {
    renderList([]);

    expect(
      screen.getByText("Keine Geburtstage in dieser Woche"),
    ).toBeInTheDocument();
  });

  // "Montag" alone still leaves the reader counting — the day names the date.
  it("names weekday AND date for a day that is not today", () => {
    renderList([
      child({
        id: "2",
        name: "Mika Klein",
        date: "2026-09-07",
        isToday: false,
      }),
    ]);

    expect(screen.getByText("Mo, 07.09.")).toBeInTheDocument();
    expect(screen.queryByText("Heute")).not.toBeInTheDocument();
  });

  it("groups the week by day and marks only today", () => {
    renderList([
      child({
        id: "2",
        name: "Mika Klein",
        date: "2026-09-07",
        isToday: false,
      }),
      child(),
      child({
        id: "3",
        name: "Emil Braun",
        date: "2026-09-12",
        isToday: false,
      }),
      child({ id: "4", name: "Ida Stern", date: "2026-09-12", isToday: false }),
    ]);

    expect(screen.getAllByText("Heute")).toHaveLength(1);
    // Two children on Saturday share one day label.
    expect(screen.getAllByText("Sa, 12.09.")).toHaveLength(1);
    const names = screen
      .getAllByText(/Adler|Klein|Braun|Stern/)
      .map((node) => node.textContent);
    expect(names).toEqual([
      "Mika Klein",
      "Lina Adler",
      "Emil Braun",
      "Ida Stern",
    ]);
  });

  // The OGS celebrates afterwards: a birthday earlier in the week is over.
  it("says the age was reached for an earlier day", () => {
    renderList([
      child({ id: "2", date: "2026-09-07", isToday: false, age: 7 }),
    ]);

    expect(screen.getByText("Delfine · 1a · wurde 7")).toBeInTheDocument();
  });

  // Datenschutz: a colleague's row carries neither an age nor a group.
  it("marks a staff entry as Team, without age or group", () => {
    renderList([child(), staff]);

    expect(screen.getByText("Anna Berg")).toBeInTheDocument();
    expect(screen.getByText("Team")).toBeInTheDocument();
    expect(screen.getAllByText(/wird /)).toHaveLength(1);
  });

  it("renders placeholders while loading", () => {
    const { container } = render(
      <BirthdayList
        celebrations={[]}
        today={TODAY}
        isLoading={true}
        emptyTitle="Keine Geburtstage in dieser Woche"
      />,
    );

    expect(container.querySelectorAll(".animate-pulse").length).toBeGreaterThan(
      0,
    );
    expect(
      screen.queryByText("Keine Geburtstage in dieser Woche"),
    ).not.toBeInTheDocument();
  });
});
