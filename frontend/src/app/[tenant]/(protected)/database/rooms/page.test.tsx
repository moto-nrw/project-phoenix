import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import RoomsPage from "./page";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

/** Text of a FormErrorInput, as the shared kit components render it. */
function errorText(error: unknown): string | null {
  if (!error) return null;
  return typeof error === "string"
    ? error
    : (error as { message: string }).message;
}

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
const mockRemove = vi.fn();
vi.mock("@/lib/database/service-factory", () => ({
  createCrudService: vi.fn(() => ({
    getList: vi.fn(),
    getOne: mockGetOne,
    create: mockCreate,
    update: mockUpdate,
    remove: mockRemove,
  })),
}));

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

const mockToastSuccess = vi.fn();
const mockToastError = vi.fn();
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: vi.fn(() => ({
    success: mockToastSuccess,
    error: mockToastError,
  })),
}));

vi.mock("~/components/ui/confirm-delete-modal", () => ({
  ConfirmDeleteModal: ({
    isOpen,
    onConfirm,
    onClose,
    error,
  }: {
    isOpen: boolean;
    onConfirm?: () => void;
    onClose?: () => void;
    error?: unknown;
  }) =>
    isOpen ? (
      <div data-testid="confirmation-modal">
        {errorText(error) ? (
          <span data-testid="delete-error">{errorText(error)}</span>
        ) : null}
        <button type="button" data-testid="confirm-delete" onClick={onConfirm}>
          Confirm
        </button>
        <button type="button" data-testid="cancel-delete" onClick={onClose}>
          Cancel
        </button>
      </div>
    ) : null,
}));

vi.mock("~/components/database/database-page-layout", () => ({
  DatabasePageLayout: ({
    children,
    loading,
    intro,
    search,
    error,
    empty,
    overlays,
  }: {
    children: ReactNode;
    loading: boolean;
    intro?: { title: string; description?: ReactNode; actions?: ReactNode };
    search?: ReactNode;
    error?: unknown;
    empty?: {
      title: string;
      description?: string;
      icon?: ReactNode;
      action?: ReactNode;
    } | null;
    overlays?: ReactNode;
  }) => (
    <div data-testid="database-layout" data-loading={loading}>
      {intro ? (
        <div data-testid="page-intro">
          <h1>{intro.title}</h1>
          {intro.description}
          {intro.actions}
          {search}
        </div>
      ) : null}
      {/* Fehler und Leerzustand liefert das Geruest, nicht die Seite. */}
      {error ? <div data-testid="page-error">{errorText(error)}</div> : null}
      {!error && empty ? (
        <div data-testid="page-empty">
          <p>{empty.title}</p>
          {empty.description ? <p>{empty.description}</p> : null}
          {empty.action}
        </div>
      ) : null}
      {!error && !empty ? children : null}
      {overlays}
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
    errorPath,
    errorObject,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSubmit: (data: { name: string }) => Promise<void>;
    errorPath: {
      error: unknown;
      show: (error: unknown, options: { object: string }) => unknown;
    };
    errorObject?: string;
  }) => {
    // Mirrors DatabaseForm: catches the rejection from onSubmit and hands it
    // to the owner's error path, whose catalog text it renders inline.
    const submit = (data: { name: string }) => {
      void onSubmit(data).catch((err: unknown) => {
        void errorPath.show(err, { object: errorObject ?? "" });
      });
    };
    const error = errorText(errorPath.error);
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

// Test double for the collection list (#3115): one link per room, the
// object route the page computes via `objectHref`.
vi.mock("@/components/rooms/rooms-list", () => ({
  RoomsList: ({
    groupDefinitions,
    objectHref,
  }: {
    groupDefinitions: Array<{
      id: string;
      title: string;
      items: Array<{ id: string; name: string }>;
    }>;
    objectHref: (room: { id: string }) => string;
  }) => (
    <div data-testid="rooms-list">
      {groupDefinitions.map((group) => (
        <div key={group.id} data-testid={`group-${group.id}`}>
          <span data-testid={`group-title-${group.id}`}>{group.title}</span>
          {group.items.map((room) => (
            <a
              key={room.id}
              data-testid={`room-row-${room.id}`}
              href={objectHref(room)}
            >
              {room.name}
            </a>
          ))}
        </div>
      ))}
    </div>
  ),
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
      error: new ApiError("Failed to fetch", 503, {
        code: "general.unavailable",
      }),
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    render(<RoomsPage />);

    expect(await screen.findByTestId("page-error")).toHaveTextContent(
      catalogText("general.unavailable", "die Liste der Räume"),
    );
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

  it("shows a refused create with the catalog text in the dialog (Issue #1356)", async () => {
    mockCreate.mockRejectedValueOnce(
      new ApiError("name taken", 409, {
        code: "general.business_rejection",
      }),
    );

    render(<RoomsPage />);

    fireEvent.click(screen.getAllByLabelText("Raum erstellen")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("room-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(screen.getByTestId("create-error")).toHaveTextContent(
        catalogText("general.business_rejection", "das Speichern des Raums"),
      );
    });
    // The modal must NOT close on a duplicate so the user can correct the name.
    expect(screen.getByTestId("room-create-modal")).toBeInTheDocument();
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("starts a reopened create dialog without the previous error", async () => {
    mockCreate.mockRejectedValueOnce(
      new ApiError("down", 503, { code: "general.unavailable" }),
    );

    render(<RoomsPage />);

    fireEvent.click(screen.getAllByLabelText("Raum erstellen")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("room-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create"));
    await waitFor(() => {
      expect(screen.getByTestId("create-error")).toHaveTextContent(
        catalogText("general.unavailable", "das Speichern des Raums"),
      );
    });

    fireEvent.click(screen.getByTestId("close-create-modal"));
    fireEvent.click(screen.getAllByLabelText("Raum erstellen")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("room-create-modal")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("create-error")).not.toBeInTheDocument();
  });

  it("links every row to the room page with the register as referrer", async () => {
    currentSearch = new URLSearchParams({ groupBy: "floor" });

    render(<RoomsPage />);

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "Raum" },
    });
    fireEvent.change(screen.getByTestId("filter-category"), {
      target: { value: "Normaler Raum" },
    });

    await waitFor(() => {
      // Path routing in the test tenant context: the link carries the slug,
      // the `from` referrer stays slug-free (the room page prefixes it itself).
      expect(screen.getByTestId("room-row-1")).toHaveAttribute(
        "href",
        `/test-tenant/rooms/1?tab=stammdaten&from=${encodeURIComponent("/database/rooms?groupBy=floor&search=Raum&category=Normaler+Raum")}`,
      );
    });
  });
});
