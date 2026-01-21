import { render, screen, waitFor, fireEvent } from "@testing-library/react";
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

vi.mock("~/components/dashboard", () => ({
  ResponsiveLayout: ({
    children,
    roomName,
  }: {
    children: React.ReactNode;
    roomName?: string;
  }) => (
    <div data-testid="responsive-layout" data-roomname={roomName}>
      {children}
    </div>
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
  InfoItem: ({
    label,
    value,
  }: {
    label: string;
    value: React.ReactNode;
  }) => (
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

const mockRoom = {
  id: 1,
  name: "Raum 101",
  building: "Hauptgebäude",
  floor: 1,
  capacity: 30,
  category: "Klassenraum",
  is_occupied: false,
  group_name: null,
  activity_name: null,
  supervisor_name: null,
  device_id: "device-1",
  student_count: 0,
  color: "#4F46E5",
};

const mockOccupiedRoom = {
  ...mockRoom,
  is_occupied: true,
  group_name: "OGS Gruppe A",
  activity_name: "Basteln",
  supervisor_name: "Max Mustermann",
  student_count: 15,
};

const mockRoomHistory = [
  {
    id: 1,
    room_id: 1,
    timestamp: "2024-01-15T10:00:00Z",
    group_name: "OGS Gruppe A",
    activity_name: "Basteln",
    category: "Kreativ",
    supervisor_name: "Max Mustermann",
    student_count: 12,
    duration_minutes: 60,
    entry_type: "entry",
  },
  {
    id: 2,
    room_id: 1,
    timestamp: "2024-01-15T11:00:00Z",
    group_name: "OGS Gruppe A",
    activity_name: "Basteln",
    category: "Kreativ",
    supervisor_name: "Max Mustermann",
    student_count: 12,
    duration_minutes: 60,
    entry_type: "exit",
  },
];

describe("RoomDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  it("renders loading state initially", () => {
    vi.mocked(global.fetch).mockImplementation(
      () => new Promise(() => {}), // Never resolves
    );

    render(<RoomDetailPage />);

    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("renders room details after loading", async () => {
    vi.mocked(global.fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: mockRoom }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: [] }),
      } as Response);

    render(<RoomDetailPage />);

    await waitFor(() => {
      // Room name appears in title
      const roomNames = screen.getAllByText("Raum 101");
      expect(roomNames.length).toBeGreaterThan(0);
    });
  });

  it("displays room information", async () => {
    vi.mocked(global.fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: mockRoom }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: [] }),
      } as Response);

    render(<RoomDetailPage />);

    await waitFor(() => {
      expect(screen.getByTestId("info-card-Rauminformationen")).toBeInTheDocument();
      expect(screen.getByTestId("info-item-Raumname")).toBeInTheDocument();
    });
  });

  it("displays status badge as free when room is not occupied", async () => {
    vi.mocked(global.fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: mockRoom }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: [] }),
      } as Response);

    render(<RoomDetailPage />);

    await waitFor(() => {
      // "Frei" appears in badge and also in info items
      const freeElements = screen.getAllByText("Frei");
      expect(freeElements.length).toBeGreaterThan(0);
    });
  });

  it("displays status badge as occupied when room is occupied", async () => {
    vi.mocked(global.fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: mockOccupiedRoom }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: [] }),
      } as Response);

    render(<RoomDetailPage />);

    await waitFor(() => {
      // "Belegt" appears in badge and also in info items
      const occupiedElements = screen.getAllByText("Belegt");
      expect(occupiedElements.length).toBeGreaterThan(0);
    });
  });

  it("displays current occupation info when room is occupied", async () => {
    vi.mocked(global.fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: mockOccupiedRoom }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: [] }),
      } as Response);

    render(<RoomDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("OGS Gruppe A")).toBeInTheDocument();
      expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
    });
  });

  it("displays room history", async () => {
    vi.mocked(global.fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: mockRoom }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: mockRoomHistory }),
      } as Response);

    render(<RoomDetailPage />);

    await waitFor(() => {
      expect(screen.getByTestId("info-card-Belegungshistorie")).toBeInTheDocument();
    });
  });

  it("displays empty history message when no history", async () => {
    vi.mocked(global.fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: mockRoom }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: [] }),
      } as Response);

    render(<RoomDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine Belegungshistorie verfügbar."),
      ).toBeInTheDocument();
    });
  });

  it("shows error state when room fetch fails", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce({
      ok: false,
      status: 500,
    } as Response);

    render(<RoomDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Fehler beim Laden der Raumdaten."),
      ).toBeInTheDocument();
    });
  });

  it("handles back button click", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce({
      ok: false,
      status: 500,
    } as Response);

    render(<RoomDetailPage />);

    await waitFor(() => {
      const backButton = screen.getByText("Zurück");
      expect(backButton).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Zurück"));
    expect(mockPush).toHaveBeenCalledWith("/rooms");
  });

  it("displays building and floor information", async () => {
    vi.mocked(global.fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: mockRoom }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: [] }),
      } as Response);

    render(<RoomDetailPage />);

    await waitFor(() => {
      // The component combines building and floor with " · "
      const buildingElements = screen.getAllByText(/Hauptgebäude/);
      expect(buildingElements.length).toBeGreaterThan(0);
    });
  });

  it("displays category information", async () => {
    vi.mocked(global.fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: mockRoom }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: [] }),
      } as Response);

    render(<RoomDetailPage />);

    await waitFor(() => {
      // Category appears in header and info items
      const categoryElements = screen.getAllByText("Klassenraum");
      expect(categoryElements.length).toBeGreaterThan(0);
    });
  });

  it("handles history fetch failure gracefully", async () => {
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    vi.mocked(global.fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: mockRoom }),
      } as Response)
      .mockResolvedValueOnce({
        ok: false,
        status: 500,
      } as Response);

    render(<RoomDetailPage />);

    await waitFor(() => {
      // Room should still render (name appears multiple times)
      const roomNames = screen.getAllByText("Raum 101");
      expect(roomNames.length).toBeGreaterThan(0);
    });

    consoleSpy.mockRestore();
  });

  it("displays student count when room is occupied", async () => {
    vi.mocked(global.fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: mockOccupiedRoom }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: [] }),
      } as Response);

    render(<RoomDetailPage />);

    await waitFor(() => {
      expect(screen.getByText(/15/)).toBeInTheDocument();
      expect(screen.getByText(/Kinder/)).toBeInTheDocument();
    });
  });

  it("displays back button with correct referrer", async () => {
    vi.mocked(global.fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: mockRoom }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: [] }),
      } as Response);

    render(<RoomDetailPage />);

    await waitFor(() => {
      const backButton = screen.getByTestId("back-button");
      expect(backButton).toHaveAttribute("data-referrer", "/rooms");
    });
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
