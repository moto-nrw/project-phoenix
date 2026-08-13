/**
 * Page-level wiring tests for the check-in/out mode on DETAILED-mode tenants
 * (#2220). The mode used to be gated on binary mode; schools with rooms and
 * activities now get the same pickup-call workflow, with one addition — a
 * student who currently sits in a room is confirmed before checkout, because
 * that checkout also ends the running room visit.
 *
 * Unlike the sibling page.school-checkin.test.tsx (binary mode, everything
 * stubbed), this file keeps the real checkoutConfirmationRoom + the real
 * ConfirmationModal so the room-vs-roomless decision is actually exercised.
 */
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { mockToggleActive, mockToggleStudent, mockUseSchoolCheckinMode } =
  vi.hoisted(() => ({
    mockToggleActive: vi.fn(),
    mockToggleStudent: vi.fn(),
    mockUseSchoolCheckinMode: vi.fn(),
  }));

vi.mock("~/lib/hooks/use-school-checkin-mode", async (importOriginal) => {
  const actual =
    await importOriginal<
      typeof import("~/lib/hooks/use-school-checkin-mode")
    >();
  return {
    ...actual,
    useSchoolCheckinMode: () => mockUseSchoolCheckinMode(),
  };
});

// Detailed mode is the default, but pin it explicitly so the test still means
// what it says if the global default ever moves.
vi.mock("~/lib/tenant-context", () => ({
  useTenant: vi.fn(() => ({ tenantSlug: "test-tenant", tenant: null })),
  useTenantSafe: vi.fn(() => ({
    tenantSlug: "test-tenant",
    tenant: { studentPhotosEnabled: true },
  })),
  useTenantSlugSafe: vi.fn(() => "test-tenant"),
  usePresenceMode: vi.fn(() => "detailed"),
  useNFCEnabled: vi.fn(() => true),
  useAttendanceWebEnabled: vi.fn(() => true),
  TenantProvider: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("~/components/students/school-checkin-fab", () => ({
  SchoolCheckinFab: (props: { isActive: boolean; onToggle: () => void }) => (
    <button
      type="button"
      data-testid="school-checkin-fab"
      data-active={props.isActive}
      onClick={props.onToggle}
    >
      fab
    </button>
  ),
}));

vi.mock("~/components/students/school-checkin-mode-mobile", () => ({
  SchoolCheckinModeMobile: (props: {
    isActive: boolean;
    onToggle: () => void;
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
    onCheckinClick?: () => void;
    onClick: () => void;
  }) => (
    <button
      type="button"
      data-testid={`student-card-${props.studentId}`}
      data-checkin-mode={props.checkinMode ?? false}
      data-checkin-state={props.checkinState ?? ""}
      onClick={props.checkinMode ? props.onCheckinClick : props.onClick}
    >
      {props.firstName} {props.lastName}
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
    primaryAction,
  }: {
    primaryAction?: React.ReactNode;
  }) => (
    <div data-testid="page-header">
      <div data-testid="header-primary-action">{primaryAction}</div>
    </div>
  ),
}));

vi.mock("@/components/ui/location-badge", () => ({
  LocationBadge: () => <span />,
}));

vi.mock("@/components/ui/student-presence-badge", () => ({
  StudentPresenceBadge: () => <span />,
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
import { useAttendanceWebEnabled } from "~/lib/tenant-context";
import StudentSearchPage from "./page";

// One student per relevant location shape: in a room (needs confirmation),
// present without a room, and gone home.
const mockStudents = [
  {
    id: "11",
    first_name: "Mia",
    second_name: "Beispiel",
    school_class: "2a",
    group_name: "Gruppe A",
    current_location: "Anwesend - Raum 2",
    has_full_access: true,
  },
  {
    id: "12",
    first_name: "Tom",
    second_name: "Beispiel",
    school_class: "2a",
    group_name: "Gruppe A",
    current_location: "Unterwegs",
    has_full_access: true,
  },
  {
    id: "13",
    first_name: "Lea",
    second_name: "Beispiel",
    school_class: "2a",
    group_name: "Gruppe A",
    current_location: "Zuhause",
    has_full_access: true,
  },
];

function activateCheckinMode() {
  mockUseSchoolCheckinMode.mockReturnValue({
    isActive: true,
    toggleActive: mockToggleActive,
    deactivate: vi.fn(),
    pendingIds: new Set<string>(),
    successCount: 0,
    toggle: mockToggleStudent,
  });
}

describe("StudentSearchPage — check-in mode on detailed-mode tenants", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAttendanceWebEnabled).mockReturnValue(true);

    mockUseSchoolCheckinMode.mockReturnValue({
      isActive: false,
      toggleActive: mockToggleActive,
      deactivate: vi.fn(),
      pendingIds: new Set<string>(),
      successCount: 0,
      toggle: mockToggleStudent,
    });

    vi.mocked(useImmutableSWR).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
    } as never);
    vi.mocked(useSWRAuth).mockImplementation((key) => {
      if (key === "search-rooms-list") {
        return { data: [], isLoading: false, error: null } as never;
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

  it("offers the check-in mode although the tenant is not in binary mode", async () => {
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-11")).toBeInTheDocument();
    });

    expect(screen.getAllByTestId("school-checkin-fab").length).toBeGreaterThan(
      0,
    );
    expect(screen.getByTestId("school-checkin-mobile")).toBeInTheDocument();
  });

  it("hides the mode when the school has web attendance switched off", async () => {
    vi.mocked(useAttendanceWebEnabled).mockReturnValue(false);
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-11")).toBeInTheDocument();
    });

    expect(screen.queryByTestId("school-checkin-fab")).not.toBeInTheDocument();
    expect(
      screen.queryByTestId("school-checkin-mobile"),
    ).not.toBeInTheDocument();
  });

  it("puts the cards into check-in mode with a room location derived as present", async () => {
    activateCheckinMode();
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-11")).toBeInTheDocument();
    });

    const card = screen.getByTestId("student-card-11");
    expect(card).toHaveAttribute("data-checkin-mode", "true");
    expect(card).toHaveAttribute("data-checkin-state", "anwesend");
  });

  it("asks before checking a student out of a running room, then toggles on confirm", async () => {
    activateCheckinMode();
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-11")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("student-card-11"));

    // Nothing is written before the confirmation.
    expect(mockToggleStudent).not.toHaveBeenCalled();
    expect(
      await screen.findByText("Aus der Betreuung abmelden?"),
    ).toBeInTheDocument();
    expect(screen.getByText("Raum 2")).toBeInTheDocument();
    // Twice: the card underneath and the dialog naming the child.
    expect(screen.getAllByText("Mia Beispiel")).toHaveLength(2);

    fireEvent.click(screen.getByRole("button", { name: "Abmelden" }));

    expect(mockToggleStudent).toHaveBeenCalledWith("11", "anwesend");
    await waitFor(() => {
      expect(
        screen.queryByText("Aus der Betreuung abmelden?"),
      ).not.toBeInTheDocument();
    });
  });

  it("writes nothing when the room confirmation is cancelled", async () => {
    activateCheckinMode();
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-11")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("student-card-11"));
    fireEvent.click(await screen.findByRole("button", { name: "Abbrechen" }));

    expect(mockToggleStudent).not.toHaveBeenCalled();
    await waitFor(() => {
      expect(
        screen.queryByText("Aus der Betreuung abmelden?"),
      ).not.toBeInTheDocument();
    });
  });

  it("checks a roomless present student out on a single tap", async () => {
    activateCheckinMode();
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-12")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("student-card-12"));

    expect(mockToggleStudent).toHaveBeenCalledWith("12", "anwesend");
    expect(
      screen.queryByText("Aus der Betreuung abmelden?"),
    ).not.toBeInTheDocument();
  });

  it("checks an absent student in on a single tap", async () => {
    activateCheckinMode();
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-13")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("student-card-13"));

    expect(mockToggleStudent).toHaveBeenCalledWith("13", "abwesend");
    expect(
      screen.queryByText("Aus der Betreuung abmelden?"),
    ).not.toBeInTheDocument();
  });
});
