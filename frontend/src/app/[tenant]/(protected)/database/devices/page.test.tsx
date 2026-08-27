import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useState, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import DevicesPage from "./page";

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
const setSelectedDevice = (id: string | null) => {
  currentSearch = new URLSearchParams();
  if (id) {
    currentSearch.set("device", id);
  }
};

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: vi.fn(() => ({ push: vi.fn(), replace: mockReplace })),
  usePathname: vi.fn(() => "/tenant/database/devices"),
  useSearchParams: () => currentSearch,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  mutate: vi.fn(),
  useTenantMutate: vi.fn(() => vi.fn()),
}));

const mockGetList = vi.fn();
const mockGetOne = vi.fn();
const mockCreate = vi.fn();
const mockUpdate = vi.fn();
const mockDelete = vi.fn();
vi.mock("@/lib/database/service-factory", () => ({
  createCrudService: vi.fn(() => ({
    getList: mockGetList,
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
    onClearAllFilters,
    actionButton,
  }: {
    search: { value: string; onChange: (value: string) => void };
    onClearAllFilters: () => void;
    actionButton?: ReactNode;
  }) => (
    <div data-testid="page-header">
      <input
        data-testid="search-input"
        value={search.value}
        onChange={(event) => search.onChange(event.target.value)}
      />
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
  // One mock serves both the create and the edit instance; the edit modal is
  // the one that receives initialData. Mirrors what DatabaseForm does in
  // production: catches the rejection from onSubmit and renders the message
  // inline. Tests assert against the resulting message, not the transport
  // mechanism.
  DatabaseFormModal: ({
    isOpen,
    onClose,
    onSubmit,
    initialData,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSubmit: (data: { name?: string; device_id?: string }) => Promise<void>;
    initialData?: unknown;
  }) => {
    const isEdit = initialData !== undefined;
    const [error, setError] = useState<string | null>(null);
    const submit = (data: { name?: string; device_id?: string }) => {
      setError(null);
      void onSubmit(data).catch((err: unknown) => {
        setError(err instanceof Error ? err.message : String(err));
      });
    };
    const handleClose = () => {
      setError(null);
      onClose();
    };
    if (!isOpen) return null;
    return isEdit ? (
      <div data-testid="device-edit-modal">
        {error ? <span data-testid="edit-error">{error}</span> : null}
        <button
          type="button"
          data-testid="submit-edit"
          onClick={() => submit({ name: "Updated Device" })}
        >
          Save
        </button>
        <button
          type="button"
          data-testid="submit-edit-duplicate"
          onClick={() =>
            submit({ name: "Updated Device", device_id: "duplicate-id" })
          }
        >
          Save Duplicate
        </button>
        <button
          type="button"
          data-testid="close-edit-modal"
          onClick={handleClose}
        >
          Close
        </button>
      </div>
    ) : (
      <div data-testid="device-create-modal">
        {error ? <span data-testid="create-error">{error}</span> : null}
        <button
          type="button"
          data-testid="submit-create"
          onClick={() => submit({ device_id: "new-device" })}
        >
          Submit
        </button>
        <button
          type="button"
          data-testid="submit-create-duplicate"
          onClick={() => submit({ device_id: "duplicate-id" })}
        >
          Submit Duplicate
        </button>
        <button
          type="button"
          data-testid="close-create-modal"
          onClick={handleClose}
        >
          Close
        </button>
      </div>
    );
  },
}));

vi.mock("@/components/devices/devices-master-detail", () => ({
  DevicesMasterDetail: ({
    groupDefinitions,
    selectedId,
    selectedDevice,
    onSelect,
    onEditClick,
    onDeleteClick,
  }: {
    groupDefinitions: Array<{
      id: string;
      title: string;
      items: Array<{ id: string; name?: string; device_id: string }>;
    }>;
    selectedId: string | null;
    selectedDevice?: {
      name?: string;
      device_id: string;
      api_key?: string;
      room_name?: string | null;
    } | null;
    onSelect: (id: string | null) => void;
    onEditClick: () => void;
    onDeleteClick: () => void;
  }) => (
    <div data-testid="devices-master-detail">
      {groupDefinitions.map((group) => (
        <div key={group.id} data-testid={`group-${group.id}`}>
          <span data-testid={`group-title-${group.id}`}>{group.title}</span>
          {group.items.map((device) => (
            <button
              type="button"
              key={device.id}
              data-testid={`device-row-${device.id}`}
              onClick={() => onSelect(device.id)}
            >
              {device.name ?? device.device_id}
            </button>
          ))}
        </div>
      ))}
      {selectedId ? (
        <div data-testid="device-detail-panel">
          <span data-testid="detail-selected-id">{selectedId}</span>
          <span data-testid="detail-device-name">
            {selectedDevice?.name ?? selectedDevice?.device_id ?? "unbekannt"}
          </span>
          <span data-testid="detail-device-room">
            {selectedDevice?.room_name ?? "kein-raum"}
          </span>
          {selectedDevice?.api_key ? (
            <span data-testid="detail-api-key">{selectedDevice.api_key}</span>
          ) : null}
          <button
            type="button"
            data-testid="trigger-edit"
            onClick={onEditClick}
          >
            Edit
          </button>
          <button
            type="button"
            data-testid="trigger-delete"
            onClick={onDeleteClick}
          >
            Delete
          </button>
          <button
            type="button"
            data-testid="trigger-deselect"
            onClick={() => onSelect(null)}
          >
            Close
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

type Device = {
  id: string;
  device_id: string;
  device_type: string;
  name?: string;
  status: string;
  is_online: boolean;
  room_name?: string | null;
  api_key?: string;
  created_at?: string;
  updated_at?: string;
};

const mockDevices: Device[] = [
  {
    id: "1",
    device_id: "kiosk-001",
    device_type: "kiosk",
    name: "Eingang Kiosk",
    status: "active",
    is_online: true,
    room_name: "Foyer",
    created_at: "2026-01-01",
    updated_at: "2026-01-02",
  },
  {
    id: "2",
    device_id: "kiosk-002",
    device_type: "kiosk",
    name: "Backup Kiosk",
    status: "offline",
    is_online: false,
    room_name: null,
    created_at: "2026-01-01",
    updated_at: "2026-01-02",
  },
];

function setSwrData(devices: Device[]) {
  vi.mocked(useSWRAuth).mockReturnValue({
    data: devices,
    isLoading: false,
    error: null,
    isValidating: false,
    mutate: vi.fn(),
  } as ReturnType<typeof useSWRAuth>);
}

describe("DevicesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    currentSearch = new URLSearchParams();
    setSwrData(mockDevices);
  });

  it("renders the page with devices data", async () => {
    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Eingang Kiosk")).toBeInTheDocument();
      expect(screen.getByText("Backup Kiosk")).toBeInTheDocument();
    });
  });

  it("shows error message when fetch fails", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error("Failed to fetch"),
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    render(<DevicesPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Laden der Geräte/),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state when no devices exist", async () => {
    setSwrData([]);

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Geräte vorhanden")).toBeInTheDocument();
    });
  });

  it("filters devices by search term", async () => {
    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Eingang Kiosk")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "Backup" },
    });

    await waitFor(() => {
      expect(screen.queryByText("Eingang Kiosk")).not.toBeInTheDocument();
      expect(screen.getByText("Backup Kiosk")).toBeInTheDocument();
    });
  });

  it("clears filters when clear button is clicked", async () => {
    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Eingang Kiosk")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "Backup" },
    });
    fireEvent.click(screen.getByTestId("clear-filters"));

    await waitFor(() => {
      expect(screen.getByTestId("search-input")).toHaveValue("");
    });
  });

  it("opens create modal when add button is clicked", async () => {
    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Eingang Kiosk")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("Gerät registrieren")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("device-create-modal")).toBeInTheDocument();
    });
  });

  it("syncs device selection into the URL when a row is clicked", async () => {
    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Eingang Kiosk")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("device-row-1"));

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith(
        "/tenant/database/devices?device=1",
        { scroll: false },
      );
    });
  });

  it("hydrates the detail panel from the device URL param using the cached list", async () => {
    setSelectedDevice("1");

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("device-detail-panel")).toBeInTheDocument();
      expect(screen.getByTestId("detail-selected-id")).toHaveTextContent("1");
      expect(screen.getByTestId("detail-device-name")).toHaveTextContent(
        "Eingang Kiosk",
      );
      expect(screen.getByTestId("detail-device-room")).toHaveTextContent(
        "Foyer",
      );
    });
    // No per-selection refetch — list DTO already carries every detail field.
    expect(mockGetOne).not.toHaveBeenCalled();
  });

  it("opens the edit modal when the detail panel edit button is clicked", async () => {
    setSelectedDevice("1");

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("device-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-edit"));

    await waitFor(() => {
      expect(screen.getByTestId("device-edit-modal")).toBeInTheDocument();
    });
  });

  it("calls update service when saving the edit modal", async () => {
    setSelectedDevice("1");
    mockUpdate.mockResolvedValueOnce({
      ...mockDevices[0],
      name: "Updated Device",
      room_name: "Werkraum",
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("device-detail-panel")).toBeInTheDocument();
    });

    // Simulate the post-mutate SWR refresh by swapping in the updated list.
    setSwrData([
      {
        ...mockDevices[0]!,
        name: "Updated Device",
        room_name: "Werkraum",
      },
      mockDevices[1]!,
    ]);

    fireEvent.click(screen.getByTestId("trigger-edit"));
    await waitFor(() => {
      expect(screen.getByTestId("device-edit-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-edit"));

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith(
        "1",
        expect.objectContaining({ name: "Updated Device" }),
      );
      expect(screen.getByTestId("detail-device-name")).toHaveTextContent(
        "Updated Device",
      );
      expect(screen.getByTestId("detail-device-room")).toHaveTextContent(
        "Werkraum",
      );
    });
  });

  it("calls delete service after confirming deletion from the detail panel", async () => {
    setSelectedDevice("1");
    mockDelete.mockResolvedValueOnce(null);

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("device-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-delete"));

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("confirm-delete"));

    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalledWith("1");
      expect(mockReplace).toHaveBeenCalledWith("/tenant/database/devices", {
        scroll: false,
      });
    });
  });

  it("shows an error toast when delete returns an error", async () => {
    setSelectedDevice("1");
    mockDelete.mockResolvedValueOnce("Gerät kann nicht gelöscht werden");

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("device-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-delete"));
    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("confirm-delete"));

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Gerät kann nicht gelöscht werden",
      );
    });
  });

  it("preserves the once-only API key in the detail panel after create", async () => {
    mockCreate.mockResolvedValueOnce({
      id: "3",
      device_id: "kiosk-003",
      device_type: "kiosk",
      name: "Neues Kiosk",
      status: "active",
      is_online: true,
      api_key: "secret-token-xyz",
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Eingang Kiosk")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("Gerät registrieren")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("device-create-modal")).toBeInTheDocument();
    });

    // Simulate the post-mutate SWR refresh that adds the new device (without
    // api_key — that field only exists on the create response).
    setSwrData([
      ...mockDevices,
      {
        id: "3",
        device_id: "kiosk-003",
        device_type: "kiosk",
        name: "Neues Kiosk",
        status: "active",
        is_online: true,
      },
    ]);

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(screen.getByTestId("detail-api-key")).toHaveTextContent(
        "secret-token-xyz",
      );
    });
    // No per-selection refetch — the api_key would otherwise be wiped.
    expect(mockGetOne).not.toHaveBeenCalledWith("3");
  });

  it("keeps the api_key snapshot when the URL update lags one render behind setCreatedDevice", async () => {
    // Production Next.js routers update useSearchParams() asynchronously after
    // router.replace, so for one render `selectedId` is still null while
    // `createdDevice` is already populated. Mimic that lag by deferring the
    // currentSearch update to a microtask. Without the `selectedId !== null`
    // guard in the cleanup effect, this race wipes the api_key snapshot before
    // the URL catches up.
    const originalReplace = mockReplace.getMockImplementation();
    mockReplace.mockImplementation((url: string) => {
      const query = url.includes("?") ? (url.split("?")[1] ?? "") : "";
      void Promise.resolve().then(() => {
        currentSearch = new URLSearchParams(query);
      });
    });

    try {
      mockCreate.mockResolvedValueOnce({
        id: "3",
        device_id: "kiosk-003",
        device_type: "kiosk",
        name: "Neues Kiosk",
        status: "active",
        is_online: true,
        api_key: "secret-token-xyz",
      });

      render(<DevicesPage />);

      await waitFor(() => {
        expect(screen.getByText("Eingang Kiosk")).toBeInTheDocument();
      });

      fireEvent.click(screen.getAllByLabelText("Gerät registrieren")[0]!);
      await waitFor(() => {
        expect(screen.getByTestId("device-create-modal")).toBeInTheDocument();
      });

      setSwrData([
        ...mockDevices,
        {
          id: "3",
          device_id: "kiosk-003",
          device_type: "kiosk",
          name: "Neues Kiosk",
          status: "active",
          is_online: true,
        },
      ]);

      fireEvent.click(screen.getByTestId("submit-create"));

      await waitFor(() => {
        expect(screen.getByTestId("detail-selected-id")).toHaveTextContent("3");
      });
      expect(screen.getByTestId("detail-api-key")).toHaveTextContent(
        "secret-token-xyz",
      );
    } finally {
      if (originalReplace) {
        mockReplace.mockImplementation(originalReplace);
      }
    }
  });

  it("drops the flashed API key after the created device is closed and reopened", async () => {
    mockCreate.mockResolvedValueOnce({
      id: "3",
      device_id: "kiosk-003",
      device_type: "kiosk",
      name: "Neues Kiosk",
      status: "active",
      is_online: true,
      api_key: "secret-token-xyz",
    });

    const { rerender } = render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Eingang Kiosk")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("Gerät registrieren")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("device-create-modal")).toBeInTheDocument();
    });

    setSwrData([
      ...mockDevices,
      {
        id: "3",
        device_id: "kiosk-003",
        device_type: "kiosk",
        name: "Neues Kiosk",
        status: "active",
        is_online: true,
      },
    ]);

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(screen.getByTestId("detail-api-key")).toHaveTextContent(
        "secret-token-xyz",
      );
    });

    fireEvent.click(screen.getByTestId("trigger-deselect"));
    rerender(<DevicesPage />);

    await waitFor(() => {
      expect(
        screen.queryByTestId("device-detail-panel"),
      ).not.toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("device-row-3"));
    rerender(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("detail-selected-id")).toHaveTextContent("3");
      expect(screen.queryByTestId("detail-api-key")).not.toBeInTheDocument();
    });
  });

  it("clears the flashed API key after editing a newly created device", async () => {
    mockCreate.mockResolvedValueOnce({
      id: "3",
      device_id: "kiosk-003",
      device_type: "kiosk",
      name: "Neues Kiosk",
      status: "active",
      is_online: true,
      api_key: "secret-token-xyz",
    });
    mockUpdate.mockResolvedValueOnce({
      id: "3",
      device_id: "kiosk-003",
      device_type: "kiosk",
      name: "Bearbeitetes Kiosk",
      status: "active",
      is_online: true,
      room_name: "Werkraum",
      created_at: "2026-01-01",
      updated_at: "2026-01-03",
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Eingang Kiosk")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("Gerät registrieren")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("device-create-modal")).toBeInTheDocument();
    });

    setSwrData([
      ...mockDevices,
      {
        id: "3",
        device_id: "kiosk-003",
        device_type: "kiosk",
        name: "Neues Kiosk",
        status: "active",
        is_online: true,
        room_name: null,
      },
    ]);

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(screen.getByTestId("detail-api-key")).toHaveTextContent(
        "secret-token-xyz",
      );
    });

    // Edit triggers another SWR refresh with the updated row.
    setSwrData([
      ...mockDevices,
      {
        id: "3",
        device_id: "kiosk-003",
        device_type: "kiosk",
        name: "Bearbeitetes Kiosk",
        status: "active",
        is_online: true,
        room_name: "Werkraum",
      },
    ]);

    fireEvent.click(screen.getByTestId("trigger-edit"));
    await waitFor(() => {
      expect(screen.getByTestId("device-edit-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-edit"));

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith(
        "3",
        expect.objectContaining({ name: "Updated Device" }),
      );
      expect(screen.getByTestId("detail-device-name")).toHaveTextContent(
        "Bearbeitetes Kiosk",
      );
      expect(screen.getByTestId("detail-device-room")).toHaveTextContent(
        "Werkraum",
      );
      expect(screen.queryByTestId("detail-api-key")).not.toBeInTheDocument();
    });
  });

  it("propagates update failures so the edit modal stays open and no success toast fires", async () => {
    setSelectedDevice("1");
    mockUpdate.mockRejectedValueOnce(new Error("backend exploded"));

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("device-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-edit"));
    await waitFor(() => {
      expect(screen.getByTestId("device-edit-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-edit"));

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith(
        "1",
        expect.objectContaining({ name: "Updated Device" }),
      );
    });
    // Modal must remain mounted on failure (the page rethrows so the modal
    // can show its own error UI and let the user retry).
    expect(screen.getByTestId("device-edit-modal")).toBeInTheDocument();
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("translates duplicate-ID conflicts on update into a German hint and renders inline (Issue #1356)", async () => {
    setSelectedDevice("1");
    mockUpdate.mockRejectedValueOnce(
      new Error(
        "IoT service error in UpdateDevice: Diese Geräte-ID ist bereits vergeben",
      ),
    );

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("device-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-edit"));
    await waitFor(() => {
      expect(screen.getByTestId("device-edit-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-edit-duplicate"));

    await waitFor(() => {
      expect(screen.getByTestId("edit-error")).toHaveTextContent(
        /Die Geräte-ID "duplicate-id" ist bereits vergeben/,
      );
    });
    // The modal must NOT close on a duplicate so the user can correct the ID.
    expect(screen.getByTestId("device-edit-modal")).toBeInTheDocument();
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("logs the stringified value when create rejects with a non-Error", async () => {
    // Exercises the `err instanceof Error : false` ternary branch in the
    // create catch — handler logs `String(err)` and re-throws the fallback.
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});
    mockCreate.mockRejectedValueOnce("plain-string-error");

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Eingang Kiosk")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("Gerät registrieren")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("device-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith("device_create_failed", {
        error: "plain-string-error",
      });
    });
    consoleError.mockRestore();
  });

  it("logs the stringified value when update rejects with a non-Error", async () => {
    setSelectedDevice("1");
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});
    mockUpdate.mockRejectedValueOnce("plain-string-error");

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("device-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-edit"));
    await waitFor(() => {
      expect(screen.getByTestId("device-edit-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-edit"));

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith("failed to update device", {
        device_id: "1",
        error: "plain-string-error",
      });
    });
    consoleError.mockRestore();
  });

  it("matches duplicate-ID errors on update via the HTTP 409 branch", async () => {
    setSelectedDevice("1");
    mockUpdate.mockRejectedValueOnce(
      new Error("Request failed with status 409"),
    );

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("device-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-edit"));
    await waitFor(() => {
      expect(screen.getByTestId("device-edit-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-edit-duplicate"));

    await waitFor(() => {
      expect(screen.getByTestId("edit-error")).toHaveTextContent(
        /bereits vergeben/,
      );
    });
    // Modal stays open so the user can correct the ID.
    expect(screen.getByTestId("device-edit-modal")).toBeInTheDocument();
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("surfaces a German duplicate-ID hint that includes the rejected device_id", async () => {
    mockCreate.mockRejectedValueOnce(
      new Error(
        "IoT service error in CreateDevice: Diese Geräte-ID ist bereits vergeben",
      ),
    );

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Eingang Kiosk")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("Gerät registrieren")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("device-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create-duplicate"));

    await waitFor(() => {
      expect(screen.getByTestId("create-error")).toHaveTextContent(
        /Die Geräte-ID "duplicate-id" ist bereits vergeben/,
      );
    });
    // The modal must NOT close on a duplicate so the user can correct the ID.
    expect(screen.getByTestId("device-create-modal")).toBeInTheDocument();
    // No success toast on failure.
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("matches duplicate-ID errors when the backend uses an HTTP 409 message", async () => {
    mockCreate.mockRejectedValueOnce(
      new Error("Request failed with status 409"),
    );

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Eingang Kiosk")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("Gerät registrieren")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("device-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create-duplicate"));

    await waitFor(() => {
      expect(screen.getByTestId("create-error")).toHaveTextContent(
        /bereits vergeben/,
      );
    });
  });

  it("falls back to a generic error when the failure is not a duplicate", async () => {
    mockCreate.mockRejectedValueOnce(new Error("network unreachable"));

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Eingang Kiosk")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("Gerät registrieren")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("device-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(screen.getByTestId("create-error")).toHaveTextContent(
        /Fehler beim Erstellen des Geräts/,
      );
    });
  });

  it("clears the create error when the user closes the create modal", async () => {
    mockCreate.mockRejectedValueOnce(new Error("network unreachable"));

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Eingang Kiosk")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("Gerät registrieren")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("device-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create"));
    await waitFor(() => {
      expect(screen.getByTestId("create-error")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("close-create-modal"));
    await waitFor(() => {
      expect(
        screen.queryByTestId("device-create-modal"),
      ).not.toBeInTheDocument();
    });

    // Reopen the modal: the previous error must be gone.
    fireEvent.click(screen.getAllByLabelText("Gerät registrieren")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("device-create-modal")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("create-error")).not.toBeInTheDocument();
  });

  it("keeps the created device detail mounted when the current search hides it", async () => {
    mockCreate.mockResolvedValueOnce({
      id: "3",
      device_id: "kiosk-003",
      device_type: "kiosk",
      name: "Neues Kiosk",
      status: "active",
      is_online: true,
      api_key: "secret-token-xyz",
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Eingang Kiosk")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "unmatched-filter" },
    });

    await waitFor(() => {
      expect(screen.getByText("Keine Geräte gefunden")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("Gerät registrieren")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("device-create-modal")).toBeInTheDocument();
    });

    setSwrData([
      ...mockDevices,
      {
        id: "3",
        device_id: "kiosk-003",
        device_type: "kiosk",
        name: "Neues Kiosk",
        status: "active",
        is_online: true,
      },
    ]);

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(screen.getByTestId("detail-selected-id")).toHaveTextContent("3");
      expect(screen.getByTestId("detail-api-key")).toHaveTextContent(
        "secret-token-xyz",
      );
    });
  });
});
