/**
 * Tests for Active Supervisions Page
 * Tests the rendering states and user interactions of the active supervisions dashboard
 */
import { render, screen, waitFor, cleanup } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { token: "test-token" } },
    status: "authenticated",
  })),
}));

// Mock next/navigation
const mockPush = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
}));

// Mock ResponsiveLayout
vi.mock("~/components/dashboard", () => ({
  ResponsiveLayout: ({
    children,
    activeSupervisionName,
  }: {
    children: React.ReactNode;
    activeSupervisionName?: string;
  }) => (
    <div data-testid="responsive-layout" data-room={activeSupervisionName}>
      {children}
    </div>
  ),
}));

// Mock Loading component
vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading">Loading...</div>,
}));

// Mock PageHeaderWithSearch
vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({
    title,
    badge,
  }: {
    title: string;
    badge?: { count: number };
  }) => (
    <div data-testid="page-header" data-count={badge?.count}>
      {title}
    </div>
  ),
}));

// Mock Alert
vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message, type }: { message: string; type: string }) => (
    <div data-testid={`alert-${type}`}>{message}</div>
  ),
}));

// Mock Modal
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
}));

// Mock activeService
vi.mock("~/lib/active-api", () => ({
  activeService: {
    getActiveGroupVisitsWithDisplay: vi.fn(() => Promise.resolve([])),
    getActiveGroupSupervisors: vi.fn(() => Promise.resolve([])),
    endSupervision: vi.fn(() => Promise.resolve()),
  },
}));

// Mock SSEErrorBoundary
vi.mock("~/components/sse/SSEErrorBoundary", () => ({
  SSEErrorBoundary: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="sse-boundary">{children}</div>
  ),
}));

// Mock UnclaimedRooms
vi.mock("~/components/active", () => ({
  UnclaimedRooms: () => <div data-testid="unclaimed-rooms" />,
}));

// Mock LocationBadge
vi.mock("@/components/ui/location-badge", () => ({
  LocationBadge: () => <div data-testid="location-badge">Location</div>,
}));

// Mock EmptyStudentResults
vi.mock("~/components/ui/empty-student-results", () => ({
  EmptyStudentResults: () => <div data-testid="empty-results">No results</div>,
}));

// Mock StudentCard components
vi.mock("~/components/students/student-card", () => ({
  StudentCard: ({
    firstName,
    lastName,
  }: {
    firstName: string;
    lastName: string;
  }) => (
    <div data-testid="student-card">
      {firstName} {lastName}
    </div>
  ),
  StudentInfoRow: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="student-info-row">{children}</div>
  ),
  SchoolClassIcon: () => <span data-testid="school-class-icon" />,
  GroupIcon: () => <span data-testid="group-icon" />,
}));

// Mock SWR hook
vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
}));

import { useSWRAuth } from "~/lib/swr";
import MeinRaumPage from "./page";

describe("MeinRaumPage (Active Supervisions)", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
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
    render(<MeinRaumPage />);

    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("renders with SSE error boundary wrapper", () => {
    render(<MeinRaumPage />);

    expect(screen.getByTestId("sse-boundary")).toBeInTheDocument();
  });

  it("renders within responsive layout", async () => {
    render(<MeinRaumPage />);

    expect(screen.getByTestId("responsive-layout")).toBeInTheDocument();
  });

  it("shows no access state when user has no active supervision", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        supervisedGroups: [],
        unclaimedGroups: [],
        currentStaff: { id: "1" },
        educationalGroups: [],
        firstRoomVisits: [],
        firstRoomId: null,
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine aktive Raum-Aufsicht"),
      ).toBeInTheDocument();
    });
  });

  it("shows unclaimed rooms component when user has groups to claim", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        supervisedGroups: [],
        unclaimedGroups: [{ id: "1", name: "Schulhof" }],
        currentStaff: { id: "1" },
        educationalGroups: [],
        firstRoomVisits: [],
        firstRoomId: null,
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<MeinRaumPage />);

    await waitFor(() => {
      expect(screen.getByTestId("unclaimed-rooms")).toBeInTheDocument();
    });
  });

  it("shows loading state when SWR is loading", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: true,
      error: null,
      mutate: mockMutate,
      isValidating: false,
    } as never);

    render(<MeinRaumPage />);

    // Should show loading state while SWR is loading
    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("displays supervised room with students", async () => {
    // First call: dashboard data, Second call: per-room visits (return null to skip)
    const dashboardData = {
      supervisedGroups: [
        { id: "1", name: "Schulhof", room: { id: "10", name: "Schulhof" } },
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
    });
  });
});

describe("Active Supervisions helper functions", () => {
  it("filters students by search term", () => {
    const student = {
      name: "Max Mustermann",
      first_name: "Max",
      second_name: "Mustermann",
    };

    const searchLower = "max";
    const matches =
      (student.name?.toLowerCase().includes(searchLower) ?? false) ||
      (student.first_name?.toLowerCase().includes(searchLower) ?? false) ||
      (student.second_name?.toLowerCase().includes(searchLower) ?? false);

    expect(matches).toBe(true);
  });

  it("filters students by group name", () => {
    const students = [
      { id: "1", group_name: "OGS Gruppe A" },
      { id: "2", group_name: "OGS Gruppe B" },
      { id: "3", group_name: "OGS Gruppe A" },
    ];

    const groupFilter = "OGS Gruppe A" as string;
    const filtered = students.filter(
      (s) => groupFilter === "all" || s.group_name === groupFilter,
    );

    expect(filtered).toHaveLength(2);
    expect(filtered.map((s) => s.id)).toEqual(["1", "3"]);
  });

  it("detects Schulhof room for special behavior", () => {
    const SCHULHOF_ROOM_NAME = "Schulhof";
    const room = { room_name: "Schulhof" };

    const isSchulhof = room?.room_name === SCHULHOF_ROOM_NAME;
    expect(isSchulhof).toBe(true);

    const regularRoom = { room_name: "Raum 101" };
    const isSchulhofRegular = regularRoom?.room_name === SCHULHOF_ROOM_NAME;
    expect(isSchulhofRegular).toBe(false);
  });
});
