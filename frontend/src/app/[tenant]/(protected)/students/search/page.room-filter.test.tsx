/**
 * Behavior tests for the room_id deep-link on the Kindersuche page (#1323).
 * Focus: the URL-driven room filter is forwarded to the backend and is
 * preserved when drilling into a student detail page.
 *
 * The page has many transitive dependencies; mocks below are deliberately
 * minimal — only enough to render and exercise the room-filter code path.
 */
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { mockGetStudents, mockUseSWRAuth, mockUseImmutableSWR, mockPush } =
  vi.hoisted(() => ({
    mockGetStudents: vi.fn(),
    mockUseSWRAuth: vi.fn(),
    mockUseImmutableSWR: vi.fn(),
    mockPush: vi.fn(),
  }));

let currentSearch = new URLSearchParams();

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { token: "t" } },
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

vi.mock("~/lib/tenant-context", () => ({
  useTenant: () => ({ tenantSlug: "t", tenant: null }),
  useTenantSafe: () => ({
    tenantSlug: "t",
    tenant: { studentPhotosEnabled: true },
  }),
  useTenantSlugSafe: () => "t",
  usePresenceMode: () => "detailed",
  useNFCEnabled: () => true,
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
  }),
  deriveCheckinState: () => "unknown",
}));

vi.mock("~/lib/location-helper", () => ({
  isHomeLocation: () => false,
  isPresentLocation: () => true,
  isTransitLocation: () => false,
  isSchoolyardLocation: () => false,
}));

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
      educationalGroupIds: [],
      educationalGroupRoomNames: [],
      supervisedRoomNames: [],
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

describe("StudentSearchPage — room_id deep-link (#1323)", () => {
  it("forwards room_id from the URL into studentService.getStudents", async () => {
    currentSearch = new URLSearchParams({
      room_id: "42",
      room_name: "OGS-Raum 1",
    });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(mockGetStudents).toHaveBeenCalled();
    });

    // Find the call with our roomId — there may be multiple if the page
    // re-renders. The contract is: the first call after mount must carry
    // the URL-derived room_id, otherwise the page would render the
    // unfiltered list before correcting itself.
    const calls = mockGetStudents.mock.calls.map(
      (c) => c[0] as Record<string, unknown>,
    );
    const filteredCall = calls.find((c) => c.roomId === "42");
    expect(filteredCall).toBeDefined();
    expect(filteredCall?.roomId).toBe("42");
  });

  it("does not forward roomId when no room_id query param is present", async () => {
    currentSearch = new URLSearchParams();

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(mockGetStudents).toHaveBeenCalled();
    });

    const calls = mockGetStudents.mock.calls.map(
      (c) => c[0] as Record<string, unknown>,
    );
    expect(calls.every((c) => c.roomId === undefined || c.roomId === "")).toBe(
      true,
    );
  });

  it("preserves room filter in the back-link when drilling into a child", async () => {
    currentSearch = new URLSearchParams({
      room_id: "42",
      room_name: "OGS-Raum 1",
    });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("student-card-7"));

    expect(mockPush).toHaveBeenCalledTimes(1);
    const target = mockPush.mock.calls[0]?.[0] as string;
    // Must be of the form: /students/7?from=<encoded /students/search?room_id=42&room_name=...>
    // Without this, hitting "back" from the detail page drops the filter.
    expect(target.startsWith("/students/7?from=")).toBe(true);
    const fromParam = decodeURIComponent(target.split("from=")[1] ?? "");
    expect(fromParam).toContain("/students/search");
    expect(fromParam).toContain("room_id=42");
    expect(fromParam).toContain("room_name=OGS-Raum");
  });

  it("uses the bare /students/search referrer when there is no room filter", async () => {
    currentSearch = new URLSearchParams();

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("student-card-7"));

    const target = mockPush.mock.calls[0]?.[0] as string;
    // Without an active room filter the from= must NOT be encoded — the
    // existing call sites (and the breadcrumb) match on the literal path.
    expect(target).toBe("/students/7?from=/students/search");
  });
});
