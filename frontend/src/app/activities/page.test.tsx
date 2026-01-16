import {
  render,
  screen,
  fireEvent,
  waitFor,
  within,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import ActivitiesPage from "./page";

const { mockToastSuccess } = vi.hoisted(() => ({
  mockToastSuccess: vi.fn(),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
  }),
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
    actionButton,
  }: {
    search: { value: string; onChange: (v: string) => void };
    filters?: Array<{
      id: string;
      onChange: (v: string | string[]) => void;
    }>;
    onClearAllFilters: () => void;
    actionButton?: React.ReactNode;
  }) => {
    const categoryFilter = filters?.find((filter) => filter.id === "category");
    const myActivitiesFilter = filters?.find(
      (filter) => filter.id === "myActivities",
    );

    return (
      <div data-testid="page-header">
        <input
          data-testid="search-input"
          value={search.value}
          onChange={(e) => search.onChange(e.target.value)}
        />
        <button
          data-testid="filter-category"
          onClick={() => categoryFilter?.onChange("2")}
        >
          Category
        </button>
        <button
          data-testid="filter-my"
          onClick={() => myActivitiesFilter?.onChange("my")}
        >
          My Activities
        </button>
        <button data-testid="clear-filters" onClick={onClearAllFilters}>
          Clear
        </button>
        {actionButton}
      </div>
    );
  },
}));

vi.mock("~/components/activities/activity-management-modal", () => ({
  ActivityManagementModal: ({
    isOpen,
    activity,
    onSuccess,
  }: {
    isOpen: boolean;
    activity: { name: string } | null;
    onSuccess: (message?: string) => void;
  }) =>
    isOpen ? (
      <div data-testid="activity-management-modal">
        <span>{activity?.name}</span>
        <button
          data-testid="management-success"
          onClick={() => onSuccess("Gespeichert")}
        >
          Save
        </button>
      </div>
    ) : null,
}));

vi.mock("~/components/activities/quick-create-modal", () => ({
  QuickCreateActivityModal: ({
    isOpen,
    onClose,
    onSuccess,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSuccess: () => void;
  }) =>
    isOpen ? (
      <div data-testid="quick-create-modal">
        <button data-testid="quick-create-success" onClick={onSuccess}>
          Success
        </button>
        <button data-testid="quick-create-close" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
}));

import { useSWRAuth } from "~/lib/swr";

const mockActivities = [
  {
    id: "a1",
    name: "Schach",
    max_participant: 10,
    is_open_ags: true,
    supervisor_id: "staff-1",
    supervisors: [
      {
        id: "sup-1",
        staff_id: "staff-1",
        is_primary: true,
        first_name: "Anna",
        last_name: "Meyer",
      },
    ],
    ag_category_id: "1",
    category_name: "Sport",
    created_at: new Date(),
    updated_at: new Date(),
  },
  {
    id: "a2",
    name: "Kunst",
    max_participant: 12,
    is_open_ags: false,
    supervisor_id: "staff-2",
    supervisors: [
      {
        id: "sup-2",
        staff_id: "staff-2",
        is_primary: true,
        full_name: "Ben Schulz",
      },
    ],
    ag_category_id: "2",
    category_name: "Kunst",
    created_at: new Date(),
    updated_at: new Date(),
  },
];

const mockCategories = [
  {
    id: "1",
    name: "Sport",
    created_at: new Date(),
    updated_at: new Date(),
  },
  {
    id: "2",
    name: "Kunst",
    created_at: new Date(),
    updated_at: new Date(),
  },
];

const mockStaff = { id: "staff-1", person_id: "person-1" };

describe("ActivitiesPage", () => {
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockMutate.mockClear();
    mockToastSuccess.mockClear();
    vi.mocked(useSWRAuth).mockReturnValue({
      data: {
        activities: mockActivities,
        categories: mockCategories,
        currentStaff: mockStaff,
      },
      isLoading: false,
      error: null,
      mutate: mockMutate,
    } as never);
  });

  it("renders activities and opens management modal", async () => {
    render(<ActivitiesPage />);

    expect(screen.getByText("Schach")).toBeInTheDocument();
    expect(screen.getByText("Kunst")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Schach/i }));

    const modal = screen.getByTestId("activity-management-modal");
    expect(modal).toBeInTheDocument();
    expect(within(modal).getByText("Schach")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("management-success"));

    await waitFor(() => {
      expect(mockToastSuccess).toHaveBeenCalledWith("Gespeichert");
      expect(mockMutate).toHaveBeenCalled();
    });
  });

  it("filters activities by search and my activities", async () => {
    render(<ActivitiesPage />);

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "Kunst" },
    });

    await waitFor(() => {
      expect(screen.queryByText("Schach")).not.toBeInTheDocument();
      expect(screen.getByText("Kunst")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("clear-filters"));

    await waitFor(() => {
      expect(screen.getByText("Schach")).toBeInTheDocument();
      expect(screen.getByText("Kunst")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("filter-my"));

    await waitFor(() => {
      expect(screen.getByText("Schach")).toBeInTheDocument();
      expect(screen.queryByText("Kunst")).not.toBeInTheDocument();
    });
  });

  it("opens quick create modal from the FAB", () => {
    render(<ActivitiesPage />);

    const buttons = screen.getAllByLabelText("Aktivität erstellen");
    const lastButton = buttons[buttons.length - 1];
    if (lastButton) {
      fireEvent.click(lastButton);
    }

    expect(screen.getByTestId("quick-create-modal")).toBeInTheDocument();
  });
});
