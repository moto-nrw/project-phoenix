import { render, screen } from "@testing-library/react";
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
          { schoolId: "3", schoolName: "OGS Nord", month: "2026-01-01" },
          { schoolId: "3", schoolName: "OGS Nord", month: "2026-02-01" },
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
      screen.getByText(/Kein Vertrag an der Schule in diesem Monat/),
    ).toBeInTheDocument();
  });
});
