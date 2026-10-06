import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useState, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import DevicesPage from "./page";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

/** Text of a FormErrorInput, as the shared kit components render it. */
function errorText(error: unknown): string | null {
  if (!error) return null;
  return typeof error === "string"
    ? error
    : (error as { message: string }).message;
}

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
const mockRemove = vi.fn();
vi.mock("@/lib/database/service-factory", () => {
  const service = () => ({
    getList: mockGetList,
    getOne: mockGetOne,
    create: mockCreate,
    update: mockUpdate,
    remove: mockRemove,
  });
  return {
    createCrudService: vi.fn(service),
  };
});

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
  // production: catches the rejection from onSubmit and hands it to the
  // shared error path, whose catalog text it renders inline.
  DatabaseFormModal: ({
    isOpen,
    onClose,
    onSubmit,
    initialData,
    errorPath,
    errorObject,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSubmit: (data: { name?: string; device_id?: string }) => Promise<void>;
    initialData?: unknown;
    errorPath?: {
      error: unknown;
      show: (error: unknown, options: { object: string }) => unknown;
    };
    errorObject?: string;
  }) => {
    const isEdit = initialData !== undefined;
    const submit = (data: { name?: string; device_id?: string }) => {
      void onSubmit(data).catch((err: unknown) => {
        void errorPath?.show(err, { object: errorObject ?? "" });
      });
    };
    const handleClose = () => onClose();
    const error = errorText(errorPath?.error);
    if (!isOpen || isEdit) return null;
    return (
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
  // Bearbeitet wird im Detailbereich (BAUARTEN-SPEC Bauart 2 Regel 3). Der
  // Doppel steht fuer die eingebettete DatabaseForm: sie faengt die Ablehnung
  // von onSaveDevice ab und zeigt die Meldung im Formular.
  DevicesMasterDetail: ({
    groupDefinitions,
    selectedId,
    selectedDevice,
    onSelect,
    onSaveDevice,
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
    onSaveDevice: (data: {
      name?: string;
      device_id?: string;
    }) => Promise<void>;
    onDeleteClick: () => void;
  }) => {
    const [editing, setEditing] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const submit = (data: { name?: string; device_id?: string }) => {
      setError(null);
      void onSaveDevice(data)
        .then(() => setEditing(false))
        .catch((err: unknown) => {
          setError(err instanceof Error ? err.message : String(err));
        });
    };
    return (
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
              onClick={() => setEditing(true)}
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
            {editing ? (
              <div data-testid="device-edit-form">
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
                    submit({
                      name: "Updated Device",
                      device_id: "duplicate-id",
                    })
                  }
                >
                  Save Duplicate
                </button>
                <button
                  type="button"
                  data-testid="cancel-edit"
                  onClick={() => setEditing(false)}
                >
                  Cancel
                </button>
              </div>
            ) : null}
          </div>
        ) : null}
      </div>
    );
  },
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
      error: new ApiError("Failed to fetch", 503, {
        code: "general.unavailable",
      }),
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    render(<DevicesPage />);

    expect(await screen.findByTestId("page-error")).toHaveTextContent(
      catalogText("general.unavailable", "die Liste der Geräte"),
    );
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

  it("opens the inline edit form when the detail panel edit button is clicked", async () => {
    setSelectedDevice("1");

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("device-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-edit"));

    await waitFor(() => {
      expect(screen.getByTestId("device-edit-form")).toBeInTheDocument();
    });
  });

  it("calls update service when saving the inline edit form", async () => {
    setSelectedDevice("1");
    mockUpdate.mockResolvedValueOnce({
      ...mockDevices[0],
      name: "Updated Device",
      room_name: "Werkraum",
    });

    const { rerender } = render(<DevicesPage />);

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
      expect(screen.getByTestId("device-edit-form")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-edit"));

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith(
        "1",
        expect.objectContaining({ name: "Updated Device" }),
      );
    });
    // SWR re-renders the page with the revalidated list.
    rerender(<DevicesPage />);
    await waitFor(() => {
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
    mockRemove.mockResolvedValueOnce(true);

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
      expect(mockRemove).toHaveBeenCalledWith("1");
      expect(mockReplace).toHaveBeenCalledWith("/tenant/database/devices", {
        scroll: false,
      });
    });
  });

  it("keeps a delete error in the confirmation dialog", async () => {
    setSelectedDevice("1");
    mockRemove.mockRejectedValueOnce(
      new ApiError("protected", 403, { code: "general.permission" }),
    );

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("device-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-delete"));
    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("confirm-delete"));

    expect(await screen.findByTestId("delete-error")).toHaveTextContent(
      catalogText("general.permission", "das Löschen des Geräts"),
    );
    expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
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
      expect(screen.getByTestId("device-edit-form")).toBeInTheDocument();
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

  it("propagates update failures so the inline edit form stays open and no success toast fires", async () => {
    setSelectedDevice("1");
    mockUpdate.mockRejectedValueOnce(new Error("backend exploded"));

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("device-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-edit"));
    await waitFor(() => {
      expect(screen.getByTestId("device-edit-form")).toBeInTheDocument();
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
    expect(screen.getByTestId("device-edit-form")).toBeInTheDocument();
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("shows a taken device ID with its own text in the dialog (Issue #1356)", async () => {
    mockCreate.mockRejectedValueOnce(
      new ApiError("device id taken", 409, {
        code: "iot.device_id_taken",
        errors: [{ field: "device_id", reason: "taken" }],
      }),
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
        catalogText("iot.device_id_taken", "das Gerät"),
      );
    });
    // The modal must NOT close on a duplicate so the user can correct the ID.
    expect(screen.getByTestId("device-create-modal")).toBeInTheDocument();
    // No success toast on failure.
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("shows the catalog text when create fails for another reason", async () => {
    mockCreate.mockRejectedValueOnce(
      new ApiError("network unreachable", 503, {
        code: "general.unavailable",
      }),
    );

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
        catalogText("general.unavailable", "das Gerät"),
      );
    });
  });

  it("clears the create error when the user closes the create modal", async () => {
    mockCreate.mockRejectedValueOnce(
      new ApiError("network unreachable", 503, {
        code: "general.unavailable",
      }),
    );

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
