/**
 * Page-level wiring tests for StudentSearchPage + school check-in/out.
 * Asserts the FAB renders on binary-mode tenants and that StudentCard
 * receives the correct check-in props from the hook.
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
    if (loc === "Zuhause" || loc === "") return "abwesend";
    if (loc) return "anwesend";
    return "unknown";
  },
  checkoutConfirmationRoom: () => null,
}));

// Page gates the toggle on binary mode; override the global mock
// (src/test/setup.ts defaults to "detailed") so the button renders.
vi.mock("~/lib/tenant-context", () => ({
  useTenant: vi.fn(() => ({ tenantSlug: "test-tenant", tenant: null })),
  useTenantSafe: vi.fn(() => ({
    tenantSlug: "test-tenant",
    tenant: { studentPhotosEnabled: true },
  })),
  useTenantSlugSafe: vi.fn(() => "test-tenant"),
  usePresenceMode: vi.fn(() => "binary"),
  useAttendanceWebEnabled: vi.fn(() => true),
  useNFCEnabled: vi.fn(() => true),
  TenantProvider: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("~/components/students/school-checkin-fab", () => ({
  SchoolCheckinFab: (props: {
    isActive: boolean;
    onToggle: () => void;
    successCount: number;
    pendingCount: number;
  }) => {
    mockFab(props);
    return (
      <button
        type="button"
        data-testid="school-checkin-fab"
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
        onClick={props.checkinMode ? props.onCheckinClick : props.onClick}
      >
        {props.firstName} {props.lastName}
      </button>
    );
  },
  SchoolClassIcon: () => <span />,
  GroupIcon: () => <span />,
  DepartureModeIcon: () => <span />,
  StudentInfoRow: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  PickupTimeRow: () => <div />,
  ArrivalTimeRow: () => <div />,
}));

const mockPush = vi.fn();
const mockSearchParams = new URLSearchParams();
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { token: "t" } },
    status: "authenticated",
  })),
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: vi.fn() }),
  useSearchParams: () => mockSearchParams,
}));
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mockPush }),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
  useBreadcrumb: () => ({ breadcrumb: {}, setBreadcrumb: vi.fn() }),
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading" />,
}));

vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message }: { message: string }) => <div>{message}</div>,
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

vi.mock("@/components/ui/location-badge", () => ({
  LocationBadge: () => <span />,
}));

vi.mock("@/components/ui/student-presence-badge", () => ({
  StudentPresenceBadge: () => <span />,
}));

vi.mock("~/lib/location-helper", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/location-helper")>();
  return {
    ...actual,
    isHomeLocation: (l?: string) => l === "Zuhause",
    isPresentLocation: (l?: string) =>
      !!l && l !== "Zuhause" && l !== "Schulhof",
    isTransitLocation: () => false,
    isSchoolyardLocation: (l?: string) => l === "Schulhof",
  };
});

vi.mock("~/lib/student-helpers", () => ({
  SCHOOL_YEAR_FILTER_OPTIONS: [{ value: "all", label: "Alle" }],
  getSchoolYear: () => "1",
}));

vi.mock("~/lib/pickup-helpers", () => ({
  useMinuteClock: () => new Date(),
}));

vi.mock("~/components/students/tracking-indicators", () => ({
  TrackingIndicators: () => <span />,
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
    getStudents: vi.fn(() => Promise.resolve({ students: [] })),
  },
  groupService: {
    getGroups: vi.fn(() => Promise.resolve([])),
  },
}));

vi.mock("~/lib/swr", () => ({
  useImmutableSWR: vi.fn(),
  useSWRAuth: vi.fn(),
  mutate: vi.fn(),
  useTenantMutate: () => vi.fn(),
}));

import { useImmutableSWR, useSWRAuth } from "~/lib/swr";
import StudentSearchPage from "./page";

const mockStudents = [
  {
    id: "7",
    first_name: "Max",
    second_name: "Mustermann",
    school_class: "1a",
    group_name: "Gruppe A",
    current_location: "Anwesend",
    has_full_access: true,
  },
];

describe("StudentSearchPage — school check-in wiring", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSearchParams.delete("status");

    mockUseSchoolCheckinMode.mockReturnValue({
      isActive: false,
      toggleActive: mockToggleActive,
      deactivate: vi.fn(),
      pendingIds: new Set<string>(),
      successCount: 0,
      toggle: mockToggleStudent,
      selectionActive: false,
      setSelectionActive: vi.fn(),
      selectedIds: new Set<string>(),
      toggleSelected: vi.fn(),
      clearSelection: vi.fn(),
      isBulkRunning: false,
      runBulk: vi.fn(),
    });

    vi.mocked(useImmutableSWR).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
    } as never);
    vi.mocked(useSWRAuth).mockImplementation((key) => {
      if (key === "search-rooms-list") {
        return {
          data: [],
          isLoading: false,
          error: null,
        } as never;
      }

      return {
        data: { students: mockStudents },
        isLoading: false,
        error: null,
      } as never;
    });
  });

  afterEach(() => {
    cleanup();
  });

  it("renders the SchoolCheckinFab on binary-mode tenants", async () => {
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    expect(screen.getByTestId("school-checkin-fab")).toBeInTheDocument();
  });

  it("clicking the FAB fires hook.toggleActive", async () => {
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("school-checkin-fab"));
    expect(mockToggleActive).toHaveBeenCalledTimes(1);
  });

  it("forwards checkinMode=true and derived state when active", async () => {
    mockUseSchoolCheckinMode.mockReturnValue({
      isActive: true,
      toggleActive: mockToggleActive,
      deactivate: vi.fn(),
      pendingIds: new Set<string>(),
      successCount: 0,
      toggle: mockToggleStudent,
      selectionActive: false,
      setSelectionActive: vi.fn(),
      selectedIds: new Set<string>(),
      toggleSelected: vi.fn(),
      clearSelection: vi.fn(),
      isBulkRunning: false,
      runBulk: vi.fn(),
    });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    const card = screen.getByTestId("student-card-7");
    expect(card).toHaveAttribute("data-checkin-mode", "true");
    expect(card).toHaveAttribute("data-checkin-state", "anwesend");
  });

  it("clicking a card in check-in mode calls hook.toggle and does not navigate", async () => {
    mockUseSchoolCheckinMode.mockReturnValue({
      isActive: true,
      toggleActive: mockToggleActive,
      deactivate: vi.fn(),
      pendingIds: new Set<string>(),
      successCount: 0,
      toggle: mockToggleStudent,
      selectionActive: false,
      setSelectionActive: vi.fn(),
      selectedIds: new Set<string>(),
      toggleSelected: vi.fn(),
      clearSelection: vi.fn(),
      isBulkRunning: false,
      runBulk: vi.fn(),
    });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("student-card-7"));
    expect(mockToggleStudent).toHaveBeenCalledWith("7", "anwesend");
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("clicking a card when inactive navigates (existing behavior)", async () => {
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("student-card-7"));
    expect(mockPush).toHaveBeenCalledWith("/students/7?from=/students/search");
    expect(mockToggleStudent).not.toHaveBeenCalled();
  });

  it("forwards pending count to the FAB via prop", async () => {
    mockUseSchoolCheckinMode.mockReturnValue({
      isActive: true,
      toggleActive: mockToggleActive,
      deactivate: vi.fn(),
      pendingIds: new Set<string>(["7", "8"]),
      successCount: 0,
      toggle: mockToggleStudent,
      selectionActive: false,
      setSelectionActive: vi.fn(),
      selectedIds: new Set<string>(),
      toggleSelected: vi.fn(),
      clearSelection: vi.fn(),
      isBulkRunning: false,
      runBulk: vi.fn(),
    });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    expect(screen.getByTestId("school-checkin-fab")).toHaveAttribute(
      "data-pending",
      "2",
    );
  });
});
