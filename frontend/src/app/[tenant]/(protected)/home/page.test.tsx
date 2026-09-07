import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import HomePage from "./page";
import type {
  HomeBlockPlacement,
  HomeBlockPolicies,
  HomeLayoutOverrides,
} from "~/lib/home-blocks";

/**
 * Die Startseite als Brett (#2180): Standardansicht je Rolle, alles
 * verschiebbar, und nichts, was die Person nicht abrufen darf.
 *
 * Was IN einer Karte steht, prüft home-block-content.test.tsx; welche
 * Abfragen die Fläche auslöst, page.home-layout.test.tsx.
 */

const mockRedirect = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  redirect: (url: string) => mockRedirect(url),
}));

const mockSession = {
  user: {
    id: "1",
    name: "Anna Müller",
    email: "admin@test.com",
    token: "test-token",
    isAdmin: true,
    firstName: "Anna",
  },
  expires: "2099-12-31",
};

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({ data: mockSession, status: "authenticated" })),
}));

vi.mock("~/lib/auth-utils", () => ({
  isAdmin: vi.fn((session) => session?.user?.isAdmin ?? false),
  hasEffectiveAdminScope: vi.fn((session) => session?.user?.isAdmin ?? false),
  hasPermission: vi.fn((session) => session?.user?.isAdmin ?? false),
  hasRole: vi.fn(() => false),
}));

vi.mock("~/lib/change-request-access", () => ({
  canOpenRequestsPage: vi.fn((session) => session?.user?.isAdmin ?? false),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
  useBreadcrumb: vi.fn(() => ({ breadcrumb: {}, setBreadcrumb: vi.fn() })),
  BreadcrumbProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

vi.mock("~/lib/usercontext-context", () => ({
  UserContextProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="user-context-provider">{children}</div>
  ),
}));

vi.mock("~/components/enrollment/phase-expiry-warnings", () => ({
  PhaseExpiryWarnings: () => <div data-testid="phase-expiry-warnings" />,
}));

vi.mock("~/components/home/staff-notices-block", () => ({
  StaffNoticesBlock: () => <div data-testid="staff-notices-block" />,
}));
vi.mock("~/components/home/day-flow-block", () => ({
  DayFlowBlock: () => <div data-testid="day-flow-block" />,
}));
vi.mock("~/components/home/open-requests-block", () => ({
  OpenRequestsBlock: () => <div data-testid="open-requests-block" />,
}));
vi.mock("~/components/time-tracking/betreuungsplan-heute-card", () => ({
  BetreuungsplanHeuteCard: () => <div data-testid="my-day-block" />,
}));

vi.mock("~/lib/tenant-context", () => ({
  useNFCEnabled: vi.fn(() => true),
  useOpenCareGroupMode: vi.fn(() => false),
  usePresenceMode: vi.fn(() => "detailed"),
  useTenantSlugSafe: vi.fn(() => "test-tenant"),
  useTenantRoutingModeSafe: vi.fn(() => "path"),
  useTimetableEnabled: vi.fn(() => true),
}));

vi.mock("~/lib/dashboard-helpers", () => ({
  formatRecentActivityTime: vi.fn(() => "14:05"),
  getActivityStatusColor: vi.fn(() => "bg-moto-green"),
  getGroupStatusColor: vi.fn(() => "bg-moto-green"),
}));

vi.mock("~/lib/swr/hooks", () => ({ useSWRAuth: vi.fn() }));

const layoutState = {
  blocks: [] as readonly HomeBlockPlacement[],
  overrides: {} as HomeLayoutOverrides,
  policies: {} as HomeBlockPolicies,
};
const save = vi.fn().mockResolvedValue(undefined);
const reset = vi.fn().mockResolvedValue(undefined);

vi.mock("~/lib/hooks/use-home-layout", () => ({
  useHomeLayout: () => ({
    state: { ...layoutState, canManagePolicies: true },
    isLoading: false,
    save,
    reset,
  }),
}));

import { useSession } from "next-auth/react";
import { hasEffectiveAdminScope, isAdmin } from "~/lib/auth-utils";
import { useSWRAuth } from "~/lib/swr/hooks";

const analytics = {
  studentsPresent: 150,
  studentsInRooms: 120,
  studentsInTransit: 20,
  studentsOnPlayground: 10,
  studentsSick: 4,
  studentsExcused: 3,
  studentsHome: 33,
  activeOGSGroups: 8,
  activeActivities: 5,
  capacityUtilization: 0.75,
  supervisorsToday: 10,
  recentActivity: [],
  currentActivities: [],
  activeGroupsSummary: [],
};

function mockSWR(data: unknown, error?: Error) {
  return {
    data,
    isLoading: false,
    error,
    mutate: vi.fn(),
    isValidating: false,
  } as unknown as ReturnType<typeof useSWRAuth>;
}

describe("Startseite", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    layoutState.blocks = [];
    layoutState.overrides = {};
    layoutState.policies = {};
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });
    vi.mocked(isAdmin).mockReturnValue(true);
    vi.mocked(hasEffectiveAdminScope).mockReturnValue(true);
    vi.mocked(useSWRAuth).mockReturnValue(mockSWR(analytics));
  });

  it("begrüßt mit dem Vornamen", async () => {
    render(<HomePage />);

    await waitFor(() =>
      expect(screen.getByText(/Guten .*, Anna/)).toBeInTheDocument(),
    );
  });

  it("zeigt der Leitung ihre Standardansicht", async () => {
    render(<HomePage />);

    await waitFor(() =>
      expect(screen.getByTestId("home-board")).toBeInTheDocument(),
    );
    expect(
      screen.getByTestId("home-block-tile.students_present"),
    ).toBeInTheDocument();
    expect(
      screen.getByTestId("home-block-section.open_requests"),
    ).toBeInTheDocument();
    // Der eigene Tag gehört in die Betreuungsansicht, nicht in die der Leitung.
    expect(
      screen.queryByTestId("home-block-section.my_day"),
    ).not.toBeInTheDocument();
  });

  // Seit #2180 ist die Startseite für jede Rolle offen: nicht die Rolle
  // entscheidet, was zu sehen ist, sondern das Recht hinter jedem Baustein.
  it("öffnet sich für eine Betreuungskraft mit deren Standardansicht", async () => {
    vi.mocked(isAdmin).mockReturnValue(false);
    vi.mocked(hasEffectiveAdminScope).mockReturnValue(false);
    vi.mocked(useSession).mockReturnValue({
      data: { ...mockSession, user: { ...mockSession.user, isAdmin: false } },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<HomePage />);

    await waitFor(() =>
      expect(screen.queryByText("Kein Zugriff")).not.toBeInTheDocument(),
    );
    // Ohne Rechte auf die Betriebszahlen bleiben die Kennzahlen weg; die
    // Bausteine mit eigener Quelle stehen da, sobald ihr Recht reicht.
    expect(screen.queryByText("Kinder anwesend")).not.toBeInTheDocument();
    expect(screen.getByText("Ihre Startseite ist leer")).toBeInTheDocument();
  });

  it("zeigt die Skelettfläche, solange die Sitzung lädt", () => {
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "loading",
      update: vi.fn(),
    });

    render(<HomePage />);

    expect(screen.getByTestId("dashboard-skeleton")).toBeInTheDocument();
  });

  it("lädt die Ablaufwarnungen der Anmeldephasen nur mit Adminzuschnitt", () => {
    vi.mocked(hasEffectiveAdminScope).mockReturnValue(false);

    render(<HomePage />);

    expect(
      screen.queryByTestId("phase-expiry-warnings"),
    ).not.toBeInTheDocument();
  });

  it("behält die Zahlen, wenn eine spätere Abfrage fehlschlägt", async () => {
    vi.mocked(useSWRAuth).mockReturnValue(
      mockSWR(analytics, new Error("fetch failed")),
    );

    render(<HomePage />);

    await waitFor(() => {
      expect(
        screen.getByText("Fehler beim Laden der Dashboard-Daten"),
      ).toBeInTheDocument();
      expect(screen.getByText("150")).toBeInTheDocument();
    });
  });

  it("leitet zur Anmeldung, wenn die Sitzung abgelaufen ist", async () => {
    vi.mocked(useSession).mockReturnValue({
      data: { ...mockSession, error: "RefreshTokenExpired" },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<HomePage />);

    await waitFor(() =>
      expect(mockRedirect).toHaveBeenCalledWith("/test-tenant/"),
    );
  });

  it("leitet zur Anmeldung, wenn das Token fehlt", async () => {
    vi.mocked(useSession).mockReturnValue({
      data: { ...mockSession, user: { ...mockSession.user, token: undefined } },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<HomePage />);

    await waitFor(() =>
      expect(mockRedirect).toHaveBeenCalledWith("/test-tenant/"),
    );
  });
});

/** Die übrigen Bausteine der Leitungsansicht, damit die Fläche klein bleibt. */
const HIDDEN_DEFAULTS: HomeLayoutOverrides = {
  "tile.students_sick": false,
  "tile.students_excused": false,
  "tile.students_home": false,
  "section.open_requests": false,
  "section.active_groups": false,
  "section.day_flow": false,
};

describe("Startseite anpassen", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    layoutState.blocks = [
      { key: "tile.students_present", span: 1 },
      { key: "section.staff_notices", span: 2 },
    ];
    // Der Rest der Standardansicht ist bereits entfernt, damit diese Fläche
    // genau zwei Karten hat und die Erwartungen unten lesbar bleiben.
    layoutState.overrides = { ...HIDDEN_DEFAULTS };
    layoutState.policies = {};
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });
    vi.mocked(isAdmin).mockReturnValue(true);
    vi.mocked(hasEffectiveAdminScope).mockReturnValue(true);
    vi.mocked(useSWRAuth).mockReturnValue(mockSWR(analytics));
    save.mockResolvedValue(undefined);
    reset.mockResolvedValue(undefined);
  });

  const startEditing = () => {
    render(<HomePage />);
    fireEvent.click(screen.getByRole("button", { name: "Anpassen" }));
  };

  it("blendet die Werkzeuge erst im Anpassen-Modus ein", () => {
    render(<HomePage />);
    expect(
      screen.queryByRole("button", { name: /entfernen/ }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Anpassen" }));

    expect(
      screen.getByRole("button", { name: "Kinder anwesend entfernen" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Bausteine hinzufügen")).toBeInTheDocument();
  });

  it("macht eine Karte breiter und speichert die neue Breite", async () => {
    startEditing();

    fireEvent.click(
      screen.getByRole("button", { name: "Tagesinformationen breiter" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Fertig" }));

    await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    expect(save.mock.calls[0]?.[1]).toEqual([
      { key: "tile.students_present", span: 1 },
      { key: "section.staff_notices", span: 4 },
    ]);
  });

  // Eine Kennzahl hat nur eine Breite: der Knopf dafür ist da, aber tot.
  it("lässt eine Kennzahl nicht breiter werden", () => {
    startEditing();

    expect(
      screen.getByRole("button", { name: "Kinder anwesend breiter" }),
    ).toBeDisabled();
  });

  it("verschiebt eine Karte nach vorne", async () => {
    startEditing();

    fireEvent.click(
      screen.getByRole("button", { name: "Tagesinformationen nach vorne" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Fertig" }));

    await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    expect(save.mock.calls[0]?.[1]).toEqual([
      { key: "section.staff_notices", span: 2 },
      { key: "tile.students_present", span: 1 },
    ]);
  });

  // Entfernt muss entfernt bleiben, sonst holt die Standardansicht den
  // Baustein beim nächsten Aufruf zurück.
  it("merkt sich einen entfernten Baustein als Abweichung", async () => {
    startEditing();

    fireEvent.click(
      screen.getByRole("button", { name: "Kinder anwesend entfernen" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Fertig" }));

    await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    expect(save.mock.calls[0]?.[0]).toEqual({
      ...HIDDEN_DEFAULTS,
      "tile.students_present": false,
    });
    expect(save.mock.calls[0]?.[1]).toEqual([
      { key: "section.staff_notices", span: 2 },
    ]);
  });

  it("nimmt einen Baustein aus dem Hinzufügen-Menü ans Ende auf", async () => {
    startEditing();

    fireEvent.click(screen.getByRole("button", { name: /Geburtstage/ }));
    fireEvent.click(screen.getByRole("button", { name: "Fertig" }));

    await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    expect(save.mock.calls[0]?.[1]?.at(-1)).toEqual({
      key: "section.birthdays",
      span: 2,
    });
  });

  it("verwirft die Änderungen bei Abbrechen", () => {
    startEditing();

    fireEvent.click(
      screen.getByRole("button", { name: "Kinder anwesend entfernen" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

    expect(save).not.toHaveBeenCalled();
    expect(
      screen.getByTestId("home-block-tile.students_present"),
    ).toBeInTheDocument();
  });

  it("stellt die Standardansicht wieder her", async () => {
    startEditing();

    fireEvent.click(
      screen.getByRole("button", { name: "Standard wiederherstellen" }),
    );

    await waitFor(() => expect(reset).toHaveBeenCalledTimes(1));
    expect(save).not.toHaveBeenCalled();
  });
});
