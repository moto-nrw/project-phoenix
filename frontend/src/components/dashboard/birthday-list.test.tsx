import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";

import { BirthdayList } from "./birthday-list";
import type { BirthdayCelebration } from "~/lib/birthdays-api";

function child(
  overrides: Partial<BirthdayCelebration> = {},
): BirthdayCelebration {
  return {
    kind: "student",
    id: "1",
    name: "Lina Adler",
    groupName: "Delfine",
    schoolClass: "1a",
    date: "2026-08-03",
    age: 8,
    isToday: true,
    ...overrides,
  };
}

const staff: BirthdayCelebration = {
  kind: "staff",
  id: "7",
  name: "Anna Berg",
  date: "2026-08-03",
  isToday: true,
};

describe("BirthdayList", () => {
  it("renders names, not a bare count — congratulating someone needs the name", () => {
    render(<BirthdayList celebrations={[child()]} isLoading={false} />);

    expect(screen.getByText("Lina Adler")).toBeInTheDocument();
    // The class is printed verbatim: schools store it as "1a" or as
    // "Klasse 1a", and prefixing a label produced "Klasse Klasse 1a".
    expect(screen.getByText("Delfine · 1a · wird 8")).toBeInTheDocument();
    expect(screen.getByText("Heute")).toBeInTheDocument();
  });

  it("says so when nobody is celebrating instead of rendering an empty card", () => {
    render(<BirthdayList celebrations={[]} isLoading={false} />);

    expect(screen.getByText("Heute keine Geburtstage")).toBeInTheDocument();
  });

  // A Monday carries the weekend, and "Samstag" alone still leaves the reader
  // counting backwards — the badge has to name the date too.
  it("names weekday AND date for a birthday that was not today", () => {
    render(
      <BirthdayList
        celebrations={[
          child({
            id: "2",
            name: "Mika Klein",
            date: "2026-08-01",
            isToday: false,
          }),
        ]}
        isLoading={false}
      />,
    );

    expect(screen.getByText("Sa, 01.08.")).toBeInTheDocument();
    expect(screen.queryByText("Heute")).not.toBeInTheDocument();
  });

  // The badge answers "wann" and nothing else. Children and staff are told
  // apart by their section, so no pill ever carries two meanings.
  it("separates children and staff by section instead of by badge", () => {
    render(<BirthdayList celebrations={[child(), staff]} isLoading={false} />);

    expect(screen.getByText("Kinder")).toBeInTheDocument();
    expect(screen.getByText("Team")).toBeInTheDocument();
    // Both rows carry the same "when" badge, one per row.
    expect(screen.getAllByText("Heute")).toHaveLength(2);
  });

  it("omits the section headings when only children are celebrating", () => {
    render(<BirthdayList celebrations={[child()]} isLoading={false} />);

    expect(screen.queryByText("Kinder")).not.toBeInTheDocument();
    expect(screen.queryByText("Team")).not.toBeInTheDocument();
  });

  // Datenschutz: a colleague's row carries neither an age nor a group.
  it("shows a staff entry without age or group", () => {
    render(<BirthdayList celebrations={[staff]} isLoading={false} />);

    expect(screen.getByText("Anna Berg")).toBeInTheDocument();
    expect(screen.queryByText(/wird /)).not.toBeInTheDocument();
    expect(screen.queryByText(/Delfine/)).not.toBeInTheDocument();
  });

  it("renders placeholders while loading", () => {
    const { container } = render(
      <BirthdayList celebrations={[]} isLoading={true} />,
    );

    expect(container.querySelectorAll(".animate-pulse").length).toBeGreaterThan(
      0,
    );
    expect(
      screen.queryByText("Heute keine Geburtstage"),
    ).not.toBeInTheDocument();
  });
});
