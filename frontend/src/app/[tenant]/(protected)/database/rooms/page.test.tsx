import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useState, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import RoomsPage from "./page";

const mockTenantMutate = vi.hoisted(() => vi.fn(() => Promise.resolve()));
const mockRefreshRoomConsumers = vi.hoisted(() =>
  vi.fn(() => Promise.resolve()),
);

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { id: "1", token: "test-token" }, expires: "2099-01-01" },
    status: "authenticated",
  })),
}));

let currentSearch = new URLSearchParams();
const mockReplace = vi.fn((url: string) => {
  const query = url.includes("?") ? (url.split("?")[1] ?? "") : "";
  currentSearch = new URLSearchParams(query);
});
const setSelectedRoom = (id: string | null) => {
  currentSearch = new URLSearchParams();
  if (id) {
    currentSearch.set("room", id);
  }
};

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: vi.fn(() => ({ push: vi.fn(), replace: mockReplace })),
  usePathname: vi.fn(() => "/tenant/database/rooms"),
  useSearchParams: () => currentSearch,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  mutate: vi.fn(),
  useTenantMutate: vi.fn(() => mockTenantMutate),
  // Added with Issue #1324 — page invalidates badge consumer caches after a
  // room save. Tests only assert that the returned refresher is called.
  useTenantMutateMatching: vi.fn(() => mockRefreshRoomConsumers),
}));

const mockGetOne = vi.fn();
const mockCreate = vi.fn();
const mockUpdate = vi.fn();
const mockDelete = vi.fn();
vi.mock("@/lib/database/service-factory", () => ({
  createCrudService: vi.fn(() => ({
    getList: vi.fn(),
    getOne: mockGetOne,
    create: mockCreate,
    update: mockUpdate,
    delete: mockDelete,
  })),
}));

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

const mockToastSuccess = vi.fn();
const mockToastError = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
  useToast: vi.fn(() => ({
    success: mockToastSuccess,
    error: mockToastError,
  })),
}));

vi.mock("~/components/database/database-page-layout", () => ({
  DatabasePageLayout: ({
    children,
    loading,
    intro,
  }: {
    children: ReactNode;
    loading: boolean;
    intro?: { title: string; actions?: ReactNode };
  }) => (
    <div data-testid="database-layout" data-loading={loading}>
      {intro ? (
        <div data-testid="page-intro">
          <h1>{intro.title}</h1>
          {intro.actions}
        </div>
      ) : null}
      {children}
    </div>
  ),
}));

vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: ({
    search,
    filters,
    onClearAllFilters,
    actionButton,
  }: {
    search: { value: string; onChange: (value: string) => void };
    filters: Array<{
      id: string;
      value: string;
      onChange: (value: string) => void;
      options?: Array<{ value: string; label: string }>;
    }>;
    onClearAllFilters: () => void;
    actionButton?: ReactNode;
  }) => (
    <div data-testid="page-header">
      <input
        data-testid="search-input"
        value={search.value}
        onChange={(event) => search.onChange(event.target.value)}
      />
      {filters.map((filter) => (
        <select
          key={filter.id}
          data-testid={`filter-${filter.id}`}
          value={filter.value}
          onChange={(event) => filter.onChange(event.target.value)}
        >
          {filter.options?.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
      ))}
      <button
        type="button"
        data-testid="clear-filters"
        onClick={onClearAllFilters}
      >
        Clear
      </button>
      {actionButton}
    </div>
  ),
}));

vi.mock("~/components/ui/database/database-form-modal", () => ({
  DatabaseFormModal: ({
    isOpen,
    onClose,
    onSubmit,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSubmit: (data: { name: string }) => Promise<void>;
  }) => {
    // Mirrors DatabaseForm: catches the rejection from onSubmit and renders
    // the message inline. Tests assert against the resulting message.
    const [error, setError] = useState<string | null>(null);
    const submit = (data: { name: string }) => {
      setError(null);
      void onSubmit(data).catch((err: unknown) => {
        setError(err instanceof Error ? err.message : String(err));
      });
    };
    return isOpen ? (
      <div data-testid="room-create-modal">
        {error ? <span data-testid="create-error">{error}</span> : null}
        <button
          type="button"
          data-testid="submit-create"
          onClick={() => submit({ name: "Neuer Raum" })}
        >
          Submit
        </button>
        <button
          type="button"
          data-testid="close-create-modal"
          onClick={onClose}
        >
          Close
        </button>
      </div>
    ) : null;
  },
}));

vi.mock("@/components/rooms/rooms-master-detail", () => ({
  RoomsMasterDetail: ({
    groupDefinitions,
    selectedId,
    selectedRoom,
    onSelect,
    onSaveRoom,
    onDeleteClick,
  }: {
    groupDefinitions: Array<{
      id: string;
      title: string;
      items: Array<{ id: string; name: string }>;
    }>;
    selectedId: string | null;
    selectedRoom?: { name: string } | null;
    onSelect: (id: string | null) => void;
    onSaveRoom: (data: { name: string }) => Promise<void>;
    onDeleteClick: () => void;
  }) => (
    <div data-testid="rooms-master-detail">
      {groupDefinitions.map((group) => (
        <div key={group.id} data-testid={`group-${group.id}`}>
          <span data-testid={`group-title-${group.id}`}>{group.title}</span>
          {group.items.map((room) => (
            <button
              type="button"
              key={room.id}
              data-testid={`room-row-${room.id}`}
              onClick={() => onSelect(room.id)}
            >
              {room.name}
            </button>
          ))}
        </div>
      ))}
      {selectedId ? (
        <div data-testid="room-detail-panel">
          <span data-testid="detail-selected-id">{selectedId}</span>
          <span data-testid="detail-room-name">
            {selectedRoom?.name ?? "unbekannt"}
          </span>
          <button
            type="button"
            data-testid="trigger-update"
            onClick={() => void onSaveRoom({ name: "Updated Room" })}
          >
            Save
          </button>
          <button
            type="button"
            data-testid="trigger-deselect"
            onClick={() => onSelect(null)}
          >
            Close
          </button>
          <button
            type="button"
            data-testid="trigger-delete"
            onClick={onDeleteClick}
          >
            Delete
          </button>
        </div>
      ) : null}
    </div>
  ),
}));

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: ({
    isOpen,
    onConfirm,
    onClose,
  }: {
    isOpen: boolean;
    onConfirm?: () => void;
    onClose?: () => void;
  }) =>
    isOpen ? (
      <div data-testid="confirmation-modal">
        <button type="button" data-testid="confirm-delete" onClick={onConfirm}>
          Confirm
        </button>
        <button type="button" data-testid="cancel-delete" onClick={onClose}>
          Cancel
        </button>
      </div>
    ) : null,
}));

import { useSWRAuth } from "~/lib/swr";
import { ROOM_LIST_CACHE_KEYS } from "~/lib/swr/room-derived-caches";

const mockRooms = [
  {
    id: "1",
    name: "Raum 101",
    category: "Normaler Raum",
    capacity: 30,
    building: "Hauptgebäude",
    floor: 1,
    isOccupied: false,
  },
  {
    id: "2",
    name: "Turnhalle",
    category: "Sport",
    capacity: 100,
    building: "Sporthalle",
    floor: 0,
    isOccupied: true,
  },
];

describe("RoomsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    currentSearch = new URLSearchParams();

    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    mockGetOne.mockImplementation((id: string) =>
      Promise.resolve(mockRooms.find((room) => room.id === id) ?? null),
    );
  });

  it("renders the page with rooms data", async () => {
    render(<RoomsPage />);

    await waitFor(() => {
      expect(screen.getByText("Raum 101")).toBeInTheDocument();
      expect(screen.getByText("Turnhalle")).toBeInTheDocument();
    });
  });

  it("shows loading state when data is loading", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    render(<RoomsPage />);

    expect(screen.getByTestId("database-layout")).toHaveAttribute(
      "data-loading",
      "true",
    );
  });

  it("shows error message when fetch fails", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error("Failed to fetch"),
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    render(<RoomsPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Laden der Räume/),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state when no rooms exist", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    render(<RoomsPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Räume vorhanden")).toBeInTheDocument();
    });
  });

  it("filters rooms by search term", async () => {
    render(<RoomsPage />);

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "101" },
    });

    await waitFor(() => {
      expect(screen.getByText("Raum 101")).toBeInTheDocument();
      expect(screen.queryByText("Turnhalle")).not.toBeInTheDocument();
    });
  });

  it("clears all filters when clear button is clicked", async () => {
    render(<RoomsPage />);

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "Turn" },
    });
    fireEvent.change(screen.getByTestId("filter-category"), {
      target: { value: "Sport" },
    });

    fireEvent.click(screen.getByTestId("clear-filters"));

    await waitFor(() => {
      expect(screen.getByTestId("search-input")).toHaveValue("");
      expect(screen.getByTestId("filter-category")).toHaveValue("all");
    });
  });

  it("opens create modal when create button is clicked", async () => {
    render(<RoomsPage />);

    fireEvent.click(screen.getAllByLabelText("Raum erstellen")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("room-create-modal")).toBeInTheDocument();
    });
  });

  it("calls create service when submitting the create modal", async () => {
    mockCreate.mockResolvedValueOnce({ id: "3", name: "Neuer Raum" });

    render(<RoomsPage />);

    fireEvent.click(screen.getAllByLabelText("Raum erstellen")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("room-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalled();
    });
    await waitFor(() => {
      for (const key of ROOM_LIST_CACHE_KEYS) {
        expect(mockTenantMutate).toHaveBeenCalledWith(key);
      }
    });
  });

  it("re-throws create errors so the form can render them inline (Issue #1356)", async () => {
    mockCreate.mockRejectedValueOnce(
      new Error(
        "facilities error during CreateRoom: Ein Raum mit diesem Namen existiert bereits",
      ),
    );

    render(<RoomsPage />);

    fireEvent.click(screen.getAllByLabelText("Raum erstellen")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("room-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(screen.getByTestId("create-error")).toHaveTextContent(
        /existiert bereits/,
      );
    });
    // The modal must NOT close on a duplicate so the user can correct the name.
    expect(screen.getByTestId("room-create-modal")).toBeInTheDocument();
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("logs the stringified value when create rejects with a non-Error", async () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});
    mockCreate.mockRejectedValueOnce("plain-string-error");

    render(<RoomsPage />);

    fireEvent.click(screen.getAllByLabelText("Raum erstellen")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("room-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith("failed to create room", {
        error: "plain-string-error",
      });
    });
    consoleError.mockRestore();
  });

  it("syncs room selection into the URL when a row is clicked", async () => {
    render(<RoomsPage />);

    fireEvent.click(screen.getByTestId("room-row-1"));

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith(
        "/tenant/database/rooms?room=1",
        { scroll: false },
      );
    });
  });

  it("hydrates the detail panel from the room URL param", async () => {
    setSelectedRoom("1");

    render(<RoomsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("room-detail-panel")).toBeInTheDocument();
      expect(screen.getByTestId("detail-selected-id")).toHaveTextContent("1");
    });
  });

  it("removes the room URL param when the detail panel is closed", async () => {
    setSelectedRoom("1");

    render(<RoomsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("room-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-deselect"));

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith("/tenant/database/rooms", {
        scroll: false,
      });
    });
  });

  it("calls update service when saving from the inline detail panel", async () => {
    setSelectedRoom("1");
    mockUpdate.mockResolvedValueOnce(undefined);

    render(<RoomsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("room-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-update"));

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith(
        "1",
        expect.objectContaining({ name: "Updated Room" }),
      );
    });
    await waitFor(() => {
      for (const key of ROOM_LIST_CACHE_KEYS) {
        expect(mockTenantMutate).toHaveBeenCalledWith(key);
      }
      expect(mockRefreshRoomConsumers).toHaveBeenCalled();
    });
  });

  it("calls delete service after confirming deletion from the detail panel", async () => {
    setSelectedRoom("1");
    mockDelete.mockResolvedValueOnce(null);

    render(<RoomsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("room-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-delete"));

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("confirm-delete"));

    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalledWith("1");
      expect(mockReplace).toHaveBeenCalledWith("/tenant/database/rooms", {
        scroll: false,
      });
    });
    await waitFor(() => {
      for (const key of ROOM_LIST_CACHE_KEYS) {
        expect(mockTenantMutate).toHaveBeenCalledWith(key);
      }
    });
  });

  it("shows an error toast when delete returns an error", async () => {
    setSelectedRoom("1");
    mockDelete.mockResolvedValueOnce("Raum kann nicht gelöscht werden");

    render(<RoomsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("room-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-delete"));
    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("confirm-delete"));

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Raum kann nicht gelöscht werden",
      );
    });
  });
});
