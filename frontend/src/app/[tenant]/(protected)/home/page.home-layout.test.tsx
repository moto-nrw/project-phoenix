import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import HomePage from "./page";
import type {
  HomeBlockPlacement,
  HomeBlockPolicies,
  HomeLayoutOverrides,
} from "~/lib/home-blocks";

/**
 * Datenabfragen der Startseite (#2875, #2180).
 *
 * Ein Baustein, der nicht auf der Fläche steht, darf keine Last erzeugen. Die
 * Anordnung ist beim ersten Rendern noch nicht da; solange gilt die
 * Standardansicht der Rolle — auf ihre Ankunft zu WARTEN würde die Startseite
 * bei einer hängenden Abfrage leer lassen.
 */

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  redirect: vi.fn(),
}));

const mockSession = {
  user: {
    id: "1",
    name: "Test Admin",
    email: "admin@test.com",
    token: "test-token",
    isAdmin: true,
    firstName: "Test",
  },
  expires: "2099-12-31",
};

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({ data: mockSession, status: "authenticated" })),
}));

vi.mock("~/lib/auth-utils", () => ({
  isAdmin: vi.fn(() => true),
  hasEffectiveAdminScope: vi.fn(() => true),
  hasPermission: vi.fn(() => true),
  hasRole: vi.fn(() => true),
}));

vi.mock("~/lib/change-request-access", () => ({
  canOpenRequestsPage: vi.fn(() => true),
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

// Bausteine mit eigener Quelle laden für sich und haben eigene Tests.
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
  canManagePolicies: true,
  isLoading: false,
};

const save = vi.fn();
const reset = vi.fn();

vi.mock("~/lib/hooks/use-home-layout", () => ({
  useHomeLayout: () => ({
    state: {
      blocks: layoutState.blocks,
      overrides: layoutState.overrides,
      policies: layoutState.policies,
      canManagePolicies: layoutState.canManagePolicies,
    },
    isLoading: layoutState.isLoading,
    save,
    reset,
  }),
}));

import { useSWRAuth } from "~/lib/swr/hooks";

/** Die SWR-Schlüssel, mit denen die Seite in diesem Rendern gefragt hat. */
function requestedKeys(): (string | null)[] {
  return vi.mocked(useSWRAuth).mock.calls.map((call) => call[0]);
}

describe("Startseite — Abfragen nicht platzierter Bausteine", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    layoutState.blocks = [];
    layoutState.overrides = {};
    layoutState.policies = {};
    layoutState.canManagePolicies = true;
    layoutState.isLoading = false;
    vi.mocked(useSWRAuth).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: undefined,
      mutate: vi.fn(),
      isValidating: false,
    } as unknown as ReturnType<typeof useSWRAuth>);
  });

  it("fragt die Betriebszahlen, weil die Standardansicht der Leitung Kennzahlen zeigt", async () => {
    render(<HomePage />);

    await waitFor(() =>
      expect(screen.getByTestId("user-context-provider")).toBeInTheDocument(),
    );
    expect(requestedKeys()).toContain("dashboard-analytics");
  });

  it("fragt die Betriebszahlen nicht, wenn kein Baustein daraus lebt", async () => {
    layoutState.blocks = [{ key: "section.staff_notices", span: 2 }];
    layoutState.overrides = {
      "tile.students_present": false,
      "tile.students_sick": false,
      "tile.students_excused": false,
      "tile.students_home": false,
      "section.active_groups": false,
      "section.open_requests": false,
      "section.day_flow": false,
      "section.my_day": false,
    };

    render(<HomePage />);

    await waitFor(() =>
      expect(screen.getByTestId("user-context-provider")).toBeInTheDocument(),
    );
    expect(requestedKeys()).not.toContain("dashboard-analytics");
  });

  it("fragt die Geburtstage nur, wenn die Karte auf der Fläche steht", async () => {
    render(<HomePage />);

    await waitFor(() =>
      expect(screen.getByTestId("user-context-provider")).toBeInTheDocument(),
    );
    // Nicht in der Standardansicht: die Startseite ist ein Einstieg, die Karte
    // holt man sich über "Anpassen" dazu.
    expect(requestedKeys()).not.toContain("birthday-overview");

    vi.mocked(useSWRAuth).mockClear();
    layoutState.blocks = [{ key: "section.birthdays", span: 2 }];
    render(<HomePage />);

    expect(requestedKeys()).toContain("birthday-overview");
  });

  it("zeigt die Standardansicht, wenn die Anordnung nicht geladen werden kann", async () => {
    layoutState.isLoading = true;

    render(<HomePage />);

    await waitFor(() =>
      expect(screen.getByText("Kinder anwesend")).toBeInTheDocument(),
    );
    expect(requestedKeys()).toContain("dashboard-analytics");
  });

  it("lässt einen von der Schule abgeschalteten Baustein weg", async () => {
    layoutState.policies = { "tile.students_present": "disabled" };

    render(<HomePage />);

    await waitFor(() =>
      expect(screen.getByTestId("user-context-provider")).toBeInTheDocument(),
    );
    expect(screen.queryByText("Kinder anwesend")).not.toBeInTheDocument();
    // Die übrigen Kennzahlen leben weiter aus derselben Abfrage.
    expect(requestedKeys()).toContain("dashboard-analytics");
  });

  it("stellt einen verpflichtenden Baustein auf, auch wenn er entfernt wurde", async () => {
    layoutState.blocks = [{ key: "section.staff_notices", span: 2 }];
    layoutState.overrides = { "tile.students_sick": false };
    layoutState.policies = { "tile.students_sick": "required" };

    render(<HomePage />);

    await waitFor(() => expect(screen.getByText("Krank")).toBeInTheDocument());
  });
});
