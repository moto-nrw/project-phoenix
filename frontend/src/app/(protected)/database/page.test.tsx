import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import DatabasePage from "./page";

const mockSession = {
  user: {
    id: "1",
    name: "Test User",
    email: "test@test.com",
    token: "test-token",
  },
  expires: "2099-12-31",
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

vi.mock("~/components/ui/loading", () => ({
  Loading: ({ fullPage }: { fullPage?: boolean }) => (
    <div data-testid="loading" data-fullpage={fullPage} aria-label="Lädt..." />
  ),
}));

vi.mock("~/hooks/useIsMobile", () => ({
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
import { useIsMobile } from "~/hooks/useIsMobile";

describe("DatabasePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });
    vi.mocked(useIsMobile).mockReturnValue(false);
    vi.mocked(global.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockCountsResponse),
    } as Response);
  });

  it("renders the database page with layout", async () => {
    render(<DatabasePage />);

    // Page renders without layout wrapper (now in app layout)
    await waitFor(() => {
      expect(screen.getByText("Kinder")).toBeInTheDocument();
    });
  });

  it("displays loading state initially", () => {
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "loading",
      update: vi.fn(),
    });

    render(<DatabasePage />);

    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("displays data sections with counts after loading", async () => {
    render(<DatabasePage />);

    await waitFor(() => {
      expect(screen.getByText("Kinder")).toBeInTheDocument();
      expect(screen.getByText("Betreuer")).toBeInTheDocument();
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
    vi.mocked(global.fetch).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          ...mockCountsResponse,
          data: {
            ...mockCountsResponse.data,
            students: 1,
          },
        }),
    } as Response);

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
    vi.mocked(global.fetch).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          ...mockCountsResponse,
          data: {
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
            },
          },
        }),
    } as Response);

    render(<DatabasePage />);

    await waitFor(() => {
      expect(screen.getByText("Kinder")).toBeInTheDocument();
      expect(screen.queryByText("Betreuer")).not.toBeInTheDocument();
      expect(screen.queryByText("Räume")).not.toBeInTheDocument();
    });
  });

  it("handles 401 unauthorized response gracefully", async () => {
    vi.mocked(global.fetch).mockResolvedValue({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ error: "Unauthorized" }),
    } as Response);

    render(<DatabasePage />);

    // Should not crash and not show any sections
    await waitFor(() => {
      expect(screen.queryByText("Kinder")).not.toBeInTheDocument();
    });
  });

  it("handles 403 forbidden response gracefully", async () => {
    vi.mocked(global.fetch).mockResolvedValue({
      ok: false,
      status: 403,
      json: () => Promise.resolve({ error: "Forbidden" }),
    } as Response);

    render(<DatabasePage />);

    // Should not crash and not show any sections
    await waitFor(() => {
      expect(screen.queryByText("Kinder")).not.toBeInTheDocument();
    });
  });

  it("handles fetch error gracefully", async () => {
    // eslint-disable-next-line @typescript-eslint/no-empty-function
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    vi.mocked(global.fetch).mockRejectedValue(new Error("Network error"));

    render(<DatabasePage />);

    await waitFor(() => {
      expect(consoleSpy).toHaveBeenCalledWith(
        "Error fetching counts:",
        expect.any(Error),
      );
    });

    consoleSpy.mockRestore();
  });

  it("displays descriptions for each section", async () => {
    render(<DatabasePage />);

    await waitFor(() => {
      expect(
        screen.getByText("Kinderdaten verwalten und bearbeiten"),
      ).toBeInTheDocument();
      expect(
        screen.getByText("Daten der Betreuer und Zuordnungen verwalten"),
      ).toBeInTheDocument();
      expect(
        screen.getByText("Räume und Ausstattung verwalten"),
      ).toBeInTheDocument();
    });
  });

  it("has correct links for each section", async () => {
    render(<DatabasePage />);

    await waitFor(() => {
      const studentsLink = screen.getByRole("link", { name: /Kinder/i });
      expect(studentsLink).toHaveAttribute("href", "/database/students");

      const teachersLink = screen.getByRole("link", { name: /Betreuer/i });
      expect(teachersLink).toHaveAttribute("href", "/database/teachers");

      const roomsLink = screen.getByRole("link", { name: /Räume/i });
      expect(roomsLink).toHaveAttribute("href", "/database/rooms");
    });
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
        title: "Kinder",
        href: "/database/students",
      },
      {
        id: "teachers",
        title: "Betreuer",
        href: "/database/teachers",
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
