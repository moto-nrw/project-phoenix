/**
 * Page-level wiring tests for StudentSearchPage + school check-in/out.
 * Asserts the toggle renders in the header and that StudentCard receives
 * the correct check-in props from the hook.
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
    if (loc === "Zuhause" || loc === "") return "abwesend";
    if (loc) return "anwesend";
    return "unknown";
  },
}));

// Page gates the toggle on binary mode; override the global mock
// (src/test/setup.ts defaults to "detailed") so the button renders.
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
        onClick={props.checkinMode ? props.onCheckinClick : props.onClick}
      >
        {props.firstName} {props.lastName}
      </button>
    );
  },
  SchoolClassIcon: () => <span />,
  GroupIcon: () => <span />,
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

vi.mock("@/components/ui/location-badge", () => ({
  LocationBadge: () => <span />,
}));

vi.mock("@/components/ui/student-presence-badge", () => ({
  StudentPresenceBadge: () => <span />,
}));

vi.mock("~/lib/location-helper", () => ({
  isHomeLocation: (l?: string) => l === "Zuhause",
  isPresentLocation: (l?: string) => !!l && l !== "Zuhause" && l !== "Schulhof",
  isTransitLocation: () => false,
  isSchoolyardLocation: (l?: string) => l === "Schulhof",
}));

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
      toggle: mockToggleStudent,
    });

    vi.mocked(useImmutableSWR).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
    } as never);
    vi.mocked(useSWRAuth).mockReturnValue({
      data: { students: mockStudents },
      isLoading: false,
      error: null,
    } as never);
  });

  afterEach(() => {
    cleanup();
  });

  it("renders the SchoolCheckinToggle in the header", async () => {
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    const toggles = screen.queryAllByTestId(/school-checkin-toggle/);
    expect(toggles.length).toBeGreaterThan(0);
  });

  it("clicking the toggle fires hook.toggleActive", async () => {
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    fireEvent.click(screen.queryAllByTestId(/school-checkin-toggle/)[0]!);
    expect(mockToggleActive).toHaveBeenCalledTimes(1);
  });

  it("forwards checkinMode=true and derived state when active", async () => {
    mockUseSchoolCheckinMode.mockReturnValue({
      isActive: true,
      toggleActive: mockToggleActive,
      deactivate: vi.fn(),
      pendingIds: new Set<string>(),
      toggle: mockToggleStudent,
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
      toggle: mockToggleStudent,
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
});
