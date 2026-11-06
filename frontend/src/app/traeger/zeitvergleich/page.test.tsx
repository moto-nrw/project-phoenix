import { render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { LOCATION_COLORS, getAccessibleTextColor } from "~/lib/location-helper";
import type { OverrunEvaluation } from "~/lib/traeger-care-time-overruns-api";

const { api, mockUseSession, clock } = vi.hoisted(() => ({
  clock: { today: "2026-09-09" },
  api: {
    fetchOverrunSummary: vi.fn(),
    fetchOverrunChildren: vi.fn(),
  },
  mockUseSession: vi.fn(),
}));

vi.mock("~/lib/traeger-care-time-overruns-api", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("~/lib/traeger-care-time-overruns-api")
  >()),
  ...api,
}));
vi.mock("next-auth/react", () => ({ useSession: () => mockUseSession() }));
vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => clock.today,
}));
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  usePathname: () => "/zeitvergleich",
  useSearchParams: () => new URLSearchParams(),
}));

import TraegerZeitvergleichPage from "./page";

const EVALUATION: OverrunEvaluation = {
  from: "2026-09-01",
  to: "2026-09-09",
  toleranceMinutes: 15,
  total: { children: 1, overrunDays: 2, unbookedDays: 1, extraMinutes: 540 },
  schools: [
    {
      schoolId: "3",
      schoolName: "OGS Am Lindenhof",
      dataFrom: "2026-09-01",
      comparedDays: 40,
      uncomparedDays: 0,
      figures: {
        children: 1,
        overrunDays: 2,
        unbookedDays: 1,
        extraMinutes: 540,
      },
    },
    {
      schoolId: "4",
      schoolName: "OGS Süd",
      dataFrom: "2026-09-03",
      comparedDays: 12,
      uncomparedDays: 0,
      figures: {
        children: 0,
        overrunDays: 0,
        unbookedDays: 0,
        extraMinutes: 0,
      },
    },
  ],
  children: [],
};

function session(permissions: string[]) {
  return {
    data: { user: { id: "A", scope: "org", orgId: 7, permissions } },
    status: "authenticated",
  };
}

function renderPage() {
  render(
    <ToastProvider>
      <TraegerZeitvergleichPage />
    </ToastProvider>,
  );
}

describe("Länger als gebucht", () => {
  beforeEach(() => {
    clock.today = "2026-09-09";
    api.fetchOverrunSummary.mockReset();
    api.fetchOverrunChildren.mockReset();
  });

  it("shows the figures per school without names for the reports right", async () => {
    mockUseSession.mockReturnValue(session(["traeger_reports:read"]));
    api.fetchOverrunSummary.mockResolvedValue(EVALUATION);
    renderPage();

    const table = await screen.findByRole("table");
    expect(within(table).getByText("OGS Am Lindenhof")).toBeInTheDocument();
    expect(within(table).getByText("9 Std.")).toBeInTheDocument();
    expect(within(table).getByText("davon 1 ohne Buchung")).toBeInTheDocument();
    expect(
      within(table).getByText("Check-ins erst ab 03.09.2026 gespeichert"),
    ).toBeInTheDocument();
    expect(api.fetchOverrunSummary).toHaveBeenCalledWith({
      from: "2026-09-01",
      to: "2026-09-09",
      schoolId: undefined,
    });
    expect(api.fetchOverrunChildren).not.toHaveBeenCalled();
    expect(
      screen.getByText(/Die Namen der Kinder sehen nur Personen mit dem Recht/),
    ).toBeInTheDocument();
  });

  it("lists the children by name for the contracts right, overruns in red", async () => {
    mockUseSession.mockReturnValue(
      session(["traeger_reports:read", "traeger_contracts:manage"]),
    );
    api.fetchOverrunChildren.mockResolvedValue({
      ...EVALUATION,
      children: [
        {
          schoolId: "3",
          schoolName: "OGS Am Lindenhof",
          studentId: "11",
          firstName: "Sina",
          lastName: "Soll",
          schoolClass: "3b",
          overrunDays: 2,
          unbookedDays: 1,
          extraMinutes: 540,
        },
      ],
    });
    renderPage();

    const names = await screen.findAllByText("Sina Soll");
    const row =
      names.map((name) => name.closest("tr")).find((tr) => tr !== null) ?? null;
    expect(row).not.toBeNull();
    const extra = within(row as HTMLElement).getByText("9 Std.");
    expect(extra).toHaveStyle({
      color: getAccessibleTextColor(LOCATION_COLORS.DANGER),
    });
    expect(api.fetchOverrunSummary).not.toHaveBeenCalled();
    expect(names.length).toBeGreaterThan(0);
    expect(
      screen.queryByText(/Die Namen der Kinder sehen nur Personen/),
    ).not.toBeInTheDocument();
  });

  it("shows a zero day without red", async () => {
    mockUseSession.mockReturnValue(session(["traeger_reports:read"]));
    api.fetchOverrunSummary.mockResolvedValue(EVALUATION);
    renderPage();
    await screen.findAllByText("OGS Süd");
    const table = screen.getAllByRole("table")[0] as HTMLElement;
    const zero = within(table).getAllByText("0 Min.")[0];
    expect(zero?.getAttribute("style")).toBeNull();
  });

  it("never shows missing check-ins as no deviation", async () => {
    mockUseSession.mockReturnValue(
      session(["traeger_reports:read", "traeger_contracts:manage"]),
    );
    const none = {
      children: 0,
      overrunDays: 0,
      unbookedDays: 0,
      extraMinutes: 0,
    };
    api.fetchOverrunChildren.mockResolvedValue({
      ...EVALUATION,
      total: none,
      schools: [
        { ...EVALUATION.schools[0]!, comparedDays: 0, figures: none },
        {
          ...EVALUATION.schools[1]!,
          dataFrom: "2026-09-20",
          comparedDays: 0,
          figures: none,
        },
      ],
      children: [],
    });
    renderPage();

    const titles = await screen.findAllByText(
      "Für diesen Zeitraum gibt es keine Check-ins.",
    );
    expect(titles.length).toBeGreaterThan(0);
    const table = screen.getAllByRole("table")[0] as HTMLElement;
    expect(
      within(table).getAllByText("Keine Check-ins im Zeitraum").length,
    ).toBeGreaterThan(0);
    expect(
      within(table).getAllByText("Keine Check-ins mehr gespeichert").length,
    ).toBeGreaterThan(0);
    expect(
      within(table).getAllByText(", nichts verglichen").length,
    ).toBeGreaterThan(0);
    // No zero that reads as "nobody stayed longer", anywhere on the page.
    expect(within(table).queryByText("0")).not.toBeInTheDocument();
    expect(within(table).queryByText("0 Min.")).not.toBeInTheDocument();
    expect(
      screen.queryByText("0 Kinder waren länger da als gebucht."),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("Kein Kind war länger da als gebucht."),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("In diesem Monat passen alle Zeiten zur Buchung."),
    ).not.toBeInTheDocument();
  });

  it("marks a school without check-ins and claims no full all-clear", async () => {
    mockUseSession.mockReturnValue(
      session(["traeger_reports:read", "traeger_contracts:manage"]),
    );
    api.fetchOverrunChildren.mockResolvedValue({
      ...EVALUATION,
      schools: [
        EVALUATION.schools[0]!,
        { ...EVALUATION.schools[1]!, dataFrom: "2026-09-01", comparedDays: 0 },
      ],
      children: [],
    });
    renderPage();

    expect(
      (
        await screen.findAllByText(
          "Verglichen sind nur Tage mit Check-in und Buchung. Andere Tage zählen hier nicht.",
        )
      ).length,
    ).toBeGreaterThan(0);
    const table = screen.getAllByRole("table")[0] as HTMLElement;
    expect(
      within(table).getAllByText("Keine Check-ins im Zeitraum").length,
    ).toBeGreaterThan(0);
    expect(
      screen.queryByText("In diesem Monat passen alle Zeiten zur Buchung."),
    ).not.toBeInTheDocument();
  });

  it("claims no all-clear while check-in days could not be compared", async () => {
    mockUseSession.mockReturnValue(
      session(["traeger_reports:read", "traeger_contracts:manage"]),
    );
    api.fetchOverrunChildren.mockResolvedValue({
      ...EVALUATION,
      schools: [
        EVALUATION.schools[0]!,
        {
          ...EVALUATION.schools[1]!,
          dataFrom: "2026-09-01",
          uncomparedDays: 4,
        },
      ],
      children: [],
    });
    renderPage();

    expect(
      (
        await screen.findAllByText(
          "Verglichen sind nur Tage mit Check-in und Buchung. Andere Tage zählen hier nicht.",
        )
      ).length,
    ).toBeGreaterThan(0);
    const table = screen.getAllByRole("table")[0] as HTMLElement;
    expect(
      within(table).getAllByText(
        "4 Tage nicht verglichen: ohne Plan oder ohne Auschecken",
      ).length,
    ).toBeGreaterThan(0);
    expect(
      screen.getAllByText(
        "In den verglichenen Tagen war kein Kind länger da als gebucht.",
      ).length,
    ).toBeGreaterThan(0);
    expect(
      screen.queryByText("Kein Kind war länger da als gebucht."),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("In diesem Monat passen alle Zeiten zur Buchung."),
    ).not.toBeInTheDocument();
  });

  it("names check-ins it could not compare instead of claiming none", async () => {
    mockUseSession.mockReturnValue(
      session(["traeger_reports:read", "traeger_contracts:manage"]),
    );
    const none = {
      children: 0,
      overrunDays: 0,
      unbookedDays: 0,
      extraMinutes: 0,
    };
    api.fetchOverrunChildren.mockResolvedValue({
      ...EVALUATION,
      total: none,
      schools: [
        {
          ...EVALUATION.schools[0]!,
          comparedDays: 0,
          uncomparedDays: 6,
          figures: none,
        },
        {
          ...EVALUATION.schools[1]!,
          dataFrom: "2026-09-01",
          comparedDays: 0,
          uncomparedDays: 0,
          figures: none,
        },
      ],
      children: [],
    });
    renderPage();

    expect(
      (
        await screen.findAllByText(
          "Die Check-ins in diesem Zeitraum ließen sich nicht vergleichen.",
        )
      ).length,
    ).toBeGreaterThan(0);
    const table = screen.getAllByRole("table")[0] as HTMLElement;
    const lindenhof = within(table)
      .getAllByText("OGS Am Lindenhof")[0]!
      .closest("tr") as HTMLElement;
    expect(
      within(lindenhof).getByText(
        "6 Tage nicht verglichen: ohne Plan oder ohne Auschecken",
      ),
    ).toBeInTheDocument();
    expect(
      within(lindenhof).queryByText("Keine Check-ins im Zeitraum"),
    ).not.toBeInTheDocument();
    const sued = within(table)
      .getAllByText("OGS Süd")[0]!
      .closest("tr") as HTMLElement;
    expect(
      within(sued).getByText("Keine Check-ins im Zeitraum"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Für diesen Zeitraum gibt es keine Check-ins."),
    ).not.toBeInTheDocument();
    expect(within(table).queryByText("0")).not.toBeInTheDocument();
  });

  it("follows the current month across midnight", async () => {
    mockUseSession.mockReturnValue(session(["traeger_reports:read"]));
    api.fetchOverrunSummary.mockResolvedValue(EVALUATION);
    clock.today = "2026-09-30";
    const { rerender } = render(
      <ToastProvider>
        <TraegerZeitvergleichPage />
      </ToastProvider>,
    );
    await waitFor(() =>
      expect(api.fetchOverrunSummary).toHaveBeenLastCalledWith({
        from: "2026-09-01",
        to: "2026-09-30",
        schoolId: undefined,
      }),
    );

    clock.today = "2026-10-01";
    rerender(
      <ToastProvider>
        <TraegerZeitvergleichPage />
      </ToastProvider>,
    );
    await waitFor(() =>
      expect(api.fetchOverrunSummary).toHaveBeenLastCalledWith({
        from: "2026-10-01",
        to: "2026-10-01",
        schoolId: undefined,
      }),
    );
  });

  it("shows a known overrun although no day was fully compared", async () => {
    mockUseSession.mockReturnValue(session(["traeger_reports:read"]));
    const early = {
      children: 1,
      overrunDays: 1,
      unbookedDays: 0,
      extraMinutes: 45,
    };
    const none = {
      children: 0,
      overrunDays: 0,
      unbookedDays: 0,
      extraMinutes: 0,
    };
    api.fetchOverrunSummary.mockResolvedValue({
      ...EVALUATION,
      total: early,
      schools: [
        {
          ...EVALUATION.schools[0]!,
          comparedDays: 0,
          uncomparedDays: 1,
          figures: early,
        },
        {
          ...EVALUATION.schools[1]!,
          dataFrom: "2026-09-01",
          comparedDays: 0,
          uncomparedDays: 0,
          figures: none,
        },
      ],
    });
    renderPage();

    await screen.findAllByText("OGS Am Lindenhof");
    const table = screen.getAllByRole("table")[0] as HTMLElement;
    const lindenhof = within(table)
      .getAllByText("OGS Am Lindenhof")[0]!
      .closest("tr") as HTMLElement;
    expect(within(lindenhof).getByText("45 Min.")).toHaveStyle({
      color: getAccessibleTextColor(LOCATION_COLORS.DANGER),
    });
    expect(
      within(lindenhof).getByText(
        "1 Tag nicht verglichen: ohne Plan oder ohne Auschecken",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Ein Kind war länger da als gebucht."),
    ).toBeInTheDocument();
  });
});
