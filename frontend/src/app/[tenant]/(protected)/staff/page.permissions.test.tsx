// Der Berechtigungs-Split der /staff-Seite (#1417 Tranche 2a): die
// aggregierte Schul-Übersicht läuft mit users:read, die Zeitkonten-Tabelle
// verlangt time_tracking:manage. Ohne die zweite Berechtigung darf der
// Umschalter nicht erscheinen UND der Overview-Request nicht gefeuert werden —
// eine Seite, die still ein 403 im Netzwerk-Tab produziert, gilt als kaputt.

import { render, screen } from "@testing-library/react";
import { useSession } from "next-auth/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import StaffPage from "./page";

const getTimeAccounts = vi.hoisted(() => vi.fn());
const getDashboardSummary = vi.hoisted(() => vi.fn());
const swrKeys = vi.hoisted(() => [] as (string | null)[]);

vi.mock("next-auth/react", () => ({ useSession: vi.fn() }));
vi.mock("next/navigation", () => ({ redirect: vi.fn() }));
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn() }),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null, fetcher: () => Promise<unknown>) => {
    swrKeys.push(key);
    // Genau wie useSWRAuth selbst: ein null-Key fetcht nicht.
    if (key !== null) void fetcher();
    return { data: undefined, isLoading: false, error: null };
  },
}));

vi.mock("~/lib/staff-api", () => ({
  staffService: { getAllStaff: vi.fn().mockResolvedValue([]) },
}));

vi.mock("~/lib/staff-overview-api", () => ({
  staffOverviewService: { getTimeAccounts, getDashboardSummary },
}));

vi.mock("~/components/staff/staff-pending-inbox", () => ({
  StaffPendingInbox: () => <div />,
  useStaffPendingInbox: () => ({ rows: [], canReview: false }),
}));

vi.mock("~/components/staff/school-overview-section", () => ({
  SchoolOverviewSection: () => <div data-testid="schul-uebersicht" />,
}));

function mockSession(permissions: string[]) {
  vi.mocked(useSession).mockReturnValue({
    data: {
      user: { token: "t", permissions, role: "staff" },
      expires: "",
    },
    status: "authenticated",
    update: vi.fn(),
  } as unknown as ReturnType<typeof useSession>);
}

describe("/staff — Berechtigungs-Split", () => {
  beforeEach(() => {
    swrKeys.length = 0;
    getTimeAccounts.mockReset().mockResolvedValue({
      year: 2026,
      month: 7,
      rows: [],
    });
  });

  it("zeigt die Schul-Übersicht auch ohne time_tracking:manage", () => {
    mockSession(["users:read"]);

    render(<StaffPage />);

    expect(screen.getByTestId("schul-uebersicht")).toBeInTheDocument();
  });

  it("blendet den Zeitkonten-Umschalter ohne time_tracking:manage aus und fragt die Zeitkonten nicht ab", () => {
    mockSession(["users:read"]);

    render(<StaffPage />);

    expect(screen.queryByText("Zeitkonten")).not.toBeInTheDocument();
    expect(getTimeAccounts).not.toHaveBeenCalled();
    expect(swrKeys.some((key) => key?.startsWith("staff-time-accounts"))).toBe(
      false,
    );
  });

  it("zeigt den Umschalter mit time_tracking:manage", () => {
    mockSession(["users:read", "time_tracking:manage"]);

    render(<StaffPage />);

    expect(screen.getByText("Zeitkonten")).toBeInTheDocument();
  });

  it("fragt die Zeitkonten auch mit Berechtigung erst beim Umschalten ab", () => {
    mockSession(["users:read", "time_tracking:manage"]);

    render(<StaffPage />);

    // Default-Ansicht ist Status; die Tabelle kostet erst dann einen Request,
    // wenn sie auch angezeigt wird.
    expect(getTimeAccounts).not.toHaveBeenCalled();
  });
});
