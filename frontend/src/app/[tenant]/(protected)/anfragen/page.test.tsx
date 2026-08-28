import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import AnfragenPage from "./page";
import type { AggregatedRequestFilters } from "~/components/students/aggregated-request-list";
import { canOpenRequestsPage } from "~/lib/change-request-access";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";

const { mockUseSession } = vi.hoisted(() => ({ mockUseSession: vi.fn() }));

vi.mock("next-auth/react", () => ({
  useSession: (): ReturnType<typeof mockUseSession> => mockUseSession(),
}));

vi.mock("~/lib/hooks/use-require-permission", () => ({
  useRequirePermission: vi.fn(),
}));

// Die aggregierte Liste ist separat getestet; hier zählt nur, mit welcher
// Ansicht und welchen Filtern die Seite sie rendert.
vi.mock("~/components/students/aggregated-request-list", () => ({
  AggregatedRequestList: ({
    view,
    filters,
  }: {
    view: string;
    filters: AggregatedRequestFilters;
  }) => (
    <div
      data-testid="aggregated-list"
      data-view={view}
      data-filters={JSON.stringify(filters)}
    />
  ),
}));

// Ebenso die Abwesenheitsliste des Mitarbeitende-Reiters (#2433).
vi.mock("~/components/staff/staff-absence-request-list", () => ({
  StaffAbsenceRequestList: ({
    view,
    filters,
  }: {
    view: string;
    filters: { search: string; types: readonly string[] };
  }) => (
    <div
      data-testid="absence-list"
      data-view={view}
      data-filters={JSON.stringify(filters)}
    />
  ),
}));

// NavigationTabs beobachtet seine Scroll-Breite; jsdom kennt ResizeObserver
// nicht (gleiches Muster wie die Master-Detail-Tests).
class MockResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
vi.stubGlobal("ResizeObserver", MockResizeObserver);

const mockUseRequirePermission = vi.mocked(useRequirePermission);

/** Sitzung mit den angegebenen Rechten, ohne Admin-Rolle. */
function sessionWith(permissions: readonly string[]) {
  return {
    data: { user: { id: "7", roles: ["user"], permissions } },
    status: "authenticated" as const,
  };
}

function listProbe() {
  const probe = screen.getByTestId("aggregated-list");
  return {
    view: probe.getAttribute("data-view"),
    filters: JSON.parse(
      probe.getAttribute("data-filters") ?? "{}",
    ) as AggregatedRequestFilters,
  };
}

function absenceProbe() {
  const probe = screen.getByTestId("absence-list");
  return {
    view: probe.getAttribute("data-view"),
    filters: JSON.parse(probe.getAttribute("data-filters") ?? "{}") as {
      search: string;
      types: string[];
    },
  };
}

/** Der Umschalter in der Reiterzeile: Offen gegen Historie. */
function umschalten(label: "Offen" | "Historie") {
  fireEvent.click(screen.getByRole("button", { name: label }));
}

describe("AnfragenPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseRequirePermission.mockReturnValue({
      isReady: true,
      isLoading: false,
    });
    mockUseSession.mockReturnValue(sessionWith(["users:update"]));
  });

  it("shows a loading state until the permission check is ready", () => {
    mockUseRequirePermission.mockReturnValue({
      isReady: false,
      isLoading: true,
    });

    render(<AnfragenPage />);

    // Der Ladezustand kommt jetzt aus dem Gerüst; es beschriftet ihn mit dem
    // Seitentitel.
    expect(screen.getByLabelText("Anfragen wird geladen…")).toBeInTheDocument();
    expect(screen.queryByTestId("aggregated-list")).toBeNull();
  });

  // Der Zugriff hängt an der geteilten Regel, nicht an einer zweiten
  // Aufzählung in der Seite — dieselbe Regel tragen Sidebar-Eintrag, mobile
  // Navigation und Zähler-Badge.
  it("öffnet die Seite über canOpenRequestsPage", () => {
    render(<AnfragenPage />);

    expect(mockUseRequirePermission).toHaveBeenCalledWith(canOpenRequestsPage);
  });

  it("rendert die aggregierte Liste in der Offen-Ansicht", () => {
    render(<AnfragenPage />);

    expect(
      screen.getByRole("heading", { name: "Anfragen" }),
    ).toBeInTheDocument();
    const probe = listProbe();
    expect(probe.view).toBe("open");
    expect(probe.filters.search).toBe("");
    expect(probe.filters.types).toEqual([]);
  });

  it("wechselt per Historie-Schalter auf die Historien-Ansicht", () => {
    render(<AnfragenPage />);

    umschalten("Historie");
    expect(listProbe().view).toBe("history");

    umschalten("Offen");
    expect(listProbe().view).toBe("open");
  });

  it("reicht den Suchbegriff serverseitig an die Liste durch", () => {
    render(<AnfragenPage />);

    // PageHeaderWithSearch rendert das Suchfeld doppelt (mobil + Desktop);
    // beide tragen denselben Zustand.
    const [input] = screen.getAllByPlaceholderText("Kind suchen…");
    fireEvent.change(input!, { target: { value: "Emma" } });

    expect(listProbe().filters.search).toBe("Emma");
  });

  it("hält Status- und Zeitraum-Filter aus der Offen-Ansicht heraus", () => {
    render(<AnfragenPage />);

    const probe = listProbe();
    expect(probe.filters.statuses).toEqual([]);
    expect(probe.filters.from).toBeUndefined();
    expect(probe.filters.to).toBeUndefined();
  });

  it("bietet den Direkt-Korrektur-Filter nur in der Historie an", () => {
    render(<AnfragenPage />);

    fireEvent.click(screen.getAllByRole("button", { name: /Filter/ })[0]!);
    expect(screen.queryAllByText("Direkt-Korrekturen")).toHaveLength(0);

    umschalten("Historie");
    fireEvent.click(screen.getAllByText("Direkt-Korrekturen")[0]!);
    expect(listProbe().filters.types).toEqual(["direct_correction"]);

    // Zurück in der Arbeitsliste dürfen Korrekturen nie auftauchen, auch
    // nicht als übrig gebliebener Filter (#2436).
    umschalten("Offen");
    const open = listProbe();
    expect(open.view).toBe("open");
    expect(open.filters.types).toEqual([]);

    // Der inkompatible Filter wird beim Wechsel gelöscht und kehrt nicht
    // still zurück, wenn die Historie wieder geöffnet wird.
    umschalten("Historie");
    expect(listProbe().filters.types).toEqual([]);
  });

  // Anmeldungsänderungen (#2435): eigene Art im selben Reiter, eigenes Recht.
  it("lädt Anmeldungsänderungen nur mit config:manage mit", () => {
    render(<AnfragenPage />);
    expect(listProbe().filters.includeEnrollment).toBe(false);

    mockUseSession.mockReturnValue(
      sessionWith(["users:update", "config:manage"]),
    );
    render(<AnfragenPage />);
    expect(
      JSON.parse(
        screen
          .getAllByTestId("aggregated-list")[1]!
          .getAttribute("data-filters") ?? "{}",
      ),
    ).toMatchObject({ includeEnrollment: true, includeAggregated: true });
  });

  it("bietet die Anfrageart Anmeldung nur mit config:manage an", () => {
    render(<AnfragenPage />);
    fireEvent.click(screen.getAllByRole("button", { name: /Filter/ })[0]!);
    expect(screen.queryAllByText("Anmeldung")).toHaveLength(0);
  });

  it("bietet offene Abmeldungen als eigene Aufgabenart an", () => {
    mockUseSession.mockReturnValue(
      sessionWith(["users:update", "users:delete"]),
    );
    render(<AnfragenPage />);

    fireEvent.click(screen.getAllByRole("button", { name: /Filter/ })[0]!);
    fireEvent.click(screen.getAllByText("Abmeldungen")[0]!);
    expect(listProbe().filters.types).toEqual(["care_withdrawal"]);

    umschalten("Historie");
    expect(listProbe().filters.types).toEqual([]);
    expect(screen.queryAllByText("Abmeldungen")).toHaveLength(0);
  });

  it("zeigt einer Person mit nur config:manage ausschließlich die Anmeldungen", () => {
    mockUseSession.mockReturnValue(sessionWith(["config:manage"]));

    render(<AnfragenPage />);

    const probe = listProbe();
    // Der Aggregator verlangt users:update oder users:absence — ohne beides
    // darf er gar nicht erst gefragt werden, sonst antwortet er 403.
    expect(probe.filters.includeAggregated).toBe(false);
    expect(probe.filters.includeEnrollment).toBe(true);

    fireEvent.click(screen.getAllByRole("button", { name: /Filter/ })[0]!);
    expect(screen.getAllByText("Anmeldung")[0]).toBeInTheDocument();
    expect(screen.queryAllByText("Stammdaten")).toHaveLength(0);
  });

  it("zeigt einer Person mit users:absence die Liste ohne Art-Filter", () => {
    mockUseSession.mockReturnValue(
      sessionWith(["users:read", "users:absence"]),
    );

    render(<AnfragenPage />);

    // Die Liste selbst erscheint — das Backend engt sie serverseitig auf die
    // Entschuldigungen ein (#2232). Der Art-Filter wäre drei tote Optionen.
    expect(screen.getByTestId("aggregated-list")).toBeInTheDocument();
    expect(screen.queryAllByText("Anfrageart")).toHaveLength(0);
  });

  // Reiter-Sichtbarkeit nach Berechtigung (#2429): der Mitarbeitende-Reiter
  // hängt an vacation:approve, der Eltern-Reiter an canReviewChangeRequests.
  it("versteckt den Mitarbeitende-Reiter ohne vacation:approve", () => {
    render(<AnfragenPage />);

    expect(screen.queryByRole("tab", { name: "Mitarbeitende" })).toBeNull();
    expect(screen.queryByTestId("absence-list")).toBeNull();
  });

  it("zeigt mit vacation:approve beide Reiter und wechselt zu den Anträgen", () => {
    mockUseSession.mockReturnValue(
      sessionWith(["users:update", "vacation:approve"]),
    );

    render(<AnfragenPage />);

    // Eltern-Reiter ist voreingestellt aktiv.
    expect(screen.getByTestId("aggregated-list")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "Mitarbeitende" }));

    expect(absenceProbe().view).toBe("open");
    expect(screen.queryByTestId("aggregated-list")).toBeNull();
  });

  it("sucht im Mitarbeitende-Reiter nach Teammitgliedern und filtert nach Art", () => {
    mockUseSession.mockReturnValue(sessionWith(["vacation:approve"]));

    render(<AnfragenPage />);

    const [input] = screen.getAllByPlaceholderText("Teammitglied suchen…");
    fireEvent.change(input!, { target: { value: "Mira" } });
    expect(absenceProbe().filters.search).toBe("Mira");

    // Der Art-Filter der Kopfzeile reicht die gewählte Abwesenheitsart durch.
    fireEvent.click(screen.getAllByRole("button", { name: /Filter/ })[0]!);
    fireEvent.click(screen.getAllByRole("button", { name: "Fortbildung" })[0]!);
    expect(absenceProbe().filters.types).toEqual(["training"]);
  });

  it("schaltet den Mitarbeitende-Reiter auf die Historie um", () => {
    mockUseSession.mockReturnValue(sessionWith(["vacation:approve"]));

    render(<AnfragenPage />);

    umschalten("Historie");
    expect(absenceProbe().view).toBe("history");
  });

  it("zeigt mit nur vacation:approve die Anträge ohne Reiterleiste", () => {
    mockUseSession.mockReturnValue(sessionWith(["vacation:approve"]));

    render(<AnfragenPage />);

    expect(screen.getByTestId("absence-list")).toBeInTheDocument();
    // Nur ein sichtbarer Reiter → keine Reiterleiste, kein Eltern-Inhalt.
    expect(screen.queryByRole("tab", { name: "Eltern" })).toBeNull();
    expect(screen.queryByTestId("aggregated-list")).toBeNull();
  });

  it("zeigt Admins beide Reiter", () => {
    mockUseSession.mockReturnValue({
      data: { user: { id: "1", roles: ["admin"], permissions: ["admin:*"] } },
      status: "authenticated" as const,
    });

    render(<AnfragenPage />);

    expect(
      screen.getByRole("tab", { name: "Mitarbeitende" }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("aggregated-list")).toBeInTheDocument();
  });
});
