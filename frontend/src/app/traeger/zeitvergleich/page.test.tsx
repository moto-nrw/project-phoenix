import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { LOCATION_COLORS, getAccessibleTextColor } from "~/lib/location-helper";
import type { OverrunEvaluation } from "~/lib/traeger-care-time-overruns-api";

const { api, mockUseSession } = vi.hoisted(() => ({
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
    const table = await screen.findByRole("table");
    const zero = within(table).getAllByText("0 Min.")[0];
    expect(zero?.getAttribute("style")).toBeNull();
  });
});
