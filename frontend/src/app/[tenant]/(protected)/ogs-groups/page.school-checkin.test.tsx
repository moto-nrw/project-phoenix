/**
 * Page-level wiring tests for school check-in/out integration.
 * Verifies that OGSGroupPage:
 *   1. Renders the SchoolCheckinFab on binary-mode tenants.
 *   2. Forwards check-in mode state + per-student handlers to StudentCard.
 *
 * The hook itself has its own unit test (use-school-checkin-mode.test.ts);
 * here we mock it so we can drive state deterministically and focus on
 * the page's glue rather than SWR/toast infrastructure.
 *
 * NOTE: tests previously asserted on the legacy SchoolCheckinToggle which
 * lived in the page header. The toggle was replaced by a floating FAB
 * (SchoolCheckinFab) rendered outside the header. The behavioural
 * assertions are unchanged — only the mock target moved.
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
  mockFab,
} = vi.hoisted(() => ({
  mockToggleActive: vi.fn(),
  mockToggleStudent: vi.fn(),
  mockUseSchoolCheckinMode: vi.fn(),
  mockStudentCard: vi.fn(),
  mockFab: vi.fn(),
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

// Page gates the FAB on binary mode; the global tenant-provider mock
// (src/test/setup.ts) defaults to "detailed", which would hide the trigger
// and make every assertion here fail. Override to "binary".
vi.mock("~/lib/tenant-context", () => ({
  useTenant: vi.fn(() => ({ tenantSlug: "test-tenant", tenant: null })),
  useTenantSlugSafe: vi.fn(() => "test-tenant"),
  // OpenCareModeGuard (#1544) wraps the page; fixed_groups keeps it inert.
  // Its useTenantAwarePath call needs the routing-mode selector too.
  useOpenCareGroupMode: vi.fn(() => false),
  useTenantRoutingModeSafe: vi.fn(() => "subdomain"),
  // useStudentPhotosEnabled (used by ogs-groups for the avatar-clearance
  // spacer) reads tenant.studentPhotosEnabled via this selector. The mock
  // returns null which makes the hook resolve to enabled=false — fine for
  // these tests since they don't exercise the spacer path.
  useTenantSafe: vi.fn(() => null),
  usePresenceMode: vi.fn(() => "binary"),
  useNFCEnabled: vi.fn(() => true),
  TenantProvider: ({ children }: { children: React.ReactNode }) => children,
}));

// Der Auslöser existiert pro Breakpoint einmal: als Kopfaktion (inline) und
// als schwebender Knopf (floating). Die Testkennung trägt deshalb die
// Variante, damit die Abfragen eindeutig bleiben.
vi.mock("~/components/students/school-checkin-fab", () => ({
  SchoolCheckinFab: (props: {
    isActive: boolean;
    onToggle: () => void;
    successCount: number;
    pendingCount: number;
    variant: string;
  }) => {
    mockFab(props);
    return (
      <button
        type="button"
        data-testid={`school-checkin-fab-${props.variant}`}
        data-active={props.isActive}
        data-pending={props.pendingCount}
        data-success-count={props.successCount}
        onClick={props.onToggle}
      >
        fab
      </button>
    );
  },
}));

// SchoolCheckinModeMobile is page-tested in its own file; mock here so we
// don't have to expand the location-helper mock surface.
vi.mock("~/components/students/school-checkin-mode-mobile", () => ({
  SchoolCheckinModeMobile: (props: {
    isActive: boolean;
    onToggle: () => void;
    successCount: number;
    pendingCount: number;
  }) => (
    <button
      type="button"
      data-testid="school-checkin-mobile"
      data-active={props.isActive}
      onClick={props.onToggle}
    >
      mobile
    </button>
  ),
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
        type="button"
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
  hasEffectiveAdminScope: () => false,
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

vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
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

// Partial mock via importOriginal — MotoConceptIcon (substitution badge)
// pulls in MOTO_COLOR_PALETTE, which must stay real.
vi.mock("~/lib/location-helper", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/location-helper")>();
  return {
    ...actual,
    LOCATION_STATUSES: { PRESENT: "Anwesend" },
    isHomeLocation: (loc?: string) => loc === "Zuhause",
    isSchoolyardLocation: (loc?: string) => loc === "Schulhof",
    isTransitLocation: () => false,
    parseLocation: (loc?: string) => ({ status: loc ?? "Anwesend" }),
    LOCATION_COLORS: { GROUP_ROOM: "#83CD2D" },
  };
});

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
  useMinuteClock: () => new Date(),
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

const mockTenantMutate = vi.hoisted(() => vi.fn());

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  useTenantMutate: () => mockTenantMutate,
}));

vi.mock("./ogs-group-helpers", () => ({
  countCheckedInStudents: (students: Array<{ current_location?: string }>) =>
    students.filter((student) => student.current_location !== "Zuhause").length,
  formatGroupLabelWithAttendance: (group: {
    name: string;
    present_count?: number;
    student_count?: number;
  }) =>
    group.student_count === undefined
      ? group.name
      : `${group.name} ${group.present_count ?? 0}/${group.student_count}`,
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
      successCount: 0,
      toggle: mockToggleStudent,
    });

    // Seed SWR with one group + one student so cards render.
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        groups: [
          {
            id: "1",
            name: "Gruppe A",
            roomId: "10",
            roomName: "Raum 1",
            viaSubstitution: false,
            isPersonal: true,
          },
        ],
        groupId: "1",
        students: [
          {
            id: 42,
            first_name: "Max",
            last_name: "Mustermann",
            school_class: "",
            current_location: "Anwesend",
            sick: false,
            excused: false,
            class_trip: false,
          },
        ],
        roomStatus: { "42": { in_group_room: true } },
        pickupTimes: new Map(),
        trackingIndicators: { labels: [], results: {} },
        transfers: [],
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

  it("renders the SchoolCheckinFab on binary-mode tenants", async () => {
    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-42")).toBeInTheDocument();
    });

    expect(screen.getByTestId("school-checkin-fab-inline")).toBeInTheDocument();
  });

  it("clicking the FAB calls toggleActive on the hook", async () => {
    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-42")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("school-checkin-fab-inline"));
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
      successCount: 0,
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
      successCount: 0,
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

  it("forwards pending count to the FAB via prop", async () => {
    mockUseSchoolCheckinMode.mockReturnValue({
      isActive: true,
      toggleActive: mockToggleActive,
      deactivate: vi.fn(),
      pendingIds: new Set<string>(["42", "99"]),
      successCount: 0,
      toggle: mockToggleStudent,
    });

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-42")).toBeInTheDocument();
    });

    expect(screen.getByTestId("school-checkin-fab-inline")).toHaveAttribute(
      "data-pending",
      "2",
    );
  });

  it("forwards success count to the FAB via prop", async () => {
    mockUseSchoolCheckinMode.mockReturnValue({
      isActive: true,
      toggleActive: mockToggleActive,
      deactivate: vi.fn(),
      pendingIds: new Set<string>(),
      successCount: 5,
      toggle: mockToggleStudent,
    });

    render(<OGSGroupPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-42")).toBeInTheDocument();
    });

    expect(screen.getByTestId("school-checkin-fab-inline")).toHaveAttribute(
      "data-success-count",
      "5",
    );
  });

  it("marks the card as pending when its id is in pendingIds", async () => {
    mockUseSchoolCheckinMode.mockReturnValue({
      isActive: true,
      toggleActive: mockToggleActive,
      deactivate: vi.fn(),
      pendingIds: new Set<string>(["42"]),
      successCount: 0,
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
