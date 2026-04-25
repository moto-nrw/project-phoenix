/**
 * Page-level wiring tests for school check-in/out integration.
 * Verifies that OGSGroupPage:
 *   1. Renders the SchoolCheckinToggle in the header action slot.
 *   2. Forwards check-in mode state + per-student handlers to StudentCard.
 *
 * The hook itself has its own unit test (use-school-checkin-mode.test.ts);
 * here we mock it so we can drive state deterministically and focus on
 * the page's glue rather than SWR/toast infrastructure.
 */
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// ── Hoisted mocks ───────────────────────────────────────────────────────────
const {
  mockToggleActive,
  mockToggleStudent,
  mockUseSchoolCheckinMode,
  mockStudentCard,
  mockDesktopToggle,
  mockMobileToggle,
} = vi.hoisted(() => ({
  mockToggleActive: vi.fn(),
  mockToggleStudent: vi.fn(),
  mockUseSchoolCheckinMode: vi.fn(),
  mockStudentCard: vi.fn(),
  mockDesktopToggle: vi.fn(),
  mockMobileToggle: vi.fn(),
}));

vi.mock("~/lib/hooks/use-school-checkin-mode", () => ({
  useSchoolCheckinMode: () => mockUseSchoolCheckinMode(),
  deriveCheckinState: (loc?: string | null) => {
    if (loc === "Schulhof") return "schulhof";
    if (loc === "Zuhause") return "abwesend";
    if (loc && loc !== "") return "anwesend";
    return "unknown";
  },
}));

// Page gates the toggle on binary mode; the global tenant-provider mock
// (src/test/setup.ts) defaults to "detailed", which would hide the button
// and make every assertion here fail. Override to "binary".
vi.mock("~/components/tenant/tenant-provider", () => ({
  useTenant: vi.fn(() => ({ tenantSlug: "test-tenant", tenant: null })),
  useTenantSlugSafe: vi.fn(() => "test-tenant"),
  usePresenceMode: vi.fn(() => "binary"),
  TenantProvider: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("~/components/students/school-checkin-toggle", () => ({
  SchoolCheckinToggle: (props: {
    isActive: boolean;
    onToggle: () => void;
    pendingCount?: number;
  }) => {
    mockDesktopToggle(props);
    return (
      <button
        data-testid="school-checkin-toggle"
        data-active={props.isActive}
        data-pending={props.pendingCount ?? 0}
        onClick={props.onToggle}
      >
        toggle
      </button>
    );
  },
  SchoolCheckinToggleMobile: (props: {
    isActive: boolean;
    onToggle: () => void;
    pendingCount?: number;
  }) => {
    mockMobileToggle(props);
    return (
      <button
        data-testid="school-checkin-toggle-mobile"
        data-active={props.isActive}
        onClick={props.onToggle}
      >
        mobile-toggle
      </button>
    );
  },
}));

// Capture StudentCard prop calls so we can assert checkin props end to end.
vi.mock("~/components/students/student-card", () => ({
  StudentCard: (props: {
    studentId: string;
    firstName?: string;
    lastName?: string;
    checkinMode?: boolean;
    checkinState?: string;
    isCheckinPending?: boolean;
    onCheckinClick?: () => void;
    onClick: () => void;
  }) => {
    mockStudentCard(props);
    return (
      <button
        data-testid={`student-card-${props.studentId}`}
        data-checkin-mode={props.checkinMode ?? false}
        data-checkin-state={props.checkinState ?? ""}
        data-checkin-pending={props.isCheckinPending ?? false}
        onClick={props.checkinMode ? props.onCheckinClick : props.onClick}
      >
        {props.firstName} {props.lastName}
      </button>
    );
  },
  StudentInfoRow: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  PickupTimeRow: () => <div />,
  ArrivalTimeRow: () => <div />,
}));

// ── Standard page mocks ─────────────────────────────────────────────────────
function createLocalStorageMock() {
  const store: Record<string, string> = {};
  return {
    getItem: (k: string) => store[k] ?? null,
    setItem: (k: string, v: string) => {
      store[k] = v;
    },
    removeItem: (k: string) => {
      delete store[k];
    },
    clear: () => {
      for (const k of Object.keys(store)) delete store[k];
    },
  };
}
Object.defineProperty(window, "localStorage", {
  value: createLocalStorageMock(),
  writable: true,
  configurable: true,
});

vi.mock("~/lib/auth-utils", () => ({
  isAdmin: () => false,
  isCaregiver: () => true,
  hasRole: (_: unknown, r: string) => r === "user",
}));

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { token: "t" } },
    status: "authenticated",
  })),
}));

const mockPush = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
  useSearchParams: () => ({ get: (_k?: string) => null }),
  redirect: vi.fn(),
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mockPush }),
}));

const mockToast = {
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
};
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => mockToast,
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
  useBreadcrumb: () => ({ breadcrumb: {}, setBreadcrumb: vi.fn() }),
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading" />,
}));

vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({
    actionButton,
    mobileActionButton,
  }: {
    actionButton?: React.ReactNode;
    mobileActionButton?: React.ReactNode;
  }) => (
    <div data-testid="page-header">
      <div data-testid="header-action">{actionButton}</div>
      <div data-testid="header-mobile-action">{mobileActionButton}</div>
    </div>
  ),
}));

vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message }: { message: string }) => <div>{message}</div>,
}));

vi.mock("~/components/ui/empty-student-results", () => ({
  EmptyStudentResults: () => <div data-testid="empty-results" />,
}));

vi.mock("~/lib/api", () => ({
  studentService: {
    getStudents: vi.fn(() => Promise.resolve({ students: [] })),
  },
}));

vi.mock("~/lib/location-helper", () => ({
  LOCATION_STATUSES: { PRESENT: "Anwesend" },
  isHomeLocation: (loc?: string) => loc === "Zuhause",
  isSchoolyardLocation: (loc?: string) => loc === "Schulhof",
  isTransitLocation: () => false,
  parseLocation: (loc?: string) => ({ status: loc ?? "Anwesend" }),
  LOCATION_COLORS: { GROUP_ROOM: "#83CD2D" },
}));

vi.mock("@/components/ui/location-badge", () => ({
  LocationBadge: () => <span />,
}));

vi.mock("@/components/ui/student-presence-badge", () => ({
  StudentPresenceBadge: () => <span />,
}));

vi.mock("~/components/sse/SSEErrorBoundary", () => ({
  SSEErrorBoundary: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

vi.mock("~/components/groups/group-transfer-modal", () => ({
  GroupTransferModal: () => null,
}));

vi.mock("~/lib/group-transfer-api", () => ({
  groupTransferService: {
    getAllAvailableStaff: vi.fn(() => Promise.resolve([])),
    getActiveTransfersForGroup: vi.fn(() => Promise.resolve([])),
    transferGroup: vi.fn(),
    cancelTransferBySubstitutionId: vi.fn(),
  },
}));

vi.mock("~/lib/pickup-schedule-api", () => ({
  fetchBulkPickupTimes: vi.fn(() => Promise.resolve(new Map())),
}));

vi.mock("~/lib/pickup-helpers", () => ({
  getPickupUrgency: () => "none",
  useMinuteClock: () => new Date(),
  combinePickupNotes: () => undefined,
}));

vi.mock("~/lib/active-api", () => ({
  activeService: {
    getTrackingIndicators: vi.fn(() =>
      Promise.resolve({ labels: [], results: {} }),
    ),
  },
}));

vi.mock("~/components/students/tracking-indicators", () => ({
  TrackingIndicators: () => <span />,
}));

vi.mock("~/lib/hooks/use-user-context", () => ({
  useUserContext: () => ({
    userContext: { currentStaff: null },
    isLoading: false,
    error: undefined,
    isReady: true,
  }),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  useTenantMutate: () => vi.fn(),
}));

vi.mock("./ogs-group-helpers", () => ({
  isStudentInGroupRoom: () => true,
  matchesSearchFilter: () => true,
  matchesAttendanceFilter: () => true,
}));

vi.mock("~/components/auth/role-guard", () => ({
  RoleGuard: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

import { useSWRAuth } from "~/lib/swr";
import OGSGroupPage from "./page";

describe("OGSGroupPage — school check-in wiring", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: async () => ({ success: true, data: {} }),
      }),
    ) as unknown as typeof fetch;

    // Default hook state — inactive, nothing pending.
    mockUseSchoolCheckinMode.mockReturnValue({
      isActive: false,
      toggleActive: mockToggleActive,
      deactivate: vi.fn(),
      pendingIds: new Set<string>(),
      toggle: mockToggleStudent,
    });

    // Seed SWR with one group + one student so cards render.
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: 1,
            name: "Gruppe A",
            room_id: 10,
            room: { id: 10, name: "Raum 1" },
          },
        ],
        students: [
          {
            id: 42,
            first_name: "Max",
            last_name: "Mustermann",
            current_location: "Anwesend",
          },
        ],
        roomStatus: {
          student_room_status: { "42": { in_group_room: true } },
        },
        substitutions: [],
        pickupTimes: [],
        firstGroupId: "1",
      },
      isLoading: false,
      error: null,
      mutate: vi.fn(),
      isValidating: false,
    } as never);
  });

  afterEach(() => {
    cleanup();
  });

  it("renders the SchoolCheckinToggle in the header action slot", async () => {
    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-42")).toBeInTheDocument();
    });

    // At least one toggle variant should be present (desktop OR mobile,
    // depending on the simulated viewport — both are wired up).
    const toggles = screen.queryAllByTestId(/school-checkin-toggle/);
    expect(toggles.length).toBeGreaterThan(0);
  });

  it("clicking the toggle calls toggleActive on the hook", async () => {
    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-42")).toBeInTheDocument();
    });

    const toggles = screen.queryAllByTestId(/school-checkin-toggle/);
    fireEvent.click(toggles[0]!);
    expect(mockToggleActive).toHaveBeenCalledTimes(1);
  });

  it("passes checkinMode=false to StudentCard when hook is inactive", async () => {
    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-42")).toBeInTheDocument();
    });

    const card = screen.getByTestId("student-card-42");
    expect(card).toHaveAttribute("data-checkin-mode", "false");
  });

  it("passes checkinMode=true + derived state when hook is active", async () => {
    mockUseSchoolCheckinMode.mockReturnValue({
      isActive: true,
      toggleActive: mockToggleActive,
      deactivate: vi.fn(),
      pendingIds: new Set<string>(),
      toggle: mockToggleStudent,
    });

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-42")).toBeInTheDocument();
    });

    const card = screen.getByTestId("student-card-42");
    expect(card).toHaveAttribute("data-checkin-mode", "true");
    expect(card).toHaveAttribute("data-checkin-state", "anwesend");
  });

  it("clicking a student card in check-in mode fires hook.toggle with state", async () => {
    mockUseSchoolCheckinMode.mockReturnValue({
      isActive: true,
      toggleActive: mockToggleActive,
      deactivate: vi.fn(),
      pendingIds: new Set<string>(),
      toggle: mockToggleStudent,
    });

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-42")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("student-card-42"));
    expect(mockToggleStudent).toHaveBeenCalledWith("42", "anwesend");
    // Did not navigate in check-in mode.
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("clicking a student card when inactive navigates (no toggle)", async () => {
    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-42")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("student-card-42"));
    expect(mockPush).toHaveBeenCalledWith("/students/42?from=/ogs-groups");
    expect(mockToggleStudent).not.toHaveBeenCalled();
  });

  it("surfaces pending count on the toggle via prop", async () => {
    mockUseSchoolCheckinMode.mockReturnValue({
      isActive: true,
      toggleActive: mockToggleActive,
      deactivate: vi.fn(),
      pendingIds: new Set<string>(["42", "99"]),
      toggle: mockToggleStudent,
    });

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-42")).toBeInTheDocument();
    });

    // Either desktop or mobile toggle captures pendingCount=2.
    const captured = [
      ...mockDesktopToggle.mock.calls,
      ...mockMobileToggle.mock.calls,
    ].map((c) => c[0] as { pendingCount?: number });
    expect(captured.some((p) => p.pendingCount === 2)).toBe(true);
  });

  it("marks the card as pending when its id is in pendingIds", async () => {
    mockUseSchoolCheckinMode.mockReturnValue({
      isActive: true,
      toggleActive: mockToggleActive,
      deactivate: vi.fn(),
      pendingIds: new Set<string>(["42"]),
      toggle: mockToggleStudent,
    });

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-42")).toBeInTheDocument();
    });

    expect(screen.getByTestId("student-card-42")).toHaveAttribute(
      "data-checkin-pending",
      "true",
    );
  });
});
