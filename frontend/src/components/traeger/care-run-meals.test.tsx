import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CareRunMealsCard, MealPositionLine } from "./care-run-meals";

describe("MealPositionLine", () => {
  it("writes a Mensa month as MK-001 does", () => {
    render(
      <ul>
        <MealPositionLine
          position={{
            mealMonth: "2026-01-01",
            mode: "per_meal",
            quantity: 2,
            unitCents: 480,
            amountCents: 960,
          }}
        />
      </ul>,
    );
    expect(
      screen.getByText("2 Mittagessen im Januar 2026"),
    ).toBeInTheDocument();
    expect(screen.getByText("Mensa · je 4,80 €")).toBeInTheDocument();
    expect(screen.getByText("9,60 €")).toBeInTheDocument();
  });

  it("names a flat amount as the Mensa-Pauschale of the month", () => {
    render(
      <ul>
        <MealPositionLine
          position={{
            mealMonth: "2026-03-01",
            mode: "flat",
            quantity: 1,
            unitCents: 5500,
            amountCents: 5500,
          }}
        />
      </ul>,
    );
    expect(screen.getByText("Mensa-Pauschale März 2026")).toBeInTheDocument();
  });
});

describe("CareRunMealsCard", () => {
  it("shows nothing while every month is released and billed", () => {
    const { container } = render(<CareRunMealsCard gaps={[]} unbilled={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("names the months a school has not released and the children left out", () => {
    render(
      <CareRunMealsCard
        gaps={[
          {
            schoolId: "3",
            schoolName: "OGS Nord",
            month: "2026-01-01",
            expired: false,
          },
          {
            schoolId: "3",
            schoolName: "OGS Nord",
            month: "2026-02-01",
            expired: false,
          },
        ]}
        unbilled={[
          {
            schoolId: "3",
            schoolName: "OGS Nord",
            studentId: "7",
            childFirstName: "Ben",
            childLastName: "Fremd",
            month: "2026-03-01",
            mealDays: 4,
            reason: "no_contract",
          },
        ]}
      />,
    );
    expect(
      screen.getByText("2 Monate ohne Freigabe · 1 nicht abgerechnet"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Freigabe fehlt für Januar 2026, Februar 2026."),
    ).toBeInTheDocument();
    expect(screen.getByText("Ben Fremd")).toBeInTheDocument();
    expect(
      screen.getByText(
        "1 Kind mit 4 Essen nicht abgerechnet. Kein Vertrag an der Schule in diesem Monat.",
      ),
    ).toBeInTheDocument();
  });

  it("sums the children of a school month and folds a long list", () => {
    const unbilled = Array.from({ length: 7 }, (_, index) => ({
      schoolId: "3",
      schoolName: "OGS Nord",
      studentId: String(index + 1),
      childFirstName: "Kind",
      childLastName: `Nummer ${index + 1}`,
      month: "2026-03-01",
      mealDays: 2,
      reason: "no_contract" as const,
    }));
    render(<CareRunMealsCard gaps={[]} unbilled={unbilled} />);
    expect(screen.getByText("OGS Nord · März 2026")).toBeInTheDocument();
    expect(
      screen.getByText(
        "7 Kinder mit 14 Essen nicht abgerechnet. Kein Vertrag an der Schule in diesem Monat.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Kind Nummer 5")).toBeInTheDocument();
    expect(screen.queryByText("Kind Nummer 7")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "2 weitere anzeigen" }));
    expect(screen.getByText("Kind Nummer 7")).toBeInTheDocument();
  });

  it("names months past the attendance retention apart from the missing releases", () => {
    render(
      <CareRunMealsCard
        gaps={[
          {
            schoolId: "3",
            schoolName: "OGS Nord",
            month: "2026-01-01",
            expired: true,
          },
          {
            schoolId: "3",
            schoolName: "OGS Nord",
            month: "2026-02-01",
            expired: true,
          },
          {
            schoolId: "3",
            schoolName: "OGS Nord",
            month: "2026-03-01",
            expired: false,
          },
        ]}
        unbilled={[]}
      />,
    );
    expect(
      screen.getByText("1 Monat ohne Freigabe · 2 nicht mehr abrechenbar"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Freigabe fehlt für März 2026."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Nicht mehr abrechenbar: Januar 2026, Februar 2026. Die Anwesenheiten dieser Monate sind nach der Aufbewahrungsfrist gelöscht.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
