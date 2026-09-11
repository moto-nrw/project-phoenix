import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { StudentsInRoomSection } from "./students-in-room-section";
import { useAttendanceWebEnabled } from "~/lib/tenant-context";
import { useOptionalSupervision } from "~/lib/supervision-context";

/** Default supervision context: no school-wide overview (#2380). */
const EMPTY_SUPERVISION = {
  hasGroups: false,
  isLoadingGroups: false,
  groups: [],
  isSupervising: false,
  supervisedRooms: [],
  isLoadingSupervision: false,
  overviewEnabled: false,
  refresh: vi.fn(),
};

vi.mock("~/lib/supervision-context", () => ({
  useOptionalSupervision: vi.fn(),
}));

// ----------------------------------------------------------------------------
// Mocks
// ----------------------------------------------------------------------------

const {
  mockPush,
  mockUseSearchParams,
  mockUseSWRAuth,
  mockUseTenantMutateMatching,
  mockGetStudentCurrentVisit,
  mockUpdateVisit,
  mockMoveStudentsToActiveGroup,
  mockGetActiveGroups,
  mockGetStaffActiveSupervisions,
  mockToastSuccess,
  mockUseSession,
} = vi.hoisted(() => ({
  mockPush: vi.fn(),
  mockUseSearchParams: vi.fn(() => ({
    get: vi.fn(() => null),
    toString: vi.fn(() => "room=42"),
  })),
  mockUseSWRAuth: vi.fn(),
  mockUseTenantMutateMatching: vi.fn(),
  mockGetStudentCurrentVisit: vi.fn(),
  mockUpdateVisit: vi.fn(),
  mockMoveStudentsToActiveGroup: vi.fn(),
  mockGetActiveGroups: vi.fn(),
  mockGetStaffActiveSupervisions: vi.fn(),
  mockToastSuccess: vi.fn(),
  mockUseSession: vi.fn(),
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mockPush }),
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => mockUseSearchParams(),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => mockUseSession(),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (...args: unknown[]) => mockUseSWRAuth(...args),
  useTenantMutateMatching: (...args: unknown[]) =>
    mockUseTenantMutateMatching(...args),
}));

vi.mock("~/lib/active-service", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/active-service")>();
  return {
    ...actual,
    activeService: {
      getActiveGroups: (...args: unknown[]) => mockGetActiveGroups(...args),
      getStudentCurrentVisit: (...args: unknown[]) =>
        mockGetStudentCurrentVisit(...args),
      updateVisit: (...args: unknown[]) => mockUpdateVisit(...args),
      moveStudentsToActiveGroup: (...args: unknown[]) =>
        mockMoveStudentsToActiveGroup(...args),
      getStaffActiveSupervisions: (...args: unknown[]) =>
        mockGetStaffActiveSupervisions(...args),
    },
  };
});

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
  }),
}));

// Light mocks for visual primitives, we want to assert behaviors (text,
// button visibility, click handlers), not the exact rendering of every UI
// atom imported transitively. The "Kinder im Raum" section now renders
// its own <section aria-label> wrapper instead of going through InfoCard
// (#1323 review, quiet section headers across the slide-over), so no
// info-card mock is needed.

vi.mock("~/components/ui/button", () => ({
  Button: ({
    onClick,
    children,
    isLoading: _isLoading,
    loadingText: _loadingText,
    variant: _variant,
    size: _size,
    ...rest
  }: React.ButtonHTMLAttributes<HTMLButtonElement> & {
    isLoading?: boolean;
    loadingText?: string;
    variant?: string;
    size?: string;
  }) => (
    <button type="button" onClick={onClick} {...rest}>
      {children}
    </button>
  ),
}));

vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message }: { message: string }) => (
    <div role="alert">{message}</div>
  ),
}));

// Loading widget is no longer used here (replaced with a content-shaped
// StudentRowSkeleton in #1323 review). Tests below assert the skeleton's
// data-testid + role="status" instead of the old "loading" testid, same
// business assertion (a loading state IS shown while SWR fetches), new
// selector that matches the new skeleton shape.

vi.mock("~/components/students/compact-student-card", () => ({
  CompactStudentCard: ({
    studentId,
    firstName,
    lastName,
    schoolClass,
    groupName,
  }: {
    studentId: string;
    firstName?: string;
    lastName?: string;
    schoolClass?: string;
    groupName?: string;
  }) => (
    <div
      data-testid={`compact-student-card-${studentId}`}
      data-school-class={schoolClass}
      data-group-name={groupName}
    >
      <span>
        {firstName} {lastName}
      </span>
    </div>
  ),
}));

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

interface MockStudent {
  id: string;
  first_name: string;
  second_name: string;
  school_class: string;
  group_name?: string;
}

interface MockActiveGroup {
  id: string;
  roomId: string;
  isActive: boolean;
  /** Running supervisions on the session (any staff), as the API reports. */
  supervisorCount?: number;
}

interface MockRoom {
  id: string;
  name: string;
}

interface MockSupervisor {
  id: string;
  staffId: string;
  activeGroupId: string;
  startTime: Date;
  isActive: boolean;
  createdAt: Date;
  updatedAt: Date;
}

const makeStudent = (overrides: Partial<MockStudent> = {}): MockStudent => ({
  id: "1",
  first_name: "Anna",
  second_name: "Müller",
  school_class: "1a",
  group_name: "Bären",
  ...overrides,
});

const makeSupervisions = (
  activeGroups: readonly MockActiveGroup[],
): MockSupervisor[] =>
  activeGroups.map((group, index) => ({
    id: String(index + 1),
    staffId: "20",
    activeGroupId: group.id,
    startTime: new Date("2026-05-14T08:00:00.000Z"),
    isActive: true,
    createdAt: new Date("2026-05-14T08:00:00.000Z"),
    updatedAt: new Date("2026-05-14T08:00:00.000Z"),
  }));

let roomStudentsState: {
  data?: {
    students: MockStudent[];
    pagination?: { total_records: number };
  };
  error?: unknown;
  isLoading?: boolean;
};
let activeGroupsState: { data: MockActiveGroup[] };
let roomsState: { data: MockRoom[] };
let currentStaffState: { data?: { id: string }; error?: unknown };
let activeSupervisionsState: { data: MockSupervisor[]; error?: unknown };
let sourceVisitGroupIDs: Map<string, string> | undefined;

const setSWR = (state: typeof roomStudentsState) => {
  roomStudentsState = state;
};

const setBulkData = ({
  // "842" is the active group running in the source room ("42"). By
  // default the current staff member supervises every listed session
  // (makeSupervisions), so both the push and the pull rule (#2969) apply.
  activeGroups = [
    { id: "842", roomId: "42", isActive: true },
    { id: "900", roomId: "9000", isActive: true },
    { id: "901", roomId: "9001", isActive: true },
  ],
  rooms = [
    { id: "9000", name: "Raum 6" },
    { id: "9001", name: "Atelier" },
  ],
  currentStaff = { id: "20" },
  activeSupervisions = makeSupervisions(activeGroups),
}: {
  activeGroups?: MockActiveGroup[];
  rooms?: MockRoom[];
  currentStaff?: { id: string } | undefined;
  activeSupervisions?: MockSupervisor[];
} = {}) => {
  activeGroupsState = { data: activeGroups };
  roomsState = { data: rooms };
  currentStaffState = { data: currentStaff };
  activeSupervisionsState = { data: activeSupervisions };
};

beforeEach(() => {
  vi.mocked(useAttendanceWebEnabled).mockReturnValue(true);
  vi.mocked(useOptionalSupervision).mockReturnValue(EMPTY_SUPERVISION);
  mockPush.mockReset();
  mockUseSWRAuth.mockReset();
  mockUseTenantMutateMatching.mockReset();
  mockGetStudentCurrentVisit.mockReset();
  mockUpdateVisit.mockReset();
  mockMoveStudentsToActiveGroup.mockReset();
  mockGetActiveGroups.mockReset();
  mockGetStaffActiveSupervisions.mockReset();
  mockToastSuccess.mockReset();
  mockUseSession.mockReset();
  mockUseSession.mockReturnValue({
    data: {
      user: {
        token: "staff-token",
        roles: ["user"],
        permissions: ["visits:update"],
        isAdmin: false,
      },
    },
    status: "authenticated",
    update: vi.fn(),
  });
  mockUseSearchParams.mockReset();
  mockUseSearchParams.mockReturnValue({
    get: vi.fn(() => null),
    toString: vi.fn(() => "room=42"),
  });
  mockUseTenantMutateMatching.mockReturnValue(vi.fn());
  setSWR({ data: { students: [] } });
  setBulkData();
  sourceVisitGroupIDs = undefined;
  mockUseSWRAuth.mockImplementation((key: string | null) => {
    if (key === null) {
      return { data: undefined, error: null, isLoading: false };
    }
    if (key.startsWith("room-students-")) {
      return {
        data: roomStudentsState.data,
        error: roomStudentsState.error ?? null,
        isLoading: roomStudentsState.isLoading ?? false,
      };
    }
    if (key === "room-bulk-active-groups") {
      return {
        data: activeGroupsState.data,
        error: null,
        isLoading: false,
      };
    }
    if (key === "room-bulk-current-staff") {
      return {
        data: currentStaffState.data,
        error: currentStaffState.error ?? null,
        isLoading: false,
      };
    }
    if (key === "room-bulk-active-supervisions-20") {
      return {
        data: activeSupervisionsState.data,
        error: activeSupervisionsState.error ?? null,
        isLoading: false,
      };
    }
    if (key.startsWith("room-bulk-source-visits-")) {
      const supervisedSource = activeGroupsState.data.find(
        (group) =>
          group.roomId === "42" &&
          activeSupervisionsState.data.some(
            (supervision) => supervision.activeGroupId === group.id,
          ),
      );
      return {
        data: new Map(
          (roomStudentsState.data?.students ?? []).flatMap((student) => {
            const activeGroupId =
              sourceVisitGroupIDs?.get(student.id) ?? supervisedSource?.id;
            return activeGroupId ? [[student.id, { activeGroupId }]] : [];
          }),
        ),
        error: null,
        isLoading: false,
      };
    }
    if (key === "room-bulk-rooms") {
      return {
        data: roomsState.data,
        error: null,
        isLoading: false,
      };
    }
    return { data: undefined, error: null, isLoading: false };
  });
});

// ----------------------------------------------------------------------------
// Tests
// ----------------------------------------------------------------------------

describe("StudentsInRoomSection", () => {
  describe("loading and error states", () => {
    it("shows the content-shaped skeleton while fetching", () => {
      setSWR({ data: undefined, isLoading: true });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      // Skeleton mirrors the populated CompactStudentCard list shape so
      // the section doesn't visibly resize when real students arrive
      // (#1323 review). role="status" + aria-label preserves the AT
      // announcement contract the previous Loading widget provided.
      expect(
        screen.getByTestId("students-in-room-skeleton"),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("status", { name: "Kinderliste wird geladen" }),
      ).toBeInTheDocument();
    });

    it("renders an error alert if the fetch fails", () => {
      setSWR({ data: undefined, error: new Error("network down") });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent(/Liste der Kinder/);
      // Error and empty states must stay mutually exclusive.
      expect(
        screen.queryByText(/Aktuell keine Kinder/),
      ).not.toBeInTheDocument();
    });

    it("shows the empty placeholder when no children are present", () => {
      setSWR({ data: { students: [] } });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      expect(
        screen.getByText(/Aktuell keine Kinder im Raum/),
      ).toBeInTheDocument();
      // Do not deep-link to an empty filtered Kindersuche.
      expect(
        screen.queryByRole("button", { name: /Alle Kinder/ }),
      ).not.toBeInTheDocument();
    });
  });

  describe("populated list", () => {
    it("renders one card per student", () => {
      setSWR({
        data: {
          students: [
            makeStudent({ id: "1", first_name: "Anna" }),
            makeStudent({ id: "2", first_name: "Ben" }),
            makeStudent({ id: "3", first_name: "Carla" }),
          ],
        },
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      expect(screen.getByTestId("compact-student-card-1")).toBeInTheDocument();
      expect(screen.getByTestId("compact-student-card-2")).toBeInTheDocument();
      expect(screen.getByTestId("compact-student-card-3")).toBeInTheDocument();
    });

    it("forwards school_class and group_name to the compact card", () => {
      // The minimal card still needs class and group identifiers.
      setSWR({
        data: {
          students: [
            makeStudent({
              id: "1",
              school_class: "2b",
              group_name: "Füchse",
            }),
          ],
        },
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      const card = screen.getByTestId("compact-student-card-1");
      expect(card.getAttribute("data-school-class")).toBe("2b");
      expect(card.getAttribute("data-group-name")).toBe("Füchse");
    });

    it("uses the singular noun when exactly one child is present", () => {
      setSWR({ data: { students: [makeStudent({ id: "1" })] } });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      // Pluralization regression guard: "1 Kind" not "1 Kinder".
      // The count and noun render in sibling nodes, so search by parent text.
      const subline = screen.getByText((_, node) => {
        const text = node?.textContent ?? "";
        return /\b1\s+Kind\s+aktuell\s+anwesend\b/.test(text);
      });
      expect(subline).toBeInTheDocument();
    });

    it("uses the plural noun when more than one child is present", () => {
      setSWR({
        data: {
          students: [makeStudent({ id: "1" }), makeStudent({ id: "2" })],
        },
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      const subline = screen.getByText((_, node) => {
        const text = node?.textContent ?? "";
        return /\b2\s+Kinder\s+aktuell\s+anwesend\b/.test(text);
      });
      expect(subline).toBeInTheDocument();
    });
  });

  describe("count source", () => {
    it("uses pagination.total_records over the rendered length", () => {
      // Even if the API ever truncates the response (default backend
      // page size is 50), the count badge must stay honest. The visible
      // card list can be shorter than the count without lying about it.
      setSWR({
        data: {
          students: [makeStudent({ id: "1" }), makeStudent({ id: "2" })],
          pagination: { total_records: 87 },
        },
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      const subline = screen.getByText((_, node) => {
        const text = node?.textContent ?? "";
        return /\b87\s+Kinder\s+aktuell\s+anwesend\b/.test(text);
      });
      expect(subline).toBeInTheDocument();
    });

    it("falls back to students.length when pagination metadata is missing", () => {
      setSWR({
        data: {
          students: [makeStudent({ id: "1" }), makeStudent({ id: "2" })],
        },
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      const subline = screen.getByText((_, node) => {
        const text = node?.textContent ?? "";
        return /\b2\s+Kinder\s+aktuell\s+anwesend\b/.test(text);
      });
      expect(subline).toBeInTheDocument();
    });
  });

  describe("truncation notice", () => {
    it("renders an overflow status when total_records exceeds rendered count", () => {
      // Hard-coded pageSize=200 caps the response. When a room ever
      // exceeds it, the section must surface the gap and point at the
      // Kindersuche escape hatch, otherwise staff can't see or open
      // the missing children (#1374).
      setSWR({
        data: {
          students: [
            makeStudent({ id: "1" }),
            makeStudent({ id: "2" }),
            makeStudent({ id: "3" }),
          ],
          pagination: { total_records: 250 },
        },
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      const notice = screen.getByRole("status");
      expect(notice).toHaveTextContent(/3 von 250 Kindern/);
      expect(notice).toHaveTextContent(/247/);
      expect(notice).toHaveTextContent(/Alle Kinder/);
    });

    it("does NOT render the notice when total_records matches rendered count", () => {
      setSWR({
        data: {
          students: [makeStudent({ id: "1" }), makeStudent({ id: "2" })],
          pagination: { total_records: 2 },
        },
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      expect(screen.queryByRole("status")).not.toBeInTheDocument();
    });
  });

  describe("bulk room move", () => {
    it("uses one source-visit cache key for equivalent source group sets", () => {
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });
      setBulkData({
        activeGroups: [
          { id: "source-b", roomId: "42", isActive: true },
          { id: "source-a", roomId: "42", isActive: true },
        ],
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      expect(mockUseSWRAuth).toHaveBeenCalledWith(
        'room-bulk-source-visits-["source-a","source-b"]',
        expect.any(Function),
      );
    });

    it("renders the roster read-only when web attendance is disabled", () => {
      vi.mocked(useAttendanceWebEnabled).mockReturnValue(false);
      setSWR({ data: { students: [makeStudent()] } });
      setBulkData();

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
      expect(screen.queryByText("Kinder auswählen")).not.toBeInTheDocument();
      expect(screen.getByTestId("compact-student-card-1")).toBeInTheDocument();
    });
    it("selects multiple children and moves their current visits to the selected room", async () => {
      const refreshCaches = vi.fn().mockResolvedValue(undefined);
      mockUseTenantMutateMatching.mockReturnValue(refreshCaches);
      setSWR({
        data: {
          students: [
            makeStudent({ id: "7", first_name: "Anna" }),
            makeStudent({ id: "8", first_name: "Ben", second_name: "Schulz" }),
          ],
        },
      });
      mockMoveStudentsToActiveGroup.mockResolvedValue({
        moved: [7, 8],
        unchanged: [],
        skipped: [],
        active_group_id: 900,
        room_id: 9000,
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      expect(mockUseTenantMutateMatching).toHaveBeenCalledWith(
        expect.arrayContaining(["rooms-list", "room-bulk-source-visits-"]),
      );
      fireEvent.click(
        screen.getByRole("checkbox", { name: /Anna Müller auswählen/ }),
      );
      fireEvent.click(
        screen.getByRole("checkbox", { name: /Ben Schulz auswählen/ }),
      );
      fireEvent.click(screen.getByLabelText("Zielraum"));
      fireEvent.click(screen.getByRole("option", { name: "Raum 6" }));
      fireEvent.click(screen.getByRole("button", { name: "In Raum setzen" }));

      await waitFor(() => {
        expect(mockMoveStudentsToActiveGroup).toHaveBeenCalledWith(
          ["7", "8"],
          "900",
        );
      });
      expect(mockGetStudentCurrentVisit).not.toHaveBeenCalled();
      expect(mockUpdateVisit).not.toHaveBeenCalled();
      expect(refreshCaches).toHaveBeenCalledTimes(1);
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "2 Kinder nach Raum 6 bewegt.",
      );
    });

    it("keeps the move action disabled when no target room is selected", () => {
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      fireEvent.click(
        screen.getByRole("checkbox", { name: /Anna Müller auswählen/ }),
      );

      expect(mockGetStudentCurrentVisit).not.toHaveBeenCalled();
      expect(mockUpdateVisit).not.toHaveBeenCalled();
      expect(mockMoveStudentsToActiveGroup).not.toHaveBeenCalled();
      expect(
        screen.getByRole("button", { name: "In Raum setzen" }),
      ).toBeDisabled();
    });

    it("treats a child already in the target room as a successful no-op", async () => {
      const refreshCaches = vi.fn().mockResolvedValue(undefined);
      mockUseTenantMutateMatching.mockReturnValue(refreshCaches);
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });
      mockMoveStudentsToActiveGroup.mockResolvedValue({
        moved: [],
        unchanged: [7],
        skipped: [],
        active_group_id: 900,
        room_id: 9000,
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      fireEvent.click(
        screen.getByRole("checkbox", { name: /Anna Müller auswählen/ }),
      );
      fireEvent.click(screen.getByLabelText("Zielraum"));
      fireEvent.click(screen.getByRole("option", { name: "Raum 6" }));
      fireEvent.click(screen.getByRole("button", { name: "In Raum setzen" }));

      await waitFor(() => {
        expect(mockMoveStudentsToActiveGroup).toHaveBeenCalledWith(
          ["7"],
          "900",
        );
      });
      expect(mockGetStudentCurrentVisit).not.toHaveBeenCalled();
      expect(mockUpdateVisit).not.toHaveBeenCalled();
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "1 Kind nach Raum 6 bewegt.",
      );
      expect(refreshCaches).toHaveBeenCalledTimes(1);
    });

    it("reports a partial failure when a selected child has no active visit", async () => {
      const refreshCaches = vi.fn().mockResolvedValue(undefined);
      mockUseTenantMutateMatching.mockReturnValue(refreshCaches);
      setSWR({
        data: {
          students: [
            makeStudent({ id: "7", first_name: "Anna" }),
            makeStudent({ id: "8", first_name: "Ben", second_name: "Schulz" }),
          ],
        },
      });
      mockMoveStudentsToActiveGroup.mockResolvedValue({
        moved: [8],
        unchanged: [],
        skipped: [{ student_id: 7, reason: "not_present" }],
        active_group_id: 900,
        room_id: 9000,
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      fireEvent.click(
        screen.getByRole("checkbox", { name: /Anna Müller auswählen/ }),
      );
      fireEvent.click(
        screen.getByRole("checkbox", { name: /Ben Schulz auswählen/ }),
      );
      fireEvent.click(screen.getByLabelText("Zielraum"));
      fireEvent.click(screen.getByRole("option", { name: "Raum 6" }));
      fireEvent.click(screen.getByRole("button", { name: "In Raum setzen" }));

      await waitFor(() => {
        expect(screen.getByRole("alert")).toHaveTextContent(
          "1 von 2 Kindern konnten nicht bewegt werden.",
        );
      });
      expect(mockMoveStudentsToActiveGroup).toHaveBeenCalledWith(
        ["7", "8"],
        "900",
      );
      expect(mockGetStudentCurrentVisit).not.toHaveBeenCalled();
      expect(mockUpdateVisit).not.toHaveBeenCalled();
      expect(mockToastSuccess).not.toHaveBeenCalled();
      expect(refreshCaches).toHaveBeenCalledTimes(1);
    });

    it("selects a child when clicking the row content without opening the profile", () => {
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      fireEvent.click(screen.getByTestId("compact-student-card-7"));

      expect(mockPush).not.toHaveBeenCalled();
      expect(
        screen.getByRole("checkbox", { name: /Anna Müller auswählen/ }),
      ).toBeChecked();
    });

    it("reports active selection state to the drawer wrapper", async () => {
      const onSelectionActiveChange = vi.fn();
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });

      render(
        <StudentsInRoomSection
          roomId="42"
          roomName="OGS-Raum 1"
          onSelectionActiveChange={onSelectionActiveChange}
        />,
      );

      fireEvent.click(
        screen.getByRole("checkbox", { name: /Anna Müller auswählen/ }),
      );

      await waitFor(() => {
        expect(onSelectionActiveChange).toHaveBeenLastCalledWith(true);
      });
    });

    it("excludes the current room from the target room list", () => {
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });
      setBulkData({
        activeGroups: [
          { id: "current", roomId: "42", isActive: true },
          { id: "target", roomId: "9000", isActive: true },
        ],
        rooms: [
          { id: "42", name: "OGS-Raum 1" },
          { id: "9000", name: "Raum 6" },
        ],
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      fireEvent.click(screen.getByLabelText("Zielraum"));
      expect(screen.queryByRole("option", { name: "OGS-Raum 1" })).toBeNull();
      expect(
        screen.getByRole("option", { name: "Raum 6" }),
      ).toBeInTheDocument();
    });

    it("offers a source supervisor every room a colleague supervises, but no unsupervised room (#2969)", () => {
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });
      setBulkData({
        activeGroups: [
          { id: "source", roomId: "42", isActive: true, supervisorCount: 1 },
          // Running session, but nobody supervises it right now.
          { id: "unsupervised", roomId: "9000", isActive: true },
          // Supervised by a colleague only.
          {
            id: "colleague",
            roomId: "9001",
            isActive: true,
            supervisorCount: 1,
          },
          // Supervised by the current staff member.
          { id: "own", roomId: "9002", isActive: true, supervisorCount: 1 },
        ],
        rooms: [
          { id: "9000", name: "Aula" },
          { id: "9001", name: "Raum 6" },
          { id: "9002", name: "Atelier" },
        ],
        activeSupervisions: makeSupervisions([
          { id: "source", roomId: "42", isActive: true },
          { id: "own", roomId: "9002", isActive: true },
        ]),
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      expect(
        screen.getByText("Zur Auswahl stehen Räume mit laufender Aufsicht."),
      ).toBeInTheDocument();
      fireEvent.click(screen.getByLabelText("Zielraum"));
      expect(screen.queryByRole("option", { name: "Aula" })).toBeNull();
      expect(
        screen.getByRole("option", { name: "Raum 6" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("option", { name: "Atelier" }),
      ).toBeInTheDocument();
    });

    it("keeps push targets available when another source session is not supervised (#2969)", () => {
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });
      setBulkData({
        activeGroups: [
          {
            id: "supervised-source",
            roomId: "42",
            isActive: true,
            supervisorCount: 1,
          },
          { id: "unsupervised-source", roomId: "42", isActive: true },
          {
            id: "colleague",
            roomId: "9001",
            isActive: true,
            supervisorCount: 1,
          },
        ],
        rooms: [{ id: "9001", name: "Raum 6" }],
        activeSupervisions: makeSupervisions([
          { id: "supervised-source", roomId: "42", isActive: true },
        ]),
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      fireEvent.click(screen.getByLabelText("Zielraum"));
      expect(
        screen.getByRole("option", { name: "Raum 6" }),
      ).toBeInTheDocument();
    });

    it("selects and clears only children from supervised source sessions for a colleague target (#2969)", () => {
      sourceVisitGroupIDs = new Map([
        ["7", "supervised-source"],
        ["8", "unsupervised-source"],
      ]);
      setSWR({
        data: {
          students: [
            makeStudent({ id: "7" }),
            makeStudent({ id: "8", first_name: "Ben" }),
          ],
        },
      });
      setBulkData({
        activeGroups: [
          {
            id: "supervised-source",
            roomId: "42",
            isActive: true,
            supervisorCount: 1,
          },
          { id: "unsupervised-source", roomId: "42", isActive: true },
          {
            id: "colleague",
            roomId: "9001",
            isActive: true,
            supervisorCount: 1,
          },
        ],
        rooms: [{ id: "9001", name: "Raum 6" }],
        activeSupervisions: makeSupervisions([
          { id: "supervised-source", roomId: "42", isActive: true },
        ]),
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      fireEvent.click(screen.getByLabelText("Zielraum"));
      fireEvent.click(screen.getByRole("option", { name: "Raum 6" }));
      fireEvent.click(screen.getByRole("button", { name: "Alle auswählen" }));

      expect(screen.getByText("1 von 1 ausgewählt")).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Aufheben" }),
      ).toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Aufheben" }));
      expect(screen.getByText("Kinder auswählen")).toBeInTheDocument();
    });

    it("lets a source supervisor push a child into a colleague's room (#2969)", async () => {
      mockMoveStudentsToActiveGroup.mockResolvedValue({
        moved: ["7"],
        unchanged: [],
        skipped: [],
      });
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });
      setBulkData({
        activeGroups: [
          { id: "source", roomId: "42", isActive: true, supervisorCount: 1 },
          {
            id: "colleague",
            roomId: "9001",
            isActive: true,
            supervisorCount: 2,
          },
        ],
        rooms: [{ id: "9001", name: "Raum 6" }],
        activeSupervisions: makeSupervisions([
          { id: "source", roomId: "42", isActive: true },
        ]),
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      fireEvent.click(
        screen.getByRole("checkbox", { name: /Anna Müller auswählen/ }),
      );
      fireEvent.click(screen.getByLabelText("Zielraum"));
      fireEvent.click(screen.getByRole("option", { name: "Raum 6" }));
      fireEvent.click(screen.getByRole("button", { name: "In Raum setzen" }));

      await waitFor(() => {
        expect(mockMoveStudentsToActiveGroup).toHaveBeenCalledWith(
          ["7"],
          "colleague",
        );
      });
    });

    it("explains when a source supervisor has no supervised target room right now (#2969)", () => {
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });
      setBulkData({
        activeGroups: [
          { id: "source", roomId: "42", isActive: true, supervisorCount: 1 },
          { id: "unsupervised", roomId: "9000", isActive: true },
        ],
        rooms: [{ id: "9000", name: "Aula" }],
        activeSupervisions: makeSupervisions([
          { id: "source", roomId: "42", isActive: true },
        ]),
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      expect(
        screen.getByText(
          "Zurzeit hat kein anderer Raum eine laufende Aufsicht.",
        ),
      ).toBeInTheDocument();
      expect(screen.getByLabelText("Zielraum")).toBeDisabled();
    });

    it("keeps all active target rooms visible for admins", () => {
      mockUseSession.mockReturnValue({
        data: {
          user: {
            token: "admin-token",
            roles: ["admin"],
            permissions: ["admin:*"],
            isAdmin: true,
          },
        },
        status: "authenticated",
        update: vi.fn(),
      });
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });
      setBulkData({
        activeGroups: [
          { id: "900", roomId: "9000", isActive: true },
          { id: "901", roomId: "9001", isActive: true },
        ],
        rooms: [
          { id: "9000", name: "Raum 6" },
          { id: "9001", name: "Atelier" },
        ],
        activeSupervisions: [],
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      fireEvent.click(screen.getByLabelText("Zielraum"));
      expect(
        screen.getByRole("option", { name: "Raum 6" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("option", { name: "Atelier" }),
      ).toBeInTheDocument();
    });

    it("excludes rooms with multiple active sessions from the target room list", () => {
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });
      setBulkData({
        activeGroups: [
          { id: "source", roomId: "42", isActive: true },
          { id: "first", roomId: "9000", isActive: true },
          { id: "second", roomId: "9000", isActive: true },
          { id: "single", roomId: "9001", isActive: true, supervisorCount: 1 },
        ],
        rooms: [
          { id: "9000", name: "Raum mit Konflikt" },
          { id: "9001", name: "Raum 6" },
        ],
        activeSupervisions: makeSupervisions([
          { id: "source", roomId: "42", isActive: true },
        ]),
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      fireEvent.click(screen.getByLabelText("Zielraum"));
      expect(
        screen.queryByRole("option", { name: "Raum mit Konflikt" }),
      ).toBeNull();
      expect(
        screen.getByRole("option", { name: "Raum 6" }),
      ).toBeInTheDocument();
    });

    it("offers an own pull target in an otherwise ambiguous room", () => {
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });
      setBulkData({
        activeGroups: [
          { id: "source", roomId: "42", isActive: true, supervisorCount: 1 },
          { id: "mine", roomId: "9000", isActive: true, supervisorCount: 1 },
          { id: "other", roomId: "9000", isActive: true },
        ],
        rooms: [{ id: "9000", name: "Raum mit Konflikt" }],
        activeSupervisions: makeSupervisions([
          { id: "source", roomId: "42", isActive: true },
          { id: "mine", roomId: "9000", isActive: true },
        ]),
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      fireEvent.click(screen.getByLabelText("Zielraum"));
      expect(
        screen.getByRole("option", { name: "Raum mit Konflikt" }),
      ).toBeInTheDocument();
    });

    it("lets a target supervisor pull a child out of a colleague's room, limited to own rooms (#2969)", async () => {
      mockMoveStudentsToActiveGroup.mockResolvedValue({
        moved: ["7"],
        unchanged: [],
        skipped: [],
      });
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });
      setBulkData({
        activeGroups: [
          // Source supervised by a colleague, not by the current staff member.
          { id: "source", roomId: "42", isActive: true, supervisorCount: 1 },
          { id: "target", roomId: "9000", isActive: true, supervisorCount: 1 },
          // Another colleague's room: visible to source supervisors only.
          { id: "foreign", roomId: "9001", isActive: true, supervisorCount: 1 },
        ],
        rooms: [
          { id: "9000", name: "Raum 6" },
          { id: "9001", name: "Aula" },
        ],
        activeSupervisions: makeSupervisions([
          { id: "target", roomId: "9000", isActive: true },
        ]),
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      expect(
        screen.getByText(
          "Zur Auswahl stehen nur Räume, die Sie selbst beaufsichtigen.",
        ),
      ).toBeInTheDocument();
      fireEvent.click(
        screen.getByRole("checkbox", { name: /Anna Müller auswählen/ }),
      );
      fireEvent.click(screen.getByLabelText("Zielraum"));
      expect(screen.queryByRole("option", { name: "Aula" })).toBeNull();
      fireEvent.click(screen.getByRole("option", { name: "Raum 6" }));
      fireEvent.click(screen.getByRole("button", { name: "In Raum setzen" }));

      await waitFor(() => {
        expect(mockMoveStudentsToActiveGroup).toHaveBeenCalledWith(
          ["7"],
          "target",
        );
      });
    });

    it("hides the bulk-move UI when the staff member supervises neither the source nor a target (#2969)", () => {
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });
      setBulkData({
        activeGroups: [
          { id: "source", roomId: "42", isActive: true, supervisorCount: 1 },
          { id: "target", roomId: "9000", isActive: true, supervisorCount: 1 },
        ],
        rooms: [{ id: "9000", name: "Raum 6" }],
        activeSupervisions: [],
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      // Backend rejects the move with 403 (push-or-pull rule), so neither
      // the toolbar nor the per-row checkboxes may render.
      expect(
        screen.queryByRole("button", { name: "In Raum setzen" }),
      ).toBeNull();
      expect(
        screen.queryByRole("checkbox", { name: /Anna Müller auswählen/ }),
      ).toBeNull();
      // The student list itself stays visible.
      expect(screen.getByText("Anna Müller")).toBeInTheDocument();
    });

    it("hides the bulk-move UI when the source room has no active group", () => {
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });
      setBulkData({
        activeGroups: [{ id: "target", roomId: "9000", isActive: true }],
        rooms: [{ id: "9000", name: "Raum 6" }],
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      expect(
        screen.queryByRole("button", { name: "In Raum setzen" }),
      ).toBeNull();
      expect(
        screen.queryByRole("checkbox", { name: /Anna Müller auswählen/ }),
      ).toBeNull();
    });

    it("keeps the bulk-move UI for admins without source supervision", () => {
      mockUseSession.mockReturnValue({
        data: {
          user: {
            token: "admin-token",
            roles: ["admin"],
            permissions: ["admin:*"],
            isAdmin: true,
          },
        },
        status: "authenticated",
        update: vi.fn(),
      });
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });
      setBulkData({
        activeGroups: [{ id: "target", roomId: "9000", isActive: true }],
        rooms: [{ id: "9000", name: "Raum 6" }],
        activeSupervisions: [],
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      expect(
        screen.getByRole("button", { name: "In Raum setzen" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("checkbox", { name: /Anna Müller auswählen/ }),
      ).toBeInTheDocument();
    });
  });

  describe("navigation", () => {
    it("encodes the full URL as from= in the slide-over context (preserves filters)", () => {
      // The section now only renders inside the slide-over (legacy
      // /rooms/{id} subpage is a server-side redirect). The from= URL
      // must carry the complete current query string, including any
      // grid filters (search, building, status), so the user lands
      // back on their narrowed view, not a reset grid.
      mockUseSearchParams.mockReturnValue({
        get: vi.fn(() => null),
        toString: vi.fn(
          () => "search=foo&building=Main&status=occupied&room=42",
        ),
      });
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      fireEvent.click(
        screen.getByRole("button", { name: /Anna Müller Profil öffnen/ }),
      );

      // ? inside from= must be URL-encoded; & inside likewise. Without
      // encoding the outer query parser splits at the inner '?' and
      // sees only the first param.
      expect(mockPush).toHaveBeenCalledWith(
        `/students/7?from=${encodeURIComponent(
          "/rooms?search=foo&building=Main&status=occupied&room=42",
        )}`,
      );
    });

    it("falls back to /rooms?room={id} when no query string is present", () => {
      mockUseSearchParams.mockReturnValue({
        get: vi.fn(() => null),
        toString: vi.fn(() => ""),
      });
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      fireEvent.click(
        screen.getByRole("button", { name: /Anna Müller Profil öffnen/ }),
      );

      expect(mockPush).toHaveBeenCalledWith(
        `/students/7?from=${encodeURIComponent("/rooms?room=42")}`,
      );
    });

    it("opens Kindersuche pre-filtered with both room_id and room_name", () => {
      setSWR({
        data: {
          students: [makeStudent({ id: "1" }), makeStudent({ id: "2" })],
        },
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      fireEvent.click(
        screen.getByRole("button", { name: /Alle Kinder öffnen/ }),
      );

      expect(mockPush).toHaveBeenCalledTimes(1);
      const target = mockPush.mock.calls[0]?.[0] as string;
      expect(target).toMatch(/^\/students\/search\?/);
      // room_name must be URL-encoded. The seed value "OGS-Raum 1" contains
      // a space and a hyphen and would break a naive concatenation.
      expect(target).toContain("room_id=42");
      expect(target).toContain("room_name=OGS-Raum+1");
    });
  });
});
