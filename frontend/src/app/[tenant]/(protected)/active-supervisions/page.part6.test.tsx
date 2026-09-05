/**
 * Tests for Active Supervisions Page
 * Tests the rendering states and user interactions of the active supervisions dashboard
 *
 * NOTE: split into 12 files (page.test.tsx + page.part2..12.test.tsx). The full-dashboard
 * render tests in the "MeinRaumPage (Active Supervisions)" describe are memory-heavy under
 * happy-dom + v8 coverage (~1.5 GB heap each), so a single combined file OOMs the Vitest
 * worker. Those heavy tests are pre-split into (N/M) chunks of 3 renders each, one chunk per
 * file (a 3-render chunk fits comfortably in a 6 GB heap; CI runs with 8 GB). All other
 * describes render cheaply and are packed together. All files share the identical mock header
 * below. When adding a heavy full-dashboard render test, keep it to its own small file.
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

// Mock Loading component
vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading">Loading...</div>,
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
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { useNFCEnabled } from "~/lib/tenant-context";
import MeinRaumPage from "./page";

const defaultPageHeader = vi
  .mocked(PageHeaderWithSearch)
  .getMockImplementation()!;

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

describe("MeinRaumPage (Active Supervisions) (5/5)", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useNFCEnabled).mockReturnValue(true);
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

  it("labels unplanned rows as participants for spontaneous rosters", async () => {
    const dashboardData = {
      supervisedGroups: [
        { id: "1", name: "Aula", room: { id: "10", name: "Aula" } },
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
              title: "Malen",
              activeGroupId: "1",
              isSpontaneous: true,
            },
            rows: [
              {
                studentId: "104",
                studentName: "Jan Peters",
                schoolClass: "2a",
                groupName: "Sonnengruppe",
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
      expect(screen.getByText("Malen")).toBeInTheDocument();
      expect(screen.getByText("Aktiv")).toBeInTheDocument();
      expect(screen.getByText("Teilnehmende (1)")).toBeInTheDocument();
      expect(screen.getByText("2a · Sonnengruppe")).toBeInTheDocument();
      expect(screen.queryByText("Ungeplant (1)")).not.toBeInTheDocument();
      expect(
        screen.queryByText("2a · Sonnengruppe · ungeplant"),
      ).not.toBeInTheDocument();
    });
  });

  it("keeps not-scheduled children out of Erwartet and the bulk confirm (#1747)", async () => {
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
                careDayStatus: "scheduled",
              },
              {
                studentId: "102",
                studentName: "Nora NichtGeplant",
                schoolClass: "3c",
                groupName: "OGS Gruppe C",
                planned: true,
                isUnplanned: false,
                currentlyPresent: false,
                visitId: null,
                status: "expected",
                substatus: null,
                note: null,
                careDayStatus: "not_scheduled",
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
      // Only the scheduled child is expected; the not-scheduled child gets
      // her own section and is excluded from the bulk confirm.
      expect(screen.getByText("Erwartet (1)")).toBeInTheDocument();
      expect(
        screen.getByText("Heute nicht eingeplant (1)"),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "1 erwartete bestätigen" }),
      ).toBeInTheDocument();
      expect(screen.getByText("Nora NichtGeplant")).toBeInTheDocument();
      // A walk-in stays one tap away from a check-in...
      expect(
        screen.getAllByRole("button", { name: "Einchecken" }),
      ).toHaveLength(2);
      // ...but absence marking only applies to genuinely expected children.
      expect(
        screen.getAllByRole("button", { name: "Entschuldigt" }),
      ).toHaveLength(1);
      expect(screen.getAllByRole("button", { name: "Abwesend" })).toHaveLength(
        1,
      );
    });
  });

  it("groups a status-day absence on an unbooked day as not scheduled (#1747)", async () => {
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
                // Krankmeldung stamped a day the care plan never booked: an
                // absence from care that was never owed.
                studentId: "201",
                studentName: "Klara Krank",
                schoolClass: "2b",
                groupName: "OGS Gruppe B",
                planned: true,
                isUnplanned: false,
                currentlyPresent: false,
                visitId: null,
                status: "absent",
                substatus: "sick",
                note: null,
                careDayStatus: "not_scheduled",
              },
              {
                // A human marked this child absent: a real absence.
                studentId: "202",
                studentName: "Mia Manuell",
                schoolClass: "3c",
                groupName: "OGS Gruppe C",
                planned: true,
                isUnplanned: false,
                currentlyPresent: false,
                visitId: null,
                status: "absent",
                substatus: null,
                note: null,
                careDayStatus: "unknown",
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
      expect(
        screen.getByText("Heute nicht eingeplant (1)"),
      ).toBeInTheDocument();
      expect(
        screen.getByText("Entschuldigt / Abwesend (1)"),
      ).toBeInTheDocument();
      expect(screen.getByText("Klara Krank")).toBeInTheDocument();
      expect(screen.getByText("Mia Manuell")).toBeInTheDocument();
    });
  });

  it("does not flash first-room students while a direct room URL is syncing", async () => {
    navigationMockState.roomParam = "11";
    // The aggregate re-run for the URL-targeted session never resolves in
    // this test — the point is that the first room's visits from the stale
    // payload must not be shown meanwhile (#2096).
    mockMutate.mockReturnValue(new Promise(() => undefined) as never);
    const dashboardData = {
      supervisedGroups: [
        {
          id: "1",
          name: "Raum 101",
          room_id: "10",
          room: { id: "10", name: "Raum 101" },
        },
        {
          id: "2",
          name: "Raum 102",
          room_id: "11",
          room: { id: "11", name: "Raum 102" },
        },
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
      // The URL target resolves to session "2" and triggers the aggregate
      // re-run; the first room's students never appear meanwhile.
      expect(mockMutate).toHaveBeenCalled();
      expect(screen.queryByTestId("student-card")).not.toBeInTheDocument();
      expect(screen.queryByText("Max Mustermann")).not.toBeInTheDocument();
    });
  });

  it("handles permission errors gracefully", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: false,
      error: new Error("BFF request failed: 403"),
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine aktive Raum-Aufsicht"),
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          "Sie sind aktuell in keinem Raum als Live-Aktivität registriert. Starten Sie eine Aktivität an einem Terminal, um Live-Raumdaten einzusehen.",
        ),
      ).toBeInTheDocument();
    });
  });

  it("points NFC-free tenants to the web app when no supervision is active", async () => {
    vi.mocked(useNFCEnabled).mockReturnValue(false);
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: false,
      error: new Error("BFF request failed: 403"),
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Sie sind aktuell in keinem Raum als Live-Aktivität registriert. Starten Sie eine Aktivität in der Web-App, um Live-Raumdaten einzusehen.",
        ),
      ).toBeInTheDocument();
    });
    expect(screen.queryByText(/Terminal/i)).not.toBeInTheDocument();
  });
});
