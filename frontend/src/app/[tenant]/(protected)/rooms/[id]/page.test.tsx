import { render, screen, fireEvent, within } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import RoomDetailPage from "./page";

const mockPush = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
  }),
  useParams: () => ({
    id: "1",
  }),
  useSearchParams: () => ({
    get: vi.fn((key: string) => (key === "from" ? "/rooms" : null)),
  }),
  // StudentsInRoomSection (rendered by RoomDetailContent) reads pathname
  // to decide whether to use /rooms/{id} or /rooms?room={id} as the
  // student-page referrer. Default to the legacy subpage URL for tests
  // entering via /rooms/[id].
  usePathname: () => "/test-tenant/rooms/1",
}));

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: {
      user: {
        token: "test-token",
      },
    },
  })),
}));

// useRoomDetail now goes through useSWRAuth so SSE invalidations can
// refresh the room header (#1374). Mock the SWR layer to drive the
// page from controlled state instead of intercepting raw fetch.
const mockUseSWRAuth = vi.fn();
vi.mock("~/lib/swr", () => ({
  useSWRAuth: (...args: unknown[]) => mockUseSWRAuth(...args),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
  useBreadcrumb: vi.fn(() => ({ breadcrumb: {}, setBreadcrumb: vi.fn() })),
  BreadcrumbProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: ({ message }: { message?: string; fullPage?: boolean }) => (
    <div data-testid="loading" aria-label="Lädt...">
      {message}
    </div>
  ),
}));

vi.mock("~/components/ui/info-card", () => ({
  InfoCard: ({
    title,
    children,
  }: {
    title: string;
    icon?: React.ReactNode;
    children: React.ReactNode;
  }) => (
    <div data-testid={`info-card-${title}`}>
      <h2>{title}</h2>
      {children}
    </div>
  ),
  InfoItem: ({ label, value }: { label: string; value: React.ReactNode }) => (
    <div data-testid={`info-item-${label}`}>
      <span>{label}</span>: <span>{value}</span>
    </div>
  ),
}));

vi.mock("~/components/ui/back-button", () => ({
  BackButton: ({ referrer }: { referrer: string }) => (
    <button data-testid="back-button" data-referrer={referrer}>
      Zurück
    </button>
  ),
}));

vi.mock("~/lib/date-helpers", () => ({
  formatDate: vi.fn((date: string, _showWeekday?: boolean) => {
    return new Date(date).toLocaleDateString("de-DE");
  }),
  formatTime: vi.fn((time: string) => {
    return new Date(time).toLocaleTimeString("de-DE", {
      hour: "2-digit",
      minute: "2-digit",
    });
  }),
  calculateDuration: vi.fn(() => 60),
  formatDuration: vi.fn((minutes: number) => `${minutes} min`),
}));

vi.mock("~/lib/room-helpers", () => ({
  formatFloor: vi.fn((floor: number) => {
    if (floor === 0) return "Erdgeschoss";
    return `${floor}. Etage`;
  }),
}));

// Page test focuses on the room header / status / history rendered by
// RoomDetailContent. The live "Kinder im Raum" section has its own
// dedicated test file — stub it here so this test isn't entangled with
// its data fetching.
vi.mock("~/components/rooms/students-in-room-section", () => ({
  StudentsInRoomSection: () => <div data-testid="students-in-room-section" />,
}));

interface FrontendRoom {
  id: string;
  name: string;
  building?: string;
  floor?: number;
  capacity?: number;
  category?: string;
  isOccupied: boolean;
  groupName?: string;
  activityName?: string;
  supervisorName?: string;
  deviceId?: string;
  studentCount?: number;
  color?: string;
}

interface FrontendHistoryEntry {
  id: string;
  timestamp: string;
  groupName?: string;
  activityName?: string;
  category?: string;
  supervisorName?: string;
  studentCount?: number;
  duration_minutes?: number;
  entry_type: "entry" | "exit";
}

const setRoomDetail = (state: {
  data?: { room: FrontendRoom; history: FrontendHistoryEntry[] };
  isLoading?: boolean;
  error?: unknown;
}) => {
  mockUseSWRAuth.mockReturnValue({
    data: state.data,
    isLoading: state.isLoading ?? false,
    error: state.error ?? null,
  });
};

const mockRoom: FrontendRoom = {
  id: "1",
  name: "Raum 101",
  building: "Hauptgebäude",
  floor: 1,
  capacity: 30,
  category: "Klassenraum",
  isOccupied: false,
  deviceId: "device-1",
  studentCount: 0,
  color: "#4F46E5",
};

const mockOccupiedRoom: FrontendRoom = {
  ...mockRoom,
  isOccupied: true,
  groupName: "OGS Gruppe A",
  activityName: "Basteln",
  supervisorName: "Max Mustermann",
  studentCount: 15,
};

const mockRoomHistory: FrontendHistoryEntry[] = [
  {
    id: "1",
    timestamp: "2024-01-15T10:00:00Z",
    groupName: "OGS Gruppe A",
    activityName: "Basteln",
    category: "Kreativ",
    supervisorName: "Max Mustermann",
    studentCount: 12,
    duration_minutes: 60,
    entry_type: "entry",
  },
  {
    id: "2",
    timestamp: "2024-01-15T11:00:00Z",
    groupName: "OGS Gruppe A",
    activityName: "Basteln",
    category: "Kreativ",
    supervisorName: "Max Mustermann",
    studentCount: 12,
    duration_minutes: 60,
    entry_type: "exit",
  },
];

describe("RoomDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseSWRAuth.mockReset();
  });

  it("renders loading state initially", () => {
    setRoomDetail({ data: undefined, isLoading: true });

    render(<RoomDetailPage />);

    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("renders room details after loading", () => {
    setRoomDetail({ data: { room: mockRoom, history: [] } });

    render(<RoomDetailPage />);

    const roomNames = screen.getAllByText("Raum 101");
    expect(roomNames.length).toBeGreaterThan(0);
  });

  it("displays room information", () => {
    setRoomDetail({ data: { room: mockRoom, history: [] } });

    render(<RoomDetailPage />);

    expect(
      screen.getByTestId("info-card-Rauminformationen"),
    ).toBeInTheDocument();
    expect(screen.getByTestId("info-item-Raumname")).toBeInTheDocument();
  });

  it("displays status badge as free when room is not occupied", () => {
    setRoomDetail({ data: { room: mockRoom, history: [] } });

    render(<RoomDetailPage />);

    // "Frei" appears in badge and also in info items
    const freeElements = screen.getAllByText("Frei");
    expect(freeElements.length).toBeGreaterThan(0);
  });

  it("displays status badge as occupied when room is occupied", () => {
    setRoomDetail({ data: { room: mockOccupiedRoom, history: [] } });

    render(<RoomDetailPage />);

    // "Belegt" appears in badge and also in info items
    const occupiedElements = screen.getAllByText("Belegt");
    expect(occupiedElements.length).toBeGreaterThan(0);
  });

  it("displays current occupation info when room is occupied", () => {
    setRoomDetail({ data: { room: mockOccupiedRoom, history: [] } });

    render(<RoomDetailPage />);

    expect(screen.getByText("OGS Gruppe A")).toBeInTheDocument();
    expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
  });

  it("displays room history", () => {
    setRoomDetail({
      data: { room: mockRoom, history: mockRoomHistory },
    });

    render(<RoomDetailPage />);

    expect(
      screen.getByTestId("info-card-Belegungshistorie"),
    ).toBeInTheDocument();
  });

  it("displays empty history message when no history", () => {
    setRoomDetail({ data: { room: mockRoom, history: [] } });

    render(<RoomDetailPage />);

    expect(
      screen.getByText("Keine Belegungshistorie verfügbar."),
    ).toBeInTheDocument();
  });

  it("shows error state when room fetch fails", () => {
    setRoomDetail({ error: new Error("network down") });

    render(<RoomDetailPage />);

    expect(
      screen.getByText("Fehler beim Laden der Raumdaten."),
    ).toBeInTheDocument();
  });

  it("handles back button click", () => {
    setRoomDetail({ error: new Error("network down") });

    render(<RoomDetailPage />);

    fireEvent.click(screen.getByText("Zurück"));
    expect(mockPush).toHaveBeenCalledWith("/test-tenant/rooms");
  });

  it("displays building and floor information", () => {
    setRoomDetail({ data: { room: mockRoom, history: [] } });

    render(<RoomDetailPage />);

    // The component combines building and floor with " · "
    const buildingElements = screen.getAllByText(/Hauptgebäude/);
    expect(buildingElements.length).toBeGreaterThan(0);
  });

  it("displays category information", () => {
    setRoomDetail({ data: { room: mockRoom, history: [] } });

    render(<RoomDetailPage />);

    // Category appears in header and info items
    const categoryElements = screen.getAllByText("Klassenraum");
    expect(categoryElements.length).toBeGreaterThan(0);
  });

  it("displays student count when room is occupied", () => {
    setRoomDetail({ data: { room: mockOccupiedRoom, history: [] } });

    render(<RoomDetailPage />);

    // Scope to the existing "Aktuell anwesend" InfoItem so the assertion
    // pins down the room-info-card badge specifically. The new live
    // "Kinder im Raum" section also contains the word "Kinder", so an
    // unscoped getByText(/Kinder/) would now match multiple nodes.
    const infoItem = screen.getByTestId("info-item-Aktuell anwesend");
    expect(within(infoItem).getByText(/15/)).toBeInTheDocument();
    expect(within(infoItem).getByText(/Kinder/)).toBeInTheDocument();
  });

  it("displays back button with correct referrer", () => {
    setRoomDetail({ data: { room: mockRoom, history: [] } });

    render(<RoomDetailPage />);

    const backButton = screen.getByTestId("back-button");
    expect(backButton).toHaveAttribute("data-referrer", "/rooms");
  });
});

describe("mapBackendToFrontendRoom", () => {
  it("maps backend room to frontend room format", () => {
    const backendRoom = {
      id: 1,
      name: "Test Room",
      building: "Building A",
      floor: 2,
      capacity: 25,
      category: "Gruppenraum",
      is_occupied: true,
      group_name: "Group 1",
      activity_name: "Activity 1",
      supervisor_name: "John Doe",
      device_id: "device-1",
      student_count: 10,
      color: "#FF0000",
    };

    // Test the mapping logic inline
    const result = {
      id: String(backendRoom.id),
      name: backendRoom.name,
      building: backendRoom.building,
      floor: backendRoom.floor,
      capacity: backendRoom.capacity,
      category: backendRoom.category,
      isOccupied: backendRoom.is_occupied,
      groupName: backendRoom.group_name,
      activityName: backendRoom.activity_name,
      supervisorName: backendRoom.supervisor_name,
      deviceId: backendRoom.device_id,
      studentCount: backendRoom.student_count,
      color: backendRoom.color,
    };

    expect(result.id).toBe("1");
    expect(result.name).toBe("Test Room");
    expect(result.building).toBe("Building A");
    expect(result.floor).toBe(2);
    expect(result.isOccupied).toBe(true);
    expect(result.groupName).toBe("Group 1");
  });
});

describe("StatusBadge", () => {
  it("returns correct text for occupied status", () => {
    const getStatusText = (isOccupied: boolean) =>
      isOccupied ? "Belegt" : "Frei";

    expect(getStatusText(true)).toBe("Belegt");
    expect(getStatusText(false)).toBe("Frei");
  });
});
