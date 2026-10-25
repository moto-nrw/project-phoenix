import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import {
  HOLIDAY_CONTRIBUTION,
  HOLIDAY_REGISTRATION,
} from "~/test/fixtures/holiday-registrations";
import { HolidayContributionCard } from "./holiday-contribution-card";

describe("HolidayContributionCard", () => {
  it("shows the lines of every child and calls a received registration a preview", () => {
    render(
      <HolidayContributionCard
        contribution={HOLIDAY_CONTRIBUTION}
        registration={HOLIDAY_REGISTRATION}
      />,
    );

    expect(
      screen.getByText(
        "Vorschau mit den Preisen und Regeln von heute. „Bestätigen“ hält den Beitrag fest.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Lia Muster")).toBeInTheDocument();
    expect(screen.getByText("Ben Muster")).toBeInTheDocument();
    expect(
      screen.getByText("Geschwister: 2. Kind, 50 % weniger"),
    ).toBeInTheDocument();
    expect(screen.getByText("-22,50 €")).toBeInTheDocument();
    expect(screen.getByText("Gesamt: 90,00 €")).toBeInTheDocument();
  });

  it("names the day a confirmed contribution was frozen", () => {
    render(
      <HolidayContributionCard
        contribution={{
          ...HOLIDAY_CONTRIBUTION,
          frozen: true,
          confirmedAt: "2026-09-08T10:00:00Z",
        }}
        registration={{ ...HOLIDAY_REGISTRATION, status: "confirmed" }}
      />,
    );

    expect(
      screen.getByText(
        "Festgehalten am 08.09.2026. Spätere Preise und Regeln ändern ihn nicht.",
      ),
    ).toBeInTheDocument();
  });
});
