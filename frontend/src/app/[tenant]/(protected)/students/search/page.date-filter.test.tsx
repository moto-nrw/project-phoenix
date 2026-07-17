/**
 * Behavior tests for the planning-date selection on the Kindersuche (#1939).
 * Focus: the `date` URL param drives the backend fetch, defaults to today,
 * survives the drill-down back-link, and rejects malformed values.
 *
 * Mock setup mirrors page.room-filter.test.tsx — deliberately minimal.
 */
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { parseISODate, toISODate } from "~/lib/date-helpers";

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

vi.mock("~/components/ui/date-picker", () => ({
  DatePicker: () => <div data-testid="date-picker" />,
}));

vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message }: { message: string }) => <div>{message}</div>,
}));

vi.mock("@/components/ui/location-badge", () => ({
  LocationBadge: () => <span />,
}));

vi.mock("@/components/ui/student-presence-badge", () => ({
  StudentPresenceBadge: () => <span data-testid="presence-badge" />,
}));

vi.mock("~/components/ui/data-table", () => ({
  DataTableStatusBadge: ({
    active,
    activeLabel,
    inactiveLabel,
  }: {
    active: boolean;
    activeLabel: string;
    inactiveLabel: string;
  }) => (
    <span data-testid="planning-badge">
      {active ? activeLabel : inactiveLabel}
    </span>
  ),
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
    locationBadge,
  }: {
    studentId: string;
    onClick: () => void;
    locationBadge: React.ReactNode;
  }) => (
    <button data-testid={`student-card-${studentId}`} onClick={onClick}>
      card-{studentId}
      {locationBadge}
    </button>
  ),
  SchoolClassIcon: () => <span />,
  GroupIcon: () => <span />,
  StudentInfoRow: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  PickupTimeRow: () => <div />,
  ArrivalTimeRow: () => <div />,
  StudentAbsenceRow: ({ label }: { label: string }) => <div>{label}</div>,
  StudentPendingExcusedRow: () => <div />,
}));

import StudentSearchPage from "./page";

function isoDaysFromToday(days: number): string {
  const date = parseISODate(toISODate(new Date()));
  date.setDate(date.getDate() + days);
  return toISODate(date);
}

const mockStudent = {
  id: "7",
  first_name: "Max",
  second_name: "Mustermann",
  school_class: "1a",
  current_location: "Anwesend",
  has_full_access: true,
  day_planning_status: "comes_today",
};

beforeEach(() => {
  vi.clearAllMocks();
  currentSearch = new URLSearchParams();
  localStorage.clear();

  mockUseImmutableSWR.mockReturnValue({
    data: [],
    isLoading: false,
    error: null,
  });

  mockUseSWRAuth.mockImplementation(
    (key: string | null, fetcher?: () => Promise<unknown>) => {
      if (fetcher) {
        void fetcher().catch(() => undefined);
      }
      if (key === "search-rooms-list") {
        return { data: [], isLoading: false, error: null };
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

describe("StudentSearchPage — planning date (#1939)", () => {
  it("forwards a non-today date from the URL into studentService.getStudents", async () => {
    const tomorrow = isoDaysFromToday(1);
    currentSearch = new URLSearchParams({ date: tomorrow });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(mockGetStudents).toHaveBeenCalled();
    });

    const calls = mockGetStudents.mock.calls.map(
      (c) => c[0] as Record<string, unknown>,
    );
    expect(calls.some((c) => c.date === tomorrow)).toBe(true);
  });

  it("omits the date for today so today's requests stay unchanged", async () => {
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(mockGetStudents).toHaveBeenCalled();
    });

    const calls = mockGetStudents.mock.calls.map(
      (c) => c[0] as Record<string, unknown>,
    );
    expect(calls.every((c) => c.date === undefined)).toBe(true);
  });

  it("falls back to today when the date param is malformed", async () => {
    currentSearch = new URLSearchParams({ date: "18.07.2026" });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(mockGetStudents).toHaveBeenCalled();
    });

    const calls = mockGetStudents.mock.calls.map(
      (c) => c[0] as Record<string, unknown>,
    );
    expect(calls.every((c) => c.date === undefined)).toBe(true);
  });

  it("shows the planning badge instead of the live presence badge on other days", async () => {
    const tomorrow = isoDaysFromToday(1);
    currentSearch = new URLSearchParams({ date: tomorrow });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    expect(screen.getByTestId("planning-badge")).toHaveTextContent("Erwartet");
    expect(screen.queryByTestId("presence-badge")).not.toBeInTheDocument();

    // The date-context hint names the selected day, never "heute".
    expect(screen.getByText(/Geplante Anwesenheit für/)).toBeInTheDocument();
  });

  it("keeps the live presence badge for today", async () => {
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    expect(screen.getByTestId("presence-badge")).toBeInTheDocument();
    expect(screen.queryByTestId("planning-badge")).not.toBeInTheDocument();
  });

  it("preserves the date in the back-link when drilling into a child", async () => {
    const tomorrow = isoDaysFromToday(1);
    currentSearch = new URLSearchParams({ date: tomorrow });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("student-card-7"));

    const target = mockPush.mock.calls[0]?.[0] as string;
    expect(target.startsWith("/students/7?from=")).toBe(true);
    const fromParam = decodeURIComponent(target.split("from=")[1] ?? "");
    expect(fromParam).toContain(`date=${tomorrow}`);
  });
});
