/**
 * Meine Gruppe und Aktuelle Aufsicht als Reiter von „Alle Kinder“, wenn die
 * Schule Kinder nur noch dort finden lässt (#1321). Die Reiter setzen die
 * vorhandenen Filter Gruppe und Raum; die Menüeinträge halten Gruppen- und
 * Aufsichtsseite erreichbar.
 *
 * Mocks wie im Raumfilter-Test, mit umschaltbarer Einstellung, eigenen
 * Gruppen und eigener Aufsicht.
 */
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const {
  mockGetStudents,
  mockUseSWRAuth,
  mockUseImmutableSWR,
  mockPush,
  flags,
} = vi.hoisted(() => ({
  mockGetStudents: vi.fn(),
  mockUseSWRAuth: vi.fn(),
  mockUseImmutableSWR: vi.fn(),
  mockPush: vi.fn(),
  flags: { childSearchOnly: false },
}));

let currentSearch = new URLSearchParams();

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { token: "t", roles: ["user"] } },
    status: "authenticated",
  })),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: vi.fn() }),
  useSearchParams: () => currentSearch,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mockPush }),
}));

vi.mock("~/lib/supervision-context", () => ({
  useOptionalSupervision: () => ({
    hasGroups: true,
    isSupervising: true,
    isLoadingGroups: false,
    isLoadingSupervision: false,
    overviewEnabled: false,
    supervisedRooms: [{ id: "42", name: "Werkraum", groupId: "900" }],
    ownSupervisedRooms: [{ id: "42", name: "Werkraum", groupId: "900" }],
    groups: [{ id: "5", name: "Bären", is_personal: true }],
    refresh: () => undefined,
  }),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantRoutingModeSafe: () => "subdomain",
  useChildSearchOnlyNavigation: () => flags.childSearchOnly,
  useTenant: () => ({ tenantSlug: "t", tenant: null }),
  useTenantSafe: () => ({
    tenantSlug: "t",
    tenant: { studentPhotosEnabled: true },
  }),
  useTenantSlugSafe: () => "t",
  usePresenceMode: () => "detailed",
  useAttendanceWebEnabled: vi.fn(() => true),
  useNFCEnabled: () => true,
  useOpenCareGroupMode: () => false,
  TenantProvider: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
  useBreadcrumb: () => ({ breadcrumb: {}, setBreadcrumb: vi.fn() }),
}));

vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: () => <div data-testid="header" />,
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading" />,
}));

vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message }: { message: string }) => <div>{message}</div>,
}));

vi.mock("@/components/ui/location-badge", () => ({
  LocationBadge: () => <span />,
}));

vi.mock("@/components/ui/student-presence-badge", () => ({
  StudentPresenceBadge: () => <span />,
}));

vi.mock("~/components/students/tracking-indicators", () => ({
  TrackingIndicators: () => <span />,
}));

vi.mock("~/components/students/school-checkin-fab", () => ({
  SchoolCheckinFab: () => <span />,
}));

vi.mock("~/components/students/school-checkin-mode-mobile", () => ({
  SchoolCheckinModeMobile: () => <span />,
}));

vi.mock("~/lib/hooks/use-school-checkin-mode", () => ({
  useSchoolCheckinMode: () => ({
    isActive: false,
    toggleActive: vi.fn(),
    deactivate: vi.fn(),
    pendingIds: new Set<string>(),
    successCount: 0,
    toggle: vi.fn(),
    selectionActive: false,
    setSelectionActive: vi.fn(),
    selectedIds: new Set<string>(),
    toggleSelected: vi.fn(),
    clearSelection: vi.fn(),
    isBulkRunning: false,
    runBulk: vi.fn(),
  }),
  deriveCheckinState: () => "unknown",
  checkoutConfirmationRoom: () => null,
}));

vi.mock("~/lib/location-helper", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/location-helper")>();
  return {
    ...actual,
    isHomeLocation: () => false,
    isPresentLocation: () => true,
    isTransitLocation: () => false,
    isSchoolyardLocation: () => false,
  };
});

vi.mock("~/lib/student-helpers", () => ({
  SCHOOL_YEAR_FILTER_OPTIONS: [{ value: "all", label: "Alle" }],
  getSchoolYear: () => "1",
}));

vi.mock("~/lib/pickup-helpers", () => ({
  useMinuteClock: () => new Date(),
}));

vi.mock("~/lib/active-api", () => ({
  activeService: {
    getTrackingIndicators: vi.fn(() =>
      Promise.resolve({ labels: [], results: {} }),
    ),
  },
}));

vi.mock("~/lib/hooks/use-user-context", () => ({
  useUserContext: () => ({
    userContext: {
      educationalGroupIds: ["5"],
      educationalGroupRoomNames: [],
      supervisedRoomNames: ["Veralteter Raum"],
      supervisedGroups: [
        {
          id: "900",
          name: "Werken",
          roomId: "91",
          room: { id: "91", name: "Veralteter Raum" },
        },
      ],
    },
  }),
}));

vi.mock("~/lib/api", () => ({
  studentService: {
    getStudents: (...args: unknown[]) => mockGetStudents(...args),
  },
  groupService: {
    getGroups: vi.fn(() => Promise.resolve([])),
  },
  roomService: {
    getRooms: vi.fn(() => Promise.resolve([])),
  },
}));

vi.mock("~/lib/swr", () => ({
  useImmutableSWR: (...args: unknown[]) => mockUseImmutableSWR(...args),
  useSWRAuth: (key: string | null, fetcher?: () => Promise<unknown>) =>
    mockUseSWRAuth(key, fetcher),
  mutate: vi.fn(),
  useTenantMutate: () => vi.fn(),
}));

vi.mock("~/components/students/student-card", () => ({
  StudentCard: ({
    studentId,
    onClick,
  }: {
    studentId: string;
    onClick: () => void;
  }) => (
    <button
      type="button"
      data-testid={`student-card-${studentId}`}
      onClick={onClick}
    >
      card-{studentId}
    </button>
  ),
  SchoolClassIcon: () => <span />,
  GroupIcon: () => <span />,
  DepartureModeIcon: () => <span />,
  StudentInfoRow: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  PickupTimeRow: () => <div />,
  ArrivalTimeRow: () => <div />,
}));

import StudentSearchPage from "./page";

const mockStudent = {
  id: "7",
  first_name: "Max",
  second_name: "Mustermann",
  school_class: "1a",
  current_location: "Anwesend",
  has_full_access: true,
};

beforeEach(() => {
  vi.clearAllMocks();
  currentSearch = new URLSearchParams();
  localStorage.clear();
  flags.childSearchOnly = true;

  // Default: no in-flight tracking-indicators result.
  mockUseImmutableSWR.mockReturnValue({
    data: [],
    isLoading: false,
    error: null,
  });

  // Capture the SWR fetcher and invoke it so the studentService spy actually
  // sees the call — useSWRAuth's real impl runs the fetcher synchronously
  // for the test path; we mimic that here.
  mockUseSWRAuth.mockImplementation(
    (key: string | null, fetcher?: () => Promise<unknown>) => {
      if (fetcher) {
        // Fire-and-forget — we only assert on mockGetStudents call shape.
        void fetcher().catch(() => undefined);
      }
      if (key === "search-rooms-list") {
        return {
          data: [],
          isLoading: false,
          error: null,
        };
      }
      return {
        data: { students: [mockStudent] },
        isLoading: false,
        error: null,
      };
    },
  );

  mockGetStudents.mockResolvedValue({ students: [mockStudent] });
});

afterEach(() => {
  cleanup();
  localStorage.clear();
});

function lastStudentFilters(): Record<string, unknown> | undefined {
  const calls = mockGetStudents.mock.calls;
  return calls[calls.length - 1]?.[0] as Record<string, unknown> | undefined;
}

describe("StudentSearchPage — Kinder nur über Alle Kinder (#1321)", () => {
  it("shows no tabs while the school keeps the separate entries", () => {
    flags.childSearchOnly = false;

    render(<StudentSearchPage />);

    expect(screen.queryByRole("tab", { name: "Meine Gruppe" })).toBeNull();
    expect(screen.queryByRole("tab", { name: "Meine Aufsicht" })).toBeNull();
  });

  it("offers Alle, Meine Gruppe and Meine Aufsicht, starting on Alle", () => {
    render(<StudentSearchPage />);

    expect(screen.getByRole("tab", { name: "Alle" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByRole("tab", { name: "Meine Gruppe" })).toHaveAttribute(
      "aria-selected",
      "false",
    );
    expect(screen.getByRole("tab", { name: "Meine Aufsicht" })).toBeTruthy();
  });

  it("filters to the own groups on Meine Gruppe", async () => {
    render(<StudentSearchPage />);

    fireEvent.click(screen.getByRole("tab", { name: "Meine Gruppe" }));

    await waitFor(() => {
      expect(lastStudentFilters()?.groupId).toEqual(["5"]);
    });
    expect(lastStudentFilters()?.roomId).toBeUndefined();
    expect(window.location.search).toContain("group_id=5");
    expect(screen.getByRole("tab", { name: "Meine Gruppe" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("filters to the current supervised room instead of cached user context", async () => {
    render(<StudentSearchPage />);

    fireEvent.click(screen.getByRole("tab", { name: "Meine Aufsicht" }));

    await waitFor(() => {
      expect(lastStudentFilters()?.roomId).toBe("42");
    });
    expect(lastStudentFilters()?.groupId).toEqual([]);

    fireEvent.click(screen.getByRole("tab", { name: "Alle" }));

    await waitFor(() => {
      expect(lastStudentFilters()?.roomId).toBeUndefined();
    });
    expect(lastStudentFilters()?.groupId).toEqual([]);
  });

  it("marks the tab that matches a filter restored from the URL", () => {
    currentSearch = new URLSearchParams({
      room_id: "42",
      room_name: "Werkraum",
    });

    render(<StudentSearchPage />);

    expect(screen.getByRole("tab", { name: "Meine Aufsicht" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("keeps the scope tabs keyboard reachable for a stale room filter", () => {
    currentSearch = new URLSearchParams({
      room_id: "91",
      room_name: "Veralteter Raum",
    });

    render(<StudentSearchPage />);

    const all = screen.getByRole("tab", { name: "Alle" });
    expect(all).toHaveAttribute("aria-selected", "false");
    expect(all).toHaveAttribute("tabindex", "0");
    expect(screen.getByRole("tab", { name: "Meine Gruppe" })).toHaveAttribute(
      "aria-selected",
      "false",
    );
    expect(screen.getByRole("tab", { name: "Meine Aufsicht" })).toHaveAttribute(
      "aria-selected",
      "false",
    );
  });

  it("keeps the group and supervision pages reachable from the menu", async () => {
    render(<StudentSearchPage />);

    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));

    expect(
      await screen.findByRole("menuitem", { name: /Zur Aufsicht/ }),
    ).toHaveAttribute("href", "/active-supervisions");
    expect(
      screen.getByRole("menuitem", { name: /Zur Gruppenseite/ }),
    ).toHaveAttribute("href", "/ogs-groups");
  });

  it("adds no menu entries while the school keeps the separate entries", async () => {
    flags.childSearchOnly = false;

    render(<StudentSearchPage />);
    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));

    expect(
      await screen.findByRole("menuitem", { name: /Abwesenheiten/ }),
    ).toBeTruthy();
    expect(screen.queryByRole("menuitem", { name: /Zur Aufsicht/ })).toBeNull();
    expect(
      screen.queryByRole("menuitem", { name: /Zur Gruppenseite/ }),
    ).toBeNull();
  });

  it("counts the children in the OGS in the status line", async () => {
    render(<StudentSearchPage />);

    // Das Kind der Mocks ist „Anwesend“, also in einem Raum der OGS.
    expect(
      await screen.findByText(
        "1 Kind · 1 in der OGS · 0 in der Schule · 0 zuhause · 0 krank",
      ),
    ).toBeTruthy();
  });
});
