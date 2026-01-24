import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

// Create hoisted mocks for service functions
const { mockGetList, mockGetOne, mockCreate, mockUpdate, mockDelete } =
  vi.hoisted(() => ({
    mockGetList: vi.fn(),
    mockGetOne: vi.fn(),
    mockCreate: vi.fn(),
    mockUpdate: vi.fn(),
    mockDelete: vi.fn(),
  }));

// Mock auth-client
vi.mock("~/lib/auth-client", () => ({
  useSession: vi.fn(() => ({
    data: { user: { id: "1", name: "Test User" } },
    isPending: false,
  })),
}));

// Mock next/navigation
vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
}));

// Mock service factory using hoisted mocks
vi.mock("@/lib/database/service-factory", () => ({
  createCrudService: vi.fn(() => ({
    getList: mockGetList,
    getOne: mockGetOne,
    create: mockCreate,
    update: mockUpdate,
    delete: mockDelete,
  })),
}));

// Mock hooks
vi.mock("~/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

vi.mock("~/hooks/useDeleteConfirmation", () => ({
  useDeleteConfirmation: vi.fn(() => ({
    showConfirmModal: false,
    handleDeleteClick: vi.fn(),
    handleDeleteCancel: vi.fn(),
    confirmDelete: vi.fn(),
  })),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: vi.fn(() => ({
    success: vi.fn(),
    error: vi.fn(),
  })),
}));

// Mock UI components
vi.mock("~/components/database/database-page-layout", () => ({
  DatabasePageLayout: ({
    children,
    loading,
  }: {
    children: React.ReactNode;
    loading: boolean;
  }) => (
    <div data-testid="database-layout" data-loading={loading}>
      {children}
    </div>
  ),
}));

vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({
    search,
    onClearAllFilters,
    actionButton,
  }: {
    search: { value: string; onChange: (v: string) => void };
    onClearAllFilters: () => void;
    actionButton?: React.ReactNode;
  }) => (
    <div data-testid="page-header">
      <input
        data-testid="search-input"
        value={search.value}
        onChange={(e) => search.onChange(e.target.value)}
      />
      <button data-testid="clear-filters" onClick={onClearAllFilters}>
        Clear
      </button>
      {actionButton}
    </div>
  ),
}));

vi.mock("@/components/devices", () => ({
  DeviceCreateModal: ({
    isOpen,
    onClose,
    onCreate,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onCreate: (data: { name: string; device_id: string }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="device-create-modal">
        <button
          data-testid="submit-create"
          onClick={() =>
            void onCreate({ name: "New Device", device_id: "DEV001" })
          }
        >
          Submit
        </button>
        <button data-testid="close-create-modal" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
  DeviceDetailModal: ({
    isOpen,
    device,
    onClose,
    onEdit,
    onDelete,
  }: {
    isOpen: boolean;
    device: { name: string | null; device_id: string } | null;
    onClose: () => void;
    onEdit: () => void;
    onDelete: () => void;
    onDeleteClick: () => void;
  }) =>
    isOpen && device ? (
      <div data-testid="device-detail-modal">
        <span data-testid="detail-device-name">
          {device.name ?? device.device_id}
        </span>
        <button data-testid="edit-button" onClick={onEdit}>
          Edit
        </button>
        <button data-testid="delete-button" onClick={onDelete}>
          Delete
        </button>
        <button data-testid="close-detail-modal" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
  DeviceEditModal: ({
    isOpen,
    onClose,
    onSave,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSave: (data: { name: string }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="device-edit-modal">
        <button
          data-testid="submit-edit"
          onClick={() => void onSave({ name: "Updated Device" })}
        >
          Save
        </button>
        <button data-testid="close-edit-modal" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
}));

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: () => <div data-testid="confirmation-modal" />,
}));

vi.mock("@/lib/iot-helpers", () => ({
  getDeviceTypeDisplayName: vi.fn((type: string) => {
    const names: Record<string, string> = {
      rfid_reader: "RFID Reader",
      terminal: "Terminal",
    };
    return names[type] ?? type;
  }),
}));

// Import component after mocks
import DevicesPage from "./page";
import { useSession } from "~/lib/auth-client";

const mockDevices = [
  {
    id: "1",
    name: "Device Alpha",
    device_id: "DEV001",
    device_type: "rfid_reader",
  },
  {
    id: "2",
    name: "Device Beta",
    device_id: "DEV002",
    device_type: "terminal",
  },
  {
    id: "3",
    name: null,
    device_id: "DEV003",
    device_type: "rfid_reader",
  },
];

describe("DevicesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();

    mockGetList.mockResolvedValue({ data: mockDevices });
    mockGetOne.mockImplementation((id: string) =>
      Promise.resolve(mockDevices.find((d) => d.id === id)),
    );
  });

  it("renders the page with devices data", async () => {
    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Device Alpha")).toBeInTheDocument();
      expect(screen.getByText("Device Beta")).toBeInTheDocument();
    });
  });

  it("shows loading state when session is pending", () => {
    vi.mocked(useSession).mockReturnValue({
      data: { user: { id: "1", name: "Test" } },
      isPending: true,
    } as ReturnType<typeof useSession>);

    render(<DevicesPage />);

    const layout = screen.getByTestId("database-layout");
    expect(layout).toHaveAttribute("data-loading", "true");
  });

  it("shows error message when fetch fails", async () => {
    mockGetList.mockRejectedValueOnce(new Error("Failed to fetch"));

    render(<DevicesPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Laden der Geräte/),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state when no devices exist", async () => {
    mockGetList.mockResolvedValueOnce({ data: [] });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Geräte vorhanden")).toBeInTheDocument();
    });
  });

  it("shows not found message when search has no matches", async () => {
    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Device Alpha")).toBeInTheDocument();
    });

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "xyz123" } });

    await waitFor(() => {
      expect(screen.getByText("Keine Geräte gefunden")).toBeInTheDocument();
    });
  });

  it("filters devices by name", async () => {
    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Device Alpha")).toBeInTheDocument();
    });

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Alpha" } });

    await waitFor(() => {
      expect(screen.getByText("Device Alpha")).toBeInTheDocument();
      expect(screen.queryByText("Device Beta")).not.toBeInTheDocument();
    });
  });

  it("filters devices by device_id", async () => {
    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Device Alpha")).toBeInTheDocument();
    });

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "DEV002" } });

    await waitFor(() => {
      expect(screen.getByText("Device Beta")).toBeInTheDocument();
      expect(screen.queryByText("Device Alpha")).not.toBeInTheDocument();
    });
  });

  it("filters devices by device_type", async () => {
    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Device Alpha")).toBeInTheDocument();
    });

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "terminal" } });

    await waitFor(() => {
      expect(screen.getByText("Device Beta")).toBeInTheDocument();
      expect(screen.queryByText("Device Alpha")).not.toBeInTheDocument();
    });
  });

  it("clears search when clear button is clicked", async () => {
    render(<DevicesPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "test" } });

    expect(searchInput).toHaveValue("test");

    const clearButton = screen.getByTestId("clear-filters");
    fireEvent.click(clearButton);

    await waitFor(() => {
      expect(searchInput).toHaveValue("");
    });
  });

  it("opens create modal when create button is clicked", async () => {
    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Device Alpha")).toBeInTheDocument();
    });

    // There are two create buttons (desktop and mobile) - use first one
    const createButtons = screen.getAllByLabelText("Gerät registrieren");
    fireEvent.click(createButtons[0]);

    await waitFor(() => {
      expect(screen.getByTestId("device-create-modal")).toBeInTheDocument();
    });
  });

  it("closes create modal when close is clicked", async () => {
    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Device Alpha")).toBeInTheDocument();
    });

    // There are two create buttons (desktop and mobile) - use first one
    const createButtons = screen.getAllByLabelText("Gerät registrieren");
    fireEvent.click(createButtons[0]);

    await waitFor(() => {
      expect(screen.getByTestId("device-create-modal")).toBeInTheDocument();
    });

    const closeButton = screen.getByTestId("close-create-modal");
    fireEvent.click(closeButton);

    await waitFor(() => {
      expect(
        screen.queryByTestId("device-create-modal"),
      ).not.toBeInTheDocument();
    });
  });

  it("opens detail modal when device row is clicked", async () => {
    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Device Alpha")).toBeInTheDocument();
    });

    const deviceRow = screen.getByText("Device Alpha").closest("button");
    if (deviceRow) {
      fireEvent.click(deviceRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("device-detail-modal")).toBeInTheDocument();
      expect(screen.getByTestId("detail-device-name")).toHaveTextContent(
        "Device Alpha",
      );
    });
  });

  it("opens edit modal when edit button is clicked in detail modal", async () => {
    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Device Alpha")).toBeInTheDocument();
    });

    const deviceRow = screen.getByText("Device Alpha").closest("button");
    if (deviceRow) {
      fireEvent.click(deviceRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("device-detail-modal")).toBeInTheDocument();
    });

    const editButton = screen.getByTestId("edit-button");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByTestId("device-edit-modal")).toBeInTheDocument();
    });
  });

  it("calls create service when submitting create form", async () => {
    mockCreate.mockResolvedValueOnce({
      id: "4",
      name: "New Device",
      device_id: "DEV004",
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Device Alpha")).toBeInTheDocument();
    });

    // Open create modal - there are two create buttons (desktop and mobile)
    const createButtons = screen.getAllByLabelText("Gerät registrieren");
    fireEvent.click(createButtons[0]);

    await waitFor(() => {
      expect(screen.getByTestId("device-create-modal")).toBeInTheDocument();
    });

    // Submit the form
    const submitButton = screen.getByTestId("submit-create");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalled();
    });
  });

  it("calls update service when saving edit form", async () => {
    mockUpdate.mockResolvedValueOnce({ id: "1", name: "Updated Device" });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Device Alpha")).toBeInTheDocument();
    });

    // Select a device to open detail modal
    const deviceRow = screen.getByText("Device Alpha").closest("button");
    if (deviceRow) {
      fireEvent.click(deviceRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("device-detail-modal")).toBeInTheDocument();
    });

    // Click edit button
    const editButton = screen.getByTestId("edit-button");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByTestId("device-edit-modal")).toBeInTheDocument();
    });

    // Submit edit form
    const submitButton = screen.getByTestId("submit-edit");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalled();
    });
  });

  it("calls delete service when deleting a device", async () => {
    mockDelete.mockResolvedValueOnce({});

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Device Alpha")).toBeInTheDocument();
    });

    // Select a device to open detail modal
    const deviceRow = screen.getByText("Device Alpha").closest("button");
    if (deviceRow) {
      fireEvent.click(deviceRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("device-detail-modal")).toBeInTheDocument();
    });

    // Click delete button
    const deleteButton = screen.getByTestId("delete-button");
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalled();
    });
  });

  it("closes detail modal when close button is clicked", async () => {
    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Device Alpha")).toBeInTheDocument();
    });

    // Open detail modal
    const deviceRow = screen.getByText("Device Alpha").closest("button");
    if (deviceRow) {
      fireEvent.click(deviceRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("device-detail-modal")).toBeInTheDocument();
    });

    // Close the modal
    const closeButton = screen.getByTestId("close-detail-modal");
    fireEvent.click(closeButton);

    await waitFor(() => {
      expect(
        screen.queryByTestId("device-detail-modal"),
      ).not.toBeInTheDocument();
    });
  });

  it("closes edit modal when close button is clicked", async () => {
    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Device Alpha")).toBeInTheDocument();
    });

    // Open detail modal first
    const deviceRow = screen.getByText("Device Alpha").closest("button");
    if (deviceRow) {
      fireEvent.click(deviceRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("device-detail-modal")).toBeInTheDocument();
    });

    // Open edit modal
    const editButton = screen.getByTestId("edit-button");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByTestId("device-edit-modal")).toBeInTheDocument();
    });

    // Close edit modal
    const closeButton = screen.getByTestId("close-edit-modal");
    fireEvent.click(closeButton);

    await waitFor(() => {
      expect(screen.queryByTestId("device-edit-modal")).not.toBeInTheDocument();
    });
  });

  it("displays device_id when device has no name", async () => {
    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("DEV003")).toBeInTheDocument();
    });
  });

  it("displays device type badge", async () => {
    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getAllByText("RFID Reader").length).toBeGreaterThan(0);
      expect(screen.getByText("Terminal")).toBeInTheDocument();
    });
  });

  it("handles getList returning non-array data gracefully", async () => {
    mockGetList.mockResolvedValueOnce({ data: null });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Geräte vorhanden")).toBeInTheDocument();
    });
  });
});
