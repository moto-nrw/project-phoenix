import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useState, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import GroupsPage from "./page";

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
const setSelectedGroup = (id: string | null) => {
  currentSearch = new URLSearchParams();
  if (id) {
    currentSearch.set("group", id);
  }
};

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: vi.fn(() => ({ push: vi.fn(), replace: mockReplace })),
  usePathname: vi.fn(() => "/tenant/database/groups"),
  useSearchParams: () => currentSearch,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  mutate: vi.fn(),
  useTenantMutate: vi.fn(() => vi.fn()),
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
    search,
  }: {
    children: ReactNode;
    loading: boolean;
    intro?: { title: string; description?: ReactNode; actions?: ReactNode };
    search?: ReactNode;
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
      <div data-testid="group-create-modal">
        {error ? <span data-testid="create-error">{error}</span> : null}
        <button
          type="button"
          data-testid="submit-create"
          onClick={() => submit({ name: "Neue Gruppe" })}
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

vi.mock("@/components/groups/groups-master-detail", () => ({
  GroupsMasterDetail: ({
    groups,
    selectedId,
    selectedGroup,
    onSelect,
    onSaveGroup,
    onDeleteClick,
  }: {
    groups: Array<{ id: string; name: string }>;
    selectedId: string | null;
    selectedGroup?: { name: string } | null;
    onSelect: (id: string | null) => void;
    onSaveGroup: (data: { name: string }) => Promise<void>;
    onDeleteClick: () => void;
  }) => (
    <div data-testid="groups-master-detail">
      {groups.map((group) => (
        <button
          type="button"
          key={group.id}
          data-testid={`group-row-${group.id}`}
          onClick={() => onSelect(group.id)}
        >
          {group.name}
        </button>
      ))}
      {selectedId ? (
        <div data-testid="group-detail-panel">
          <span data-testid="detail-selected-id">{selectedId}</span>
          <span data-testid="detail-group-name">
            {selectedGroup?.name ?? "unbekannt"}
          </span>
          <button
            type="button"
            data-testid="trigger-update"
            onClick={() => void onSaveGroup({ name: "Updated Group" })}
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

const mockGroups = [
  {
    id: "1",
    name: "Gruppe Rot",
    room_name: "Raum 101",
    representative_name: "Frau Müller",
    student_count: 12,
  },
  {
    id: "2",
    name: "Gruppe Blau",
    room_name: "Raum 202",
    representative_name: "Herr Schmidt",
    student_count: 8,
  },
];

describe("GroupsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    currentSearch = new URLSearchParams();

    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockGroups,
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    mockGetOne.mockImplementation((id: string) =>
      Promise.resolve(mockGroups.find((group) => group.id === id) ?? null),
    );
  });

  it("renders the page with groups data", async () => {
    render(<GroupsPage />);

    await waitFor(() => {
      expect(screen.getByText("Gruppe Rot")).toBeInTheDocument();
      expect(screen.getByText("Gruppe Blau")).toBeInTheDocument();
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

    render(<GroupsPage />);

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

    render(<GroupsPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Laden der Gruppen/),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state when no groups exist", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    render(<GroupsPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Gruppen vorhanden")).toBeInTheDocument();
    });
  });

  it("filters groups by search term", async () => {
    render(<GroupsPage />);

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "Rot" },
    });

    await waitFor(() => {
      expect(screen.getByText("Gruppe Rot")).toBeInTheDocument();
      expect(screen.queryByText("Gruppe Blau")).not.toBeInTheDocument();
    });
  });

  it("filters groups by representative name in search", async () => {
    render(<GroupsPage />);

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "Müller" },
    });

    await waitFor(() => {
      expect(screen.getByText("Gruppe Rot")).toBeInTheDocument();
      expect(screen.queryByText("Gruppe Blau")).not.toBeInTheDocument();
    });
  });

  it("clears all filters when clear button is clicked", async () => {
    render(<GroupsPage />);

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "Rot" },
    });
    fireEvent.change(screen.getByTestId("filter-room"), {
      target: { value: "Raum 101" },
    });

    fireEvent.click(screen.getByTestId("clear-filters"));

    await waitFor(() => {
      expect(screen.getByTestId("search-input")).toHaveValue("");
      expect(screen.getByTestId("filter-room")).toHaveValue("all");
    });
  });

  it("opens create modal when create button is clicked", async () => {
    render(<GroupsPage />);

    fireEvent.click(screen.getAllByLabelText("Gruppe erstellen")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("group-create-modal")).toBeInTheDocument();
    });
  });

  it("calls create service when submitting the create modal", async () => {
    mockCreate.mockResolvedValueOnce({ id: "3", name: "Neue Gruppe" });

    render(<GroupsPage />);

    fireEvent.click(screen.getAllByLabelText("Gruppe erstellen")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("group-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalled();
    });
  });

  it("re-throws create errors so the form can render them inline (Issue #1356)", async () => {
    mockCreate.mockRejectedValueOnce(
      new Error(
        "education error during CreateGroup: Eine Gruppe mit diesem Namen existiert bereits",
      ),
    );

    render(<GroupsPage />);

    fireEvent.click(screen.getAllByLabelText("Gruppe erstellen")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("group-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(screen.getByTestId("create-error")).toHaveTextContent(
        /existiert bereits/,
      );
    });
    // The modal must NOT close on a duplicate so the user can correct the name.
    expect(screen.getByTestId("group-create-modal")).toBeInTheDocument();
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("logs the stringified value when create rejects with a non-Error", async () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});
    // Reject with a plain string — exercises the `String(createError)` branch
    // of the `createError instanceof Error ? ... : ...` ternary in the catch.
    mockCreate.mockRejectedValueOnce("plain-string-error");

    render(<GroupsPage />);

    fireEvent.click(screen.getAllByLabelText("Gruppe erstellen")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("group-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith("failed to create group", {
        error: "plain-string-error",
      });
    });
    consoleError.mockRestore();
  });

  it("syncs group selection into the URL when a row is clicked", async () => {
    render(<GroupsPage />);

    fireEvent.click(screen.getByTestId("group-row-1"));

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith(
        "/tenant/database/groups?group=1",
        { scroll: false },
      );
    });
  });

  it("hydrates the detail panel from the group URL param", async () => {
    setSelectedGroup("1");

    render(<GroupsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("group-detail-panel")).toBeInTheDocument();
      expect(screen.getByTestId("detail-selected-id")).toHaveTextContent("1");
    });
  });

  it("removes the group URL param when the detail panel is closed", async () => {
    setSelectedGroup("1");

    render(<GroupsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("group-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-deselect"));

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith("/tenant/database/groups", {
        scroll: false,
      });
    });
  });

  it("calls update service when saving from the inline detail panel", async () => {
    setSelectedGroup("1");
    mockUpdate.mockResolvedValueOnce(undefined);

    render(<GroupsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("group-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-update"));

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith(
        "1",
        expect.objectContaining({ name: "Updated Group" }),
      );
    });
  });

  it("calls delete service after confirming deletion from the detail panel", async () => {
    setSelectedGroup("1");
    mockDelete.mockResolvedValueOnce(null);

    render(<GroupsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("group-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-delete"));

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("confirm-delete"));

    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalledWith("1");
      expect(mockReplace).toHaveBeenCalledWith("/tenant/database/groups", {
        scroll: false,
      });
    });
  });

  it("shows an error toast when delete returns an error", async () => {
    setSelectedGroup("1");
    mockDelete.mockResolvedValueOnce("Gruppe kann nicht gelöscht werden");

    render(<GroupsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("group-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-delete"));
    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("confirm-delete"));

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Gruppe kann nicht gelöscht werden",
      );
    });
  });
});
