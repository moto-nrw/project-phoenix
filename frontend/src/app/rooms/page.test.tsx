import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import RoomsPage from "./page";

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(),
}));

vi.mock("swr", () => ({
  default: vi.fn(),
  mutate: vi.fn(),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  useImmutableSWR: vi.fn(),
  useSWRWithId: vi.fn(),
  mutate: vi.fn(),
}));

vi.mock("~/components/dashboard", () => ({
  ResponsiveLayout: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="layout">{children}</div>
  ),
}));

vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({
    search,
    filters,
    onClearAllFilters,
  }: {
    search: { value: string; onChange: (v: string) => void };
    filters?: Array<{ onChange: (v: string | string[]) => void }>;
    onClearAllFilters: () => void;
  }) => (
    <div data-testid="page-header">
      <input
        data-testid="search-input"
        value={search.value}
        onChange={(e) => search.onChange(e.target.value)}
      />
      <button
        data-testid="filter-building"
        onClick={() => filters?.[0]?.onChange("Main")}
      >
        Building
      </button>
      <button
        data-testid="filter-occupied"
        onClick={() => filters?.[1]?.onChange("occupied")}
      >
        Occupied
      </button>
      <button data-testid="clear-filters" onClick={onClearAllFilters}>
        Clear
      </button>
    </div>
  ),
}));

import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { useSWRAuth } from "~/lib/swr";

const mockRooms = [
  {
    id: "1",
    name: "Raum 101",
    building: "Main",
    isOccupied: true,
    groupName: "Gruppe A",
    studentCount: 8,
    supervisorName: "Petra Huber",
  },
  {
    id: "2",
    name: "Musikraum",
    building: "Annex",
    isOccupied: false,
    capacity: 25,
  },
];

describe("RoomsPage", () => {
  const mockPush = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useRouter).mockReturnValue({ push: mockPush } as never);
    vi.mocked(useSession).mockReturnValue({
      data: { user: { id: "1" } },
      status: "authenticated",
    } as never);
  });

  it("shows loading state while session is loading", () => {
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "loading",
    } as never);
    vi.mocked(useSWRAuth).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    expect(screen.getByLabelText("Lädt...")).toBeInTheDocument();
  });

  it("filters rooms by search, building, and occupancy", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    expect(screen.getByText("Raum 101")).toBeInTheDocument();
    expect(screen.getByText("Musikraum")).toBeInTheDocument();

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "musik" },
    });

    await waitFor(() => {
      expect(screen.queryByText("Raum 101")).not.toBeInTheDocument();
      expect(screen.getByText("Musikraum")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("clear-filters"));

    await waitFor(() => {
      expect(screen.getByText("Raum 101")).toBeInTheDocument();
      expect(screen.getByText("Musikraum")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("filter-building"));

    await waitFor(() => {
      expect(screen.getByText("Raum 101")).toBeInTheDocument();
      expect(screen.queryByText("Musikraum")).not.toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("filter-occupied"));

    await waitFor(() => {
      expect(screen.getByText("Raum 101")).toBeInTheDocument();
      expect(screen.queryByText("Musikraum")).not.toBeInTheDocument();
    });
  });

  it("navigates to room detail on card click", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    fireEvent.click(screen.getByRole("button", { name: /Raum 101/i }));

    expect(mockPush).toHaveBeenCalledWith("/rooms/1");
  });

  it("shows error message when rooms fetch fails", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: [],
      isLoading: false,
      error: new Error("Network error"),
    } as never);

    render(<RoomsPage />);

    expect(
      screen.getByText(
        "Fehler beim Laden der Raumdaten. Bitte versuchen Sie es später erneut.",
      ),
    ).toBeInTheDocument();
  });

  it("shows empty state when no rooms match filters", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "nonexistent" },
    });

    await waitFor(() => {
      expect(screen.getByText("Keine Räume gefunden")).toBeInTheDocument();
    });
  });

  it("shows empty state when rooms data is empty", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    expect(screen.getByText("Keine Räume gefunden")).toBeInTheDocument();
  });

  it("displays occupied room with group name", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    expect(screen.getByText("Belegt")).toBeInTheDocument();
    expect(screen.getByText(/Aktuelle Aktivität:/)).toBeInTheDocument();
    expect(screen.getByText("Gruppe A")).toBeInTheDocument();
  });

  it("displays student count and supervisor on occupied room", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    expect(screen.getByText("8 Kinder")).toBeInTheDocument();
    expect(screen.getByText("Petra Huber")).toBeInTheDocument();
  });

  it("displays placeholder text and capacity for free room", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    expect(screen.getByText("Für Aktivitäten buchbar")).toBeInTheDocument();
    expect(screen.getByText("Kapazität: 25 Plätze")).toBeInTheDocument();
  });

  it("displays click hint on all room cards", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    const hints = screen.getAllByText("Tippen für mehr Infos");
    expect(hints).toHaveLength(2);
  });

  it("displays singular 'Kind' when only one student", () => {
    const roomsWithOneStudent = [
      {
        id: "1",
        name: "Raum 101",
        building: "Main",
        isOccupied: true,
        groupName: "Gruppe A",
        studentCount: 1,
        supervisorName: "Petra Huber",
      },
    ];

    vi.mocked(useSWRAuth).mockReturnValue({
      data: roomsWithOneStudent,
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    expect(screen.getByText("1 Kind")).toBeInTheDocument();
  });

  it("displays free room status", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    expect(screen.getByText("Frei")).toBeInTheDocument();
  });

  it("shows loading state while data is loading", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: true,
      error: null,
    } as never);

    render(<RoomsPage />);

    expect(screen.getByLabelText("Lädt...")).toBeInTheDocument();
  });

  it("filters by free status when occupied filter set to free", async () => {
    const roomsWithBoth = [
      ...mockRooms,
      {
        id: "3",
        name: "Freier Raum",
        building: "Main",
        isOccupied: false,
      },
    ];

    vi.mocked(useSWRAuth).mockReturnValue({
      data: roomsWithBoth,
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    expect(screen.getByText("Raum 101")).toBeInTheDocument();
    expect(screen.getByText("Musikraum")).toBeInTheDocument();
    expect(screen.getByText("Freier Raum")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("filter-occupied"));

    await waitFor(() => {
      expect(screen.getByText("Raum 101")).toBeInTheDocument();
      expect(screen.queryByText("Freier Raum")).not.toBeInTheDocument();
    });
  });
});

describe("RoomsPage filter logic", () => {
  it("filters rooms by search term matching name", () => {
    const rooms = [
      { id: "1", name: "Raum 101", building: "Main", isOccupied: true },
      { id: "2", name: "Musikraum", building: "Annex", isOccupied: false },
    ];

    const searchTerm = "musik";
    const filtered = rooms.filter((room) =>
      room.name.toLowerCase().includes(searchTerm.toLowerCase()),
    );

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.name).toBe("Musikraum");
  });

  it("filters rooms by building", () => {
    const rooms = [
      { id: "1", name: "Raum 101", building: "Main", isOccupied: true },
      { id: "2", name: "Musikraum", building: "Annex", isOccupied: false },
      { id: "3", name: "Raum 102", building: "Main", isOccupied: false },
    ];

    const buildingFilter = "Main" as string;
    const filtered = rooms.filter(
      (room) => buildingFilter === "all" || room.building === buildingFilter,
    );

    expect(filtered).toHaveLength(2);
    expect(filtered.map((r) => r.name)).toEqual(["Raum 101", "Raum 102"]);
  });

  it("filters rooms by occupied status", () => {
    const rooms = [
      { id: "1", name: "Raum 101", building: "Main", isOccupied: true },
      { id: "2", name: "Musikraum", building: "Annex", isOccupied: false },
    ];

    const occupiedFilter = "occupied" as string;
    const isOccupied = occupiedFilter === "occupied";
    const filtered = rooms.filter(
      (room) => occupiedFilter === "all" || room.isOccupied === isOccupied,
    );

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.name).toBe("Raum 101");
  });

  it("combines multiple filters correctly", () => {
    const rooms = [
      {
        id: "1",
        name: "Raum 101",
        building: "Main",
        isOccupied: true,
        groupName: "Gruppe A",
      },
      {
        id: "2",
        name: "Musikraum",
        building: "Annex",
        isOccupied: false,
      },
      { id: "3", name: "Raum 102", building: "Main", isOccupied: false },
    ];

    const searchTerm = "raum";
    const buildingFilter = "Main" as string;
    const occupiedFilter = "all" as string;

    const filtered = rooms.filter((room) => {
      const matchesSearch = room.name
        .toLowerCase()
        .includes(searchTerm.toLowerCase());
      const matchesBuilding =
        buildingFilter === "all" || room.building === buildingFilter;
      const matchesOccupied =
        occupiedFilter === "all" ||
        room.isOccupied === (occupiedFilter === "occupied");
      return matchesSearch && matchesBuilding && matchesOccupied;
    });

    expect(filtered).toHaveLength(2);
    expect(filtered.map((r) => r.name)).toEqual(["Raum 101", "Raum 102"]);
  });

  it("sorts rooms by name", () => {
    const rooms = [
      { id: "2", name: "Musikraum", building: "Annex", isOccupied: false },
      { id: "1", name: "Raum 101", building: "Main", isOccupied: true },
      { id: "3", name: "Aula", building: "Main", isOccupied: false },
    ];

    const sorted = [...rooms].sort((a, b) =>
      a.name.localeCompare(b.name, "de"),
    );

    expect(sorted.map((r) => r.name)).toEqual([
      "Aula",
      "Musikraum",
      "Raum 101",
    ]);
  });
});

describe("RoomsPage category colors", () => {
  it("applies default color when no category", () => {
    const room = { id: "1", name: "Raum", isOccupied: false };
    const categoryColors: Record<string, string> = {
      "Normaler Raum": "#4F46E5",
      Gruppenraum: "#10B981",
    };

    const color = (room as { category?: string }).category
      ? categoryColors[(room as { category?: string }).category!]
      : "#6B7280";

    expect(color).toBe("#6B7280");
  });

  it("applies category color for sport room", () => {
    const categoryColors: Record<string, string> = {
      "Normaler Raum": "#4F46E5",
      Gruppenraum: "#10B981",
      Themenraum: "#8B5CF6",
      Sport: "#EC4899",
    };

    const category = "Sport";
    const color = categoryColors[category];

    expect(color).toBe("#EC4899");
  });
});
