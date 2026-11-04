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
    // Woche und Zeitmodell ohne Trennpunkt (#5684).
    expect(
      screen.getAllByText("04.01.2027 bis 08.01.2027: bis 16 Uhr").length,
    ).toBeGreaterThan(0);
    expect(screen.getByText("Ben Muster")).toBeInTheDocument();
    expect(
      screen.getByText("Geschwister: 2. Kind, 50 % weniger"),
    ).toBeInTheDocument();
    expect(screen.getByText("-22,50 €")).toBeInTheDocument();
    expect(screen.getByText("Gesamt")).toBeInTheDocument();
    expect(screen.getByText("90,00 €")).toBeInTheDocument();
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

  it("asks to check an invoice whose amount no longer fits the siblings", () => {
    render(
      <HolidayContributionCard
        contribution={{
          ...HOLIDAY_CONTRIBUTION,
          frozen: true,
          confirmedAt: "2026-09-08T10:00:00Z",
          invoiced: true,
          correctionNeeded: true,
        }}
        registration={{ ...HOLIDAY_REGISTRATION, status: "confirmed" }}
      />,
    );

    expect(screen.getByText("Rechnung prüfen")).toBeInTheDocument();
    expect(
      screen.getByText(/Die Geschwister dieser Anmeldung haben sich geändert/),
    ).toBeInTheDocument();
    expect(screen.getByText("90,00 €")).toBeInTheDocument();
  });

  it("shows no correction hint while the invoice fits", () => {
    render(
      <HolidayContributionCard
        contribution={{ ...HOLIDAY_CONTRIBUTION, frozen: true, invoiced: true }}
        registration={{ ...HOLIDAY_REGISTRATION, status: "confirmed" }}
      />,
    );

    expect(screen.getByText("Gesamt")).toBeInTheDocument();
    expect(screen.queryByText("Rechnung prüfen")).not.toBeInTheDocument();
  });
});
