import {
  render as rtlRender,
  screen,
  fireEvent,
  waitFor,
} from "@testing-library/react";
import type { ReactElement } from "react";
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
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

function render(ui: ReactElement) {
  return rtlRender(ui, { wrapper: ToastProvider });
}

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

  it("links each room tile to the room page with the filters as referrer", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    // Ohne Filter führt der Rückweg schlicht auf die Übersicht. Der Link
    // trägt den Mandanten-Slug, der Rückweg bleibt slug-frei.
    expect(screen.getByRole("link", { name: "Raum 101" })).toHaveAttribute(
      "href",
      `/test-tenant/rooms/1?from=${encodeURIComponent("/rooms")}`,
    );

    fireEvent.click(screen.getByTestId("filter-building"));

    await waitFor(() => {
      expect(screen.getByRole("link", { name: "Raum 101" })).toHaveAttribute(
        "href",
        `/test-tenant/rooms/1?from=${encodeURIComponent("/rooms?building=Main")}`,
      );
    });
    // Öffnen ist ein Link, kein Push: Mittelklick und „in neuem Tab" gehen.
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("shows the catalog load error with retry when rooms fetch fails", async () => {
    const mutate = vi.fn();
    vi.mocked(useSWRAuth).mockReturnValue({
      data: [],
      isLoading: false,
      error: new ApiError("down", 503, { code: "general.unavailable" }),
      mutate,
    } as never);

    render(<RoomsPage />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Räume"),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(mutate).toHaveBeenCalled();
  });

  it("shows a failed export as a toast with retry", async () => {
    mockExportRoomSnapshot.mockRejectedValueOnce(
      new ApiError("boom", 500, { code: "general.server" }),
    );
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    fireEvent.click(
      screen.getByRole("menuitem", { name: /Wer ist wo als Excel/ }),
    );

    expect(
      await screen.findByText(
        catalogText("general.server", "die Liste „Wer ist wo“"),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => {
      expect(mockExportRoomSnapshot).toHaveBeenCalledTimes(2);
    });
    expect(mockExportRoomSnapshot).toHaveBeenLastCalledWith(
      expect.objectContaining({ format: "xlsx" }),
    );
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
    // Das Exportmenü hängt seit der Kopfkarten-Umstellung an der PageIntro
    // (echte OverflowMenu), nicht mehr am gemockten Seitenkopf.
    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    fireEvent.click(
      screen.getByRole("menuitem", { name: /Wer ist wo als PDF/ }),
    );

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

  it("links the transit work list to its own page with the active filters", async () => {
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

    expect(screen.getByRole("link", { name: /Unterwegs/i })).toHaveAttribute(
      "href",
      `/test-tenant/rooms/unterwegs?from=${encodeURIComponent("/rooms")}`,
    );

    fireEvent.click(screen.getByTestId("filter-building"));

    await waitFor(() => {
      expect(screen.getByRole("link", { name: /Unterwegs/i })).toHaveAttribute(
        "href",
        `/test-tenant/rooms/unterwegs?from=${encodeURIComponent("/rooms?building=Main")}`,
      );
    });
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

  it("traegt keine Tipp-Hinweiszeile mehr auf den Raumkacheln", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    // Die Zeile stand auf jeder Kachel, war am Rechner falsch („Tippen") und
    // sagte nichts, was die Kachel nicht schon zeigt.
    expect(screen.queryByText("Tippen für mehr Infos")).not.toBeInTheDocument();
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
