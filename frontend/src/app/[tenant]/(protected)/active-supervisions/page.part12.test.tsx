/**
 * Tests for the "Kind ungeplant hinzufügen" panel (issue #2387): selection
 * flow with multiple search results. Shares the identical mock header with
 * page.test.tsx / page.part2..11.test.tsx (see the note in page.part5.test.tsx);
 * heavy full-dashboard renders stay at <=3 per file.
 */
import { render, screen, waitFor, cleanup } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

const navigationMockState = vi.hoisted(() => ({
  roomParam: null as string | null,
}));

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
const mockRedirect = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
  useSearchParams: () => ({
    get: (key: string) =>
      key === "room" ? navigationMockState.roomParam : null,
  }),
  redirect: (url: string) => mockRedirect(url),
}));

// Mock breadcrumb context
vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
  useBreadcrumb: vi.fn(() => ({ breadcrumb: {}, setBreadcrumb: vi.fn() })),
  BreadcrumbProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

// Mock PageHeaderWithSearch (vi.fn wrapper enables mockImplementation in enhanced tests)
vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: vi.fn(
    ({ title, badge }: { title: string; badge?: { count: number } }) => (
      <div data-testid="page-header" data-count={badge?.count}>
        {title}
      </div>
    ),
  ),
}));

// Mock Alert
vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message, type }: { message: string; type: string }) => (
    <div data-testid={`alert-${type}`}>{message}</div>
  ),
}));

// Mock Modal and ConfirmationModal
vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    children,
    title,
  }: {
    isOpen: boolean;
    children: React.ReactNode;
    title: string;
  }) =>
    isOpen ? (
      <div data-testid="modal" data-title={title}>
        {children}
      </div>
    ) : null,
  ConfirmationModal: ({
    isOpen,
    children,
    title,
  }: {
    isOpen: boolean;
    children: React.ReactNode;
    title: string;
  }) =>
    isOpen ? (
      <div data-testid="confirmation-modal" data-title={title}>
        {children}
      </div>
    ) : null,
}));

// Mock activeService
vi.mock("~/lib/active-api", () => ({
  activeService: {
    getActiveGroupVisitsWithDisplay: vi.fn(() => Promise.resolve([])),
    getActiveGroupSupervisors: vi.fn(() => Promise.resolve([])),
    endSupervision: vi.fn(() => Promise.resolve()),
    claimActiveGroup: vi.fn(() => Promise.resolve()),
    getTrackingIndicators: vi.fn(() =>
      Promise.resolve({ labels: [], results: {} }),
    ),
  },
}));

// Mock SSEErrorBoundary
vi.mock("~/components/sse/SSEErrorBoundary", () => ({
  SSEErrorBoundary: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="sse-boundary">{children}</div>
  ),
}));

// Mock UnclaimedRooms
vi.mock("~/components/active/unclaimed-rooms", () => ({
  UnclaimedRooms: () => <div data-testid="unclaimed-rooms" />,
}));

vi.mock("~/components/rooms/transit-students-section", () => ({
  TransitStudentsSection: () => <div data-testid="transit-students-section" />,
}));

// Mock LocationBadge
vi.mock("@/components/ui/location-badge", () => ({
  LocationBadge: () => <div data-testid="location-badge">Location</div>,
}));

// Mock EmptyStudentResults
vi.mock("~/components/ui/empty-student-results", () => ({
  EmptyStudentResults: () => <div data-testid="empty-results">No results</div>,
}));

// Mock location-helper
// Partial mock: keep the real MOTO_COLOR_PALETTE/LOCATION_COLORS exports
// (moto-duotone-icon.tsx reads MOTO_COLOR_PALETTE at module scope) and only
// stub the location-parsing predicates used by this test file.
vi.mock("~/lib/location-helper", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/location-helper")>();
  return {
    ...actual,
    isHomeLocation: vi.fn(() => false),
    isSchoolyardLocation: vi.fn(() => false),
    isTransitLocation: vi.fn(() => false),
    parseLocation: vi.fn(() => ({ room: "Room 1", status: "Anwesend" })),
  };
});

// Mock pickup-helpers
vi.mock("~/lib/pickup-helpers", () => ({
  useMinuteClock: () => new Date("2026-01-15T12:00:00"),
}));

// Mock pickup-schedule-api
vi.mock("~/lib/pickup-schedule-api", () => ({
  fetchBulkPickupTimes: vi.fn(() => Promise.resolve(new Map())),
}));

// Mock student-arrival-api
vi.mock("~/lib/student-arrival-api", () => ({
  fetchBulkArrivalTimes: vi.fn(() => Promise.resolve(new Map())),
}));

// Mock StudentCard components
vi.mock("~/components/students/student-card", () => ({
  StudentCard: ({
    firstName,
    lastName,
    extraContent,
  }: {
    firstName: string;
    lastName: string;
    extraContent?: React.ReactNode;
  }) => (
    <div data-testid="student-card">
      {firstName} {lastName}
      {extraContent}
    </div>
  ),
  StudentInfoRow: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="student-info-row">{children}</div>
  ),
  SchoolClassIcon: () => <span data-testid="school-class-icon" />,
  GroupIcon: () => <span data-testid="group-icon" />,
  PickupTimeRow: ({
    pickupTime,
    isException,
    notes,
    isHome,
  }: {
    pickupTime?: string;
    isException: boolean;
    notes?: string;
    isHome: boolean;
    now: Date;
  }) => (
    <div
      data-testid="pickup-time-row"
      data-pickup-time={pickupTime ?? ""}
      data-is-exception={String(isException)}
      data-is-home={String(isHome)}
    >
      {pickupTime && <>Abholzeit: {pickupTime} Uhr</>}
      {!pickupTime && isException && (notes || "Abwesend")}
      {!pickupTime && !isException && <>Abholzeit: —</>}
      {notes && <span>({notes})</span>}
    </div>
  ),
  ArrivalTimeRow: ({
    arrivalTime,
    isException,
    isAbsent,
    notes,
    isHome,
  }: {
    arrivalTime?: string;
    isException: boolean;
    isAbsent: boolean;
    notes?: string;
    isHome: boolean;
    now: Date;
  }) => (
    <div
      data-testid="arrival-time-row"
      data-arrival-time={arrivalTime ?? ""}
      data-is-exception={String(isException)}
      data-is-absent={String(isAbsent)}
      data-is-home={String(isHome)}
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

// Mock SWR hook
vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(() => ({
    data: null,
    isLoading: true,
    error: null,
    mutate: vi.fn(),
    isValidating: false,
  })),
  useTenantMutate: vi.fn(() => vi.fn()),
}));

import { useSWRAuth } from "~/lib/swr";
import {
  useAttendanceWebEnabled,
  useShowTimetableCounts,
} from "~/lib/tenant-context";
import MeinRaumPage from "./page";

vi.mock("~/lib/student-api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/student-api")>();
  return {
    ...actual,
    fetchStudents: vi.fn(() => Promise.resolve({ students: [] })),
  };
});

vi.mock("~/lib/timetable-operations-api", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/timetable-operations-api")>();
  return {
    ...actual,
    timetableOperationsApi: {
      ...actual.timetableOperationsApi,
      checkIn: vi.fn(() =>
        Promise.resolve({
          instance: {
            id: "99",
            title: "Kreativ AG",
            activeGroupId: "1",
            isSpontaneous: false,
          },
          rows: [],
        }),
      ),
    },
  };
});

import { fireEvent } from "@testing-library/react";
import { fetchStudents } from "~/lib/student-api";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";

const makeStudent = (id: string, first: string, last: string) => ({
  id,
  name: `${first} ${last}`,
  first_name: first,
  second_name: last,
  school_class: "2c",
  group_name: "OGS Gruppe A",
  current_location: "",
});

describe("AddUnplannedStudentForm selection flow (#2387)", () => {
  const mockMutate = vi.fn();

  const dashboardData = {
    supervisedGroups: [
      { id: "1", name: "Raum 101", room: { id: "10", name: "Raum 101" } },
    ],
    unclaimedGroups: [],
    currentStaff: { id: "1" },
    educationalGroups: [],
    firstRoomVisits: [],
    firstRoomId: "10",
  };

  const rosterData = {
    instance: {
      id: "99",
      title: "Kreativ AG",
      activeGroupId: "1",
      isSpontaneous: false,
    },
    rows: [],
  };
  let currentRosterData = rosterData;

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAttendanceWebEnabled).mockReturnValue(true);
    vi.mocked(useShowTimetableCounts).mockReturnValue(true);
    navigationMockState.roomParam = null;
    currentRosterData = rosterData;
    global.fetch = vi.fn();
    vi.mocked(useSWRAuth).mockImplementation(((key: string | null) => {
      if (key?.startsWith("active-supervision-dashboard")) {
        return {
          data: dashboardData,
          isLoading: false,
          error: null,
          mutate: mockMutate,
          isValidating: false,
        };
      }
      if (key?.startsWith("timetable-roster-active-group")) {
        return {
          data: currentRosterData,
          isLoading: false,
          error: null,
          mutate: mockMutate,
          isValidating: false,
        };
      }
      return {
        data: null,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      };
    }) as never);
  });

  afterEach(() => {
    cleanup();
  });

  const searchFor = async (value: string) => {
    const input = await screen.findByRole("searchbox", {
      name: "Kind ungeplant suchen",
    });
    fireEvent.change(input, { target: { value } });
  };

  it("requires a selection and resets it when the instance or search changes", async () => {
    vi.mocked(fetchStudents).mockResolvedValue({
      students: [
        makeStudent("201", "Marie", "Beier"),
        makeStudent("202", "Marie", "Garschagen"),
        makeStudent("203", "Lara Marie", "Brüggemann"),
      ],
    });

    const { rerender } = render(<MeinRaumPage />);
    await searchFor("Marie");

    const beierCard = await screen.findByRole("button", {
      name: /Marie Beier/,
    });
    const addButton = screen.getByRole("button", { name: "Hinzufügen" });

    expect(addButton).toBeDisabled();
    expect(
      screen.getByText("Bitte ein Kind aus der Liste antippen."),
    ).toBeInTheDocument();

    fireEvent.click(beierCard);

    expect(timetableOperationsApi.checkIn).not.toHaveBeenCalled();
    expect(beierCard).toHaveAttribute("aria-pressed", "true");
    expect(addButton).toBeEnabled();
    expect(
      screen.queryByText("Bitte ein Kind aus der Liste antippen."),
    ).not.toBeInTheDocument();

    currentRosterData = {
      ...rosterData,
      instance: { ...rosterData.instance, id: "100", title: "Sport" },
    };
    rerender(<MeinRaumPage />);

    expect(screen.getByRole("button", { name: /Marie Beier/ })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    expect(screen.getByRole("button", { name: "Hinzufügen" })).toBeDisabled();

    await searchFor("Lara");

    expect(screen.queryByRole("button", { name: /Marie Beier/ })).toBeNull();
    expect(screen.getByRole("button", { name: "Hinzufügen" })).toBeDisabled();
    expect(timetableOperationsApi.checkIn).not.toHaveBeenCalled();
  });

  it("switches the selection when another result is tapped", async () => {
    vi.mocked(fetchStudents).mockResolvedValue({
      students: [
        makeStudent("201", "Marie", "Beier"),
        makeStudent("202", "Marie", "Garschagen"),
      ],
    });

    render(<MeinRaumPage />);
    await searchFor("Marie");

    const beierCard = await screen.findByRole("button", {
      name: /Marie Beier/,
    });
    fireEvent.click(beierCard);
    const garschagenCard = screen.getByRole("button", {
      name: /Marie Garschagen/,
    });
    fireEvent.click(garschagenCard);

    expect(beierCard).toHaveAttribute("aria-pressed", "false");
    expect(garschagenCard).toHaveAttribute("aria-pressed", "true");

    vi.mocked(timetableOperationsApi.checkIn).mockRejectedValueOnce(
      new Error("check-in failed"),
    );
    fireEvent.click(screen.getByRole("button", { name: "Hinzufügen" }));

    await waitFor(() => {
      expect(
        screen.getByText("Kind konnte nicht zur Aktivität hinzugefügt werden."),
      ).toBeInTheDocument();
    });
    expect(garschagenCard).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "Hinzufügen" })).toBeEnabled();

    fireEvent.click(screen.getByRole("button", { name: "Hinzufügen" }));

    await waitFor(() => {
      expect(timetableOperationsApi.checkIn).toHaveBeenCalledTimes(2);
      expect(timetableOperationsApi.checkIn).toHaveBeenLastCalledWith(
        "99",
        "202",
      );
    });
  });

  it("keeps the quick path with a single match: button active without selection", async () => {
    vi.mocked(fetchStudents).mockResolvedValue({
      students: [makeStudent("201", "Marie", "Beier")],
    });

    render(<MeinRaumPage />);
    await searchFor("Marie Beier");

    await screen.findByRole("button", { name: /Marie Beier/ });
    const addButton = screen.getByRole("button", { name: "Hinzufügen" });

    expect(addButton).toBeEnabled();
    expect(
      screen.queryByText("Bitte ein Kind aus der Liste antippen."),
    ).not.toBeInTheDocument();

    fireEvent.click(addButton);

    await waitFor(() => {
      expect(timetableOperationsApi.checkIn).toHaveBeenCalledWith("99", "201");
    });
  });
});
