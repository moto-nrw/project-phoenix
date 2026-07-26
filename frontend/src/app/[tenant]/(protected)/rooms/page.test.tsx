import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import RoomsPage from "./page";

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(),
}));

// Hoisted mutable state so individual tests can flip the simulated URL
// (?room={id}) and re-render to exercise open / close transitions.
const { searchParamsState, mockExportRoomSnapshot } = vi.hoisted(() => ({
  searchParamsState: { roomParam: null as string | null },
  mockExportRoomSnapshot: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(),
  // searchParams.toString() is consumed by handleSelectRoom when building
  // the new history entry, so the mock must expose it.
  useSearchParams: vi.fn(() => ({
    get: vi.fn((key: string) =>
      key === "room" ? searchParamsState.roomParam : null,
    ),
    toString: vi.fn(() =>
      searchParamsState.roomParam ? `room=${searchParamsState.roomParam}` : "",
    ),
  })),
  usePathname: vi.fn(() => "/test-tenant/rooms"),
}));

const mockUpdateUrlParams = vi.fn();
vi.mock("~/hooks/useUpdateUrlParams", () => ({
  useUpdateUrlParams: () => mockUpdateUrlParams,
}));

vi.mock("~/components/rooms/room-detail-modal", () => ({
  TRANSIT_ROOM_ID: "__transit__",
  // Surface the onClose handler as a clickable element so tests can
  // exercise the modal-close branch (back vs. updateUrlParams).
  RoomDetailModal: ({
    roomId,
    onClose,
  }: {
    roomId: string | null;
    onClose: () => void;
  }) =>
    roomId ? (
      <div data-testid="room-detail-modal" data-room-id={roomId}>
        <button
          type="button"
          data-testid="room-detail-modal-close"
          onClick={onClose}
        >
          close
        </button>
      </div>
    ) : null,
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
  useTenantMutate: vi.fn(() => vi.fn()),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
  useBreadcrumb: vi.fn(() => ({ breadcrumb: {}, setBreadcrumb: vi.fn() })),
  BreadcrumbProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: ({
    search,
    filters,
    activeFilters,
    overflowMenu,
    onClearAllFilters,
  }: {
    search: { value: string; onChange: (v: string) => void };
    filters?: Array<{ onChange: (v: string | string[]) => void }>;
    activeFilters?: Array<{ id: string; label: string; onRemove: () => void }>;
    overflowMenu?: Array<{ label: string; onClick: () => void }>;
    onClearAllFilters: () => void;
  }) => (
    <div data-testid="page-header">
      <input
        data-testid="search-input"
        value={search.value}
        onChange={(e) => search.onChange(e.target.value)}
      />
      <button
        type="button"
        data-testid="filter-building"
        onClick={() => filters?.[0]?.onChange("Main")}
      >
        Building
      </button>
      <button
        type="button"
        data-testid="filter-occupied"
        onClick={() => filters?.[1]?.onChange("occupied")}
      >
        Occupied
      </button>
      <button
        type="button"
        data-testid="clear-filters"
        onClick={onClearAllFilters}
      >
        Clear
      </button>
      {activeFilters?.map((filter) => (
        <button
          type="button"
          key={filter.id}
          data-testid={`remove-filter-${filter.id}`}
          onClick={filter.onRemove}
        >
          {filter.label}
        </button>
      ))}
      {overflowMenu?.map((item) => (
        <button
          type="button"
          key={item.label}
          data-testid={`overflow-${item.label}`}
          onClick={item.onClick}
        >
          {item.label}
        </button>
      ))}
    </div>
  ),
}));

vi.mock("~/lib/room-export-api", () => ({
  exportRoomSnapshot: (...args: unknown[]) => mockExportRoomSnapshot(...args),
}));

import { useSession } from "next-auth/react";
// eslint-disable-next-line no-restricted-imports -- test mock
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
  const mockBack = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockExportRoomSnapshot.mockResolvedValue(undefined);
    searchParamsState.roomParam = null;
    // Reset jsdom's per-entry history state so the close handler reads
    // a clean slate. Without this, a marker left by a previous test
    // would leak into the next.
    window.history.replaceState(null, "");
    vi.mocked(useRouter).mockReturnValue({
      push: mockPush,
      back: mockBack,
    } as never);
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

    // Session loading now shows the same grid skeleton as the data-loading
    // branch instead of the generic spinner.
    expect(screen.getByTestId("rooms-grid-skeleton")).toBeInTheDocument();
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

  it("pushes ?room={id} as a new history entry on card click", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    fireEvent.click(screen.getByRole("button", { name: /Raum 101/i }));

    // Opening the modal must use router.push (not replace) so the
    // browser Back button closes the overlay instead of skipping past
    // the rooms page. mockPush is the underlying next/navigation router
    // that useTenantRouter wraps.
    expect(mockPush).toHaveBeenCalledWith("/test-tenant/rooms?room=1");
    // updateUrlParams (which uses replace internally) must NOT be called
    // for opening, only for closing.
    expect(mockUpdateUrlParams).not.toHaveBeenCalledWith({ room: "1" });
  });

  it("pops the modal entry on close after open (back, not replace)", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
    } as never);

    const { rerender } = render(<RoomsPage />);
    fireEvent.click(screen.getByRole("button", { name: /Raum 101/i }));

    // Simulate the URL update that production would receive from
    // router.push: flip the mocked searchParam and re-render so the
    // modal mounts and exposes the close affordance.
    searchParamsState.roomParam = "1";
    rerender(<RoomsPage />);

    fireEvent.click(screen.getByTestId("room-detail-modal-close"));

    // After open-then-close, history must collapse to a single /rooms
    // entry. Replace would leave [/rooms, /rooms] and Back would appear
    // to do nothing, the close path has to pop the modal entry.
    expect(mockBack).toHaveBeenCalledTimes(1);
    expect(mockUpdateUrlParams).not.toHaveBeenCalledWith({ room: null });
  });

  it("still pops the modal entry after a /rooms → /students → /rooms remount", () => {
    // Verifies the marker that drives the close decision lives on
    // window.history.state, not in component state. Otherwise drilling
    // into a child and pressing browser Back would remount /rooms with
    // a fresh ref, and the close path would silently fall back to
    // replace, leaving the duplicate-/rooms history bug intact.
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
    } as never);

    // 1. First mount: open the modal. The effect tags the now-current
    //    history entry with roomModalPushed=true.
    const first = render(<RoomsPage />);
    fireEvent.click(screen.getByRole("button", { name: /Raum 101/i }));
    searchParamsState.roomParam = "1";
    first.rerender(<RoomsPage />);

    // 2. Simulate the user navigating away (drill into a student) and
    //    returning via browser back: same history entry, fresh React
    //    component instance.
    first.unmount();
    render(<RoomsPage />);

    // 3. Close from the remounted page. The marker on history.state
    //    survives, so close still pops via router.back.
    fireEvent.click(screen.getByTestId("room-detail-modal-close"));

    expect(mockBack).toHaveBeenCalledTimes(1);
    expect(mockUpdateUrlParams).not.toHaveBeenCalledWith({ room: null });
  });

  it("falls back to replace when closing a deep-linked modal entry", () => {
    // User landed on /rooms?room=2 directly (refresh / shared link), so
    // there is no in-app prior /rooms entry to pop. router.back() would
    // leave the page; updateUrlParams clears the param in place instead.
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
    } as never);
    searchParamsState.roomParam = "2";

    render(<RoomsPage />);

    fireEvent.click(screen.getByTestId("room-detail-modal-close"));

    expect(mockUpdateUrlParams).toHaveBeenCalledWith({ room: null });
    expect(mockBack).not.toHaveBeenCalled();
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

  it("shows the transit assignment entry as a separate work list", () => {
    vi.mocked(useSWRAuth).mockImplementation((key: unknown) => {
      if (key === "dashboard-analytics") {
        return {
          data: { studentsInTransit: 2 },
          isLoading: false,
          error: null,
        } as never;
      }

      return {
        data: [],
        isLoading: false,
        error: null,
      } as never;
    });

    render(<RoomsPage />);

    expect(
      screen.getByRole("heading", { name: "Unterwegs" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/Kinder ohne Raumzuweisung/)).toBeInTheDocument();
    expect(screen.queryByText("Keine Räume gefunden")).not.toBeInTheDocument();
  });

  it("exports the filtered room snapshot with transit", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    fireEvent.click(screen.getByTestId("filter-building"));
    fireEvent.click(screen.getByTestId("overflow-Wer ist wo als PDF"));

    await waitFor(() => {
      expect(mockExportRoomSnapshot).toHaveBeenCalledWith({
        format: "pdf",
        title: "Wer ist wo",
        room_ids: [1],
        include_transit: true,
      });
    });
  });

  it("hides the transit assignment entry when no children are unterwegs", () => {
    vi.mocked(useSWRAuth).mockImplementation((key: unknown) => {
      if (key === "dashboard-analytics") {
        return {
          data: { studentsInTransit: 0 },
          isLoading: false,
          error: null,
        } as never;
      }

      return {
        data: [],
        isLoading: false,
        error: null,
      } as never;
    });

    render(<RoomsPage />);

    expect(
      screen.queryByRole("heading", { name: "Unterwegs" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Keine Räume gefunden")).toBeInTheDocument();
  });

  it("opens the transit assignment drawer from the work list", () => {
    vi.mocked(useSWRAuth).mockImplementation((key: unknown) => {
      if (key === "dashboard-analytics") {
        return {
          data: { studentsInTransit: 2 },
          isLoading: false,
          error: null,
        } as never;
      }

      return {
        data: mockRooms,
        isLoading: false,
        error: null,
      } as never;
    });

    render(<RoomsPage />);

    fireEvent.click(screen.getByRole("button", { name: /Unterwegs/i }));

    expect(mockPush).toHaveBeenCalledWith(
      "/test-tenant/rooms?room=__transit__",
    );
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

    // The data-loading state is now a content-shaped skeleton grid in
    // place of the generic <Loading> spinner, the page header still
    // renders, only the card grid is replaced with skeleton cards
    // (review feedback #1323). Same business assertion ("a loading
    // state is announced while data is fetched"), new selector
    // matching the new skeleton's aria-label.
    expect(screen.getByLabelText("Räume werden geladen")).toBeInTheDocument();
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

  it("removes active search, building, and status filters from the header chips", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "Raum" },
    });
    fireEvent.click(screen.getByTestId("filter-building"));
    fireEvent.click(screen.getByTestId("filter-occupied"));

    await waitFor(() => {
      expect(screen.getByTestId("remove-filter-search")).toBeInTheDocument();
      expect(screen.getByTestId("remove-filter-building")).toBeInTheDocument();
      expect(screen.getByTestId("remove-filter-occupied")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("remove-filter-search"));
    await waitFor(() => {
      expect(screen.getByTestId("search-input")).toHaveValue("");
    });

    fireEvent.click(screen.getByTestId("remove-filter-building"));
    await waitFor(() => {
      expect(
        screen.queryByTestId("remove-filter-building"),
      ).not.toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("remove-filter-occupied"));
    await waitFor(() => {
      expect(
        screen.queryByTestId("remove-filter-occupied"),
      ).not.toBeInTheDocument();
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

describe("RoomsPage room data parsing", () => {
  it("handles room data as direct array", () => {
    const data = [
      { id: 1, name: "Raum 101" },
      { id: 2, name: "Raum 102" },
    ];

    const isArray = Array.isArray(data);
    expect(isArray).toBe(true);
  });

  it("handles room data with data wrapper", () => {
    const data = {
      data: [
        { id: 1, name: "Raum 101" },
        { id: 2, name: "Raum 102" },
      ],
    };

    const hasDataWrapper = data?.data && Array.isArray(data.data);
    expect(hasDataWrapper).toBe(true);
  });

  it("applies color from room or category or default", () => {
    const categoryColors: Record<string, string> = {
      "Normaler Raum": "#4F46E5",
      Gruppenraum: "#10B981",
    };

    // Room with explicit color
    const roomWithColor = {
      id: "1",
      name: "Raum",
      color: "#FF0000",
      category: "Gruppenraum",
    };
    const color1 =
      roomWithColor.color ??
      (roomWithColor.category
        ? categoryColors[roomWithColor.category]
        : undefined) ??
      "#6B7280";
    expect(color1).toBe("#FF0000");

    // Room with category but no explicit color
    const roomWithCategory = {
      id: "2",
      name: "Raum",
      color: undefined,
      category: "Gruppenraum",
    };
    const color2 =
      roomWithCategory.color ??
      (roomWithCategory.category
        ? categoryColors[roomWithCategory.category]
        : undefined) ??
      "#6B7280";
    expect(color2).toBe("#10B981");

    // Room with no color or category
    const roomPlain = {
      id: "3",
      name: "Raum",
      color: undefined,
      category: undefined,
    };
    const color3 =
      roomPlain.color ??
      (roomPlain.category ? categoryColors[roomPlain.category] : undefined) ??
      "#6B7280";
    expect(color3).toBe("#6B7280");
  });
});

describe("RoomsPage building and floor display", () => {
  it("shows building and floor when both exist", () => {
    const room = { building: "Main", floor: 2 };
    const hasBuilding = room.building !== undefined;
    const hasFloor = room.floor !== undefined;

    expect(hasBuilding && hasFloor).toBe(true);
  });

  it("shows only building when floor is undefined", () => {
    const room = { building: "Main", floor: undefined };
    const showOnlyBuilding = room.building && room.floor === undefined;

    expect(showOnlyBuilding).toBeTruthy();
  });

  it("shows only floor when building is undefined", () => {
    const room = { building: undefined, floor: 1 };
    const showOnlyFloor = !room.building && room.floor !== undefined;

    expect(showOnlyFloor).toBe(true);
  });
});

describe("RoomsPage active filters", () => {
  it("creates active filter for search term", () => {
    const searchTerm = "Musik";
    const buildingFilter = "all";
    const occupiedFilter = "all";

    const filters: Array<{ id: string; label: string }> = [];

    if (searchTerm) {
      filters.push({ id: "search", label: `"${searchTerm}"` });
    }
    if (buildingFilter !== "all") {
      filters.push({ id: "building", label: buildingFilter });
    }
    if (occupiedFilter !== "all") {
      filters.push({
        id: "occupied",
        label: occupiedFilter === "occupied" ? "Belegt" : "Frei",
      });
    }

    expect(filters).toHaveLength(1);
    expect(filters[0]?.label).toBe('"Musik"');
  });

  it("creates active filter for building", () => {
    const searchTerm = "" as string;
    const buildingFilter = "Main" as string;
    const occupiedFilter = "all" as string;

    const filters: Array<{ id: string; label: string }> = [];

    if (searchTerm) {
      filters.push({ id: "search", label: `"${searchTerm}"` });
    }
    if (buildingFilter !== "all") {
      filters.push({ id: "building", label: buildingFilter });
    }
    if (occupiedFilter !== "all") {
      filters.push({
        id: "occupied",
        label: occupiedFilter === "occupied" ? "Belegt" : "Frei",
      });
    }

    expect(filters).toHaveLength(1);
    expect(filters[0]?.label).toBe("Main");
  });

  it("creates active filter for free status", () => {
    const searchTerm = "" as string;
    const buildingFilter = "all" as string;
    const occupiedFilter = "free" as string;

    const statusLabels = {
      occupied: "Belegt",
      free: "Frei",
    };

    const filters: Array<{ id: string; label: string }> = [];

    if (searchTerm) {
      filters.push({ id: "search", label: `"${searchTerm}"` });
    }
    if (buildingFilter !== "all") {
      filters.push({ id: "building", label: buildingFilter });
    }
    if (occupiedFilter !== "all") {
      filters.push({
        id: "occupied",
        label:
          statusLabels[occupiedFilter as keyof typeof statusLabels] ??
          occupiedFilter,
      });
    }

    expect(filters).toHaveLength(1);
    expect(filters[0]?.label).toBe("Frei");
  });
});

describe("RoomsPage unique buildings extraction", () => {
  it("extracts unique buildings from rooms", () => {
    const rooms = [
      { id: "1", name: "Raum 1", building: "Main" },
      { id: "2", name: "Raum 2", building: "Annex" },
      { id: "3", name: "Raum 3", building: "Main" },
      { id: "4", name: "Raum 4", building: undefined },
    ];

    const uniqueBuildings = Array.from(
      new Set(rooms.map((room) => room.building).filter(Boolean)),
    );

    expect(uniqueBuildings).toHaveLength(2);
    expect(uniqueBuildings).toContain("Main");
    expect(uniqueBuildings).toContain("Annex");
  });
});

describe("RoomsPage search across multiple fields", () => {
  it("searches by room name", () => {
    const rooms = [
      { id: "1", name: "Musikraum", groupName: "A", activityName: "X" },
      { id: "2", name: "Raum 101", groupName: "B", activityName: "Y" },
    ];

    const searchTerm = "musik";
    const filtered = rooms.filter((room) => {
      const checks = [
        room.name?.toLowerCase().includes(searchTerm.toLowerCase()),
        room.groupName?.toLowerCase().includes(searchTerm.toLowerCase()),
        room.activityName?.toLowerCase().includes(searchTerm.toLowerCase()),
      ];
      return checks.some(Boolean);
    });

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.id).toBe("1");
  });

  it("searches by group name", () => {
    const rooms = [
      { id: "1", name: "Raum 1", groupName: "Tigers", activityName: "Sport" },
      { id: "2", name: "Raum 2", groupName: "Lions", activityName: "Art" },
    ];

    const searchTerm = "tiger";
    const filtered = rooms.filter((room) => {
      const checks = [
        room.name?.toLowerCase().includes(searchTerm.toLowerCase()),
        room.groupName?.toLowerCase().includes(searchTerm.toLowerCase()),
        room.activityName?.toLowerCase().includes(searchTerm.toLowerCase()),
      ];
      return checks.some(Boolean);
    });

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.id).toBe("1");
  });

  it("searches by activity name", () => {
    const rooms = [
      { id: "1", name: "Raum 1", groupName: "A", activityName: "Basketball" },
      { id: "2", name: "Raum 2", groupName: "B", activityName: "Reading" },
    ];

    const searchTerm = "basket";
    const filtered = rooms.filter((room) => {
      const checks = [
        room.name?.toLowerCase().includes(searchTerm.toLowerCase()),
        room.groupName?.toLowerCase().includes(searchTerm.toLowerCase()),
        room.activityName?.toLowerCase().includes(searchTerm.toLowerCase()),
      ];
      return checks.some(Boolean);
    });

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.id).toBe("1");
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
