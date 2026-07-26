import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useState, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ActivitiesPage from "./page";

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
const setSelectedActivity = (id: string | null) => {
  currentSearch = new URLSearchParams();
  if (id) {
    currentSearch.set("activity", id);
  }
};

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: vi.fn(() => ({ push: vi.fn(), replace: mockReplace })),
  usePathname: vi.fn(() => "/tenant/database/activities"),
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
  }: {
    children: ReactNode;
    loading: boolean;
  }) => (
    <div data-testid="database-layout" data-loading={loading}>
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
      <div data-testid="activity-create-modal">
        {error ? <span data-testid="create-error">{error}</span> : null}
        <button
          type="button"
          data-testid="submit-create"
          onClick={() => submit({ name: "Neue AG" })}
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

vi.mock("@/components/activities/activities-master-detail", () => ({
  ActivitiesMasterDetail: ({
    activities,
    selectedId,
    selectedActivity,
    onSelect,
    onSaveActivity,
    onDeleteClick,
  }: {
    activities: Array<{ id: string; name: string }>;
    selectedId: string | null;
    selectedActivity?: { name: string } | null;
    onSelect: (id: string | null) => void;
    onSaveActivity: (data: { name: string }) => Promise<void>;
    onDeleteClick: () => void;
  }) => {
    // Mirrors DatabaseForm: catches the rejection from onSaveActivity and
    // renders the message inline. Tests assert against the resulting message.
    const [error, setError] = useState<string | null>(null);
    const save = (data: { name: string }) => {
      setError(null);
      void onSaveActivity(data).catch((err: unknown) => {
        setError(err instanceof Error ? err.message : String(err));
      });
    };
    return (
      <div data-testid="activities-master-detail">
        {activities.map((activity) => (
          <button
            type="button"
            key={activity.id}
            data-testid={`activity-row-${activity.id}`}
            onClick={() => onSelect(activity.id)}
          >
            {activity.name}
          </button>
        ))}
        {selectedId ? (
          <div data-testid="activity-detail-panel">
            {error ? <span data-testid="update-error">{error}</span> : null}
            <span data-testid="detail-selected-id">{selectedId}</span>
            <span data-testid="detail-activity-name">
              {selectedActivity?.name ?? "unbekannt"}
            </span>
            <button
              type="button"
              data-testid="trigger-update"
              onClick={() => save({ name: "Updated AG" })}
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
    );
  },
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

const mockActivities = [
  {
    id: "1",
    name: "Fußball AG",
    category_name: "Sport",
    max_participant: 20,
    is_open_ags: false,
    supervisor_id: "1",
    supervisor_name: "Herr Fischer",
    ag_category_id: "1",
  },
  {
    id: "2",
    name: "Chor",
    category_name: "Musik",
    max_participant: 30,
    is_open_ags: false,
    supervisor_id: "2",
    supervisor_name: "Frau Weber",
    ag_category_id: "2",
  },
];

describe("ActivitiesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    currentSearch = new URLSearchParams();

    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockActivities,
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    mockGetOne.mockImplementation((id: string) =>
      Promise.resolve(
        mockActivities.find((activity) => activity.id === id) ?? null,
      ),
    );
  });

  it("renders the page with activities data", async () => {
    render(<ActivitiesPage />);

    await waitFor(() => {
      expect(screen.getByText("Fußball AG")).toBeInTheDocument();
      expect(screen.getByText("Chor")).toBeInTheDocument();
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

    render(<ActivitiesPage />);

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

    render(<ActivitiesPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Laden der Aktivitäten/),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state when no activities exist", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    render(<ActivitiesPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine Aktivitäten vorhanden"),
      ).toBeInTheDocument();
    });
  });

  it("filters activities by search term", async () => {
    render(<ActivitiesPage />);

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "Fußball" },
    });

    await waitFor(() => {
      expect(screen.getByText("Fußball AG")).toBeInTheDocument();
      expect(screen.queryByText("Chor")).not.toBeInTheDocument();
    });
  });

  it("filters activities by supervisor name in search", async () => {
    render(<ActivitiesPage />);

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "Fischer" },
    });

    await waitFor(() => {
      expect(screen.getByText("Fußball AG")).toBeInTheDocument();
      expect(screen.queryByText("Chor")).not.toBeInTheDocument();
    });
  });

  it("clears all filters when clear button is clicked", async () => {
    render(<ActivitiesPage />);

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "Fußball" },
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
    render(<ActivitiesPage />);

    fireEvent.click(screen.getAllByLabelText("Aktivität erstellen")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("activity-create-modal")).toBeInTheDocument();
    });
  });

  it("calls create service when submitting the create modal", async () => {
    mockCreate.mockResolvedValueOnce({ id: "3", name: "Neue AG" });

    render(<ActivitiesPage />);

    fireEvent.click(screen.getAllByLabelText("Aktivität erstellen")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("activity-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalled();
    });
  });

  it("re-throws create errors so the form can render them inline (Issue #1356)", async () => {
    mockCreate.mockRejectedValueOnce(
      new Error(
        "activities error during CreateActivity: Eine Aktivität mit diesem Namen existiert bereits",
      ),
    );

    render(<ActivitiesPage />);

    fireEvent.click(screen.getAllByLabelText("Aktivität erstellen")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("activity-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(screen.getByTestId("create-error")).toHaveTextContent(
        /existiert bereits/,
      );
    });
    // The modal must NOT close on a duplicate so the user can correct the name.
    expect(screen.getByTestId("activity-create-modal")).toBeInTheDocument();
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("logs the stringified value when create rejects with a non-Error", async () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});
    mockCreate.mockRejectedValueOnce("plain-string-error");

    render(<ActivitiesPage />);

    fireEvent.click(screen.getAllByLabelText("Aktivität erstellen")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("activity-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith("failed to create activity", {
        error: "plain-string-error",
      });
    });
    consoleError.mockRestore();
  });

  it("syncs activity selection into the URL when a row is clicked", async () => {
    render(<ActivitiesPage />);

    fireEvent.click(screen.getByTestId("activity-row-1"));

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith(
        "/tenant/database/activities?activity=1",
        { scroll: false },
      );
    });
  });

  it("hydrates the detail panel from the activity URL param and fetches detail data", async () => {
    setSelectedActivity("1");

    render(<ActivitiesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("activity-detail-panel")).toBeInTheDocument();
      expect(screen.getByTestId("detail-selected-id")).toHaveTextContent("1");
      expect(mockGetOne).toHaveBeenCalledWith("1");
    });
  });

  it("removes the activity URL param when the detail panel is closed", async () => {
    setSelectedActivity("1");

    render(<ActivitiesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("activity-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-deselect"));

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith("/tenant/database/activities", {
        scroll: false,
      });
    });
  });

  it("calls update service when saving from the inline detail panel", async () => {
    setSelectedActivity("1");
    mockUpdate.mockResolvedValueOnce(undefined);

    render(<ActivitiesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("activity-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-update"));

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith(
        "1",
        expect.objectContaining({ name: "Updated AG" }),
      );
    });
  });

  it("re-throws update errors so the inline form can render them (Issue #1356)", async () => {
    setSelectedActivity("1");
    mockUpdate.mockRejectedValueOnce(
      new Error(
        "activities error during UpdateActivity: Eine Aktivität mit diesem Namen existiert bereits",
      ),
    );

    render(<ActivitiesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("activity-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-update"));

    await waitFor(() => {
      expect(screen.getByTestId("update-error")).toHaveTextContent(
        /existiert bereits/,
      );
    });
    // The detail panel must stay mounted so the user can correct the name.
    expect(screen.getByTestId("activity-detail-panel")).toBeInTheDocument();
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("calls delete service after confirming deletion from the detail panel", async () => {
    setSelectedActivity("1");
    mockDelete.mockResolvedValueOnce(null);

    render(<ActivitiesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("activity-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-delete"));

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("confirm-delete"));

    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalledWith("1");
      expect(mockReplace).toHaveBeenCalledWith("/tenant/database/activities", {
        scroll: false,
      });
    });
  });

  it("shows an error toast when delete returns an error", async () => {
    setSelectedActivity("1");
    mockDelete.mockResolvedValueOnce("Aktivität kann nicht gelöscht werden");

    render(<ActivitiesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("activity-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-delete"));
    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("confirm-delete"));

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Aktivität kann nicht gelöscht werden",
      );
    });
  });
});
