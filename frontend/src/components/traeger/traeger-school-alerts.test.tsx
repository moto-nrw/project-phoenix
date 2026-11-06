import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type {
  TraegerAlertKind,
  TraegerSchoolAlerts,
  TraegerSchoolsAlerts,
} from "~/lib/traeger-api";
import { TEST_CLOCK_INSTANT } from "~/test/clock";

vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path,
}));

import { TraegerSchoolAlertsCard } from "./traeger-school-alerts";

const AS_OF = TEST_CLOCK_INSTANT.toISOString();

function school(
  id: string,
  name: string,
  alerts: TraegerAlertKind[],
  openEnrollments = 0,
): TraegerSchoolAlerts {
  return {
    id,
    name,
    slug: name,
    subdomain: name,
    city: "Melle",
    figures: {
      childrenTotal: 40,
      childrenPresent: 30,
      childrenSick: 8,
      staffTotal: 4,
      staffPresent: 1,
      careRatio: 30,
      asOf: AS_OF,
    },
    openEnrollments,
    alerts,
  };
}

function alertsOf(schools: TraegerSchoolAlerts[]): TraegerSchoolsAlerts {
  return {
    thresholds: { childrenPerStaff: 20, sickPercent: 10, openEnrollments: 1 },
    schools,
    asOf: AS_OF,
  };
}

function renderCard(
  alerts: TraegerSchoolsAlerts | null,
  options: { error?: string | null; canChange?: boolean } = {},
) {
  return render(
    <main>
      <h1>Dashboard</h1>
      <TraegerSchoolAlertsCard
        alerts={alerts}
        error={options.error ?? null}
        canChangeThresholds={options.canChange ?? false}
      />
    </main>,
  );
}

describe("TraegerSchoolAlertsCard", () => {
  it("lists each school with its hints, the thresholds once and the way in", () => {
    renderCard(
      alertsOf([
        school("2", "OGS Am Bach", ["staff_shortage", "sick_children"]),
        school("3", "OGS Lindenweg", []),
        school("4", "OGS Mühle", ["open_enrollments"], 3),
      ]),
    );

    const card = screen.getByTestId("traeger-school-alerts");
    expect(within(card).getByText("2 Schulen")).toBeInTheDocument();
    expect(
      within(card).getAllByText(/^Ihre Grenzen: mehr als 20 Kinder/),
    ).toHaveLength(1);
    expect(within(card).getByText("Zu wenig Personal:")).toBeInTheDocument();
    expect(
      within(card).getByText(
        "30 Kinder auf 1 Betreuungskraft, Schlüssel 1 : 30.",
      ),
    ).toBeInTheDocument();
    expect(within(card).getByText("Viele Krankmeldungen:")).toBeInTheDocument();
    expect(
      within(card).getByText("3 Kinder warten auf eine Entscheidung."),
    ).toBeInTheDocument();
    expect(
      within(card).getByRole("link", { name: "Zur Schule: OGS Am Bach" }),
    ).toHaveAttribute("href", "/traeger/schulen/2");
    expect(
      within(card).queryByRole("link", { name: "Zur Schule: OGS Lindenweg" }),
    ).not.toBeInTheDocument();
    expect(
      within(card).getByRole("link", { name: "Grenzen ansehen" }),
    ).toHaveAttribute("href", "/traeger/einstellungen?bereich=dashboard");
  });

  it("names the link by what the person may do", () => {
    renderCard(alertsOf([school("2", "OGS Am Bach", ["sick_children"])]), {
      canChange: true,
    });
    expect(
      screen.getByRole("link", { name: "Grenzen ändern" }),
    ).toBeInTheDocument();
  });

  it("filters the hints by school", () => {
    renderCard(
      alertsOf([
        school("2", "OGS Am Bach", ["staff_shortage"]),
        school("3", "OGS Lindenweg", []),
        school("4", "OGS Mühle", ["open_enrollments"], 3),
      ]),
    );
    const card = screen.getByTestId("traeger-school-alerts");

    fireEvent.click(
      within(card).getByRole("combobox", {
        name: "Auffälligkeiten nach Schule filtern",
      }),
    );
    fireEvent.click(screen.getByRole("option", { name: "OGS Mühle (1)" }));
    expect(
      within(card).getByRole("link", { name: "Zur Schule: OGS Mühle" }),
    ).toBeInTheDocument();
    expect(
      within(card).queryByRole("link", { name: "Zur Schule: OGS Am Bach" }),
    ).not.toBeInTheDocument();

    fireEvent.click(
      within(card).getByRole("combobox", {
        name: "Auffälligkeiten nach Schule filtern",
      }),
    );
    fireEvent.click(screen.getByRole("option", { name: "OGS Lindenweg" }));
    expect(
      within(card).getByText("An dieser Schule ist gerade nichts auffällig."),
    ).toBeInTheDocument();
    expect(
      within(card).queryByRole("link", { name: /^Zur Schule:/ }),
    ).not.toBeInTheDocument();
  });

  it("says so when no school reaches a threshold", () => {
    renderCard(alertsOf([school("2", "OGS Am Bach", [])]));
    const card = screen.getByTestId("traeger-school-alerts");
    expect(
      within(card).getByText("Gerade ist an keiner Schule etwas auffällig."),
    ).toBeInTheDocument();
    expect(within(card).getByText("Auffälligkeiten")).toBeVisible();
    expect(within(card).queryByText(/^\d+ Schulen?$/)).not.toBeInTheDocument();
  });

  it("folds more than five schools", () => {
    renderCard(
      alertsOf(
        Array.from({ length: 7 }, (_, index) =>
          school(String(index + 10), `OGS ${index + 1}`, ["sick_children"]),
        ),
      ),
    );
    const card = screen.getByTestId("traeger-school-alerts");
    expect(
      within(card).getAllByRole("link", { name: /^Zur Schule:/ }),
    ).toHaveLength(5);
    fireEvent.click(
      within(card).getByRole("button", { name: /2 weitere anzeigen/ }),
    );
    expect(
      within(card).getAllByRole("link", { name: /^Zur Schule:/ }),
    ).toHaveLength(7);
  });

  it("shows the load error instead of an empty list", () => {
    renderCard(null, {
      error: "Die Auffälligkeiten der Schulen konnten nicht geladen werden.",
    });
    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(
      screen.queryByText("Gerade ist an keiner Schule etwas auffällig."),
    ).not.toBeInTheDocument();
  });

  it("stays away while the Träger has no school", () => {
    renderCard(alertsOf([]));
    expect(screen.getByRole("heading", { name: "Dashboard" })).toBeVisible();
    expect(
      screen.queryByTestId("traeger-school-alerts"),
    ).not.toBeInTheDocument();
  });
});
