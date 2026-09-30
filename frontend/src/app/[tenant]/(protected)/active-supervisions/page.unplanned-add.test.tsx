/**
 * Tests for the Active Supervisions page with the student search and the
 * timetable check-in stubbed: the full-dashboard renders and the
 * "Kind ungeplant hinzufügen" panel (issue #2387).
 */
import { render, screen, waitFor, cleanup } from "@testing-library/react";
import { useLayoutEffect } from "react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

const navigationMockState = vi.hoisted(() => ({
  roomParam: null as string | null,
}));

function CommitProbe({
  children,
  onCommit,
}: Readonly<{ children: React.ReactNode; onCommit: () => void }>) {
  useLayoutEffect(onCommit);
  return children;
}

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
  hasPermission: () => false,
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
  useRouter: () => ({ push: mockPush, replace: vi.fn() }),
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
  // The action slot is part of the real Alert: the released-room notice and
  // the reopen banner both carry their action in it, so a stub that drops it
  // would hide the only control on those blocks.
  Alert: ({
    message,
    type,
    action,
  }: {
    message: string;
    type: string;
    action?: React.ReactNode;
  }) => (
    <div data-testid={`alert-${type}`}>
      {message}
      {action}
    </div>
  ),
}));

// Mock Modal and ConfirmationModal
vi.mock("~/components/ui/modal", () => ({
  dialogAriaProps: { role: "dialog" as const, "aria-modal": true },
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
  ActivityIcon: () => <span data-testid="activity-icon" />,
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
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import {
  useAttendanceWebEnabled,
  useShowTimetableCounts,
} from "~/lib/tenant-context";
import MeinRaumPage from "./page";

const defaultPageHeader = vi
  .mocked(PageHeaderWithSearch)
  .getMockImplementation()!;

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

beforeEach(() => {
  vi.mocked(PageHeaderWithSearch)
    .mockReset()
    .mockImplementation(defaultPageHeader);
  vi.mocked(useSWRAuth)
    .mockReset()
    .mockReturnValue({
      data: null,
      isLoading: true,
      error: null,
      mutate: vi.fn(),
      isValidating: false,
    } as never);
});

const makeStudent = (id: string, first: string, last: string) => ({
  id,
  name: `${first} ${last}`,
  first_name: first,
  second_name: last,
  school_class: "2c",
  group_name: "OGS Gruppe A",
  current_location: "",
});

describe("MeinRaumPage (Active Supervisions) (4/5)", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAttendanceWebEnabled).mockReturnValue(true);
    vi.mocked(useShowTimetableCounts).mockReturnValue(true);
    navigationMockState.roomParam = null;
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

  it("displays supervised room with students", async () => {
    // First call: dashboard data, Second call: per-room visits (return null to skip)
    const dashboardData = {
      supervisedGroups: [
        // Use a non-Schulhof room name to avoid triggering Schulhof-specific code path
        { id: "1", name: "Raum 101", room: { id: "10", name: "Raum 101" } },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "1" },
      educationalGroups: [
        { id: "2", name: "OGS Gruppe A", room: { name: "Raum 101" } },
      ],
      firstRoomVisits: [
        {
          studentId: "100",
          studentName: "Max Mustermann",
          schoolClass: "1a",
          groupName: "OGS Gruppe A",
          activeGroupId: "1",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
      ],
      firstRoomId: "1",
    };

    vi.mocked(useSWRAuth)
      .mockReturnValueOnce({
        data: dashboardData,
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never)
      .mockReturnValue({
        data: null, // Second hook (per-room visits) returns null
        isLoading: false,
        error: null,
        mutate: mockMutate,
        isValidating: false,
      } as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card")).toBeInTheDocument();
    });
  });

  it("does not flash student cards while timetable roster is still loading", async () => {
    const dashboardData = {
      supervisedGroups: [
        { id: "1", name: "Raum 101", room: { id: "10", name: "Raum 101" } },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "1" },
      educationalGroups: [
        { id: "2", name: "OGS Gruppe A", room: { name: "Raum 101" } },
      ],
      firstRoomVisits: [
        {
          studentId: "100",
          studentName: "Max Mustermann",
          schoolClass: "1a",
          groupName: "OGS Gruppe A",
          activeGroupId: "1",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
      ],
      firstRoomId: "10",
    };

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
          data: undefined,
          isLoading: true,
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

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(
        screen.getByLabelText("Aktuelle Aufsicht wird geladen…"),
      ).toBeInTheDocument();
      expect(screen.queryByTestId("student-card")).not.toBeInTheDocument();
    });
  });

  it("renders timetable roster UI when an active roster is available", async () => {
    const dashboardData = {
      supervisedGroups: [
        { id: "1", name: "Raum 101", room: { id: "10", name: "Raum 101" } },
      ],
      unclaimedGroups: [],
      currentStaff: { id: "1" },
      educationalGroups: [],
      firstRoomVisits: [
        {
          studentId: "100",
          studentName: "Max Mustermann",
          schoolClass: "1a",
          groupName: "OGS Gruppe A",
          activeGroupId: "1",
          checkInTime: new Date().toISOString(),
          isActive: true,
        },
      ],
      firstRoomId: "10",
    };

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
          data: {
            instance: {
              id: "99",
              title: "Kreativ AG",
              activeGroupId: "1",
              isSpontaneous: false,
            },
            rows: [
              {
                studentId: "100",
                studentName: "Max Mustermann",
                schoolClass: "1a",
                groupName: "OGS Gruppe A",
                planned: true,
                isUnplanned: false,
                currentlyPresent: true,
                visitId: "visit-100",
                status: "present",
                substatus: null,
                note: null,
              },
              {
                studentId: "101",
                studentName: "Erika Erwartet",
                schoolClass: "2b",
                groupName: "OGS Gruppe B",
                planned: true,
                isUnplanned: false,
                currentlyPresent: false,
                visitId: null,
                status: "expected",
                substatus: null,
                note: null,
              },
              {
                studentId: "102",
                studentName: "Lina Krank",
                schoolClass: "3c",
                groupName: "OGS Gruppe C",
                planned: true,
                isUnplanned: false,
                currentlyPresent: false,
                visitId: null,
                status: "absent",
                substatus: "sick",
                note: "Abgemeldet",
              },
              {
                studentId: "103",
                studentName: "Noah Gegangen",
                schoolClass: "4d",
                groupName: "OGS Gruppe D",
                planned: true,
                isUnplanned: false,
                currentlyPresent: false,
                visitId: "visit-103",
                status: "present",
                substatus: null,
                note: null,
              },
              {
                studentId: "104",
                studentName: "Mia Spontan",
                schoolClass: "1b",
                groupName: "OGS Gruppe A",
                planned: false,
                isUnplanned: true,
                currentlyPresent: true,
                visitId: "visit-104",
                status: "present",
                substatus: null,
                note: null,
              },
            ],
          },
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

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByText("Kreativ AG")).toBeInTheDocument();
      expect(screen.getByText("Aktiv")).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "1 erwartete bestätigen" }),
      ).toBeInTheDocument();
      expect(screen.getByText("Anwesend (1)")).toBeInTheDocument();
      expect(screen.getByText("Erwartet (1)")).toBeInTheDocument();
      expect(
        screen.getByText("Entschuldigt / Abwesend (1)"),
      ).toBeInTheDocument();
      expect(screen.getByText("Nicht mehr im Raum (1)")).toBeInTheDocument();
      expect(screen.getByText("Ungeplant (1)")).toBeInTheDocument();
      expect(
        screen.getByText("1b · OGS Gruppe A · ungeplant"),
      ).toBeInTheDocument();
      expect(screen.getByText("Krank · Abgemeldet")).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Einchecken" }),
      ).toBeInTheDocument();
      expect(
        screen.getAllByRole("button", { name: "Raum verlassen" }),
      ).toHaveLength(2);
      // Das Nachtragen ist eine Kopf-Aktion mit Dialog (#3112): die Suche
      // erscheint erst nach „Kind hinzufügen“.
      expect(
        screen.queryByRole("searchbox", { name: "Kind ungeplant suchen" }),
      ).not.toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: "Kind hinzufügen" }));
    await waitFor(() => {
      expect(
        screen.getByRole("searchbox", { name: "Kind ungeplant suchen" }),
      ).toHaveAttribute("name", "unplanned-student-search");
      expect(screen.queryByTestId("student-card")).not.toBeInTheDocument();
    });
  });

  it("hides roster attendance controls and counts when tenant settings disable them", async () => {
    vi.mocked(useAttendanceWebEnabled).mockReturnValue(false);
    vi.mocked(useShowTimetableCounts).mockReturnValue(false);
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
          data: {
            instance: {
              id: "99",
              title: "Kreativ AG",
              activeGroupId: "1",
              isSpontaneous: false,
            },
            rows: [
              {
                studentId: "100",
                studentName: "Max Anwesend",
                schoolClass: "1a",
                groupName: "OGS Gruppe A",
                planned: true,
                isUnplanned: false,
                currentlyPresent: true,
                visitId: "visit-100",
                status: "present",
                substatus: null,
                note: null,
              },
              {
                studentId: "101",
                studentName: "Erika Erwartet",
                schoolClass: "2b",
                groupName: "OGS Gruppe B",
                planned: true,
                isUnplanned: false,
                currentlyPresent: false,
                visitId: null,
                status: "expected",
                substatus: null,
                note: null,
              },
            ],
          },
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

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByText("Max Anwesend")).toBeInTheDocument();
      expect(screen.getByText("Erika Erwartet")).toBeInTheDocument();
    });
    expect(screen.getByText("Anwesend")).toBeInTheDocument();
    expect(screen.getByText("Erwartet")).toBeInTheDocument();
    expect(screen.queryByText("Anwesend (1)")).not.toBeInTheDocument();
    expect(screen.queryByText("Erwartet (1)")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /erwartete bestätigen/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Einchecken" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Raum verlassen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Beenden" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Kind hinzufügen" }),
    ).not.toBeInTheDocument();
  });
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

  // Die Suche steht im Dialog hinter der Kopf-Aktion „Kind hinzufügen“
  // (#3112); ist er schon offen, bleibt er offen.
  const searchFor = async (value: string) => {
    if (!screen.queryByRole("searchbox", { name: "Kind ungeplant suchen" })) {
      fireEvent.click(
        await screen.findByRole("button", { name: "Kind hinzufügen" }),
      );
    }
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

    expect(screen.queryByRole("button", { name: /Marie Beier/ })).toBeNull();
    expect(screen.getByRole("button", { name: "Hinzufügen" })).toBeDisabled();

    expect(
      await screen.findByRole("button", { name: /Marie Beier/ }),
    ).toHaveAttribute("aria-pressed", "false");
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

    const { rerender } = render(<MeinRaumPage />);
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

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    await waitFor(() =>
      expect(
        screen.queryByRole("searchbox", { name: "Kind ungeplant suchen" }),
      ).not.toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "Kind hinzufügen" }));
    expect(
      screen.queryByText("Kind konnte nicht zur Aktivität hinzugefügt werden."),
    ).not.toBeInTheDocument();

    await searchFor("Marie");
    fireEvent.click(
      await screen.findByRole("button", { name: /Marie Garschagen/ }),
    );
    vi.mocked(timetableOperationsApi.checkIn).mockRejectedValueOnce(
      new Error("check-in failed"),
    );
    fireEvent.click(screen.getByRole("button", { name: "Hinzufügen" }));

    await screen.findByText(
      "Kind konnte nicht zur Aktivität hinzugefügt werden.",
    );
    currentRosterData = {
      ...rosterData,
      instance: { ...rosterData.instance, id: "100", title: "Sport" },
    };
    rerender(<MeinRaumPage />);
    await waitFor(() =>
      expect(
        screen.queryByText(
          "Kind konnte nicht zur Aktivität hinzugefügt werden.",
        ),
      ).not.toBeInTheDocument(),
    );

    fireEvent.click(
      await screen.findByRole("button", { name: /Marie Garschagen/ }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Hinzufügen" }));
    await waitFor(() => {
      expect(timetableOperationsApi.checkIn).toHaveBeenCalledTimes(3);
      expect(timetableOperationsApi.checkIn).toHaveBeenLastCalledWith(
        "100",
        "202",
      );
    });
  });

  it("clears a single match on instance change, then restores the quick path", async () => {
    vi.mocked(fetchStudents).mockResolvedValue({
      students: [makeStudent("201", "Marie", "Beier")],
    });

    const unsafeCommits: boolean[] = [];
    const recordCommit = () => {
      const result = screen.queryByRole("button", { name: /Marie Beier/ });
      const addButton = screen.queryByRole("button", { name: "Hinzufügen" });
      unsafeCommits.push(
        result !== null &&
          addButton !== null &&
          !addButton.hasAttribute("disabled"),
      );
    };
    const renderPage = () => (
      <CommitProbe onCommit={recordCommit}>
        <MeinRaumPage />
      </CommitProbe>
    );
    const { rerender } = render(renderPage());
    await searchFor("Marie Beier");

    await screen.findByRole("button", { name: /Marie Beier/ });
    const addButton = screen.getByRole("button", { name: "Hinzufügen" });

    expect(addButton).toBeEnabled();
    expect(
      screen.queryByText("Bitte ein Kind aus der Liste antippen."),
    ).not.toBeInTheDocument();

    currentRosterData = {
      ...rosterData,
      instance: { ...rosterData.instance, id: "100", title: "Sport" },
    };
    unsafeCommits.length = 0;
    rerender(renderPage());

    expect(unsafeCommits).not.toContain(true);
    expect(screen.queryByRole("button", { name: /Marie Beier/ })).toBeNull();
    expect(screen.getByRole("button", { name: "Hinzufügen" })).toBeDisabled();

    await screen.findByRole("button", { name: /Marie Beier/ });
    fireEvent.click(screen.getByRole("button", { name: "Hinzufügen" }));

    await waitFor(() => {
      expect(timetableOperationsApi.checkIn).toHaveBeenCalledWith("100", "201");
    });
  });
});
