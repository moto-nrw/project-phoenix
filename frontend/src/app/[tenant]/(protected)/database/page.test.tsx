import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { describe, it, expect, vi, beforeEach } from "vitest";
import DatabasePage from "./page";
import { mockSessionData } from "~/test/mocks/next-auth";
import useSWR from "swr";

// Die Kacheln folgen den Rechten der Sitzung (#3469); die Leitung hält den
// Adminzuschnitt und sieht jede.
const mockSession = {
  ...mockSessionData(),
  user: {
    ...mockSessionData().user,
    roles: ["admin"],
    permissions: ["admin:*"],
  },
};

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: mockSession,
    status: "authenticated",
  })),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn(),
  }),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
  useBreadcrumb: vi.fn(() => ({ breadcrumb: {}, setBreadcrumb: vi.fn() })),
  BreadcrumbProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: ({ title }: { title: string }) => (
    <div data-testid="page-header">{title}</div>
  ),
}));

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

const mockCountsResponse = {
  success: true,
  message: "Counts fetched",
  data: {
    students: 100,
    teachers: 25,
    rooms: 15,
    activities: 20,
    groups: 10,
    roles: 5,
    devices: 8,
    permissionCount: 50,
    permissions: {
      canViewStudents: true,
      canViewTeachers: true,
      canViewRooms: true,
      canViewActivities: true,
      canViewGroups: true,
      canViewRoles: true,
      canViewDevices: true,
      canViewPermissions: true,
    },
  },
};

global.fetch = vi.fn(() =>
  Promise.resolve({
    ok: true,
    json: () => Promise.resolve(mockCountsResponse),
  } as Response),
);

import { useSession } from "next-auth/react";
import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { useTimetableEnabled } from "~/lib/tenant-context";

vi.mock("~/lib/tenant-context", () => ({
  useNFCEnabled: () => true,
  useTenantSlugSafe: () => "test-tenant",
  useTenantRoutingModeSafe: () => "path",
  useTimetableEnabled: vi.fn(),
}));

function mockCounts(
  data: unknown = mockCountsResponse.data,
  overrides: Record<string, unknown> = {},
) {
  vi.mocked(useSWR).mockReturnValue({
    data,
    error: undefined,
    isLoading: false,
    isValidating: false,
    mutate: vi.fn(),
    ...overrides,
  } as never);
}

describe("DatabasePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });
    vi.mocked(useIsMobile).mockReturnValue(false);
    vi.mocked(useTimetableEnabled).mockReturnValue(true);
    mockCounts();
    vi.mocked(global.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockCountsResponse),
    } as Response);
  });

  it("renders the database page with layout", async () => {
    render(<DatabasePage />);

    // Page renders without layout wrapper (now in app layout)
    await waitFor(() => {
      expect(screen.getByText("Kinderdaten")).toBeInTheDocument();
    });
  });

  it("shows the card-grid skeleton during the initial counts request", () => {
    vi.mocked(useSWR).mockReturnValue({
      data: undefined,
      error: undefined,
      isLoading: true,
      isValidating: true,
      mutate: vi.fn(),
    } as never);

    render(<DatabasePage />);

    expect(screen.getByTestId("database-index-skeleton")).toBeVisible();
    expect(screen.queryByText("Kinderdaten")).not.toBeInTheDocument();
  });

  it("displays data sections with counts after loading", async () => {
    render(<DatabasePage />);

    await waitFor(() => {
      expect(screen.getByText("Kinderdaten")).toBeInTheDocument();
      expect(screen.getByText("Personal")).toBeInTheDocument();
      expect(screen.getByText("Räume")).toBeInTheDocument();
      expect(screen.getByText("Aktivitäten")).toBeInTheDocument();
      expect(screen.getByText("Gruppen")).toBeInTheDocument();
      expect(screen.getByText("Rollen")).toBeInTheDocument();
      expect(screen.getByText("Geräte")).toBeInTheDocument();
      expect(screen.getByText("Berechtigungen")).toBeInTheDocument();
    });
  });

  it("displays correct count for each section", async () => {
    render(<DatabasePage />);

    await waitFor(() => {
      expect(screen.getByText("100 Einträge")).toBeInTheDocument(); // students
      expect(screen.getByText("25 Einträge")).toBeInTheDocument(); // teachers
      expect(screen.getByText("15 Einträge")).toBeInTheDocument(); // rooms
      expect(screen.getByText("20 Einträge")).toBeInTheDocument(); // activities
      expect(screen.getByText("10 Einträge")).toBeInTheDocument(); // groups
      expect(screen.getByText("5 Einträge")).toBeInTheDocument(); // roles
      expect(screen.getByText("8 Einträge")).toBeInTheDocument(); // devices
      expect(screen.getByText("50 Einträge")).toBeInTheDocument(); // permissions
    });
  });

  it("shows every count once, on its tile and not again in the page header", () => {
    render(<DatabasePage />);

    // Die Kacheln tragen jeden Bestand; eine Statuszeile im Kopf wiederholte
    // drei davon (#4385).
    const header = screen.getByRole("banner");
    expect(header).not.toHaveTextContent(/\d/);
    expect(screen.getAllByText("100 Einträge")).toHaveLength(1);
    expect(screen.getAllByText("25 Einträge")).toHaveLength(1);
    expect(screen.getAllByText("15 Einträge")).toHaveLength(1);
  });

  it("displays singular 'Eintrag' for count of 1", async () => {
    mockCounts({
      ...mockCountsResponse.data,
      students: 1,
    });

    render(<DatabasePage />);

    await waitFor(() => {
      expect(screen.getByText("1 Eintrag")).toBeInTheDocument();
    });
  });

  it("shows page header on mobile", async () => {
    vi.mocked(useIsMobile).mockReturnValue(true);

    render(<DatabasePage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 1, name: "Datenverwaltung" }),
      ).toBeInTheDocument();
    });
  });

  it("hides sections when user lacks permissions", async () => {
    // Eine Rolle der Schule ohne Adminzuschnitt sieht nur die Kacheln, deren
    // Route ihr Recht öffnet (#3469): hier die Kinderdaten mit users:delete.
    vi.mocked(useSession).mockReturnValue({
      data: {
        ...mockSession,
        user: {
          ...mockSession.user,
          roles: ["ogs-leitung"],
          permissions: ["users:delete"],
        },
      },
      status: "authenticated",
      update: vi.fn(),
    } as never);

    render(<DatabasePage />);

    await waitFor(() => {
      expect(screen.getByText("Kinderdaten")).toBeInTheDocument();
      expect(screen.queryByText("Personal")).not.toBeInTheDocument();
      expect(screen.queryByText("Räume")).not.toBeInTheDocument();
    });
  });

  it("shows catalog cards to administrators without explicit catalog permissions", async () => {
    const administrator = {
      ...mockSession,
      user: {
        ...mockSession.user,
        roles: ["admin"],
        permissions: [],
      },
    };
    vi.mocked(useSession).mockReturnValue({
      data: administrator,
      status: "authenticated",
      update: vi.fn(),
    } as never);

    render(<DatabasePage />);

    await waitFor(() => {
      expect(screen.getByText("Terminkategorien")).toBeInTheDocument();
      expect(screen.getByText("Planungsspuren")).toBeInTheDocument();
      expect(screen.getByText("Schichtarten")).toBeInTheDocument();
      expect(screen.getByText("Abwesenheitsarten")).toBeInTheDocument();
    });
  });

  it("uses the tenant planning state for administrators without config:read", () => {
    const administrator = {
      ...mockSession,
      user: {
        ...mockSession.user,
        roles: ["admin"],
        permissions: [],
      },
    };
    vi.mocked(useSession).mockReturnValue({
      data: administrator,
      status: "authenticated",
      update: vi.fn(),
    } as never);
    vi.mocked(useTimetableEnabled).mockReturnValue(false);

    render(<DatabasePage />);

    // Anker: die übrigen Stammdaten-Kacheln sind geladen.
    expect(screen.getByText("Terminkategorien")).toBeInTheDocument();
    expect(screen.getByText("Abwesenheitsarten")).toBeInTheDocument();
    expect(screen.queryByText("Planungsspuren")).not.toBeInTheDocument();
    expect(screen.queryByText("Schichtarten")).not.toBeInTheDocument();
  });

  it("shows planning catalogs to delegated users while planning is on", () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        ...mockSession,
        user: {
          ...mockSession.user,
          roles: [],
          permissions: ["schedules:manage", "time_tracking:manage"],
        },
      },
      status: "authenticated",
      update: vi.fn(),
    } as never);
    vi.mocked(useTimetableEnabled).mockReturnValue(true);

    render(<DatabasePage />);

    expect(screen.getByText("Planungsspuren")).toBeInTheDocument();
    expect(screen.getByText("Schichtarten")).toBeInTheDocument();
  });

  it("hides planning catalogs for delegated users when planning is switched off", () => {
    // time_tracking:manage öffnet Schichtarten (Planung) und Abwesenheitsarten
    // (ohne Planung). Die zweite Kachel ist der Anker: das Raster ist
    // geladen, nur die Planungskataloge fehlen.
    const scheduleManager = {
      ...mockSession,
      user: {
        ...mockSession.user,
        roles: [],
        permissions: ["schedules:manage", "time_tracking:manage"],
      },
    };
    vi.mocked(useSession).mockReturnValue({
      data: scheduleManager,
      status: "authenticated",
      update: vi.fn(),
    } as never);
    vi.mocked(useTimetableEnabled).mockReturnValue(false);

    render(<DatabasePage />);

    expect(screen.getByText("Abwesenheitsarten")).toBeInTheDocument();
    expect(screen.queryByText("Planungsspuren")).not.toBeInTheDocument();
    expect(screen.queryByText("Schichtarten")).not.toBeInTheDocument();
  });

  it("handles 401 unauthorized response gracefully", async () => {
    mockCounts(null);

    render(<DatabasePage />);

    // Ohne Zahlen bleibt die Seite benutzbar: die Kacheln folgen den Rechten
    // der Sitzung, nicht den Zählern (#3469), und die Zähler fehlen still.
    await waitFor(() => {
      expect(screen.getByText("Kinderdaten")).toBeInTheDocument();
      expect(screen.queryByText("100 Einträge")).not.toBeInTheDocument();
    });
  });

  it("handles 403 forbidden response gracefully", async () => {
    mockCounts(null);

    render(<DatabasePage />);

    await waitFor(() => {
      expect(screen.getByText("Kinderdaten")).toBeInTheDocument();
      expect(screen.queryByText("100 Einträge")).not.toBeInTheDocument();
    });
  });

  it("handles fetch error gracefully", async () => {
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const error = new ApiError("Network error", 503, {
      code: "general.unavailable",
    });
    vi.mocked(useSWR).mockImplementation(((
      _key: unknown,
      _fetcher: unknown,
      options: unknown,
    ) => {
      (options as { onError?: (cause: unknown) => void }).onError?.(error);
      return {
        data: undefined,
        error,
        isLoading: false,
        isValidating: false,
        mutate: vi.fn(),
      };
    }) as unknown as typeof useSWR);

    render(<DatabasePage />);

    await waitFor(() => {
      expect(consoleSpy).toHaveBeenCalledWith("failed to fetch counts", {
        error: "Network error",
      });
    });
    // Ein Ladefehler ist nie „0 Einträge“ (#2517).
    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Zahl der Einträge"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("0 Einträge")).not.toBeInTheDocument();
    expect(screen.getAllByText("Nicht geladen").length).toBeGreaterThan(0);

    consoleSpy.mockRestore();
  });

  it("displays descriptions for each section", async () => {
    render(<DatabasePage />);

    await waitFor(() => {
      expect(
        screen.getByText("Kinder anlegen, importieren und ihre Daten pflegen"),
      ).toBeInTheDocument();
      expect(
        screen.getByText("Personaldaten und Zuordnungen verwalten"),
      ).toBeInTheDocument();
      expect(
        screen.getByText("Räume und Ausstattung verwalten"),
      ).toBeInTheDocument();
    });
  });

  it("has correct links for each section", async () => {
    render(<DatabasePage />);

    await waitFor(() => {
      // Match on each card's description: the bare section titles are
      // ambiguous now that the Exporte card mentions "Kinder-" and
      // "Raumlisten" in its own description.
      const studentsLink = screen.getByRole("link", {
        name: /Kinder anlegen/,
      });
      expect(studentsLink).toHaveAttribute(
        "href",
        "/test-tenant/database/students",
      );

      const teachersLink = screen.getByRole("link", {
        name: /Personaldaten und Zuordnungen/,
      });
      expect(teachersLink).toHaveAttribute(
        "href",
        "/test-tenant/database/personal",
      );

      const roomsLink = screen.getByRole("link", {
        name: /Räume und Ausstattung/,
      });
      expect(roomsLink).toHaveAttribute("href", "/test-tenant/database/rooms");
    });
  });

  it("keeps the intro text with the Alle Kinder link behind the info button", () => {
    render(<DatabasePage />);

    // Im Alltag kostet der Satz keine Kartenzeile (#4385); die Grenze zu
    // „Alle Kinder" bleibt auf der Seite (ADR 0008).
    expect(
      screen.queryByText(/Hier legen Sie Kinder, Personal, Räume/),
    ).toBeNull();

    fireEvent.click(
      screen.getByRole("button", { name: "Erklärung zu Datenverwaltung" }),
    );

    const panel = screen.getByRole("dialog", {
      name: "Erklärung zu Datenverwaltung",
    });
    expect(panel).toHaveTextContent(
      /Hier legen Sie Kinder, Personal, Räume und Gruppen an/,
    );
    expect(
      within(panel).getByRole("link", { name: "Alle Kinder" }),
    ).toHaveAttribute("href", "/test-tenant/students/search");
  });

  it("keeps the tenant slug on the Exporte link in path routing", async () => {
    render(<DatabasePage />);

    await waitFor(() => {
      const exportsLink = screen.getByRole("link", {
        name: /Geburtstags-, Notfall- und Raumlisten/,
      });
      expect(exportsLink).toHaveAttribute(
        "href",
        "/test-tenant/database/exports",
      );
    });
  });

  it("uses the central concept tones for section icons", async () => {
    render(<DatabasePage />);

    await waitFor(() => {
      expect(screen.getByText("Kinderdaten")).toBeInTheDocument();
    });

    expect(
      screen.getByTestId("database-section-icon-students").querySelector("svg"),
    ).toHaveAttribute("data-moto-duotone-tone", "greenVivid");
    expect(
      screen.getByTestId("database-section-icon-teachers").querySelector("svg"),
    ).toHaveAttribute("data-moto-duotone-tone", "orange");
    expect(
      screen.getByTestId("database-section-icon-groups").querySelector("svg"),
    ).toHaveAttribute("data-moto-duotone-tone", "greenDeep");
  });
});
