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

  // A Monday carries the weekend, so the card must not imply Saturday's
  // birthday is happening right now.
  it("labels a weekend birthday with its weekday", () => {
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

    expect(screen.getByText("Samstag")).toBeInTheDocument();
    expect(screen.queryByText("Heute")).not.toBeInTheDocument();
  });

  // Datenschutz: a colleague's entry carries no age and no group.
  it("shows a staff entry without age or group", () => {
    render(
      <BirthdayList
        celebrations={[
          {
            kind: "staff",
            id: "7",
            name: "Anna Berg",
            date: "2026-08-03",
            isToday: true,
          },
        ]}
        isLoading={false}
      />,
    );

    expect(screen.getByText("Anna Berg")).toBeInTheDocument();
    expect(screen.getByText("Mitarbeitende Person")).toBeInTheDocument();
    expect(screen.getByText("Team")).toBeInTheDocument();
    expect(screen.queryByText(/wird /)).not.toBeInTheDocument();
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
