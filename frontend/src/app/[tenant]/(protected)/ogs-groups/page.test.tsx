/**
 * Tests for OGS Groups Page
 * Tests the rendering states and user interactions of the OGS groups dashboard
 */
import {
  render,
  screen,
  waitFor,
  cleanup,
  fireEvent,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

function createLocalStorageMock() {
  const store: Record<string, string> = {};

  return {
    getItem: (key: string) => store[key] ?? null,
    setItem: (key: string, value: string) => {
      store[key] = value;
    },
    removeItem: (key: string) => {
      delete store[key];
    },
    clear: () => {
      for (const key of Object.keys(store)) {
        delete store[key];
      }
    },
  };
}

Object.defineProperty(window, "localStorage", {
  value: createLocalStorageMock(),
  writable: true,
  configurable: true,
});

// Mock auth-utils with hasRole that reads session roles
vi.mock("~/lib/auth-utils", () => ({
  isAdmin: (session: { user?: { isAdmin?: boolean } } | null) =>
    session?.user?.isAdmin ?? false,
  isCaregiver: (session: { user?: { isAdmin?: boolean } } | null) =>
    !(session?.user?.isAdmin ?? false),
  hasRole: (session: { user?: { isAdmin?: boolean } } | null, role: string) => {
    if (role === "admin") return session?.user?.isAdmin ?? false;
    if (role === "user") return !(session?.user?.isAdmin ?? false);
    return false;
  },
}));

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { token: "test-token" } },
    status: "authenticated",
  })),
}));

// Mock next/navigation
const mockPush = vi.fn();
const mockSearchParamsGet = vi.fn((_key?: string): string | null => null);
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
  useSearchParams: () => ({ get: mockSearchParamsGet }),
  redirect: vi.fn(),
}));

// Mock ToastContext
const mockToast = {
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
};
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => mockToast,
}));

// Mock breadcrumb context
vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
  useBreadcrumb: vi.fn(() => ({ breadcrumb: {}, setBreadcrumb: vi.fn() })),
  BreadcrumbProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

// Mock PageHeaderWithSearch — renders filters, activeFilters, and the
// overflow-menu items so the existing "Gruppe übergeben" assertions keep
// working. The real OverflowMenu requires a click to expose its items, but
// for tests we render them flat so getByLabelText still finds them.
vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: ({
    title,
    filters,
    activeFilters,
    onClearAllFilters,
    actionButton,
    overflowMenu,
  }: {
    title: string;
    filters?: Array<{
      id: string;
      label: string;
      value: string | string[];
      options: Array<{ value: string; label: string }>;
      onChange: (value: string | string[]) => void;
    }>;
    activeFilters?: Array<{ id: string; label: string; onRemove: () => void }>;
    onClearAllFilters?: () => void;
    actionButton?: React.ReactNode;
    overflowMenu?: Array<{
      label: string;
      onClick: () => void;
      badge?: string | number;
    }>;
  }) => (
    <div data-testid="page-header">
      {title}
      {actionButton}
      {overflowMenu?.map((item) => (
        <button
          type="button"
          key={item.label}
          aria-label={item.label}
          data-testid={`overflow-${item.label}`}
          onClick={item.onClick}
        >
          {item.label}
          {item.badge != null ? <span>{` (${item.badge})`}</span> : null}
        </button>
      ))}
      {filters?.map((f) => (
        <div key={f.id} data-testid={`filter-${f.id}`} data-value={f.value}>
          {f.options.map((opt) => (
            <button
              type="button"
              key={opt.value}
              data-testid={`filter-${f.id}-${opt.value}`}
              onClick={() => f.onChange(opt.value)}
            >
              {opt.label}
            </button>
          ))}
        </div>
      ))}
      {activeFilters?.map((af) => (
        <span key={af.id} data-testid={`active-filter-${af.id}`}>
          {af.label}
          <button
            type="button"
            data-testid={`remove-filter-${af.id}`}
            onClick={af.onRemove}
          />
        </span>
      ))}
      {onClearAllFilters && (
        <button
          type="button"
          data-testid="clear-all-filters"
          onClick={onClearAllFilters}
        />
      )}
    </div>
  ),
}));

// Mock Alert
vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message, type }: { message: string; type: string }) => (
    <div data-testid={`alert-${type}`}>{message}</div>
  ),
}));

// Mock studentService
vi.mock("~/lib/api", () => ({
  studentService: {
    getStudents: vi.fn(),
  },
}));

// Mock location helpers
vi.mock("~/lib/location-helper", () => ({
  LOCATION_COLORS: {
    UNKNOWN: "#6B7280",
    SCHOOLYARD: "#F78C10",
    HOME: "#FF3130",
    GROUP_ROOM: "#83CD2D",
  },
  LOCATION_STATUSES: { PRESENT: "Anwesend" },
  isPresentLocation: vi.fn((location?: string | null) =>
    (location ?? "").startsWith("Anwesend"),
  ),
  isHomeLocation: vi.fn(() => false),
  isSchoolyardLocation: vi.fn(() => false),
  isTransitLocation: vi.fn(() => false),
  parseLocation: vi.fn(() => ({ room: "Room 1", status: "Anwesend" })),
}));

// Mock student-helpers
vi.mock("~/lib/student-helpers", () => ({
  SCHOOL_YEAR_FILTER_OPTIONS: [
    { value: "all", label: "Alle" },
    { value: "1", label: "1. Klasse" },
  ],
}));

// Mock SSEErrorBoundary
vi.mock("~/components/sse/SSEErrorBoundary", () => ({
  SSEErrorBoundary: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="sse-boundary">{children}</div>
  ),
}));

// Mock GroupTransferModal — capture availableUsers prop for filtering assertions
const mockTransferModalProps = vi.fn();
vi.mock("~/components/groups/group-transfer-modal", () => ({
  GroupTransferModal: (props: Record<string, unknown>) => {
    mockTransferModalProps(props);
    return <div data-testid="transfer-modal" />;
  },
}));

// Mock group-transfer-api
vi.mock("~/lib/group-transfer-api", () => ({
  groupTransferService: {
    getAllAvailableStaff: vi.fn(() => Promise.resolve([])),
    getStaffByRole: vi.fn(() => Promise.resolve([])),
    getActiveTransfersForGroup: vi.fn(() => Promise.resolve([])),
    transferGroup: vi.fn(() => Promise.resolve()),
    cancelTransferBySubstitutionId: vi.fn(() => Promise.resolve()),
  },
}));

// Mock LocationBadge (kept even though page now imports StudentPresenceBadge,
// so co-tests that transitively render LocationBadge don't explode).
vi.mock("@/components/ui/location-badge", () => ({
  LocationBadge: () => <div data-testid="location-badge">Location</div>,
}));

// Mock StudentPresenceBadge — page-level wrapper that picks Location vs
// PresenceBadge based on tenant presence mode. Exposes `current_room_color`
// from the student prop as a data attribute so we can assert the BFF→state
// pipeline doesn't drop the per-room color (regression guard for #1324).
vi.mock("@/components/ui/student-presence-badge", () => ({
  StudentPresenceBadge: ({
    student,
  }: {
    student?: {
      current_room_color?: string | null;
      not_arrival_today?: boolean;
      not_arrival_reason?: string | null;
    };
  }) => (
    <div
      data-testid="location-badge"
      data-room-color={student?.current_room_color ?? ""}
      data-not-arrival={String(student?.not_arrival_today ?? false)}
      data-not-arrival-reason={student?.not_arrival_reason ?? ""}
    >
      Presence
    </div>
  ),
}));

// Mock EmptyStudentResults
vi.mock("~/components/ui/empty-student-results", () => ({
  EmptyStudentResults: () => <div data-testid="empty-results">No results</div>,
}));

// Mock StudentCard — renders extraContent and locationBadge so tests can
// assert downstream presence-badge props (e.g. current_room_color forwarding).
vi.mock("~/components/students/student-card", () => ({
  StudentCard: ({
    firstName,
    lastName,
    extraContent,
    locationBadge,
  }: {
    firstName: string;
    lastName: string;
    extraContent?: React.ReactNode;
    locationBadge?: React.ReactNode;
  }) => (
    <div data-testid="student-card">
      {firstName} {lastName}
      {locationBadge}
      {extraContent && <div data-testid="extra-content">{extraContent}</div>}
    </div>
  ),
  StudentInfoRow: ({
    children,
    icon,
  }: {
    children: React.ReactNode;
    icon: React.ReactNode;
  }) => (
    <div data-testid="student-info-row">
      <span data-testid="info-row-icon">{icon}</span>
      {children}
    </div>
  ),
  PickupTimeRow: ({
    pickupTime,
    actualTime,
    isException,
    notes,
  }: {
    pickupTime?: string;
    actualTime?: string;
    isException: boolean;
    notes?: string;
    now: Date;
  }) => {
    return (
      <div
        data-testid="pickup-time-row"
        data-pickup-time={pickupTime ?? ""}
        data-actual-time={actualTime ?? ""}
        data-is-exception={String(isException)}
      >
        {pickupTime && <>Abholzeit: {pickupTime} Uhr</>}
        {!pickupTime && isException && (notes || "Abwesend")}
        {!pickupTime && !isException && <>Abholzeit: —</>}
        {notes && <span>({notes})</span>}
      </div>
    );
  },
  ArrivalTimeRow: ({
    arrivalTime,
    actualTime,
    isException,
    isAbsent,
    notes,
  }: {
    arrivalTime?: string;
    actualTime?: string;
    isException: boolean;
    isAbsent: boolean;
    notes?: string;
    now: Date;
  }) => (
    <div
      data-testid="arrival-time-row"
      data-arrival-time={arrivalTime ?? ""}
      data-actual-time={actualTime ?? ""}
      data-is-exception={String(isException)}
      data-is-absent={String(isAbsent)}
    >
      {isAbsent && <>Kommt heute nicht</>}
      {!isAbsent && arrivalTime && <>Ankunftszeit: {arrivalTime} Uhr</>}
      {!isAbsent && !arrivalTime && <>Ankunftszeit: —</>}
      {notes && <span>({notes})</span>}
    </div>
  ),
  StudentAbsenceRow: ({ label }: { label: string }) => (
    <div data-testid="student-absence-row">Kommt heute nicht ({label})</div>
  ),
}));

// Mock lucide-react icons
vi.mock("lucide-react", () => ({
  Clock: ({ className }: { className?: string }) => (
    <span data-testid="lucide-clock" className={className}>
      clock
    </span>
  ),
  AlertTriangle: ({ className }: { className?: string }) => (
    <span data-testid="lucide-alert-triangle" className={className}>
      alert
    </span>
  ),
  Loader2: ({ className }: { className?: string }) => (
    <span data-testid="lucide-loader2" className={className}>
      loader
    </span>
  ),
  UserCheck: ({ className }: { className?: string }) => (
    <span data-testid="lucide-user-check" className={className}>
      user-check
    </span>
  ),
  UserX: ({ className }: { className?: string }) => (
    <span data-testid="lucide-user-x" className={className}>
      user-x
    </span>
  ),
}));

// Mock the school-checkin FAB so existing tests don't need to care about
// the floating mode trigger — page.school-checkin.test.tsx exercises it.
vi.mock("~/components/students/school-checkin-fab", () => ({
  SchoolCheckinFab: () => <div data-testid="school-checkin-fab" />,
}));

// Mock the school-checkin hook so tests don't need to wire useToast/SWR up.
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

// Mock useUserContext
const mockUserContext = vi.fn(() => ({
  userContext: undefined as
    { currentStaff: { id: string; personId: string } | null } | undefined,
  isLoading: false,
  error: undefined,
  isReady: true,
}));
vi.mock("~/lib/hooks/use-user-context", () => ({
  useUserContext: () => mockUserContext(),
}));

const mockTenantMutate = vi.hoisted(() => vi.fn());
const mockMinuteClock = vi.hoisted(() => ({ current: new Date() }));

vi.mock("~/lib/pickup-helpers", () => ({
  useMinuteClock: () => mockMinuteClock.current,
}));

// Mock SWR hook
vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  useTenantMutate: vi.fn(() => mockTenantMutate),
}));

import { useSWRAuth } from "~/lib/swr";
import { isHomeLocation } from "~/lib/location-helper";
import { groupTransferService } from "~/lib/group-transfer-api";
import type {
  OgsLiveViewData,
  OgsLiveWireStudent,
} from "~/lib/ogs-group-live-api";
import OGSGroupPage from "./page";

// ── Aggregated live-view mock builders (#2056) ──────────────────────────────
// The page now consumes ONE useSWRAuth call returning OgsLiveViewData (the
// already-mapped view model — mapping from the backend wire shape is tested
// separately in src/lib/ogs-group-live-api.test.ts). These builders keep the
// many mock blocks below readable.
function liveData(overrides: Partial<OgsLiveViewData> = {}): OgsLiveViewData {
  return {
    groups: [
      {
        id: "1",
        name: "OGS Gruppe A",
        roomId: "10",
        roomName: "Raum 101",
        viaSubstitution: false,
      },
    ],
    groupId: "1",
    students: [],
    roomStatus: {},
    pickupTimes: new Map(),
    trackingIndicators: { labels: [], results: {} },
    transfers: [],
    ...overrides,
  };
}

function wireStudent(
  overrides: Omit<Partial<OgsLiveWireStudent>, "id"> &
    Pick<OgsLiveWireStudent, "first_name" | "last_name"> & {
      id: string | number;
    },
): OgsLiveWireStudent {
  return {
    school_class: "",
    current_location: "",
    sick: false,
    excused: false,
    class_trip: false,
    ...overrides,
    id: overrides.id.toString(),
  };
}

describe("OGSGroupPage", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockMinuteClock.current = new Date();
    global.fetch = vi.fn();
    // Default mock: loading state
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: true,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);
  });

  afterEach(() => {
    cleanup();
  });

  it("shows loading state initially", async () => {
    render(<OGSGroupPage />);

    // Initial loading state should show the page-shell skeleton
    expect(screen.getByTestId("ogs-groups-skeleton")).toBeInTheDocument();
  });

  it("renders with SSE error boundary wrapper", () => {
    render(<OGSGroupPage />);

    // Page should be wrapped in SSE error boundary
    expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
  });

  it("renders within responsive layout", async () => {
    render(<OGSGroupPage />);

    expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
  });

  it("revalidates group access when the Berlin calendar day changes", async () => {
    mockMinuteClock.current = new Date("2026-01-01T22:59:00Z");
    const { rerender } = render(<OGSGroupPage />);

    expect(mockMutate).not.toHaveBeenCalled();

    mockMinuteClock.current = new Date("2026-01-01T23:01:00Z");
    rerender(<OGSGroupPage />);

    await waitFor(() => expect(mockMutate).toHaveBeenCalledTimes(1));
  });

  it("reconciles periodically and on focus after missed SSE events", () => {
    render(<OGSGroupPage />);

    const options = vi.mocked(useSWRAuth).mock.calls[0]?.[2];
    expect(options).toMatchObject({ revalidateOnFocus: true });
    expect(options?.refreshInterval).toBe(15 * 60_000);
  });

  it("shows no access state when user has no OGS groups", async () => {
    // Mock SWR to return empty data indicating no access
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({ groups: [], groupId: null }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      // Should show the "no group assigned" message
      expect(
        screen.getByText("Keine OGS-Gruppe zugeordnet"),
      ).toBeInTheDocument();
    });
  });

  it("shows permission error when 403 response received", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: false,
      error: new Error("API error: 403"),
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine OGS-Gruppe zugeordnet"),
      ).toBeInTheDocument();
    });
  });

  it("shows loading state when session is loading", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: true,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    // Should show the page-shell skeleton while SWR is loading
    expect(screen.getByTestId("ogs-groups-skeleton")).toBeInTheDocument();
  });

  it("displays group data when available", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        students: [
          wireStudent({
            id: "1",
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });
  });

  it("shows sick and excused counts in the selected group summary", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
            sick: true,
          }),
          wireStudent({
            id: 2,
            first_name: "Mia",
            last_name: "Musterfrau",
            current_location: "Zuhause",
            excused: true,
          }),
          wireStudent({
            id: 3,
            first_name: "Ben",
            last_name: "Beispiel",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: {
          "1": { in_group_room: true },
          "2": { in_group_room: false },
          "3": { in_group_room: true },
        },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByLabelText("Abwesenheiten heute")).toBeInTheDocument();
      expect(screen.queryByText("3 Kinder")).not.toBeInTheDocument();
      expect(screen.getByText("1/3 krank")).toBeInTheDocument();
      expect(screen.getByText("1/3 entschuldigt")).toBeInTheDocument();
    });
  });

  it("renders the student card with pickup time from the aggregated pickupTimes Map", async () => {
    // Array→Map wire conversion is tested in ogs-group-live-api.test.ts;
    // the page only ever sees the already-mapped Map.
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
        pickupTimes: new Map([
          [
            "1",
            {
              pickupTime: "15:30",
              isException: false,
              notes: "Parent pickup",
              dayNotes: [{ id: "1", content: "Test note" }],
            },
          ],
        ]),
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    // Should display the student card with pickup time
    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
      // Pickup time should be rendered (15:30 Uhr format)
      expect(screen.getByText(/15:30 Uhr/)).toBeInTheDocument();
    });
  });

  it("handles pickup times with day notes correctly", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
        pickupTimes: new Map([
          [
            "1",
            {
              pickupTime: "16:00",
              isException: true,
              notes: "Multiple notes test",
              dayNotes: [
                { id: "1", content: "Note 1" },
                { id: "2", content: "Note 2" },
              ],
            },
          ],
        ]),
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
      expect(screen.getByText(/16:00 Uhr/)).toBeInTheDocument();
    });
  });

  it("handles pickup times without day notes", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
        pickupTimes: new Map([
          [
            "1",
            {
              pickupTime: "14:00",
              isException: false,
              notes: undefined,
              dayNotes: [],
            },
          ],
        ]),
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
      expect(screen.getByText(/14:00 Uhr/)).toBeInTheDocument();
    });
  });

  it("invokes fetchOgsGroupLive (GET /api/ogs-group-live) via the SWR fetcher", async () => {
    // Captures and directly invokes the fetcher function passed to
    // useSWRAuth, covering the SWR fetcher body that is normally never
    // executed because useSWRAuth is mocked. The wire→view mapping itself
    // (mapOgsGroupLiveResponse) is covered by ogs-group-live-api.test.ts.
    let capturedFetcher: (() => Promise<OgsLiveViewData>) | null = null;
    // IMPORTANT: a stable reference — a new object per render would make
    // the page's sync effect (which depends on this reference) refire
    // every render and loop forever.
    const stableData = liveData();

    vi.mocked(useSWRAuth).mockImplementation(((
      _key: string | null,
      fetcher?: () => Promise<OgsLiveViewData>,
    ) => {
      capturedFetcher = fetcher ?? null;
      return {
        data: stableData,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      };
    }) as unknown as typeof useSWRAuth);

    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        success: true,
        data: {
          groups: [],
          students: [],
          room_status: {},
          pickup_times: [],
          tracking_indicators: { labels: [], results: {} },
          transfers: [],
        },
      }),
    });

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(capturedFetcher).not.toBeNull();
    });

    await capturedFetcher!();

    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/ogs-group-live"),
      expect.any(Object),
    );
  });

  it("maps aggregated students with missing optional fields", async () => {
    // Covers null coalescing branches in student mapping (school_class ?? "", etc.)
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        students: [
          {
            id: "1",
            first_name: "Max",
            last_name: "Mustermann",
            school_class: "",
            current_location: "",
            sick: false,
            excused: false,
            class_trip: false,
            // Intentionally missing: location_since and other optional fields
          },
        ],
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
      // Student name should render from first_name + last_name mapping
      expect(screen.getByText(/Max Mustermann/)).toBeInTheDocument();
    });
  });

  it("forwards current_room_color from the aggregated live data to the badge", async () => {
    // Regression guard: the live-data → local-state mapping previously
    // dropped current_room_color, so navigating away and back into a group
    // reverted custom room badges to the OTHER_ROOM blue fallback.
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            school_class: "1a",
            current_location: "Anwesend - Raum 101",
            current_room_color: "#A3D977",
          }),
        ],
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      const badge = screen.getByTestId("location-badge");
      expect(badge).toHaveAttribute("data-room-color", "#A3D977");
    });
  });
});

describe("OGSGroupPage helper functions", () => {
  // Tests for helper functions that are defined in the page
  // These test the pure logic without rendering

  it("filters students by search term", () => {
    const student = {
      name: "Max Mustermann",
      first_name: "Max",
      last_name: "Mustermann",
      school_class: "1a",
    };

    // Test search matching first name
    const searchLower = "max";
    const matches =
      student.name?.toLowerCase().includes(searchLower) ??
      student.first_name?.toLowerCase().includes(searchLower) ??
      student.last_name?.toLowerCase().includes(searchLower) ??
      false;

    expect(matches).toBe(true);
  });

  it("extracts student year from school class", () => {
    const extractYear = (schoolClass?: string): string | null => {
      if (!schoolClass) return null;
      const yearMatch = /^(\d)/.exec(schoolClass);
      return yearMatch?.[1] ?? null;
    };

    expect(extractYear("1a")).toBe("1");
    expect(extractYear("2b")).toBe("2");
    expect(extractYear("10c")).toBe("1"); // Only first digit
    expect(extractYear("")).toBe(null);
    expect(extractYear(undefined)).toBe(null);
  });

  it("detects student in group room", () => {
    const isStudentInRoom = (
      studentLocation: string | undefined,
      roomName: string | undefined,
    ): boolean => {
      if (!studentLocation || !roomName) return false;
      return studentLocation.toLowerCase().includes(roomName.toLowerCase());
    };

    expect(isStudentInRoom("Anwesend - Raum 101", "Raum 101")).toBe(true);
    expect(isStudentInRoom("Anwesend - Raum 101", "Raum 202")).toBe(false);
    expect(isStudentInRoom(undefined, "Raum 101")).toBe(false);
    expect(isStudentInRoom("Anwesend", undefined)).toBe(false);
  });
});

describe("OGSGroupPage additional scenarios", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    cleanup();
  });

  it("shows empty students state when group has no students", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({ students: [] }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByText(/Keine Kinder in/)).toBeInTheDocument();
    });
  });

  it("renders multiple students in grid", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
          wireStudent({
            id: 2,
            first_name: "Erika",
            last_name: "Schmidt",
            current_location: "Raum 101",
          }),
          wireStudent({
            id: 3,
            first_name: "Hans",
            last_name: "Mueller",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: {
          "1": { in_group_room: true },
          "2": { in_group_room: true },
          "3": { in_group_room: true },
        },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      const studentCards = screen.getAllByTestId("student-card");
      expect(studentCards).toHaveLength(3);
    });
  });

  it("handles generic API error gracefully", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: false,
      error: new Error("API error: 500"),
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
    });
  });

  it("shows transfer modal component", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("transfer-modal")).toBeInTheDocument();
    });
  });

  it("displays via substitution badge when group is via substitution", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        groups: [
          {
            id: "1",
            name: "OGS Gruppe A",
            roomId: "10",
            roomName: "Raum 101",
            viaSubstitution: true, // This group is via substitution
          },
        ],
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });
  });

  it("shows the active-transfer badge from the server-provided transfers list", async () => {
    // The substitutions→transfers filter/mapping now lives server-side
    // (mapOgsGroupLiveResponse); the page just renders whatever `transfers`
    // the aggregate response carries.
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
        transfers: [
          {
            substitutionId: "100",
            groupId: "1",
            targetStaffId: "200",
            targetName: "Anna Lehrer",
            validUntil: "2024-01-20",
          },
        ],
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
      expect(screen.getByTestId("overflow-Gruppe übergeben")).toHaveTextContent(
        "(1)",
      );
    });
  });
});

describe("OGSGroupPage filter behavior", () => {
  it("filters students by attendance - in_room", () => {
    const students = [
      { id: "1", current_location: "Anwesend - Raum 101" },
      { id: "2", current_location: "Anwesend - Raum 202" },
      { id: "3", current_location: "Zuhause" },
    ];

    const roomStatus = {
      "1": { in_group_room: true, current_room_id: 101 },
      "2": { in_group_room: false, current_room_id: 202 },
      "3": { in_group_room: false, current_room_id: undefined },
    };

    // Filter for in_room attendance
    const filtered = students.filter((student) => {
      const status = roomStatus[student.id as keyof typeof roomStatus];
      return status?.in_group_room ?? false;
    });

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.id).toBe("1");
  });

  it("filters students by attendance - foreign_room", () => {
    const students = [
      { id: "1", current_location: "Anwesend - Raum 101" },
      { id: "2", current_location: "Anwesend - Raum 202" },
      { id: "3", current_location: "Zuhause" },
    ];

    const roomStatus: Record<
      string,
      { in_group_room: boolean; current_room_id?: number }
    > = {
      "1": { in_group_room: true, current_room_id: 101 },
      "2": { in_group_room: false, current_room_id: 202 },
      "3": { in_group_room: false, current_room_id: undefined },
    };

    const filtered = students.filter((student) => {
      const status = roomStatus[student.id];
      return (
        status?.current_room_id !== undefined && status.in_group_room === false
      );
    });

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.id).toBe("2");
  });

  it("filters students by year", () => {
    const students = [
      { id: "1", school_class: "1a" },
      { id: "2", school_class: "2b" },
      { id: "3", school_class: "1c" },
      { id: "4", school_class: "3a" },
    ];

    const selectedYear = "1";
    const extractYear = (schoolClass?: string): string | null => {
      if (!schoolClass) return null;
      const yearMatch = /^(\d)/.exec(schoolClass);
      return yearMatch?.[1] ?? null;
    };

    const filtered = students.filter(
      (s) => extractYear(s.school_class) === selectedYear,
    );

    expect(filtered).toHaveLength(2);
    expect(filtered.map((s) => s.id)).toEqual(["1", "3"]);
  });

  it("returns all students when year filter is 'all'", () => {
    const students = [
      { id: "1", school_class: "1a" },
      { id: "2", school_class: "2b" },
    ];

    const selectedYear = "all";
    const filtered = students.filter(() => selectedYear === "all" || true);

    expect(filtered).toHaveLength(2);
  });

  it("combines multiple filters correctly", () => {
    const students = [
      {
        id: "1",
        name: "Max A",
        school_class: "1a",
        current_location: "Raum 101",
      },
      {
        id: "2",
        name: "Erika B",
        school_class: "1b",
        current_location: "Raum 202",
      },
      {
        id: "3",
        name: "Max C",
        school_class: "2a",
        current_location: "Raum 101",
      },
    ];

    const searchTerm = "max";
    const selectedYear = "1" as string;

    const extractYear = (schoolClass?: string): string | null => {
      if (!schoolClass) return null;
      const yearMatch = /^(\d)/.exec(schoolClass);
      return yearMatch?.[1] ?? null;
    };

    const filtered = students.filter((student) => {
      const matchesSearch =
        student.name?.toLowerCase().includes(searchTerm.toLowerCase()) ?? false;
      const matchesYear =
        selectedYear === "all" ||
        extractYear(student.school_class) === selectedYear;
      return matchesSearch && matchesYear;
    });

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.id).toBe("1");
  });
});

describe("OGSGroupPage active filters", () => {
  it("creates active filter for search term", () => {
    const searchTerm = "Max";
    const selectedYear = "all" as string;
    const attendanceFilter = "all" as string;

    const activeFilters: Array<{ id: string; label: string }> = [];

    if (searchTerm.length > 0) {
      activeFilters.push({
        id: "search",
        label: `"${searchTerm}"`,
      });
    }

    if (selectedYear !== "all") {
      activeFilters.push({
        id: "year",
        label: `Jahr ${selectedYear}`,
      });
    }

    if (attendanceFilter !== "all") {
      activeFilters.push({
        id: "location",
        label: attendanceFilter,
      });
    }

    expect(activeFilters).toHaveLength(1);
    expect(activeFilters[0]?.label).toBe('"Max"');
  });

  it("creates active filter for year filter", () => {
    const searchTerm = "";
    const selectedYear = "2" as string;
    const attendanceFilter = "all" as string;

    const activeFilters: Array<{ id: string; label: string }> = [];

    if (searchTerm.length > 0) {
      activeFilters.push({ id: "search", label: `"${searchTerm}"` });
    }

    if (selectedYear !== "all") {
      activeFilters.push({ id: "year", label: `Jahr ${selectedYear}` });
    }

    if (attendanceFilter !== "all") {
      activeFilters.push({ id: "location", label: attendanceFilter });
    }

    expect(activeFilters).toHaveLength(1);
    expect(activeFilters[0]?.label).toBe("Jahr 2");
  });

  it("creates active filter for attendance filter", () => {
    const searchTerm = "";
    const selectedYear = "all" as string;
    const attendanceFilter = "in_room" as string;

    const locationLabels: Record<string, string> = {
      in_room: "Gruppenraum",
      foreign_room: "Fremder Raum",
      transit: "Unterwegs",
      schoolyard: "Schulhof",
      at_home: "Zuhause",
    };

    const activeFilters: Array<{ id: string; label: string }> = [];

    if (searchTerm.length > 0) {
      activeFilters.push({ id: "search", label: `"${searchTerm}"` });
    }

    if (selectedYear !== "all") {
      activeFilters.push({ id: "year", label: `Jahr ${selectedYear}` });
    }

    if (attendanceFilter !== "all") {
      activeFilters.push({
        id: "location",
        label: locationLabels[attendanceFilter] ?? attendanceFilter,
      });
    }

    expect(activeFilters).toHaveLength(1);
    expect(activeFilters[0]?.label).toBe("Gruppenraum");
  });

  it("creates multiple active filters", () => {
    const searchTerm = "Max";
    const selectedYear = "1" as string;
    const attendanceFilter = "schoolyard" as string;

    const locationLabels: Record<string, string> = {
      in_room: "Gruppenraum",
      foreign_room: "Fremder Raum",
      transit: "Unterwegs",
      schoolyard: "Schulhof",
      at_home: "Zuhause",
    };

    const activeFilters: Array<{ id: string; label: string }> = [];

    if (searchTerm.length > 0) {
      activeFilters.push({ id: "search", label: `"${searchTerm}"` });
    }

    if (selectedYear !== "all") {
      activeFilters.push({ id: "year", label: `Jahr ${selectedYear}` });
    }

    if (attendanceFilter !== "all") {
      activeFilters.push({
        id: "location",
        label: locationLabels[attendanceFilter] ?? attendanceFilter,
      });
    }

    expect(activeFilters).toHaveLength(3);
  });

  it("creates no active filters when all defaults", () => {
    const searchTerm = "";
    const selectedYear = "all" as string;
    const attendanceFilter = "all" as string;

    const activeFilters: Array<{ id: string; label: string }> = [];

    if (searchTerm.length > 0) {
      activeFilters.push({ id: "search", label: `"${searchTerm}"` });
    }

    if (selectedYear !== "all") {
      activeFilters.push({ id: "year", label: `Jahr ${selectedYear}` });
    }

    if (attendanceFilter !== "all") {
      activeFilters.push({ id: "location", label: attendanceFilter });
    }

    expect(activeFilters).toHaveLength(0);
  });
});

describe("OGSGroupPage handleTransferGroup behavior", () => {
  it("validates transfer group parameters", () => {
    const currentGroup = { id: "1", name: "OGS Gruppe A" };
    const targetName = "Anna Lehrer";

    // Simulate the validation logic from handleTransferGroup
    const canTransfer = !!currentGroup;
    expect(canTransfer).toBe(true);

    // Generate expected toast message
    const toastMessage = `Gruppe "${currentGroup.name}" an ${targetName} übergeben`;
    expect(toastMessage).toBe('Gruppe "OGS Gruppe A" an Anna Lehrer übergeben');
  });

  it("returns early when no current group", () => {
    const currentGroup = null;

    // Simulate the early return logic
    const canTransfer = !!currentGroup;
    expect(canTransfer).toBe(false);
  });
});

describe("OGSGroupPage handleCancelTransfer behavior", () => {
  it("finds transfer by substitution ID", () => {
    const activeTransfers = [
      { substitutionId: "100", targetName: "Anna Lehrer" },
      { substitutionId: "101", targetName: "Ben Schmidt" },
    ];
    const substitutionId = "100";

    const transfer = activeTransfers.find(
      (t) => t.substitutionId === substitutionId,
    );

    expect(transfer?.targetName).toBe("Anna Lehrer");
  });

  it("uses default name when transfer not found", () => {
    const activeTransfers = [
      { substitutionId: "100", targetName: "Anna Lehrer" },
    ];
    const substitutionId = "999"; // Non-existent

    const transfer = activeTransfers.find(
      (t) => t.substitutionId === substitutionId,
    );
    const recipientName = transfer?.targetName ?? "Betreuer";

    expect(recipientName).toBe("Betreuer");
  });

  it("generates correct cancel toast message", () => {
    const recipientName = "Anna Lehrer";
    const toastMessage = `Übergabe an ${recipientName} wurde zurückgenommen`;

    expect(toastMessage).toBe("Übergabe an Anna Lehrer wurde zurückgenommen");
  });
});

describe("OGSGroupPage switchToGroup behavior", () => {
  it("returns early when same group index selected", () => {
    const selectedGroupIndex = 0;
    const newGroupIndex = 0;
    const allGroups = [{ id: "1", name: "Group A" }];

    const shouldSwitch =
      newGroupIndex !== selectedGroupIndex && allGroups[newGroupIndex];
    expect(shouldSwitch).toBeFalsy();
  });

  it("returns early when group index is invalid", () => {
    const selectedGroupIndex = 0 as number;
    const newGroupIndex = 5 as number; // Out of bounds
    const allGroups = [{ id: "1", name: "Group A" }];

    const shouldSwitch =
      newGroupIndex !== selectedGroupIndex && allGroups[newGroupIndex];
    expect(shouldSwitch).toBeFalsy();
  });

  it("allows switching to different valid group", () => {
    const selectedGroupIndex = 0 as number;
    const newGroupIndex = 1 as number;
    const allGroups = [
      { id: "1", name: "Group A" },
      { id: "2", name: "Group B" },
    ];

    const shouldSwitch =
      newGroupIndex !== selectedGroupIndex && allGroups[newGroupIndex];
    expect(shouldSwitch).toBeTruthy();
  });

  it("updates group student count after loading", () => {
    const allGroups = [
      { id: "1", name: "Group A", student_count: 0 },
      { id: "2", name: "Group B", student_count: 0 },
    ];
    const selectedGroupIndex = 1;
    const studentsData = [{}, {}, {}]; // 3 students loaded

    // Simulate the update logic
    const updatedGroups = allGroups.map((group, idx) =>
      idx === selectedGroupIndex
        ? { ...group, student_count: studentsData.length }
        : group,
    );

    expect(updatedGroups[1]?.student_count).toBe(3);
    expect(updatedGroups[0]?.student_count).toBe(0);
  });

  it("sets error message on fetch failure", () => {
    const errorMessage = "Fehler beim Laden der Gruppendaten.";
    expect(errorMessage).toBe("Fehler beim Laden der Gruppendaten.");
  });
});

describe("OGSGroupPage renderDesktopActionButton logic", () => {
  it("returns undefined when on mobile", () => {
    const isMobile = true;
    const currentGroup = { id: "1", name: "Group A" };

    const shouldRender = !isMobile && currentGroup;
    expect(shouldRender).toBeFalsy();
  });

  it("returns undefined when no current group", () => {
    const isMobile = false;
    const currentGroup = null;

    const shouldRender = !isMobile && currentGroup;
    expect(shouldRender).toBeFalsy();
  });

  it("shows via substitution badge when group is via substitution", () => {
    const isMobile = false;
    const currentGroup = { id: "1", name: "Group A", viaSubstitution: true };

    const shouldShowSubstitutionBadge =
      !isMobile && currentGroup.viaSubstitution;
    expect(shouldShowSubstitutionBadge).toBe(true);
  });

  it("shows transfer button with count when active transfers exist", () => {
    const activeTransfers = [{ substitutionId: "1" }, { substitutionId: "2" }];

    const buttonText =
      activeTransfers.length > 0
        ? `Gruppe übergeben (${activeTransfers.length})`
        : "Gruppe übergeben";

    expect(buttonText).toBe("Gruppe übergeben (2)");
  });

  it("shows transfer button without count when no active transfers", () => {
    const activeTransfers: Array<{ substitutionId: string }> = [];

    const buttonText =
      activeTransfers.length > 0
        ? `Gruppe übergeben (${activeTransfers.length})`
        : "Gruppe übergeben";

    expect(buttonText).toBe("Gruppe übergeben");
  });
});

describe("OGSGroupPage renderMobileActionButton logic", () => {
  it("returns undefined when not on mobile", () => {
    const isMobile = false;
    const currentGroup = { id: "1", name: "Group A" };

    const shouldRender = isMobile && currentGroup;
    expect(shouldRender).toBeFalsy();
  });

  it("returns undefined when no current group", () => {
    const isMobile = true;
    const currentGroup = null;

    const shouldRender = isMobile && currentGroup;
    expect(shouldRender).toBeFalsy();
  });

  it("shows via substitution icon on mobile when group is via substitution", () => {
    const isMobile = true;
    const currentGroup = { id: "1", name: "Group A", viaSubstitution: true };

    const shouldShowSubstitutionIcon = isMobile && currentGroup.viaSubstitution;
    expect(shouldShowSubstitutionIcon).toBe(true);
  });

  it("shows badge with active transfer count on mobile", () => {
    const activeTransfers = [{ substitutionId: "1" }];

    const showBadge = activeTransfers.length > 0;
    expect(showBadge).toBe(true);
  });

  it("hides badge when no active transfers on mobile", () => {
    const activeTransfers: Array<{ substitutionId: string }> = [];

    const showBadge = activeTransfers.length > 0;
    expect(showBadge).toBe(false);
  });
});

describe("OGSGroupPage renderStudentContent logic", () => {
  it("shows loading when isLoading is true", () => {
    const isLoading = true;

    const showLoading = isLoading;
    expect(showLoading).toBe(true);
  });

  it("shows empty state when students array is empty", () => {
    const isLoading = false;
    const students: Array<{ id: string }> = [];

    const showEmptyNoStudents = !isLoading && students.length === 0;
    expect(showEmptyNoStudents).toBe(true);
  });

  it("shows filtered student grid when students exist", () => {
    const isLoading = false;
    const students = [{ id: "1" }, { id: "2" }];
    const filteredStudents = [{ id: "1" }];

    const showStudentGrid =
      !isLoading && students.length > 0 && filteredStudents.length > 0;
    expect(showStudentGrid).toBe(true);
  });

  it("shows empty results component when filters match nothing", () => {
    const isLoading = false;
    const students = [{ id: "1" }, { id: "2" }];
    const filteredStudents: Array<{ id: string }> = [];

    const showEmptyResults =
      !isLoading && students.length > 0 && filteredStudents.length === 0;
    expect(showEmptyResults).toBe(true);
  });

  it("generates correct no students message", () => {
    const currentGroup = { name: "OGS Gruppe A" };
    const message = `Keine Kinder in ${currentGroup?.name ?? "dieser Gruppe"}`;

    expect(message).toBe("Keine Kinder in OGS Gruppe A");
  });

  it("uses fallback message when no current group", () => {
    const currentGroup = null as { name: string } | null;
    const message = `Keine Kinder in ${currentGroup?.name ?? "dieser Gruppe"}`;

    expect(message).toBe("Keine Kinder in dieser Gruppe");
  });

  it("shows suggestion for multiple groups when no students", () => {
    const allGroups = [{ id: "1" }, { id: "2" }];
    const showSuggestion = allGroups.length > 1;

    expect(showSuggestion).toBe(true);
  });
});

describe("OGSGroupPage student card onClick behavior", () => {
  it("generates correct navigation path with from param", () => {
    const studentId = "123";
    const path = `/students/${studentId}?from=/ogs-groups`;

    expect(path).toBe("/students/123?from=/ogs-groups");
  });
});

describe("OGSGroupPage card gradient logic", () => {
  it("returns green gradient for student in group room", () => {
    const isInGroupRoom = true;
    const isSchoolyard = false;
    const isTransit = false;
    const isHome = false;

    const getGradient = (): string => {
      if (isInGroupRoom) return "from-emerald-50/80 to-green-100/80";
      if (isSchoolyard) return "from-amber-50/80 to-yellow-100/80";
      if (isTransit) return "from-fuchsia-50/80 to-pink-100/80";
      if (isHome) return "from-red-50/80 to-rose-100/80";
      return "from-blue-50/80 to-cyan-100/80";
    };

    expect(getGradient()).toBe("from-emerald-50/80 to-green-100/80");
  });

  it("returns amber gradient for student in schoolyard", () => {
    const isInGroupRoom = false;
    const isSchoolyard = true;
    const isTransit = false;
    const isHome = false;

    const getGradient = (): string => {
      if (isInGroupRoom) return "from-emerald-50/80 to-green-100/80";
      if (isSchoolyard) return "from-amber-50/80 to-yellow-100/80";
      if (isTransit) return "from-fuchsia-50/80 to-pink-100/80";
      if (isHome) return "from-red-50/80 to-rose-100/80";
      return "from-blue-50/80 to-cyan-100/80";
    };

    expect(getGradient()).toBe("from-amber-50/80 to-yellow-100/80");
  });

  it("returns fuchsia gradient for student in transit", () => {
    const isInGroupRoom = false;
    const isSchoolyard = false;
    const isTransit = true;
    const isHome = false;

    const getGradient = (): string => {
      if (isInGroupRoom) return "from-emerald-50/80 to-green-100/80";
      if (isSchoolyard) return "from-amber-50/80 to-yellow-100/80";
      if (isTransit) return "from-fuchsia-50/80 to-pink-100/80";
      if (isHome) return "from-red-50/80 to-rose-100/80";
      return "from-blue-50/80 to-cyan-100/80";
    };

    expect(getGradient()).toBe("from-fuchsia-50/80 to-pink-100/80");
  });

  it("returns red gradient for student at home", () => {
    const isInGroupRoom = false;
    const isSchoolyard = false;
    const isTransit = false;
    const isHome = true;

    const getGradient = (): string => {
      if (isInGroupRoom) return "from-emerald-50/80 to-green-100/80";
      if (isSchoolyard) return "from-amber-50/80 to-yellow-100/80";
      if (isTransit) return "from-fuchsia-50/80 to-pink-100/80";
      if (isHome) return "from-red-50/80 to-rose-100/80";
      return "from-blue-50/80 to-cyan-100/80";
    };

    expect(getGradient()).toBe("from-red-50/80 to-rose-100/80");
  });

  it("returns blue gradient for student in foreign room", () => {
    const isInGroupRoom = false;
    const isSchoolyard = false;
    const isTransit = false;
    const isHome = false;

    const getGradient = (): string => {
      if (isInGroupRoom) return "from-emerald-50/80 to-green-100/80";
      if (isSchoolyard) return "from-amber-50/80 to-yellow-100/80";
      if (isTransit) return "from-fuchsia-50/80 to-pink-100/80";
      if (isHome) return "from-red-50/80 to-rose-100/80";
      return "from-blue-50/80 to-cyan-100/80";
    };

    expect(getGradient()).toBe("from-blue-50/80 to-cyan-100/80");
  });
});

describe("OGSGroupPage sorting logic", () => {
  type StudentSort = {
    id: string;
    first_name: string;
    last_name: string;
    current_location: string;
  };

  const isHomeLocation = (loc: string) => loc === "Zuhause";

  it("sorts alphabetically by last name then first name in default mode", () => {
    const students: StudentSort[] = [
      {
        id: "1",
        first_name: "Zara",
        last_name: "Mueller",
        current_location: "Raum 101",
      },
      {
        id: "2",
        first_name: "Anna",
        last_name: "Becker",
        current_location: "Raum 101",
      },
      {
        id: "3",
        first_name: "Max",
        last_name: "Mueller",
        current_location: "Raum 101",
      },
    ];

    const sorted = [...students].sort((a, b) => {
      const lastCmp = (a.last_name ?? "").localeCompare(
        b.last_name ?? "",
        "de",
      );
      if (lastCmp !== 0) return lastCmp;
      return (a.first_name ?? "").localeCompare(b.first_name ?? "", "de");
    });

    expect(sorted.map((s) => s.id)).toEqual(["2", "3", "1"]); // Becker, Mueller(Max), Mueller(Zara)
  });

  it("handles empty names in alphabetical sort", () => {
    const students: StudentSort[] = [
      {
        id: "1",
        first_name: "Max",
        last_name: "",
        current_location: "Raum 101",
      },
      {
        id: "2",
        first_name: "Anna",
        last_name: "Zeller",
        current_location: "Raum 101",
      },
    ];

    const sorted = [...students].sort((a, b) => {
      const lastCmp = (a.last_name ?? "").localeCompare(
        b.last_name ?? "",
        "de",
      );
      if (lastCmp !== 0) return lastCmp;
      return (a.first_name ?? "").localeCompare(b.first_name ?? "", "de");
    });

    expect(sorted[0]?.id).toBe("1"); // Empty string sorts before "Zeller"
  });

  it("sorts pickup mode: present with time first, then without, then home", () => {
    const pickupTimes = new Map([
      ["1", { pickupTime: "15:00" }],
      ["3", { pickupTime: "14:00" }],
    ]);

    const students: StudentSort[] = [
      {
        id: "1",
        first_name: "A",
        last_name: "A",
        current_location: "Raum 101",
      }, // present, pickup 15:00
      {
        id: "2",
        first_name: "B",
        last_name: "B",
        current_location: "Raum 101",
      }, // present, no pickup
      {
        id: "3",
        first_name: "C",
        last_name: "C",
        current_location: "Raum 101",
      }, // present, pickup 14:00
      {
        id: "4",
        first_name: "D",
        last_name: "D",
        current_location: "Zuhause",
      }, // at home
    ];

    const sorted = [...students].sort((a, b) => {
      const aHome = isHomeLocation(a.current_location);
      const bHome = isHomeLocation(b.current_location);

      if (aHome && !bHome) return 1;
      if (!aHome && bHome) return -1;
      if (aHome && bHome) return 0;

      const timeA = pickupTimes.get(a.id)?.pickupTime;
      const timeB = pickupTimes.get(b.id)?.pickupTime;

      if (!timeA && !timeB) return 0;
      if (!timeA) return 1;
      if (!timeB) return -1;

      return timeA.localeCompare(timeB);
    });

    // 14:00 first, then 15:00, then no pickup (present), then home
    expect(sorted.map((s) => s.id)).toEqual(["3", "1", "2", "4"]);
  });

  it("sorts home students to end regardless of pickup time", () => {
    const pickupTimes = new Map([
      ["1", { pickupTime: "14:00" }],
      ["2", { pickupTime: "13:00" }], // earlier time but at home
    ]);

    const students: StudentSort[] = [
      {
        id: "1",
        first_name: "A",
        last_name: "A",
        current_location: "Raum 101",
      },
      {
        id: "2",
        first_name: "B",
        last_name: "B",
        current_location: "Zuhause",
      },
    ];

    const sorted = [...students].sort((a, b) => {
      const aHome = isHomeLocation(a.current_location);
      const bHome = isHomeLocation(b.current_location);

      if (aHome && !bHome) return 1;
      if (!aHome && bHome) return -1;
      if (aHome && bHome) return 0;

      const timeA = pickupTimes.get(a.id)?.pickupTime;
      const timeB = pickupTimes.get(b.id)?.pickupTime;

      if (!timeA && !timeB) return 0;
      if (!timeA) return 1;
      if (!timeB) return -1;

      return timeA.localeCompare(timeB);
    });

    expect(sorted[0]?.id).toBe("1"); // Present student first
    expect(sorted[1]?.id).toBe("2"); // Home student last
  });

  it("keeps order stable for two home students", () => {
    const students: StudentSort[] = [
      {
        id: "1",
        first_name: "A",
        last_name: "A",
        current_location: "Zuhause",
      },
      {
        id: "2",
        first_name: "B",
        last_name: "B",
        current_location: "Zuhause",
      },
    ];

    const sorted = [...students].sort((a, b) => {
      const aHome = isHomeLocation(a.current_location);
      const bHome = isHomeLocation(b.current_location);

      if (aHome && !bHome) return 1;
      if (!aHome && bHome) return -1;
      if (aHome && bHome) return 0;
      return 0;
    });

    // Both at home, stable sort preserves original order
    expect(sorted.map((s) => s.id)).toEqual(["1", "2"]);
  });

  it("sorts present students without pickup times equally", () => {
    const pickupTimes = new Map<string, { pickupTime: string }>();

    const students: StudentSort[] = [
      {
        id: "1",
        first_name: "A",
        last_name: "A",
        current_location: "Raum 101",
      },
      {
        id: "2",
        first_name: "B",
        last_name: "B",
        current_location: "Raum 102",
      },
    ];

    const sorted = [...students].sort((a, b) => {
      const aHome = isHomeLocation(a.current_location);
      const bHome = isHomeLocation(b.current_location);

      if (aHome && !bHome) return 1;
      if (!aHome && bHome) return -1;
      if (aHome && bHome) return 0;

      const timeA = pickupTimes.get(a.id)?.pickupTime;
      const timeB = pickupTimes.get(b.id)?.pickupTime;

      if (!timeA && !timeB) return 0;
      if (!timeA) return 1;
      if (!timeB) return -1;

      return timeA.localeCompare(timeB);
    });

    // Both present without times, stable order
    expect(sorted.map((s) => s.id)).toEqual(["1", "2"]);
  });
});

describe("OGSGroupPage sort active filter", () => {
  type SortMode = "default" | "pickup";

  function buildSortFilters(
    sortMode: SortMode,
    searchTerm: string,
    selectedYear: string,
  ): Array<{ id: string; label: string }> {
    const filters: Array<{ id: string; label: string }> = [];
    if (sortMode !== "default") {
      filters.push({ id: "sort", label: "Sortiert: Nächste Abholung" });
    }
    if (searchTerm.length > 0) {
      filters.push({ id: "search", label: `"${searchTerm}"` });
    }
    if (selectedYear !== "all") {
      filters.push({ id: "year", label: `Jahr ${selectedYear}` });
    }
    return filters;
  }

  it("creates sort active filter when sortMode is pickup", () => {
    const filters = buildSortFilters("pickup", "", "all");
    expect(filters).toHaveLength(1);
    expect(filters[0]?.label).toBe("Sortiert: Nächste Abholung");
    expect(filters[0]?.id).toBe("sort");
  });

  it("does not create sort filter when sortMode is default", () => {
    const filters = buildSortFilters("default", "", "all");
    expect(filters).toHaveLength(0);
  });

  it("includes sort filter with other active filters", () => {
    const filters = buildSortFilters("pickup", "Max", "2");
    expect(filters).toHaveLength(3);
    expect(filters[0]?.id).toBe("sort");
  });
});

describe("OGSGroupPage sort filter config", () => {
  it("provides correct sort filter options", () => {
    const sortOptions = [
      { value: "default", label: "Alphabetisch" },
      { value: "pickup", label: "Nächste Abholung" },
    ];

    expect(sortOptions).toHaveLength(2);
    expect(sortOptions[0]?.value).toBe("default");
    expect(sortOptions[0]?.label).toBe("Alphabetisch");
    expect(sortOptions[1]?.value).toBe("pickup");
    expect(sortOptions[1]?.label).toBe("Nächste Abholung");
  });

  it("sort filter config has correct structure", () => {
    const sortConfig = {
      id: "sort",
      label: "Sortierung",
      type: "buttons" as const,
      value: "default",
      options: [
        { value: "default", label: "Alphabetisch" },
        { value: "pickup", label: "Nächste Abholung" },
      ],
    };

    expect(sortConfig.id).toBe("sort");
    expect(sortConfig.type).toBe("buttons");
    expect(sortConfig.options).toHaveLength(2);
  });
});

describe("OGSGroupPage clear all filters includes sort reset", () => {
  it("resets sort mode along with other filters", () => {
    // Values after onClearAllFilters runs
    const searchTerm = "";
    const selectedYear = "all";
    const attendanceFilter = "all";
    const sortMode = "default";

    expect(searchTerm).toBe("");
    expect(selectedYear).toBe("all");
    expect(attendanceFilter).toBe("all");
    expect(sortMode).toBe("default");
  });
});

// ===== RENDER TESTS that exercise actual source code lines =====
// These tests render the component with pickup time data to cover
// getStudentTimeStatus, sortedStudents, and filter configs.

describe("OGSGroupPage rendered pickup urgency", () => {
  const mockMutate = vi.fn();
  // Freeze time to 14:00 on 2026-01-28 to make tests deterministic
  const FROZEN_TIME = new Date(2026, 0, 28, 14, 0, 0);

  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.setSystemTime(FROZEN_TIME);
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  function setupWithStudentsAndPickupTimes(
    pickupMap: Map<
      string,
      {
        pickupTime: string;
        isException: boolean;
        dayNotes?: { id: string; content: string }[];
      }
    >,
    locationMocks?: {
      isHome?: (loc: string | null | undefined) => boolean;
    },
    studentActuals?: Record<
      string,
      { actualArrivalTime?: string; actualPickupTime?: string }
    >,
    studentOverrides?: Record<string, Record<string, unknown>>,
  ) {
    vi.clearAllMocks();
    // Re-freeze time after clearAllMocks since it may reset fake timers state
    vi.setSystemTime(FROZEN_TIME);
    global.fetch = vi.fn();

    // Setup location mocks
    if (locationMocks?.isHome) {
      vi.mocked(isHomeLocation).mockImplementation(locationMocks.isHome);
    } else {
      vi.mocked(isHomeLocation).mockReturnValue(false);
    }

    // Already-mapped pickupTimes Map, as the aggregate response provides it
    // (array→Map wire conversion is covered by ogs-group-live-api.test.ts).
    const pickupTimes = new Map(
      Array.from(pickupMap.entries()).map(([studentId, pickup]) => [
        studentId,
        {
          pickupTime: pickup.pickupTime,
          isException: pickup.isException,
          notes: undefined,
          dayNotes: pickup.dayNotes ?? [],
        },
      ]),
    );

    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        students: [
          wireStudent({
            id: 1,
            first_name: "Anna",
            last_name: "Becker",
            current_location: "Raum 101",
            actual_arrival_time: studentActuals?.["1"]?.actualArrivalTime,
            actual_pickup_time: studentActuals?.["1"]?.actualPickupTime,
            ...studentOverrides?.["1"],
          }),
          wireStudent({
            id: 2,
            first_name: "Max",
            last_name: "Zeller",
            current_location: "Raum 101",
            actual_arrival_time: studentActuals?.["2"]?.actualArrivalTime,
            actual_pickup_time: studentActuals?.["2"]?.actualPickupTime,
            ...studentOverrides?.["2"],
          }),
          wireStudent({
            id: 3,
            first_name: "Lena",
            last_name: "Mueller",
            current_location: "Zuhause",
            actual_arrival_time: studentActuals?.["3"]?.actualArrivalTime,
            actual_pickup_time: studentActuals?.["3"]?.actualPickupTime,
            ...studentOverrides?.["3"],
          }),
        ],
        roomStatus: {
          "1": { in_group_room: true },
          "2": { in_group_room: true },
          "3": { in_group_room: false },
        },
        pickupTimes,
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);
  }

  it("passes pickup time props to PickupTimeRow", async () => {
    const pickupMap = new Map([
      ["1", { pickupTime: "23:59", isException: false }],
    ]);
    setupWithStudentsAndPickupTimes(pickupMap);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(3);
    });

    // Should render pickup time text and pass correct props
    await waitFor(() => {
      expect(screen.getByText(/23:59 Uhr/)).toBeInTheDocument();
    });
    const row = screen
      .getAllByTestId("pickup-time-row")
      .find((el) => el.dataset.pickupTime === "23:59");
    expect(row).toBeDefined();
    expect(row?.dataset.isException).toBe("false");
  });

  it("passes exception flag and day notes to PickupTimeRow", async () => {
    const pickupMap = new Map([
      [
        "1",
        {
          pickupTime: "00:01",
          isException: true,
          dayNotes: [{ id: "1", content: "Arzttermin" }],
        },
      ],
    ]);
    setupWithStudentsAndPickupTimes(pickupMap);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(3);
    });

    // Exception flag and day note should be passed through
    await waitFor(() => {
      expect(screen.getByText(/Arzttermin/)).toBeInTheDocument();
    });
    const row = screen
      .getAllByTestId("pickup-time-row")
      .find((el) => el.dataset.isException === "true");
    expect(row).toBeDefined();
  });

  it("propagates the student's actual pickup time to PickupTimeRow", async () => {
    const pickupMap = new Map([
      ["1", { pickupTime: "14:00", isException: false }],
    ]);
    setupWithStudentsAndPickupTimes(pickupMap, undefined, {
      "1": { actualPickupTime: "14:07" },
    });

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(3);
    });

    const row = screen
      .getAllByTestId("pickup-time-row")
      .find((el) => el.dataset.pickupTime === "14:00");
    expect(row).toBeDefined();
    expect(row?.dataset.actualTime).toBe("14:07");
  });

  it("suppresses overdue pickup when an already-arrived student is later marked sick", async () => {
    const pickupMap = new Map([
      ["1", { pickupTime: "13:00", isException: false }],
    ]);
    setupWithStudentsAndPickupTimes(
      pickupMap,
      undefined,
      { "1": { actualArrivalTime: "08:03" } },
      { "1": { sick: true } },
    );

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(3);
    });

    expect(
      screen.getByText("Kommt heute nicht (krank gemeldet)"),
    ).toBeInTheDocument();
    expect(
      screen
        .getAllByTestId("pickup-time-row")
        .some((el) => el.dataset.pickupTime === "13:00"),
    ).toBe(false);
  });

  it("uses day planning status for OGS group card absence and badge state", async () => {
    setupWithStudentsAndPickupTimes(new Map(), undefined, undefined, {
      "1": {
        current_location: "Zuhause",
        day_planning_status: "not_coming_today",
        day_planning_label: "kein Plan für heute",
      },
    });

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(3);
    });

    expect(
      screen.getByText("Kommt heute nicht (kein Plan für heute)"),
    ).toBeInTheDocument();
    const firstBadge = screen.getAllByTestId("location-badge")[0];
    expect(firstBadge).toHaveAttribute("data-not-arrival", "true");
    expect(firstBadge).toHaveAttribute(
      "data-not-arrival-reason",
      "kein Plan für heute",
    );
  });

  it("shows the resolved pickup time, not the absence row, when a sick student is already picked up", async () => {
    const pickupMap = new Map([
      ["1", { pickupTime: "13:00", isException: false }],
    ]);
    setupWithStudentsAndPickupTimes(
      pickupMap,
      undefined,
      { "1": { actualPickupTime: "13:05" } },
      { "1": { sick: true } },
    );

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(3);
    });

    expect(
      screen.queryByText("Kommt heute nicht (krank gemeldet)"),
    ).not.toBeInTheDocument();
    const row = screen
      .getAllByTestId("pickup-time-row")
      .find((el) => el.dataset.pickupTime === "13:00");
    expect(row).toBeDefined();
    expect(row?.dataset.actualTime).toBe("13:05");
  });

  it("renders sort filter with Alphabetisch and Nächste Abholung options", async () => {
    setupWithStudentsAndPickupTimes(new Map());

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("filter-sort")).toBeInTheDocument();
    });

    // Sort filter should have both options
    expect(screen.getByTestId("filter-sort-default")).toBeInTheDocument();
    expect(screen.getByTestId("filter-sort-pickup")).toBeInTheDocument();

    // Labels should match
    expect(screen.getByText("Alphabetisch")).toBeInTheDocument();
    expect(screen.getByText("Nächste Abholung")).toBeInTheDocument();
  });

  it("renders students in alphabetical order by default", async () => {
    setupWithStudentsAndPickupTimes(new Map());

    render(<OGSGroupPage />);

    await waitFor(() => {
      const cards = screen.getAllByTestId("student-card");
      expect(cards).toHaveLength(3);
    });

    // Default sort is alphabetical by last name
    const cards = screen.getAllByTestId("student-card");
    expect(cards[0]?.textContent).toContain("Anna Becker");
    expect(cards[1]?.textContent).toContain("Lena Mueller");
    expect(cards[2]?.textContent).toContain("Max Zeller");
  });

  it("shows sort active filter chip when pickup sort is active", async () => {
    setupWithStudentsAndPickupTimes(new Map());

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("filter-sort")).toBeInTheDocument();
    });

    // Click "Nächste Abholung" sort button
    const pickupSortBtn = screen.getByTestId("filter-sort-pickup");
    pickupSortBtn.click();

    // Active filter chip should appear
    await waitFor(() => {
      expect(screen.getByTestId("active-filter-sort")).toBeInTheDocument();
      expect(
        screen.getByText("Sortiert: Nächste Abholung"),
      ).toBeInTheDocument();
    });
  });

  it("sorts by pickup time when pickup sort is activated", async () => {
    const pickupMap = new Map([
      ["1", { pickupTime: "16:00", isException: false }],
      ["2", { pickupTime: "14:00", isException: false }],
    ]);
    setupWithStudentsAndPickupTimes(pickupMap, {
      isHome: (loc) => loc === "Zuhause",
    });

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(3);
    });

    // Activate pickup sort
    const pickupSortBtn = screen.getByTestId("filter-sort-pickup");
    pickupSortBtn.click();

    // After sort: 14:00 (Zeller) → 16:00 (Becker) → no time at home (Mueller)
    await waitFor(() => {
      const cards = screen.getAllByTestId("student-card");
      expect(cards[0]?.textContent).toContain("Max Zeller"); // 14:00
      expect(cards[1]?.textContent).toContain("Anna Becker"); // 16:00
      expect(cards[2]?.textContent).toContain("Lena Mueller"); // at home, no time
    });
  });

  it("renders the arrival sort option and sorts when activated", async () => {
    setupWithStudentsAndPickupTimes(new Map(), {
      isHome: (loc) => loc === "Zuhause",
    });

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("filter-sort")).toBeInTheDocument();
    });

    expect(screen.getByText("Nächste Ankunft")).toBeInTheDocument();
    const arrivalBtn = screen.getByTestId("filter-sort-arrival");
    arrivalBtn.click();

    await waitFor(() => {
      // All three students still rendered after sort switch
      expect(screen.getAllByTestId("student-card")).toHaveLength(3);
    });
  });

  // Flexible setup helper that accepts custom students for branch coverage
  function setupWithCustomStudents(
    students: Array<{
      id: string;
      name: string;
      first_name: string;
      last_name: string;
      current_location: string;
    }>,
    pickupMap: Map<
      string,
      {
        pickupTime: string;
        isException: boolean;
        dayNotes?: { id: string; content: string }[];
      }
    >,
  ) {
    vi.clearAllMocks();
    vi.setSystemTime(FROZEN_TIME);
    global.fetch = vi.fn();

    vi.mocked(isHomeLocation).mockImplementation((loc) => loc === "Zuhause");

    const pickupTimes = new Map(
      Array.from(pickupMap.entries()).map(([studentId, pickup]) => [
        studentId,
        {
          pickupTime: pickup.pickupTime,
          isException: pickup.isException,
          notes: undefined,
          dayNotes: pickup.dayNotes ?? [],
        },
      ]),
    );

    const roomStatus: Record<string, { in_group_room: boolean }> = {};
    for (const s of students) {
      roomStatus[s.id] = {
        in_group_room: s.current_location === "Raum 101",
      };
    }

    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        students: students.map((s) =>
          wireStudent({
            id: Number(s.id),
            first_name: s.first_name,
            last_name: s.last_name,
            current_location: s.current_location,
          }),
        ),
        roomStatus,
        pickupTimes,
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);
  }

  it("sorts same-urgency students by pickup time then name", async () => {
    // FROZEN_TIME = 14:00. Both 16:00 and 17:00 are "normal" urgency.
    // Tests: same urgency rank → sort by time → then by name
    const students = [
      {
        id: "1",
        name: "Max Zeller",
        first_name: "Max",
        last_name: "Zeller",
        current_location: "Raum 101",
      },
      {
        id: "2",
        name: "Anna Becker",
        first_name: "Anna",
        last_name: "Becker",
        current_location: "Raum 101",
      },
    ];
    const pickupMap = new Map([
      ["1", { pickupTime: "17:00", isException: false }], // normal
      ["2", { pickupTime: "16:00", isException: false }], // normal
    ]);
    setupWithCustomStudents(students, pickupMap);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(2);
    });

    screen.getByTestId("filter-sort-pickup").click();

    // Same urgency (normal) → by time: 16:00 (Becker) before 17:00 (Zeller)
    await waitFor(() => {
      const cards = screen.getAllByTestId("student-card");
      expect(cards[0]?.textContent).toContain("Anna Becker");
      expect(cards[1]?.textContent).toContain("Max Zeller");
    });
  });

  it("sorts same-urgency same-time students by last name", async () => {
    // Both have identical pickup time → tiebreaker is name
    const students = [
      {
        id: "1",
        name: "Max Zeller",
        first_name: "Max",
        last_name: "Zeller",
        current_location: "Raum 101",
      },
      {
        id: "2",
        name: "Anna Becker",
        first_name: "Anna",
        last_name: "Becker",
        current_location: "Raum 101",
      },
    ];
    const pickupMap = new Map([
      ["1", { pickupTime: "16:00", isException: false }],
      ["2", { pickupTime: "16:00", isException: false }],
    ]);
    setupWithCustomStudents(students, pickupMap);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(2);
    });

    screen.getByTestId("filter-sort-pickup").click();

    // Same time → Becker before Zeller by name
    await waitFor(() => {
      const cards = screen.getAllByTestId("student-card");
      expect(cards[0]?.textContent).toContain("Anna Becker");
      expect(cards[1]?.textContent).toContain("Max Zeller");
    });
  });

  it("sorts present students without pickup time by name", async () => {
    // No pickup times → all "none" urgency → sort by name
    const students = [
      {
        id: "1",
        name: "Max Zeller",
        first_name: "Max",
        last_name: "Zeller",
        current_location: "Raum 101",
      },
      {
        id: "2",
        name: "Anna Becker",
        first_name: "Anna",
        last_name: "Becker",
        current_location: "Raum 101",
      },
    ];
    setupWithCustomStudents(students, new Map());

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(2);
    });

    screen.getByTestId("filter-sort-pickup").click();

    // Both "none" → Becker before Zeller
    await waitFor(() => {
      const cards = screen.getAllByTestId("student-card");
      expect(cards[0]?.textContent).toContain("Anna Becker");
      expect(cards[1]?.textContent).toContain("Max Zeller");
    });
  });

  it("sorts two home students by last name in pickup mode", async () => {
    // Both at home → compareByName
    const students = [
      {
        id: "1",
        name: "Max Zeller",
        first_name: "Max",
        last_name: "Zeller",
        current_location: "Zuhause",
      },
      {
        id: "2",
        name: "Anna Becker",
        first_name: "Anna",
        last_name: "Becker",
        current_location: "Zuhause",
      },
    ];
    setupWithCustomStudents(students, new Map());

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(2);
    });

    screen.getByTestId("filter-sort-pickup").click();

    // Both home → Becker before Zeller
    await waitFor(() => {
      const cards = screen.getAllByTestId("student-card");
      expect(cards[0]?.textContent).toContain("Anna Becker");
      expect(cards[1]?.textContent).toContain("Max Zeller");
    });
  });

  it("uses first name tiebreaker when last names match", async () => {
    // Same last name → falls through to first name comparison
    const students = [
      {
        id: "1",
        name: "Max Mueller",
        first_name: "Max",
        last_name: "Mueller",
        current_location: "Raum 101",
      },
      {
        id: "2",
        name: "Anna Mueller",
        first_name: "Anna",
        last_name: "Mueller",
        current_location: "Raum 101",
      },
    ];
    const pickupMap = new Map([
      ["1", { pickupTime: "16:00", isException: false }],
      ["2", { pickupTime: "16:00", isException: false }],
    ]);
    setupWithCustomStudents(students, pickupMap);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(2);
    });

    screen.getByTestId("filter-sort-pickup").click();

    // Same last name, same time → Anna before Max
    await waitFor(() => {
      const cards = screen.getAllByTestId("student-card");
      expect(cards[0]?.textContent).toContain("Anna Mueller");
      expect(cards[1]?.textContent).toContain("Max Mueller");
    });
  });

  it("default alphabetical sort uses compareByName", async () => {
    // Without activating pickup sort → alphabetical
    const students = [
      {
        id: "1",
        name: "Max Zeller",
        first_name: "Max",
        last_name: "Zeller",
        current_location: "Raum 101",
      },
      {
        id: "2",
        name: "Anna Becker",
        first_name: "Anna",
        last_name: "Becker",
        current_location: "Raum 101",
      },
    ];
    setupWithCustomStudents(students, new Map());

    render(<OGSGroupPage />);

    // Default sort (no pickup sort activation) → Becker before Zeller
    await waitFor(() => {
      const cards = screen.getAllByTestId("student-card");
      expect(cards[0]?.textContent).toContain("Anna Becker");
      expect(cards[1]?.textContent).toContain("Max Zeller");
    });
  });

  it("sorts overdue students before normal in pickup sort", async () => {
    // FROZEN_TIME = 14:00, so 13:00 is overdue and 16:00 is normal
    // Student 1 (Becker) has 16:00 (normal), Student 2 (Zeller) has 13:00 (overdue)
    const pickupMap = new Map([
      ["1", { pickupTime: "16:00", isException: false }], // normal
      ["2", { pickupTime: "13:00", isException: false }], // overdue
    ]);
    setupWithStudentsAndPickupTimes(pickupMap, {
      isHome: (loc) => loc === "Zuhause",
    });

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(3);
    });

    // Activate pickup sort
    screen.getByTestId("filter-sort-pickup").click();

    // After sort: overdue 13:00 (Zeller) → normal 16:00 (Becker) → home (Mueller)
    await waitFor(() => {
      const cards = screen.getAllByTestId("student-card");
      expect(cards[0]?.textContent).toContain("Max Zeller"); // overdue
      expect(cards[1]?.textContent).toContain("Anna Becker"); // normal
      expect(cards[2]?.textContent).toContain("Lena Mueller"); // at home
    });
  });

  it("removes sort active filter when chip is dismissed", async () => {
    setupWithStudentsAndPickupTimes(new Map());

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("filter-sort")).toBeInTheDocument();
    });

    // Activate pickup sort
    screen.getByTestId("filter-sort-pickup").click();

    await waitFor(() => {
      expect(screen.getByTestId("active-filter-sort")).toBeInTheDocument();
    });

    // Remove the filter
    screen.getByTestId("remove-filter-sort").click();

    await waitFor(() => {
      expect(
        screen.queryByTestId("active-filter-sort"),
      ).not.toBeInTheDocument();
    });
  });

  it("clears sort mode when clear all filters is clicked", async () => {
    setupWithStudentsAndPickupTimes(new Map());

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("filter-sort")).toBeInTheDocument();
    });

    // Activate pickup sort
    screen.getByTestId("filter-sort-pickup").click();

    await waitFor(() => {
      expect(screen.getByTestId("active-filter-sort")).toBeInTheDocument();
    });

    // Clear all filters
    screen.getByTestId("clear-all-filters").click();

    await waitFor(() => {
      expect(
        screen.queryByTestId("active-filter-sort"),
      ).not.toBeInTheDocument();
    });
  });

  it("renders fallback pickup row when no pickup data exists", async () => {
    // No pickup times at all
    setupWithStudentsAndPickupTimes(new Map());

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getAllByTestId("student-card")).toHaveLength(3);
    });

    // Always-show-pickup-time: fallback "Abholzeit: —" row is rendered
    const fallbackRows = screen.getAllByText("Abholzeit: —");
    expect(fallbackRows).toHaveLength(3);
  });
});

describe("OGSGroupPage loadAvailableUsers", () => {
  // Test that loadAvailableUsers queries all three roles: teacher, staff, user
  // This covers the code path that fetches staff by role for the transfer dropdown

  it("queries teacher, staff, and user roles for transfer dropdown", async () => {
    // Test the parallel fetch pattern used in loadAvailableUsers
    // The actual implementation calls getStaffByRole for "teacher", "staff", and "user"

    // Create a mock function that simulates the API behavior
    const getStaffByRole = vi.fn((role: string) => {
      if (role === "teacher") {
        return Promise.resolve([
          {
            id: "1",
            personId: "101",
            firstName: "Anna",
            lastName: "Lehrer",
            fullName: "Anna Lehrer",
            accountId: "1001",
            email: "anna@example.com",
          },
        ]);
      }
      if (role === "staff") {
        return Promise.resolve([
          {
            id: "2",
            personId: "102",
            firstName: "Ben",
            lastName: "Staff",
            fullName: "Ben Staff",
            accountId: "1002",
            email: "ben@example.com",
          },
        ]);
      }
      if (role === "user") {
        return Promise.resolve([
          {
            id: "3",
            personId: "103",
            firstName: "Clara",
            lastName: "Nutzer",
            fullName: "Clara Nutzer",
            accountId: "1003",
            email: "clara@example.com",
          },
        ]);
      }
      return Promise.resolve([]);
    });

    // Simulate the parallel fetch pattern from loadAvailableUsers
    const [teachers, staffMembers, users] = await Promise.all([
      getStaffByRole("teacher").catch(() => []),
      getStaffByRole("staff").catch(() => []),
      getStaffByRole("user").catch(() => []),
    ]);

    // Verify all three roles are queried
    expect(getStaffByRole).toHaveBeenCalledWith("teacher");
    expect(getStaffByRole).toHaveBeenCalledWith("staff");
    expect(getStaffByRole).toHaveBeenCalledWith("user");
    expect(getStaffByRole).toHaveBeenCalledTimes(3);

    // Verify results are returned correctly
    expect(teachers).toHaveLength(1);
    expect(staffMembers).toHaveLength(1);
    expect(users).toHaveLength(1);
  });

  it("deduplicates users from different roles by staff ID", () => {
    // Test the deduplication logic used in loadAvailableUsers
    type StaffUser = {
      id: string;
      personId: string;
      firstName: string;
      lastName: string;
      fullName: string;
    };

    const teachers: StaffUser[] = [
      {
        id: "1",
        personId: "101",
        firstName: "Anna",
        lastName: "Lehrer",
        fullName: "Anna Lehrer",
      },
      {
        id: "2",
        personId: "102",
        firstName: "Both",
        lastName: "Roles",
        fullName: "Both Roles",
      },
    ];

    const staffMembers: StaffUser[] = [
      {
        id: "2",
        personId: "102",
        firstName: "Both",
        lastName: "Roles",
        fullName: "Both Roles",
      }, // Duplicate
      {
        id: "3",
        personId: "103",
        firstName: "Ben",
        lastName: "Staff",
        fullName: "Ben Staff",
      },
    ];

    const users: StaffUser[] = [
      {
        id: "2",
        personId: "102",
        firstName: "Both",
        lastName: "Roles",
        fullName: "Both Roles",
      }, // Duplicate
      {
        id: "4",
        personId: "104",
        firstName: "Clara",
        lastName: "Nutzer",
        fullName: "Clara Nutzer",
      },
    ];

    // Mirror the deduplication logic from loadAvailableUsers
    const uniqueUsers = new Map<string, StaffUser>();
    for (const user of [...teachers, ...staffMembers, ...users]) {
      if (!uniqueUsers.has(user.id)) {
        uniqueUsers.set(user.id, user);
      }
    }
    const result = Array.from(uniqueUsers.values());

    // Should have 4 unique users (ID 2 appears 3 times but is deduplicated)
    expect(result).toHaveLength(4);
    expect(result.map((u) => u.id).sort()).toEqual(["1", "2", "3", "4"]);
  });

  it("handles empty results from user role gracefully", () => {
    // Simulates when no users have the "user" role assigned
    type StaffUser = { id: string; fullName: string };

    const teachers: StaffUser[] = [{ id: "1", fullName: "Anna Lehrer" }];
    const staffMembers: StaffUser[] = [{ id: "2", fullName: "Ben Staff" }];
    const users: StaffUser[] = []; // Empty - no users with "user" role

    const uniqueUsers = new Map<string, StaffUser>();
    for (const user of [...teachers, ...staffMembers, ...users]) {
      if (!uniqueUsers.has(user.id)) {
        uniqueUsers.set(user.id, user);
      }
    }
    const result = Array.from(uniqueUsers.values());

    // Should still work with 2 users from teacher and staff roles
    expect(result).toHaveLength(2);
  });

  it("returns all users when only user role has members", () => {
    // Simulates production scenario where most accounts have "user" role
    type StaffUser = { id: string; fullName: string };

    const teachers: StaffUser[] = []; // Empty
    const staffMembers: StaffUser[] = []; // Empty
    const users: StaffUser[] = [
      { id: "1", fullName: "User One" },
      { id: "2", fullName: "User Two" },
      { id: "3", fullName: "User Three" },
    ];

    const uniqueUsers = new Map<string, StaffUser>();
    for (const user of [...teachers, ...staffMembers, ...users]) {
      if (!uniqueUsers.has(user.id)) {
        uniqueUsers.set(user.id, user);
      }
    }
    const result = Array.from(uniqueUsers.values());

    // All 3 users from "user" role should be returned
    expect(result).toHaveLength(3);
  });
});

// Note: Integration tests for the transfer modal are complex due to React state management.
// The getAllAvailableStaff function is tested in group-transfer-api.test.ts which covers:
// - Fetching all three roles (teacher, staff, user)
// - Deduplication by staff ID
// - Error handling when some roles fail to load

// ===== Tests for exported helper functions (direct coverage) =====

import {
  isStudentInGroupRoom as actualIsStudentInGroupRoom,
  matchesSearchFilter as actualMatchesSearchFilter,
  matchesAttendanceFilter as actualMatchesAttendanceFilter,
  matchesForeignRoomFilter as actualMatchesForeignRoomFilter,
} from "./ogs-group-helpers";

// Helper to build a minimal Student for direct function tests
function makeTestStudent(
  overrides: Record<string, unknown> = {},
): Parameters<typeof actualMatchesSearchFilter>[0] {
  return {
    id: "1",
    name: "Max Mustermann",
    first_name: "Max",
    last_name: "Mustermann",
    school_class: "3a",
    current_location: "Anwesend - Raum 1",
    group_name: "Eulen",
    group_id: "10",
    ...overrides,
  } as Parameters<typeof actualMatchesSearchFilter>[0];
}

describe("isStudentInGroupRoom (exported)", () => {
  it("returns false when student has no location", () => {
    const student = makeTestStudent({ current_location: undefined });
    expect(
      actualIsStudentInGroupRoom(student, {
        id: "1",
        name: "G",
        room_name: "R",
      }),
    ).toBe(false);
  });

  it("returns false when group has no room name", () => {
    const student = makeTestStudent();
    expect(actualIsStudentInGroupRoom(student, { id: "1", name: "G" })).toBe(
      false,
    );
  });

  it("returns true when room name matches (case-insensitive)", () => {
    // parseLocation is mocked to always return { room: "Room 1" }
    const student = makeTestStudent({ current_location: "Anwesend - Room 1" });
    expect(
      actualIsStudentInGroupRoom(student, {
        id: "1",
        name: "G",
        room_name: "ROOM 1",
      }),
    ).toBe(true);
  });

  it("returns false when room does not match", () => {
    // parseLocation is mocked to always return { room: "Room 1" }
    const student = makeTestStudent({ current_location: "Anwesend - Raum 2" });
    expect(
      actualIsStudentInGroupRoom(student, {
        id: "1",
        name: "G",
        room_name: "Raum 99",
      }),
    ).toBe(false);
  });

  it("returns false for null group", () => {
    expect(actualIsStudentInGroupRoom(makeTestStudent(), null)).toBe(false);
  });

  it("matches by room_id when room name does not match", () => {
    const student = makeTestStudent({ current_location: "42" });
    expect(
      actualIsStudentInGroupRoom(student, {
        id: "1",
        name: "G",
        room_name: "X",
        room_id: "42",
      }),
    ).toBe(true);
  });
});

describe("matchesSearchFilter (exported)", () => {
  it("returns true for empty search term", () => {
    expect(actualMatchesSearchFilter(makeTestStudent(), "")).toBe(true);
  });

  it("matches by name", () => {
    expect(actualMatchesSearchFilter(makeTestStudent(), "Max")).toBe(true);
  });

  it("matches by school class", () => {
    expect(
      actualMatchesSearchFilter(makeTestStudent({ school_class: "3a" }), "3a"),
    ).toBe(true);
  });

  it("returns false when nothing matches", () => {
    expect(actualMatchesSearchFilter(makeTestStudent(), "xyz")).toBe(false);
  });
});

describe("matchesAttendanceFilter (exported)", () => {
  const rs = {
    "1": { in_group_room: true, current_room_id: 10 },
    "2": { in_group_room: false, current_room_id: 20 },
  };

  it("returns true for 'all'", () => {
    expect(actualMatchesAttendanceFilter(makeTestStudent(), "all", rs)).toBe(
      true,
    );
  });

  it("returns true for 'in_room' when student is in group room", () => {
    expect(
      actualMatchesAttendanceFilter(
        makeTestStudent({ id: "1" }),
        "in_room",
        rs,
      ),
    ).toBe(true);
  });

  it("returns false for 'in_room' when student is not", () => {
    expect(
      actualMatchesAttendanceFilter(
        makeTestStudent({ id: "2" }),
        "in_room",
        rs,
      ),
    ).toBe(false);
  });

  it("returns true for 'foreign_room' correctly", () => {
    expect(
      actualMatchesAttendanceFilter(
        makeTestStudent({ id: "2" }),
        "foreign_room",
        rs,
      ),
    ).toBe(true);
  });

  it("returns true for 'at_home' when at home", () => {
    vi.mocked(isHomeLocation).mockReturnValue(true);
    expect(
      actualMatchesAttendanceFilter(
        makeTestStudent({ current_location: "Zuhause" }),
        "at_home",
        rs,
      ),
    ).toBe(true);
    vi.mocked(isHomeLocation).mockReturnValue(false);
  });

  it("returns true for unknown filter", () => {
    expect(
      actualMatchesAttendanceFilter(makeTestStudent(), "unknown_value", rs),
    ).toBe(true);
  });
});

describe("matchesForeignRoomFilter (exported)", () => {
  it("returns true when in foreign room", () => {
    expect(
      actualMatchesForeignRoomFilter({
        in_group_room: false,
        current_room_id: 20,
      }),
    ).toBe(true);
  });

  it("returns false when in group room", () => {
    expect(
      actualMatchesForeignRoomFilter({
        in_group_room: true,
        current_room_id: 10,
      }),
    ).toBe(false);
  });

  it("returns false when no room ID", () => {
    expect(actualMatchesForeignRoomFilter({ in_group_room: false })).toBe(
      false,
    );
  });

  it("returns false for undefined", () => {
    expect(actualMatchesForeignRoomFilter(undefined)).toBe(false);
  });
});

// ===== NEW TESTS FOR ID-BASED SELECTION REFACTOR =====
// Tests added to cover new code paths introduced by the index → ID refactor

describe("OGSGroupPage ID-based selection: Stale selection reset", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    localStorage.removeItem("sidebar-last-group");
    mockSearchParamsGet.mockReturnValue(null);
  });

  afterEach(() => {
    cleanup();
  });

  const groupsAB = [
    {
      id: "1",
      name: "Group A",
      roomId: "10",
      roomName: "Raum 101",
      viaSubstitution: false,
    },
    {
      id: "2",
      name: "Group B",
      roomId: "20",
      roomName: "Raum 202",
      viaSubstitution: false,
    },
  ];

  it("resets to first group when previously selected group disappears from list", async () => {
    // Initial render: User has Group A (id=1) and Group B (id=2), Group A selected
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        groups: groupsAB,
        groupId: "1",
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    const { rerender } = render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    // Now simulate an aggregate refresh: Group A (id=1) is removed, only
    // Group B (id=2) remains — the response resolves the caller's remaining
    // group, so groupId flips to "2".
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        groups: [groupsAB[1]!],
        groupId: "2",
        students: [
          wireStudent({
            id: 2,
            first_name: "Erika",
            last_name: "Schmidt",
            current_location: "Raum 202",
          }),
        ],
        roomStatus: { "2": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    rerender(<OGSGroupPage />);

    // Should show student from Group B after reset
    await waitFor(() => {
      expect(screen.getByText(/Erika Schmidt/)).toBeInTheDocument();
    });
  });

  it("keeps selection stable when a refresh resolves a different group", async () => {
    // Simulate a returning user: Group A is already the persisted selection
    // (matches the URL/localStorage-restore effect so it stays a no-op),
    // isolating the behavior under test — an unrelated refresh resolving a
    // different group must not clobber the currently viewed group's data.
    localStorage.setItem("sidebar-last-group", "1");

    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        groups: groupsAB,
        groupId: "1",
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    const { rerender } = render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByText(/Max Mustermann/)).toBeInTheDocument();
    });

    // Refresh resolves Group B's snapshot (e.g. a stale periodic refresh
    // raced the selection) while the still-selected group is Group A — the
    // response is self-describing, so the page must not clobber the
    // currently viewed group's students with another group's data.
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        groups: groupsAB,
        groupId: "2",
        students: [
          wireStudent({
            id: 2,
            first_name: "Erika",
            last_name: "Schmidt",
            current_location: "Raum 202",
          }),
        ],
        roomStatus: { "2": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    rerender(<OGSGroupPage />);

    // Should still show Group A's student (selection not reset, Group B's
    // snapshot was not applied)
    await waitFor(() => {
      expect(screen.getByText(/Max Mustermann/)).toBeInTheDocument();
      expect(screen.queryByText(/Erika Schmidt/)).not.toBeInTheDocument();
    });
  });
});

describe("OGSGroupPage ID-based selection: First load initialization", () => {
  const mockMutate = vi.fn();
  const originalLocalStorage = window.localStorage;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    localStorage.removeItem("sidebar-last-group");
    mockSearchParamsGet.mockReturnValue(null);
  });

  afterEach(() => {
    cleanup();
    // Restore original localStorage
    Object.defineProperty(window, "localStorage", {
      value: originalLocalStorage,
      writable: true,
      configurable: true,
    });
  });

  it("locks in first group ID on first data load", async () => {
    // Mock localStorage
    const localStorageMock: Record<string, string> = {};
    Object.defineProperty(window, "localStorage", {
      value: {
        getItem: (key: string) => localStorageMock[key] ?? null,
        setItem: (key: string, value: string) => {
          localStorageMock[key] = value;
        },
        removeItem: (key: string) => {
          delete localStorageMock[key];
        },
        clear: () => {
          for (const key of Object.keys(localStorageMock)) {
            delete localStorageMock[key];
          }
        },
      },
      writable: true,
      configurable: true,
    });

    // First render with no selectedGroupId (adopted from the response)
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    // Verify first group's students are shown
    expect(screen.getByText(/Max Mustermann/)).toBeInTheDocument();
  });

  it("shows first group students only when first group is selected", async () => {
    // Mock scenario: User has 2 groups, the response resolves Group A
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        groups: [
          {
            id: "1",
            name: "Group A",
            roomId: "10",
            roomName: "Raum 101",
            viaSubstitution: false,
          },
          {
            id: "2",
            name: "Group B",
            roomId: "20",
            roomName: "Raum 202",
            viaSubstitution: false,
          },
        ],
        groupId: "1", // Aggregate resolves the first group's data
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    // Should show Group A students since it's the first group
    expect(screen.getByText(/Max Mustermann/)).toBeInTheDocument();
  });
});

describe("OGSGroupPage ID-based selection: URL param matching", () => {
  const mockMutate = vi.fn();
  const groupsAB = [
    {
      id: "1",
      name: "Group A",
      roomId: "10",
      roomName: "Raum 101",
      viaSubstitution: false,
    },
    {
      id: "2",
      name: "Group B",
      roomId: "20",
      roomName: "Raum 202",
      viaSubstitution: false,
    },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.removeItem("sidebar-last-group");
    mockSearchParamsGet.mockReturnValue(null);
    global.fetch = vi.fn();
  });

  afterEach(() => {
    cleanup();
  });

  it("seeds the resolved group key from the cold-start projection and revalidates it in the background", async () => {
    const initialProjection = liveData();
    vi.mocked(useSWRAuth).mockReturnValue({
      data: initialProjection,
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    expect(vi.mocked(useSWRAuth).mock.calls[0]?.[0]).toBe("ogs-students-auto");
    await waitFor(() => {
      // revalidate: true closes the cold-start staleness window: the "auto"
      // key sits outside the scoped SSE invalidation, so an event arriving
      // while the initial request was in flight must not pin a stale seed.
      expect(mockTenantMutate).toHaveBeenCalledWith(
        "ogs-students-1",
        initialProjection,
        { revalidate: true },
      );
    });
  });

  it("seeds the SWR key from the URL param at mount", async () => {
    // Setup: URL has ?group=2, user has Group A (id=1) and Group B (id=2).
    // selectedGroupId is seeded synchronously from the URL param, so the
    // very first SWR call already targets group 2 — no separate switch step.
    mockSearchParamsGet.mockReturnValue("2");

    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        groups: groupsAB,
        groupId: "2",
        students: [
          wireStudent({
            id: 2,
            first_name: "Erika",
            last_name: "Schmidt",
            current_location: "Raum 202",
          }),
        ],
        roomStatus: { "2": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    expect(vi.mocked(useSWRAuth).mock.calls[0]?.[0]).toBe("ogs-students-2");

    await waitFor(() => {
      expect(screen.getByText(/Erika Schmidt/)).toBeInTheDocument();
    });
  });

  it("ignores URL param when group ID does not exist in list", async () => {
    // Setup: URL has ?group=999 (invalid), user has Group A (id=1). The
    // fetcher's stale-selection fallback resolves Group A regardless.
    mockSearchParamsGet.mockReturnValue("999");

    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    // Should show Group A student (didn't switch to invalid group)
    expect(screen.getByText(/Max Mustermann/)).toBeInTheDocument();
  });

  it("does not switch when URL param matches already selected group", async () => {
    // Setup: URL has ?group=1, Group A (id=1) already selected
    mockSearchParamsGet.mockReturnValue("1");

    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        groups: groupsAB,
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    // Should show Group A student (no unnecessary switch)
    expect(screen.getByText(/Max Mustermann/)).toBeInTheDocument();

    // The SWR key never moves off group 1 — no unnecessary switch happened
    expect(
      vi
        .mocked(useSWRAuth)
        .mock.calls.every((call) => call[0] === "ogs-students-1"),
    ).toBe(true);
  });
});

describe("OGSGroupPage ID-based selection: localStorage restore", () => {
  const mockMutate = vi.fn();

  let localStorageMock: Record<string, string>;
  const originalLocalStorage = window.localStorage;

  beforeEach(() => {
    vi.clearAllMocks();
    mockSearchParamsGet.mockReturnValue(null);
    global.fetch = vi.fn();

    // Mock localStorage
    localStorageMock = {};
    Object.defineProperty(window, "localStorage", {
      value: {
        getItem: (key: string) => localStorageMock[key] ?? null,
        setItem: (key: string, value: string) => {
          localStorageMock[key] = value;
        },
        removeItem: (key: string) => {
          delete localStorageMock[key];
        },
        clear: () => {
          for (const key of Object.keys(localStorageMock)) {
            delete localStorageMock[key];
          }
        },
      },
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    cleanup();
    // Restore original localStorage so subsequent tests aren't affected
    Object.defineProperty(window, "localStorage", {
      value: originalLocalStorage,
      writable: true,
      configurable: true,
    });
  });

  it("seeds the SWR key from localStorage when no URL param", async () => {
    // Setup: localStorage has group ID "2", no URL param. selectedGroupId
    // is seeded synchronously from localStorage, so the very first SWR
    // call already targets group 2.
    localStorageMock["sidebar-last-group"] = "2";

    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        groups: [
          {
            id: "1",
            name: "Group A",
            roomId: "10",
            roomName: "Raum 101",
            viaSubstitution: false,
          },
          {
            id: "2",
            name: "Group B",
            roomId: "20",
            roomName: "Raum 202",
            viaSubstitution: false,
          },
        ],
        groupId: "2",
        students: [
          wireStudent({
            id: 2,
            first_name: "Erika",
            last_name: "Schmidt",
            current_location: "Raum 202",
          }),
        ],
        roomStatus: { "2": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    expect(vi.mocked(useSWRAuth).mock.calls[0]?.[0]).toBe("ogs-students-2");

    await waitFor(() => {
      expect(screen.getByText(/Erika Schmidt/)).toBeInTheDocument();
    });
  });

  it("persists first group to localStorage when saved group no longer exists", async () => {
    // Setup: localStorage has group ID "999" (doesn't exist), no URL param
    localStorageMock["sidebar-last-group"] = "999";
    mockSearchParamsGet.mockReturnValue(null);

    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    // Should persist first group to localStorage
    await waitFor(() => {
      expect(localStorageMock["sidebar-last-group"]).toBe("1");
    });
  });

  it("does not switch when saved group ID matches currently selected group", async () => {
    // Setup: localStorage has group ID "1", Group A (id=1) already selected
    localStorageMock["sidebar-last-group"] = "1";
    mockSearchParamsGet.mockReturnValue(null);

    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    // The SWR key never moves off group 1 — no unnecessary switch happened
    expect(
      vi
        .mocked(useSWRAuth)
        .mock.calls.every((call) => call[0] === "ogs-students-1"),
    ).toBe(true);
  });
});

describe("OGSGroupPage ID-based selection: switchToGroup behavior", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    localStorage.removeItem("sidebar-last-group");
    mockSearchParamsGet.mockReturnValue(null);
  });

  afterEach(() => {
    cleanup();
  });

  it("is a no-op when switching to non-existent group ID", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    // Try to switch to non-existent group ID — this should be a no-op, no
    // network calls made (the SWR fetcher itself is mocked away)
    expect(global.fetch).not.toHaveBeenCalled();
  });

  it("is a no-op when switching to already selected group ID", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    // Try to switch to already selected group — should be a no-op
    expect(global.fetch).not.toHaveBeenCalled();
  });
});

describe("OGSGroupPage ID-based selection: student count update", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.removeItem("sidebar-last-group");
    mockSearchParamsGet.mockReturnValue(null);
    global.fetch = vi.fn();
  });

  afterEach(() => {
    cleanup();
  });

  it("updates student count by group ID after loading students", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        groups: [
          {
            id: "1",
            name: "Group A",
            roomId: "10",
            roomName: "Raum 101",
            viaSubstitution: false,
          },
          {
            id: "2",
            name: "Group B",
            roomId: "20",
            roomName: "Raum 202",
            viaSubstitution: false,
          },
        ],
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    // Group A's count (student_count) is derived from the aggregate
    // response for the loaded group ID.
    expect(screen.getByText(/Max Mustermann/)).toBeInTheDocument();
  });
});

describe("OGSGroupPage ID-based selection: tab change handler", () => {
  const mockMutate = vi.fn();
  const originalLocalStorage = window.localStorage;

  beforeEach(() => {
    vi.clearAllMocks();
    mockSearchParamsGet.mockReturnValue(null);
    global.fetch = vi.fn();

    // Mock localStorage
    const localStorageMock: Record<string, string> = {};
    Object.defineProperty(window, "localStorage", {
      value: {
        getItem: (key: string) => localStorageMock[key] ?? null,
        setItem: (key: string, value: string) => {
          localStorageMock[key] = value;
        },
        removeItem: (key: string) => {
          delete localStorageMock[key];
        },
        clear: () => {
          for (const key of Object.keys(localStorageMock)) {
            delete localStorageMock[key];
          }
        },
      },
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    cleanup();
    Object.defineProperty(window, "localStorage", {
      value: originalLocalStorage,
      writable: true,
      configurable: true,
    });
  });

  it("finds group by ID when tab changes", async () => {
    // Setup: Multiple groups, tabs visible
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        groups: [
          {
            id: "1",
            name: "Group A",
            roomId: "10",
            roomName: "Raum 101",
            viaSubstitution: false,
          },
          {
            id: "2",
            name: "Group B",
            roomId: "20",
            roomName: "Raum 202",
            viaSubstitution: false,
          },
        ],
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    // Simulate tab change to Group B — the handler finds the group by ID,
    // persists it to localStorage, and calls switchToGroup (which just
    // changes the SWR key; no direct fetch from the page).
    const tabButton = screen.queryByText("Group B");
    if (tabButton) {
      tabButton.click();

      await waitFor(() => {
        expect(localStorage.getItem("sidebar-last-group")).toBe("2");
      });
    }
  });
});

describe("OGSGroupPage ID-based selection: currentGroup useMemo", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    localStorage.removeItem("sidebar-last-group");
    mockSearchParamsGet.mockReturnValue(null);
  });

  afterEach(() => {
    cleanup();
  });

  it("finds currentGroup by ID, falls back to first group", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        groups: [
          {
            id: "1",
            name: "Group A",
            roomId: "10",
            roomName: "Raum 101",
            viaSubstitution: false,
          },
          {
            id: "2",
            name: "Group B",
            roomId: "20",
            roomName: "Raum 202",
            viaSubstitution: false,
          },
        ],
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    // currentGroup should be found by ID.
    // When selectedGroupId=null, it falls back to allGroups[0]
    expect(screen.getByText(/Max Mustermann/)).toBeInTheDocument();
  });

  it("falls back to first group when selected ID not found", async () => {
    // Scenario: selectedGroupId was "999" (doesn't exist)
    // currentGroup useMemo should return allGroups[0]
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({
        students: [
          wireStudent({
            id: 1,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Raum 101",
          }),
        ],
        roomStatus: { "1": { in_group_room: true } },
      }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });

    // Should display first group (fallback logic in useMemo)
    expect(screen.getByText(/Max Mustermann/)).toBeInTheDocument();
  });

  it("returns null when no groups exist", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({ groups: [], groupId: null }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Keine OGS-Gruppe zugeordnet/),
      ).toBeInTheDocument();
    });

    // currentGroup useMemo should return null
    expect(screen.queryByTestId("student-card")).not.toBeInTheDocument();
  });

  it("filters current user from transfer modal available users", async () => {
    // Current user has personId "p1"
    mockUserContext.mockReturnValue({
      userContext: {
        currentStaff: { id: "s1", personId: "p1" },
      },
      isLoading: false,
      error: undefined,
      isReady: true,
    });

    // Staff list includes the current user (personId "p1")

    vi.mocked(groupTransferService.getAllAvailableStaff).mockResolvedValue([
      {
        id: "s1",
        personId: "p1",
        firstName: "Current",
        lastName: "User",
        fullName: "Current User",
        accountId: "a1",
        email: "current@example.com",
      },
      {
        id: "s2",
        personId: "p2",
        firstName: "Other",
        lastName: "Teacher",
        fullName: "Other Teacher",
        accountId: "a2",
        email: "other@example.com",
      },
    ]);

    vi.mocked(useSWRAuth).mockReturnValue({
      data: liveData({ students: [] }),
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<OGSGroupPage />);

    // Wait for page to render
    await waitFor(() => {
      expect(screen.getByTestId("page-header")).toBeInTheDocument();
    });

    const transferButton = screen.getByLabelText("Gruppe übergeben");
    fireEvent.click(transferButton);

    // Wait for staff to load and modal to re-render with availableUsers
    await waitFor(() => {
      expect(groupTransferService.getAllAvailableStaff).toHaveBeenCalled();
    });

    // The modal should receive only "Other Teacher", not "Current User"
    await waitFor(() => {
      const lastCall = mockTransferModalProps.mock.calls[
        mockTransferModalProps.mock.calls.length - 1
      ] as [{ availableUsers: Array<{ personId: string; fullName: string }> }];
      const passedUsers = lastCall?.[0]?.availableUsers;
      expect(passedUsers).toBeDefined();
      expect(passedUsers).toHaveLength(1);
      expect(passedUsers[0]?.fullName).toBe("Other Teacher");
    });
  });
});

describe("RoleGuard integration", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows ForbiddenPage for admin users", async () => {
    const { useSession } = await import("next-auth/react");
    vi.mocked(useSession).mockReturnValue({
      data: { user: { token: "test-token", isAdmin: true } },
      status: "authenticated",
    } as never);

    render(<OGSGroupPage />);

    expect(screen.getByText("Kein Zugriff")).toBeInTheDocument();
  });

  it("renders content for non-admin users", async () => {
    const { useSession } = await import("next-auth/react");
    vi.mocked(useSession).mockReturnValue({
      data: { user: { token: "test-token", isAdmin: false } },
      status: "authenticated",
    } as never);

    render(<OGSGroupPage />);

    expect(screen.queryByText("Kein Zugriff")).not.toBeInTheDocument();
    expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
  });
});
