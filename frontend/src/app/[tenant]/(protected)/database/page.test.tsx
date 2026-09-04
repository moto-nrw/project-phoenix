import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import DatabasePage from "./page";
import { mockSessionData } from "~/test/mocks/next-auth";
import useSWR from "swr";

const mockSession = mockSessionData();

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
      expect(screen.getByTestId("page-header")).toBeInTheDocument();
      expect(screen.getByText("Datenverwaltung")).toBeInTheDocument();
    });
  });

  it("hides sections when user lacks permissions", async () => {
    mockCounts({
      ...mockCountsResponse.data,
      permissions: {
        canViewStudents: true,
        canViewTeachers: false,
        canViewRooms: false,
        canViewActivities: false,
        canViewGroups: false,
        canViewRoles: false,
        canViewDevices: false,
        canViewPermissions: false,
        canViewTimetables: false,
      },
    });

    render(<DatabasePage />);

    await waitFor(() => {
      expect(screen.getByText("Kinderdaten")).toBeInTheDocument();
      expect(screen.queryByText("Personal")).not.toBeInTheDocument();
      expect(screen.queryByText("Räume")).not.toBeInTheDocument();
    });
  });

  it("handles 401 unauthorized response gracefully", async () => {
    mockCounts(null);

    render(<DatabasePage />);

    // Should not crash and not show any sections
    await waitFor(() => {
      expect(screen.queryByText("Kinderdaten")).not.toBeInTheDocument();
    });
  });

  it("handles 403 forbidden response gracefully", async () => {
    mockCounts(null);

    render(<DatabasePage />);

    // Should not crash and not show any sections
    await waitFor(() => {
      expect(screen.queryByText("Kinderdaten")).not.toBeInTheDocument();
    });
  });

  it("handles fetch error gracefully", async () => {
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const error = new Error("Network error");
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

describe("baseDataSections configuration", () => {
  it("defines correct sections", () => {
    const sectionIds = [
      "students",
      "teachers",
      "rooms",
      "activities",
      "groups",
      "roles",
      "devices",
      "permissions",
    ];

    const sections = [
      {
        id: "students",
        title: "Kinderdaten",
        href: "/database/students",
      },
      {
        id: "teachers",
        title: "Personal",
        href: "/database/personal",
      },
      {
        id: "rooms",
        title: "Räume",
        href: "/database/rooms",
      },
      {
        id: "activities",
        title: "Aktivitäten",
        href: "/database/activities",
      },
      {
        id: "groups",
        title: "Gruppen",
        href: "/database/groups",
      },
      {
        id: "roles",
        title: "Rollen",
        href: "/database/roles",
      },
      {
        id: "devices",
        title: "Geräte",
        href: "/database/devices",
      },
      {
        id: "permissions",
        title: "Berechtigungen",
        href: "/database/permissions",
      },
    ];

    expect(sections.map((s) => s.id)).toEqual(sectionIds);
    expect(sections).toHaveLength(8);
  });
});
