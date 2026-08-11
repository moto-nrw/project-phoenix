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

import { berlinTodayISO, parseISODate, toISODate } from "~/lib/date-helpers";

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
  useAttendanceWebEnabled: vi.fn(() => true),
  useNFCEnabled: () => true,
  TenantProvider: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
  useBreadcrumb: () => ({ breadcrumb: {}, setBreadcrumb: vi.fn() }),
}));

// The header is stubbed, but the filter configs it receives are the page's
// contract for which filters exist on the selected day — exposed as data so a
// test can read them without rendering the real panel.
vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: ({
    filters,
  }: {
    filters?: {
      id: string;
      value?: unknown;
      options?: { value: string }[];
    }[];
  }) => (
    <div
      data-testid="header"
      data-filters={JSON.stringify(
        (filters ?? []).map((filter) => ({
          id: filter.id,
          value: filter.value,
          options: filter.options?.map((option) => option.value),
        })),
      )}
    />
  ),
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

// Mutable clock so tests can advance the page across midnight via rerender.
let clockNow = new Date();

vi.mock("~/lib/pickup-helpers", () => ({
  useMinuteClock: () => clockNow,
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

// The planning horizon ends with the Sunday closing the CURRENT week (#1939),
// so the number of selectable future days depends on today's weekday: on a
// Sunday not even tomorrow is selectable. Anchoring on the real date would
// therefore flake this suite every Sunday, so it pins the clock to a fixed
// Monday (2026-06-01, 10:00 Berlin). Both the system time and the mocked
// useMinuteClock must be pinned: the page reads "today" from the clock for
// rendering, but the explicit-date initializer calls berlinTodayISO() with no
// argument, i.e. straight off the system clock.
const FIXED_NOW = new Date("2026-06-01T08:00:00Z");

// Anchored on the Berlin day — the page's "today" — so the tests stay stable
// when the runner's timezone disagrees with Europe/Berlin around midnight.
function isoDaysFromToday(days: number): string {
  const date = parseISODate(berlinTodayISO(FIXED_NOW));
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

/**
 * An instant whose Berlin calendar day is `days` after today, independent of
 * the runner's timezone: local midnight of the target day plus 12 hours is
 * within the same Berlin day for any runner within UTC±10.
 */
function clockAtBerlinDay(days: number): Date {
  const target = parseISODate(isoDaysFromToday(days));
  return new Date(target.getTime() + 12 * 3600 * 1000);
}

beforeEach(() => {
  vi.clearAllMocks();
  currentSearch = new URLSearchParams();
  localStorage.clear();
  // shouldAdvanceTime keeps timers running so waitFor / SWR still settle.
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(FIXED_NOW);
  clockNow = FIXED_NOW;

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
  vi.useRealTimers();
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

  it("rejects a date beyond the planning horizon and strips it from the URL", async () => {
    // The backend answers dates past the Sunday closing the CURRENT calendar
    // week with 400 (materialized timetable horizon, #1939). The initializer
    // must collapse such a URL date to today instead of firing a request the
    // backend rejects. Today is pinned to a Monday, so the horizon ends at +6
    // and +7 (the next Monday) is the first rejected day.
    const beyondHorizon = isoDaysFromToday(7);
    window.history.replaceState(
      {},
      "",
      `/students/search?date=${beyondHorizon}`,
    );
    currentSearch = new URLSearchParams({ date: beyondHorizon });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(mockGetStudents).toHaveBeenCalled();
    });

    const calls = mockGetStudents.mock.calls.map(
      (c) => c[0] as Record<string, unknown>,
    );
    expect(calls.every((c) => c.date === undefined)).toBe(true);
    await waitFor(() => {
      expect(new URLSearchParams(window.location.search).has("date")).toBe(
        false,
      );
    });

    window.history.replaceState({}, "", "/");
  });

  it("accepts a date at the end of the planning horizon", async () => {
    // Today is pinned to a Monday, so +6 is the Sunday closing the current
    // week — the far edge of the horizon must stay selectable.
    const withinHorizon = isoDaysFromToday(6);
    currentSearch = new URLSearchParams({ date: withinHorizon });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(mockGetStudents).toHaveBeenCalled();
    });

    const calls = mockGetStudents.mock.calls.map(
      (c) => c[0] as Record<string, unknown>,
    );
    expect(calls.some((c) => c.date === withinHorizon)).toBe(true);
  });

  it("strips a rejected past/aged date from the address bar on mount", async () => {
    // A bookmark or hand-edited URL carrying a past date. The initializer
    // rejects it (planning is future-only) and the page uses today; the mount
    // reconcile must also remove the stale param so the address bar and copied
    // links stop advertising a date the page no longer uses (#1939).
    const past = isoDaysFromToday(-3);
    window.history.replaceState({}, "", `/students/search?date=${past}`);
    currentSearch = new URLSearchParams({ date: past });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(new URLSearchParams(window.location.search).has("date")).toBe(
        false,
      );
    });

    window.history.replaceState({}, "", "/");
  });

  it("shows the planning badge instead of the live presence badge on other days", async () => {
    const tomorrow = isoDaysFromToday(1);
    currentSearch = new URLSearchParams({ date: tomorrow });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    expect(screen.getByTestId("planning-badge")).toHaveTextContent("Kommt");
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

  it("keeps following today when the implicit view crosses midnight", async () => {
    const { rerender } = render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    clockNow = clockAtBerlinDay(1);
    rerender(<StudentSearchPage />);

    // Still the implicit today view: no date sent, no planning hint, live badge.
    const calls = mockGetStudents.mock.calls.map(
      (c) => c[0] as Record<string, unknown>,
    );
    expect(calls.every((c) => c.date === undefined)).toBe(true);
    expect(
      screen.queryByText(/Geplante Anwesenheit für/),
    ).not.toBeInTheDocument();
    expect(screen.getByTestId("presence-badge")).toBeInTheDocument();
  });

  it("switches to live semantics when an explicit future date becomes today", async () => {
    const tomorrow = isoDaysFromToday(1);
    currentSearch = new URLSearchParams({ date: tomorrow });

    const { rerender } = render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("planning-badge")).toBeInTheDocument();
    });
    expect(
      mockGetStudents.mock.calls.some(
        (c) => (c[0] as Record<string, unknown>).date === tomorrow,
      ),
    ).toBe(true);

    clockNow = clockAtBerlinDay(1);
    rerender(<StudentSearchPage />);

    // The selected day is now today: a fresh request without the date param
    // (today semantics) and the live presence badge replace the planning view.
    await waitFor(() => {
      expect(
        mockGetStudents.mock.calls.some(
          (c) => (c[0] as Record<string, unknown>).date === undefined,
        ),
      ).toBe(true);
    });
    expect(screen.getByTestId("presence-badge")).toBeInTheDocument();
    expect(screen.queryByTestId("planning-badge")).not.toBeInTheDocument();
  });

  it("drops an explicit date once it has aged into the past (tab open across midnight)", async () => {
    const tomorrow = isoDaysFromToday(1);
    currentSearch = new URLSearchParams({ date: tomorrow });

    const { rerender } = render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("planning-badge")).toBeInTheDocument();
    });

    // Advance two Berlin days: the selected day (tomorrow) is now firmly in the
    // past. The stale date must not keep driving the backend request or the
    // view — the page falls back to live "Heute" semantics (#1939).
    clockNow = clockAtBerlinDay(2);
    rerender(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("presence-badge")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("planning-badge")).not.toBeInTheDocument();

    // The last request carries no date (today semantics), so the aged past date
    // is no longer driving the backend query.
    const dates = mockGetStudents.mock.calls.map(
      (c) => (c[0] as Record<string, unknown>).date,
    );
    expect(dates.at(-1)).toBeUndefined();
  });

  // The mocked useSWRAuth runs every fetcher regardless of the key, so the
  // assertion is on the SWR key itself — a null key is what disables the
  // request in production (#1939).
  const trackingKeys = () =>
    mockUseSWRAuth.mock.calls
      .map((c) => c[0] as string | null)
      .filter(
        (key) =>
          typeof key === "string" && key.startsWith("tracking-indicators-"),
      );

  it("requests tracking indicators for today", async () => {
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    expect(trackingKeys().length).toBeGreaterThan(0);
  });

  it("skips the tracking-indicator request on a planning date", async () => {
    currentSearch = new URLSearchParams({ date: isoDaysFromToday(1) });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    // Tracking indicators describe today's activity participation and are not
    // rendered on a planning date, so the request must not be made at all.
    expect(trackingKeys()).toEqual([]);
  });

  function statusFilterOptions(): string[] {
    const raw =
      screen.getByTestId("header").getAttribute("data-filters") ?? "[]";
    const filters = JSON.parse(raw) as {
      id: string;
      options?: string[];
    }[];
    return filters.find((filter) => filter.id === "attendance")?.options ?? [];
  }

  it("offers every status for today", async () => {
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    expect(statusFilterOptions()).toEqual([
      "all",
      "anwesend",
      "abwesend",
      "unterwegs",
      "schulhof",
      "krank",
      "klassenfahrt",
      "entschuldigt",
    ]);
  });

  it("keeps only the planned absence statuses on a planning date", async () => {
    currentSearch = new URLSearchParams({ date: isoDaysFromToday(1) });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    // krank/klassenfahrt/entschuldigt come from the status days of the selected
    // date; the location-derived buckets would answer for today only (#1939).
    expect(statusFilterOptions()).toEqual([
      "all",
      "krank",
      "klassenfahrt",
      "entschuldigt",
    ]);
  });
});
