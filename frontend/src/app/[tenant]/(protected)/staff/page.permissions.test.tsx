// Der Berechtigungs-Split der /staff-Seite (#1417 Tranche 2a): die
// aggregierte Einrichtungs-Übersicht läuft mit users:read, die Zeitkonten-Tabelle
// verlangt time_tracking:manage. Ohne die zweite Berechtigung darf der
// Umschalter nicht erscheinen UND der Overview-Request nicht gefeuert werden —
// eine Seite, die still ein 403 im Netzwerk-Tab produziert, gilt als kaputt.

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useSession } from "next-auth/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import StaffPage from "./page";

const getTimeAccounts = vi.hoisted(() => vi.fn());
const getDashboardSummary = vi.hoisted(() => vi.fn());
const getAllStaff = vi.hoisted(() => vi.fn());
const swrKeys = vi.hoisted(() => [] as (string | null)[]);
const berlinToday = vi.hoisted(() => vi.fn());
const closeMonth = vi.hoisted(() => vi.fn());
const mutateMonthClose = vi.hoisted(() => vi.fn());
const mutateDocumentDirectory = vi.hoisted(() => vi.fn());
const globalMutate = vi.hoisted(() => vi.fn());
const monthCloseRequest = vi.hoisted(() => ({
  error: null as Error | null,
}));
const documentDirectoryRequest = vi.hoisted(() => ({
  error: null as Error | null,
}));

vi.mock("next-auth/react", () => ({ useSession: vi.fn() }));
vi.mock("next/navigation", () => ({ redirect: vi.fn() }));
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn() }),
}));
vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: berlinToday,
}));
vi.mock("swr", () => ({
  useSWRConfig: () => ({ mutate: globalMutate }),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null, fetcher: () => Promise<unknown>) => {
    swrKeys.push(key);
    // Genau wie useSWRAuth selbst: ein null-Key fetcht nicht.
    if (key !== null) void fetcher();
    if (key?.includes("staff-month-close-")) {
      return {
        data: monthCloseRequest.error ? undefined : [],
        isLoading: false,
        error: monthCloseRequest.error,
        mutate: mutateMonthClose,
      };
    }
    if (key === "staff-document-directory") {
      return {
        data: documentDirectoryRequest.error ? undefined : [],
        isLoading: false,
        error: documentDirectoryRequest.error,
        mutate: mutateDocumentDirectory,
      };
    }
    return { data: undefined, isLoading: false, error: null };
  },
}));

vi.mock("~/lib/staff-api", () => ({
  staffService: { getAllStaff, getDocumentDirectory: vi.fn() },
  staffMonthCloseService: {
    getStatus: vi.fn().mockResolvedValue([]),
    closeMonth,
  },
}));

vi.mock("~/lib/staff-overview-api", () => ({
  staffOverviewService: { getTimeAccounts, getDashboardSummary },
}));

vi.mock("~/components/staff/staff-pending-inbox", () => ({
  StaffPendingInbox: () => <div />,
  useStaffPendingInbox: () => ({ rows: [], canReview: false }),
}));

vi.mock("~/components/staff/school-overview-section", () => ({
  SchoolOverviewSection: () => {
    void getDashboardSummary();
    return <div data-testid="schul-uebersicht" />;
  },
}));
vi.mock("~/components/staff/staff-audit-log", () => ({
  StaffAuditLog: () => <div data-testid="staff-audit-log" />,
}));
vi.mock("~/components/staff/month-close-modal", () => ({
  MonthCloseReasonModal: ({
    onSubmit,
  }: {
    onSubmit: (reason: string) => Promise<void>;
  }) => (
    <button type="button" onClick={() => void onSubmit("Lohnlauf")}>
      Abschluss bestätigen
    </button>
  ),
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
    berlinToday.mockReset().mockReturnValue("2026-09-01");
    closeMonth.mockReset().mockResolvedValue({});
    mutateMonthClose.mockReset().mockResolvedValue(undefined);
    mutateDocumentDirectory.mockReset().mockResolvedValue(undefined);
    globalMutate.mockReset().mockResolvedValue(undefined);
    monthCloseRequest.error = null;
    documentDirectoryRequest.error = null;
    getAllStaff.mockReset().mockResolvedValue([]);
    getDashboardSummary.mockReset().mockResolvedValue({});
    getTimeAccounts.mockReset().mockResolvedValue({
      year: 2026,
      month: 7,
      rows: [],
    });
  });

  it("zeigt die Einrichtungs-Übersicht auch ohne time_tracking:manage", () => {
    mockSession(["users:read"]);

    render(<StaffPage />);

    expect(screen.getByTestId("schul-uebersicht")).toBeInTheDocument();
    expect(getDashboardSummary).toHaveBeenCalledTimes(1);
  });

  it("öffnet für time_tracking:manage ohne users:read direkt die Zeitkonten und fragt die Personalliste nicht ab", () => {
    mockSession(["time_tracking:manage"]);

    render(<StaffPage />);

    expect(screen.queryByTestId("schul-uebersicht")).not.toBeInTheDocument();
    expect(getDashboardSummary).not.toHaveBeenCalled();
    expect(getAllStaff).not.toHaveBeenCalled();
    expect(swrKeys).not.toContain("staff-list");
    expect(getTimeAccounts).toHaveBeenCalledTimes(1);
    expect(screen.getByText(/Zeitkonten —/)).toBeInTheDocument();
    expect(
      screen.queryByRole("tab", { name: "Status" }),
    ).not.toBeInTheDocument();
  });

  it("erlaubt time_tracking:manage ohne users:read den Wechsel ins Änderungsprotokoll", () => {
    mockSession(["time_tracking:manage"]);

    render(<StaffPage />);

    const auditTab = screen.getByRole("tab", {
      name: "Änderungsprotokoll",
    });
    fireEvent.pointerDown(auditTab, { button: 0, pointerType: "mouse" });
    fireEvent.mouseDown(auditTab, { button: 0 });
    fireEvent.click(auditTab);

    expect(screen.getByTestId("staff-audit-log")).toBeInTheDocument();
    expect(
      screen.queryByRole("tab", { name: "Status" }),
    ).not.toBeInTheDocument();
    expect(getAllStaff).not.toHaveBeenCalled();
    expect(getTimeAccounts).toHaveBeenLastCalledWith({
      year: 2026,
      month: 9,
    });
  });

  it("zeigt ohne eine der beiden Leseberechtigungen eine Zugriffssperre", () => {
    mockSession([]);

    render(<StaffPage />);

    expect(screen.getByText("Kein Zugriff")).toBeInTheDocument();
    expect(getAllStaff).not.toHaveBeenCalled();
    expect(getTimeAccounts).not.toHaveBeenCalled();
    expect(getDashboardSummary).not.toHaveBeenCalled();
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

  it("leitet den laufenden Monat und die Abschlussgrenze vom Berliner Tag ab", () => {
    berlinToday.mockReturnValue("2030-01-01");
    mockSession(["time_tracking:manage"]);

    render(<StaffPage />);

    expect(getTimeAccounts).toHaveBeenCalledWith(
      expect.objectContaining({ year: 2030, month: 1 }),
    );
    expect(
      screen.getByRole("button", { name: "Nächster Monat" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Monat abschließen" }),
    ).toBeDisabled();
  });

  it("aktualisiert nach dem Abschluss beide Monatskarten-Cachefamilien", async () => {
    mockSession(["time_tracking:manage"]);
    render(<StaffPage />);

    fireEvent.click(screen.getByRole("button", { name: "Vorheriger Monat" }));
    fireEvent.click(screen.getByRole("button", { name: "Monat abschließen" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Abschluss bestätigen" }),
    );

    await waitFor(() => {
      expect(closeMonth).toHaveBeenCalledWith({
        year: 2026,
        month: 8,
        reason: "Lohnlauf",
      });
      expect(mutateMonthClose).toHaveBeenCalledTimes(1);
      expect(globalMutate).toHaveBeenCalledTimes(1);
    });

    const isAffectedKey = globalMutate.mock.calls[0]?.[0] as (
      key: unknown,
    ) => boolean;
    expect(isAffectedKey("tenant:staff-time-accounts-2026-8")).toBe(true);
    expect(isAffectedKey("tenant:staff-month-summary-12-2026-8")).toBe(true);
    expect(isAffectedKey("tenant:time-tracking-month-summary-2026-8")).toBe(
      true,
    );
    expect(isAffectedKey("tenant:staff-list")).toBe(false);
  });

  it("zeigt einen fehlgeschlagenen Abschlussstatus mit Retry statt Abschlussaktionen", async () => {
    monthCloseRequest.error = new Error("Bad Gateway");
    mockSession(["time_tracking:manage"]);

    render(<StaffPage />);

    expect(
      screen.getByText(/Der Abschlussstatus konnte nicht geladen werden/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Monat abschließen" }),
    ).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Abschlussstatus erneut laden" }),
    );

    await waitFor(() => {
      expect(mutateMonthClose).toHaveBeenCalledTimes(1);
    });
  });

  it("zeigt einen retrybaren Fehler, wenn das Dokumentenverzeichnis nicht lädt", async () => {
    documentDirectoryRequest.error = new Error("Bad Gateway");
    mockSession(["staff_documents:health"]);

    render(<StaffPage />);

    expect(
      screen.getByText("Das Personalverzeichnis konnte nicht geladen werden."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Keine Mitarbeiter/innen gefunden."),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Erneut versuchen" }));

    await waitFor(() => {
      expect(mutateDocumentDirectory).toHaveBeenCalledTimes(1);
    });
  });
});
