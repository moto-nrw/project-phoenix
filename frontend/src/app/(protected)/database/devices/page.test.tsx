/* eslint-disable @typescript-eslint/no-unsafe-return, @typescript-eslint/no-empty-function */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import DevicesPage from "./page";
import type { Device } from "@/lib/iot-helpers";

const mockUseSession = vi.fn();
vi.mock("next-auth/react", () => ({
  useSession: (opts?: { required?: boolean; onUnauthenticated?: () => void }) =>
    mockUseSession(opts),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
}));

const mockGetList = vi.fn();
const mockGetOne = vi.fn();
const mockCreate = vi.fn();
const mockUpdate = vi.fn();
const mockDelete = vi.fn();

vi.mock("@/lib/database/service-factory", () => ({
  createCrudService: () => ({
    getList: mockGetList,
    getOne: mockGetOne,
    create: mockCreate,
    update: mockUpdate,
    delete: mockDelete,
  }),
}));

const mockTransform = vi.fn((data: unknown) => data);
vi.mock("@/lib/database/configs/devices.config", () => ({
  devicesConfig: {
    name: { singular: "Gerät", plural: "Geräte" },
    form: {
      transformBeforeSubmit: (data: unknown) => mockTransform(data),
    },
  },
}));

vi.mock("~/components/database/database-page-layout", () => ({
  DatabasePageLayout: ({
    children,
    loading,
    sessionLoading,
  }: {
    children: React.ReactNode;
    loading: boolean;
    sessionLoading: boolean;
  }) =>
    loading || sessionLoading ? (
      <div data-testid="loading">Loading...</div>
    ) : (
      <div data-testid="database-layout">{children}</div>
    ),
}));

vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({
    badge,
    search,
    activeFilters,
    onClearAllFilters,
    actionButton,
  }: {
    badge?: { count: number };
    search?: { value: string; onChange: (v: string) => void };
    activeFilters?: Array<{ id: string; label: string; onRemove: () => void }>;
    onClearAllFilters?: () => void;
    actionButton?: React.ReactNode;
  }) => (
    <div data-testid="page-header">
      {badge && <span data-testid="badge-count">{badge.count}</span>}
      {search && (
        <input
          data-testid="search-input"
          value={search.value}
          onChange={(e) => search.onChange(e.target.value)}
        />
      )}
      {activeFilters && activeFilters.length > 0 && (
        <div data-testid="active-filters">
          {activeFilters.map((f) => (
            <button
              key={f.id}
              data-testid={`filter-${f.id}`}
              onClick={f.onRemove}
            >
              {f.label}
            </button>
          ))}
        </div>
      )}
      {onClearAllFilters && (
        <button data-testid="clear-filters" onClick={onClearAllFilters}>
          Clear
        </button>
      )}
      {actionButton}
    </div>
  ),
}));

vi.mock("@/components/devices", () => ({
  DeviceCreateModal: ({
    isOpen,
    onClose,
    onCreate,
    loading,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onCreate: (data: Partial<Device>) => void;
    loading: boolean;
  }) =>
    isOpen ? (
      <div data-testid="create-modal">
        <button
          data-testid="create-submit"
          disabled={loading}
          onClick={() =>
            onCreate({
              device_id: "NEW-001",
              name: "New Device",
              device_type: "rfid_reader",
            })
          }
        >
          Create
        </button>
        <button data-testid="create-close" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
  DeviceDetailModal: ({
    isOpen,
    onClose,
    device,
    onEdit,
    onDeleteClick,
    loading,
  }: {
    isOpen: boolean;
    onClose: () => void;
    device: Device;
    onEdit: () => void;
    onDelete: () => void;
    onDeleteClick: () => void;
    loading: boolean;
  }) =>
    isOpen ? (
      <div data-testid="detail-modal">
        <span data-testid="device-name">{device.name ?? device.device_id}</span>
        <button data-testid="edit-button" disabled={loading} onClick={onEdit}>
          Edit
        </button>
        <button
          data-testid="delete-button"
          disabled={loading}
          onClick={onDeleteClick}
        >
          Delete
        </button>
        <button data-testid="detail-close" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
  DeviceEditModal: ({
    isOpen,
    onClose,
    device,
    onSave,
    loading,
  }: {
    isOpen: boolean;
    onClose: () => void;
    device: Device;
    onSave: (data: Partial<Device>) => void;
    loading: boolean;
  }) =>
    isOpen ? (
      <div data-testid="edit-modal">
        <span data-testid="edit-device-id">{device.device_id}</span>
        <button
          data-testid="save-button"
          disabled={loading}
          onClick={() => onSave({ name: "Updated Device" })}
        >
          Save
        </button>
        <button data-testid="edit-close" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
}));

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: ({
    isOpen,
    onClose,
    onConfirm,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onConfirm: () => void;
  }) =>
    isOpen ? (
      <div data-testid="confirm-modal">
        <button data-testid="confirm-delete" onClick={onConfirm}>
          Confirm
        </button>
        <button data-testid="cancel-delete" onClick={onClose}>
          Cancel
        </button>
      </div>
    ) : null,
}));

vi.mock("@/lib/iot-helpers", () => ({
  getDeviceTypeDisplayName: (type: string) => type.toUpperCase(),
}));

const mockToastSuccess = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: mockToastSuccess, error: vi.fn() }),
}));

const mockUseIsMobile = vi.fn(() => false);
vi.mock("~/hooks/useIsMobile", () => ({
  useIsMobile: () => mockUseIsMobile(),
}));

const mockHandleDeleteClick = vi.fn();
const mockHandleDeleteCancel = vi.fn();
const mockConfirmDelete = vi.fn();
const mockShowConfirmModal = vi.fn(() => false);

vi.mock("~/hooks/useDeleteConfirmation", () => ({
  useDeleteConfirmation: () => ({
    showConfirmModal: mockShowConfirmModal(),
    handleDeleteClick: mockHandleDeleteClick,
    handleDeleteCancel: mockHandleDeleteCancel,
    confirmDelete: mockConfirmDelete,
  }),
}));

vi.mock("@/lib/use-notification", () => ({
  getDbOperationMessage: (op: string, type: string, name: string) =>
    `${op} ${type} ${name}`,
}));

describe("DevicesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockTransform.mockImplementation((data) => data); // Reset transform
    mockUseSession.mockReturnValue({
      data: {
        user: {
          id: "1",
          name: "Test User",
          email: "test@test.com",
          token: "tok",
          roles: ["admin"],
        },
      },
      status: "authenticated",
    });
    mockUseIsMobile.mockReturnValue(false);
    mockShowConfirmModal.mockReturnValue(false);
  });

  it("shows loading state initially", () => {
    mockGetList.mockReturnValue(new Promise(() => {}));
    render(<DevicesPage />);
    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("shows loading when session is loading", () => {
    mockUseSession.mockReturnValue({
      data: null,
      status: "loading",
    });
    mockGetList.mockResolvedValue({ data: [] });
    render(<DevicesPage />);
    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("renders devices after loading", async () => {
    mockGetList.mockResolvedValue({
      data: [
        {
          id: "1",
          device_id: "DEV-001",
          name: "Reader 1",
          device_type: "rfid_reader",
        },
      ],
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Reader 1")).toBeInTheDocument();
      expect(screen.getByText("RFID_READER")).toBeInTheDocument();
    });
  });

  it("renders device without name using device_id", async () => {
    mockGetList.mockResolvedValue({
      data: [
        {
          id: "1",
          device_id: "DEV-001",
          name: null,
          device_type: "rfid_reader",
        },
      ],
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("DEV-001")).toBeInTheDocument();
    });
  });

  it("shows empty state when no devices", async () => {
    mockGetList.mockResolvedValue({ data: [] });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Geräte vorhanden")).toBeInTheDocument();
      expect(
        screen.getByText("Es wurden noch keine Geräte registriert."),
      ).toBeInTheDocument();
    });
  });

  it("shows error when API fails", async () => {
    mockGetList.mockRejectedValue(new Error("API Error"));

    render(<DevicesPage />);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Fehler beim Laden der Geräte. Bitte versuchen Sie es später erneut.",
        ),
      ).toBeInTheDocument();
    });
  });

  it("handles non-array response from API", async () => {
    mockGetList.mockResolvedValue({ data: null });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Geräte vorhanden")).toBeInTheDocument();
    });
  });

  it("filters devices by search term (name)", async () => {
    mockGetList.mockResolvedValue({
      data: [
        {
          id: "1",
          device_id: "DEV-001",
          name: "Reader 1",
          device_type: "rfid_reader",
        },
        {
          id: "2",
          device_id: "DEV-002",
          name: "Reader 2",
          device_type: "camera",
        },
      ],
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Reader 1")).toBeInTheDocument();
    });

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Reader 2" } });

    await waitFor(() => {
      expect(screen.queryByText("Reader 1")).not.toBeInTheDocument();
      expect(screen.getByText("Reader 2")).toBeInTheDocument();
    });
  });

  it("filters devices by device_id", async () => {
    mockGetList.mockResolvedValue({
      data: [
        {
          id: "1",
          device_id: "DEV-001",
          name: "Reader 1",
          device_type: "rfid_reader",
        },
        {
          id: "2",
          device_id: "ABC-002",
          name: "Reader 2",
          device_type: "camera",
        },
      ],
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Reader 1")).toBeInTheDocument();
    });

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "ABC" } });

    await waitFor(() => {
      expect(screen.queryByText("Reader 1")).not.toBeInTheDocument();
      expect(screen.getByText("Reader 2")).toBeInTheDocument();
    });
  });

  it("filters devices by device_type", async () => {
    mockGetList.mockResolvedValue({
      data: [
        {
          id: "1",
          device_id: "DEV-001",
          name: "Reader 1",
          device_type: "rfid_reader",
        },
        {
          id: "2",
          device_id: "DEV-002",
          name: "Camera 1",
          device_type: "camera",
        },
      ],
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Reader 1")).toBeInTheDocument();
    });

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "camera" } });

    await waitFor(() => {
      expect(screen.queryByText("Reader 1")).not.toBeInTheDocument();
      expect(screen.getByText("Camera 1")).toBeInTheDocument();
    });
  });

  it("shows empty state with search term when no matches", async () => {
    mockGetList.mockResolvedValue({
      data: [
        {
          id: "1",
          device_id: "DEV-001",
          name: "Reader 1",
          device_type: "rfid_reader",
        },
      ],
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Reader 1")).toBeInTheDocument();
    });

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "nonexistent" } });

    await waitFor(() => {
      expect(screen.getByText("Keine Geräte gefunden")).toBeInTheDocument();
      expect(
        screen.getByText("Versuchen Sie einen anderen Suchbegriff."),
      ).toBeInTheDocument();
    });
  });

  it("shows active filter when searching", async () => {
    mockGetList.mockResolvedValue({
      data: [
        {
          id: "1",
          device_id: "DEV-001",
          name: "Reader 1",
          device_type: "rfid_reader",
        },
      ],
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Reader 1")).toBeInTheDocument();
    });

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Reader" } });

    await waitFor(() => {
      expect(screen.getByTestId("active-filters")).toBeInTheDocument();
      expect(screen.getByText('"Reader"')).toBeInTheDocument();
    });
  });

  it("clears search when filter is removed", async () => {
    mockGetList.mockResolvedValue({
      data: [
        {
          id: "1",
          device_id: "DEV-001",
          name: "Reader 1",
          device_type: "rfid_reader",
        },
      ],
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Reader 1")).toBeInTheDocument();
    });

    const searchInput: HTMLInputElement = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Reader" } });

    await waitFor(() => {
      expect(screen.getByTestId("filter-search")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("filter-search"));

    await waitFor(() => {
      expect(searchInput.value).toBe("");
    });
  });

  it("renders desktop create button when not mobile", async () => {
    mockGetList.mockResolvedValue({ data: [] });
    mockUseIsMobile.mockReturnValue(false);

    render(<DevicesPage />);

    await waitFor(() => {
      const buttons = screen.getAllByLabelText("Gerät registrieren");
      // Desktop button in header + mobile FAB
      expect(buttons.length).toBeGreaterThan(0);
    });
  });

  it("opens create modal when desktop button clicked", async () => {
    mockGetList.mockResolvedValue({ data: [] });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("page-header")).toBeInTheDocument();
    });

    const createButtons = screen.getAllByLabelText("Gerät registrieren");
    fireEvent.click(createButtons[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("create-modal")).toBeInTheDocument();
    });
  });

  it("opens create modal when mobile FAB clicked", async () => {
    mockGetList.mockResolvedValue({ data: [] });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("page-header")).toBeInTheDocument();
    });

    const createButtons = screen.getAllByLabelText("Gerät registrieren");
    // Mobile FAB is last button
    fireEvent.click(createButtons[createButtons.length - 1]!);

    await waitFor(() => {
      expect(screen.getByTestId("create-modal")).toBeInTheDocument();
    });
  });

  it("creates device successfully", async () => {
    mockGetList.mockResolvedValue({ data: [] });
    mockCreate.mockResolvedValue({
      id: "1",
      device_id: "NEW-001",
      name: "New Device",
      device_type: "rfid_reader",
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("page-header")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("Gerät registrieren")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("create-submit"));

    await waitFor(() => {
      expect(mockTransform).toHaveBeenCalled();
      expect(mockCreate).toHaveBeenCalled();
      expect(mockToastSuccess).toHaveBeenCalled();
      expect(screen.getByTestId("detail-modal")).toBeInTheDocument();
    });
  });

  it("handles create without transform", async () => {
    mockGetList.mockResolvedValue({ data: [] });
    mockCreate.mockResolvedValue({
      id: "1",
      device_id: "NEW-001",
      name: null,
      device_type: "rfid_reader",
    });
    mockTransform.mockReset();

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("page-header")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("Gerät registrieren")[0]!);
    await waitFor(() =>
      expect(screen.getByTestId("create-modal")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByTestId("create-submit"));

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalled();
    });
  });

  it("closes create modal", async () => {
    mockGetList.mockResolvedValue({ data: [] });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("page-header")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("Gerät registrieren")[0]!);
    await waitFor(() =>
      expect(screen.getByTestId("create-modal")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByTestId("create-close"));

    await waitFor(() => {
      expect(screen.queryByTestId("create-modal")).not.toBeInTheDocument();
    });
  });

  it("opens detail modal when device clicked", async () => {
    mockGetList.mockResolvedValue({
      data: [
        {
          id: "1",
          device_id: "DEV-001",
          name: "Reader 1",
          device_type: "rfid_reader",
        },
      ],
    });
    mockGetOne.mockResolvedValue({
      id: "1",
      device_id: "DEV-001",
      name: "Reader 1 Full",
      device_type: "rfid_reader",
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Reader 1")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Reader 1"));

    await waitFor(() => {
      expect(mockGetOne).toHaveBeenCalledWith("1");
      expect(screen.getByTestId("detail-modal")).toBeInTheDocument();
      expect(screen.getByTestId("device-name")).toHaveTextContent(
        "Reader 1 Full",
      );
    });
  });

  it("closes detail modal", async () => {
    mockGetList.mockResolvedValue({
      data: [
        {
          id: "1",
          device_id: "DEV-001",
          name: "Reader 1",
          device_type: "rfid_reader",
        },
      ],
    });
    mockGetOne.mockResolvedValue({
      id: "1",
      device_id: "DEV-001",
      name: "Reader 1",
      device_type: "rfid_reader",
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Reader 1")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Reader 1"));
    await waitFor(() =>
      expect(screen.getByTestId("detail-modal")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByTestId("detail-close"));

    await waitFor(() => {
      expect(screen.queryByTestId("detail-modal")).not.toBeInTheDocument();
    });
  });

  it("opens edit modal from detail modal", async () => {
    mockGetList.mockResolvedValue({
      data: [
        {
          id: "1",
          device_id: "DEV-001",
          name: "Reader 1",
          device_type: "rfid_reader",
        },
      ],
    });
    mockGetOne.mockResolvedValue({
      id: "1",
      device_id: "DEV-001",
      name: "Reader 1",
      device_type: "rfid_reader",
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Reader 1")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Reader 1"));
    await waitFor(() =>
      expect(screen.getByTestId("detail-modal")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByTestId("edit-button"));

    await waitFor(() => {
      expect(screen.queryByTestId("detail-modal")).not.toBeInTheDocument();
      expect(screen.getByTestId("edit-modal")).toBeInTheDocument();
    });
  });

  it("updates device successfully", async () => {
    mockGetList.mockResolvedValue({
      data: [
        {
          id: "1",
          device_id: "DEV-001",
          name: "Reader 1",
          device_type: "rfid_reader",
        },
      ],
    });
    mockGetOne.mockResolvedValue({
      id: "1",
      device_id: "DEV-001",
      name: "Reader 1",
      device_type: "rfid_reader",
    });
    mockUpdate.mockResolvedValue({});

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Reader 1")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Reader 1"));
    await waitFor(() =>
      expect(screen.getByTestId("detail-modal")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByTestId("edit-button"));
    await waitFor(() =>
      expect(screen.getByTestId("edit-modal")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByTestId("save-button"));

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith("1", { name: "Updated Device" });
      expect(mockToastSuccess).toHaveBeenCalled();
      expect(screen.queryByTestId("edit-modal")).not.toBeInTheDocument();
      expect(screen.getByTestId("detail-modal")).toBeInTheDocument();
    });
  });

  it("closes edit modal", async () => {
    mockGetList.mockResolvedValue({
      data: [
        {
          id: "1",
          device_id: "DEV-001",
          name: "Reader 1",
          device_type: "rfid_reader",
        },
      ],
    });
    mockGetOne.mockResolvedValue({
      id: "1",
      device_id: "DEV-001",
      name: "Reader 1",
      device_type: "rfid_reader",
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Reader 1")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Reader 1"));
    await waitFor(() =>
      expect(screen.getByTestId("detail-modal")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByTestId("edit-button"));
    await waitFor(() =>
      expect(screen.getByTestId("edit-modal")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByTestId("edit-close"));

    await waitFor(() => {
      expect(screen.queryByTestId("edit-modal")).not.toBeInTheDocument();
    });
  });

  it("triggers delete confirmation", async () => {
    mockGetList.mockResolvedValue({
      data: [
        {
          id: "1",
          device_id: "DEV-001",
          name: "Reader 1",
          device_type: "rfid_reader",
        },
      ],
    });
    mockGetOne.mockResolvedValue({
      id: "1",
      device_id: "DEV-001",
      name: "Reader 1",
      device_type: "rfid_reader",
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Reader 1")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Reader 1"));
    await waitFor(() =>
      expect(screen.getByTestId("detail-modal")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByTestId("delete-button"));

    expect(mockHandleDeleteClick).toHaveBeenCalled();
  });

  it("shows delete confirmation modal", async () => {
    mockShowConfirmModal.mockReturnValue(true);
    mockGetList.mockResolvedValue({
      data: [
        {
          id: "1",
          device_id: "DEV-001",
          name: "Reader 1",
          device_type: "rfid_reader",
        },
      ],
    });
    mockGetOne.mockResolvedValue({
      id: "1",
      device_id: "DEV-001",
      name: "Reader 1",
      device_type: "rfid_reader",
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Reader 1")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Reader 1"));
    await waitFor(() =>
      expect(screen.getByTestId("detail-modal")).toBeInTheDocument(),
    );

    await waitFor(() => {
      expect(screen.getByTestId("confirm-modal")).toBeInTheDocument();
    });
  });

  it("deletes device successfully", async () => {
    mockShowConfirmModal.mockReturnValue(true);
    mockGetList.mockResolvedValue({
      data: [
        {
          id: "1",
          device_id: "DEV-001",
          name: "Reader 1",
          device_type: "rfid_reader",
        },
      ],
    });
    mockGetOne.mockResolvedValue({
      id: "1",
      device_id: "DEV-001",
      name: "Reader 1",
      device_type: "rfid_reader",
    });
    mockDelete.mockResolvedValue({});
    mockConfirmDelete.mockImplementation((fn: () => void) => fn());

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Reader 1")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Reader 1"));
    await waitFor(() =>
      expect(screen.getByTestId("confirm-modal")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByTestId("confirm-delete"));

    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalledWith("1");
      expect(mockToastSuccess).toHaveBeenCalled();
    });
  });

  it("cancels delete confirmation", async () => {
    mockShowConfirmModal.mockReturnValue(true);
    mockGetList.mockResolvedValue({
      data: [
        {
          id: "1",
          device_id: "DEV-001",
          name: "Reader 1",
          device_type: "rfid_reader",
        },
      ],
    });
    mockGetOne.mockResolvedValue({
      id: "1",
      device_id: "DEV-001",
      name: "Reader 1",
      device_type: "rfid_reader",
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Reader 1")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Reader 1"));
    await waitFor(() =>
      expect(screen.getByTestId("confirm-modal")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByTestId("cancel-delete"));

    expect(mockHandleDeleteCancel).toHaveBeenCalled();
    expect(mockDelete).not.toHaveBeenCalled();
  });
});
