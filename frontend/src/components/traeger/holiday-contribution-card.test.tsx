import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import {
  HOLIDAY_CONTRIBUTION,
  HOLIDAY_REGISTRATION,
} from "~/test/fixtures/holiday-registrations";
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace("/traeger", ""),
}));

import { HolidayContributionCard } from "./holiday-contribution-card";

describe("HolidayContributionCard", () => {
  it("shows the lines of every child and calls a received registration a preview", () => {
    render(
      <HolidayContributionCard
        contribution={HOLIDAY_CONTRIBUTION}
        registration={HOLIDAY_REGISTRATION}
        canManageBillingSettings
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

  it("does not link to billing settings without access to the settings", () => {
    render(
      <HolidayContributionCard
        contribution={HOLIDAY_CONTRIBUTION}
        registration={HOLIDAY_REGISTRATION}
        canManageBillingSettings={false}
      />,
    );

    // Anker: Die angewandte Geschwisterregel ist geladen.
    expect(
      screen.getByText("Geschwister: 2. Kind, 50 % weniger"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Abrechnungsvorgaben" }),
    ).not.toBeInTheDocument();
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

  it("links an applied rule to the Abrechnungsvorgaben, but not a plain price (#5685)", () => {
    const { unmount } = render(
      <HolidayContributionCard
        contribution={HOLIDAY_CONTRIBUTION}
        registration={HOLIDAY_REGISTRATION}
        canManageBillingSettings
      />,
    );
    expect(
      screen.getByRole("link", { name: "Abrechnungsvorgaben" }),
    ).toHaveAttribute("href", "/einstellungen/abrechnung");
    unmount();

    render(
      <HolidayContributionCard
        contribution={{
          ...HOLIDAY_CONTRIBUTION,
          children: HOLIDAY_CONTRIBUTION.children.map((child) => ({
            ...child,
            lines: child.lines.filter((line) => line.kind === "base"),
          })),
        }}
        registration={HOLIDAY_REGISTRATION}
        canManageBillingSettings
      />,
    );
    expect(
      screen.queryByRole("link", { name: "Abrechnungsvorgaben" }),
    ).toBeNull();
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
      screen.getByText(/Die Rechnung zeigt noch den alten Betrag\./),
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
