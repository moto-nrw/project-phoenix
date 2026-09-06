import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import DashboardPage from "./page";
import {
  HOME_BLOCKS,
  type HomeBlockPolicies,
  type HomeLayoutOverrides,
} from "~/lib/home-blocks";

/**
 * Datenabfragen der Startseite (#2875).
 *
 * Eine ausgeblendete Kachel darf keine Last erzeugen — und die Auswahl ist
 * beim ersten Rendern noch nicht da, weshalb die Abfrage warten muss statt
 * den Standard "alles sichtbar" zu holen.
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

vi.mock("~/lib/tenant-context", () => ({
  useNFCEnabled: vi.fn(() => true),
  useOpenCareGroupMode: vi.fn(() => false),
  usePresenceMode: vi.fn(() => "detailed"),
  useTenantSlugSafe: vi.fn(() => "test-tenant"),
  useTenantRoutingModeSafe: vi.fn(() => "path"),
}));

vi.mock("~/lib/dashboard-helpers", () => ({
  formatRecentActivityTime: vi.fn(() => "12:00"),
  getActivityStatusColor: vi.fn(() => "bg-green-500"),
  getGroupStatusColor: vi.fn(() => "bg-green-500"),
}));

vi.mock("~/lib/swr/hooks", () => ({ useSWRAuth: vi.fn() }));

const layoutState = {
  overrides: {} as HomeLayoutOverrides,
  policies: {} as HomeBlockPolicies,
  canManagePolicies: true,
  isLoading: false,
};

vi.mock("~/lib/hooks/use-home-layout", () => ({
  useHomeLayout: () => ({
    state: {
      overrides: layoutState.overrides,
      policies: layoutState.policies,
      canManagePolicies: layoutState.canManagePolicies,
    },
    isLoading: layoutState.isLoading,
    save: vi.fn(),
    reset: vi.fn(),
  }),
}));

import { useSWRAuth } from "~/lib/swr/hooks";

/** Die SWR-Schlüssel, mit denen die Seite in diesem Rendern gefragt hat. */
function requestedKeys(): (string | null)[] {
  return vi.mocked(useSWRAuth).mock.calls.map((call) => call[0]);
}

describe("Startseite — Abfragen ausgeblendeter Kacheln", () => {
  beforeEach(() => {
    vi.clearAllMocks();
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

  it("fragt die Betriebszahlen, solange eine Kennzahl sichtbar ist", async () => {
    render(<DashboardPage />);

    await waitFor(() =>
      expect(screen.getByTestId("user-context-provider")).toBeInTheDocument(),
    );
    expect(requestedKeys()).toContain("dashboard-analytics");
  });

  it("fragt die Betriebszahlen nicht, wenn jede Kachel daraus ausgeblendet ist", async () => {
    layoutState.overrides = {
      "tile.students_present": false,
      "tile.students_in_rooms": false,
      "tile.students_in_transit": false,
      "tile.students_on_playground": false,
      "tile.students_sick": false,
      "tile.students_excused": false,
      "tile.students_home": false,
      "tile.active_activities": false,
      "tile.capacity_utilization": false,
      "section.recent_activity": false,
      "section.current_activities": false,
      "section.active_groups": false,
    };

    render(<DashboardPage />);

    await waitFor(() =>
      expect(screen.getByTestId("user-context-provider")).toBeInTheDocument(),
    );
    expect(requestedKeys()).not.toContain("dashboard-analytics");
    // Die Geburtstage hängen an einer eigenen Abfrage und bleiben sichtbar.
    expect(requestedKeys()).toContain("birthday-overview");
  });

  it("fragt die Geburtstage nicht, wenn die Karte ausgeblendet ist", async () => {
    layoutState.overrides = { "section.birthdays": false };

    render(<DashboardPage />);

    await waitFor(() =>
      expect(screen.getByTestId("user-context-provider")).toBeInTheDocument(),
    );
    expect(requestedKeys()).not.toContain("birthday-overview");
  });

  it("zeigt Geburtstage nach einer späteren Freigabe wieder", async () => {
    let birthdayFeatureEnabled = false;
    vi.mocked(useSWRAuth).mockImplementation(
      (key: string | null) =>
        ({
          data:
            key === "birthday-overview"
              ? { enabled: birthdayFeatureEnabled, celebrations: [] }
              : undefined,
          isLoading: false,
          error: undefined,
          mutate: vi.fn(),
          isValidating: false,
        }) as unknown as ReturnType<typeof useSWRAuth>,
    );

    const { rerender } = render(<DashboardPage />);

    await waitFor(() =>
      expect(
        requestedKeys().filter((key) => key === "birthday-overview"),
      ).toHaveLength(2),
    );
    expect(screen.queryByText("Geburtstage")).not.toBeInTheDocument();

    birthdayFeatureEnabled = true;
    rerender(<DashboardPage />);

    await waitFor(() =>
      expect(screen.getByText("Geburtstage")).toBeInTheDocument(),
    );
    expect(requestedKeys()).not.toContain(null);
  });

  it("nutzt den gemerkten Stand, während die Auswahl noch nachgeladen wird", async () => {
    // Der gemerkte Stand dieses Browsers liegt schon beim ersten Rendern vor,
    // die Abfrage bestätigt ihn nur. Deshalb wird auch dann nichts geholt,
    // was niemand sieht.
    layoutState.isLoading = true;
    layoutState.overrides = { "section.birthdays": false };

    render(<DashboardPage />);

    await waitFor(() =>
      expect(screen.getByTestId("user-context-provider")).toBeInTheDocument(),
    );
    expect(requestedKeys()).not.toContain("birthday-overview");
  });

  it("zeigt die Standardansicht, wenn die Auswahl nicht geladen werden kann", async () => {
    // Auf die Abfrage zu warten wäre der falsche Ausweg: hängt sie, bliebe die
    // Startseite leer. Ohne Vorwissen gilt deshalb die Empfehlung.
    layoutState.isLoading = true;
    layoutState.overrides = {};

    render(<DashboardPage />);

    await waitFor(() =>
      expect(screen.getByTestId("user-context-provider")).toBeInTheDocument(),
    );
    expect(requestedKeys()).toContain("dashboard-analytics");
    expect(requestedKeys()).toContain("birthday-overview");
  });

  it("bietet einen Weg zurück, wenn jede Kachel ausgeblendet ist", async () => {
    // Eine leere Fläche liest sich als Fehler. Die Seite sagt stattdessen, was
    // los ist und wie die Kacheln zurückkommen.
    layoutState.overrides = Object.fromEntries(
      HOME_BLOCKS.map((block) => [block.key, false]),
    );

    render(<DashboardPage />);

    expect(
      await screen.findByText("Ihre Startseite ist leer"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Kacheln einblenden" }),
    ).toBeInTheDocument();
  });

  it("erklärt eine von der Schule geleerte Startseite ohne wirkungslosen Knopf", async () => {
    layoutState.policies = Object.fromEntries(
      HOME_BLOCKS.map((block) => [block.key, "disabled"]),
    );
    layoutState.canManagePolicies = false;

    render(<DashboardPage />);

    expect(
      await screen.findByText(
        "Die Schule blendet alle Kacheln aus. Wenden Sie sich an Ihre Leitung.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Kacheln einblenden" }),
    ).not.toBeInTheDocument();
  });

  it("führt die Leitung zur Vorgabe, wenn sie alle Kacheln ausgeschaltet hat", async () => {
    layoutState.policies = Object.fromEntries(
      HOME_BLOCKS.map((block) => [block.key, "disabled"]),
    );

    render(<DashboardPage />);

    expect(
      await screen.findByRole("button", {
        name: "Startseite für alle öffnen",
      }),
    ).toBeInTheDocument();
  });

  it("fragt die Betriebszahlen, wenn die Schule eine Kachel verpflichtend macht", async () => {
    layoutState.overrides = {
      "tile.students_present": false,
      "tile.students_in_rooms": false,
      "tile.students_in_transit": false,
      "tile.students_on_playground": false,
      "tile.students_sick": false,
      "tile.students_excused": false,
      "tile.students_home": false,
      "tile.active_activities": false,
      "tile.capacity_utilization": false,
      "section.recent_activity": false,
      "section.current_activities": false,
      "section.active_groups": false,
    };
    layoutState.policies = { "tile.students_present": "required" };

    render(<DashboardPage />);

    await waitFor(() =>
      expect(screen.getByTestId("user-context-provider")).toBeInTheDocument(),
    );
    expect(requestedKeys()).toContain("dashboard-analytics");
  });
});
