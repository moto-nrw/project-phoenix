/**
 * Tests for DeviceDetailModal
 * Tests the rendering and functionality of the device detail modal
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { DeviceDetailModal } from "./device-detail-modal";
import type { Device } from "@/lib/iot-helpers";

// Mock Modal component
vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    onClose,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <button onClick={onClose}>Close</button>
        {children}
      </div>
    ) : null,
}));

// Mock DetailModalActions component
vi.mock("~/components/ui/detail-modal-actions", () => ({
  DetailModalActions: ({
    onEdit,
    onDelete,
  }: {
    onEdit: () => void;
    onDelete: () => void;
    entityName: string;
    entityType: string;
    onDeleteClick?: () => void;
  }) => (
    <div data-testid="detail-modal-actions">
      <button onClick={onEdit}>Edit</button>
      <button onClick={onDelete}>Delete</button>
    </div>
  ),
}));

// Mock iot-helpers
vi.mock("@/lib/iot-helpers", async () => {
  const actual = await vi.importActual("@/lib/iot-helpers");
  return {
    ...actual,
    getDeviceStatusDisplayName: (status: string) => {
      const map: Record<string, string> = {
        active: "Aktiv",
        inactive: "Inaktiv",
        maintenance: "Wartung",
      };
      return map[status] ?? status;
    },
    getDeviceTypeDisplayName: (type: string) => {
      const map: Record<string, string> = {
        rfid_reader: "RFID-Lesegerät",
        sensor: "Sensor",
      };
      return map[type] ?? type;
    },
    formatLastSeen: (date: string | null) => {
      if (!date) return "Nie";
      return "Vor 5 Minuten";
    },
  };
});

describe("DeviceDetailModal", () => {
  const mockOnClose = vi.fn();
  const mockOnEdit = vi.fn();
  const mockOnDelete = vi.fn();

  const mockDevice: Device = {
    id: "1",
    device_id: "DEV001",
    name: "Test Device",
    device_type: "rfid_reader",
    status: "active",
    is_online: true,
    last_seen: "2024-01-01T12:00:00Z",
    api_key: "test-api-key-12345",
    created_at: "2024-01-01T10:00:00Z",
    updated_at: "2024-01-01T11:00:00Z",
  };

  // Mock clipboard API
  const mockWriteText = vi.fn().mockResolvedValue(undefined);

  beforeEach(() => {
    vi.clearAllMocks();
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: mockWriteText },
      writable: true,
      configurable: true,
    });
  });

  it("renders nothing when modal is closed", () => {
    render(
      <DeviceDetailModal
        isOpen={false}
        onClose={mockOnClose}
        device={mockDevice}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders nothing when device is null", () => {
    render(
      <DeviceDetailModal
        isOpen={true}
        onClose={mockOnClose}
        device={null}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders modal with device details when open", async () => {
    render(
      <DeviceDetailModal
        isOpen={true}
        onClose={mockOnClose}
        device={mockDevice}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByText("Test Device")).toBeInTheDocument();
      expect(screen.getByText("RFID-Lesegerät")).toBeInTheDocument();
      expect(screen.getByText("DEV001")).toBeInTheDocument();
      expect(screen.getByText("Aktiv")).toBeInTheDocument();
      expect(screen.getAllByText("Online").length).toBeGreaterThan(0);
    });
  });

  it("shows loading state when loading prop is true", async () => {
    render(
      <DeviceDetailModal
        isOpen={true}
        onClose={mockOnClose}
        device={mockDevice}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
        loading={true}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Daten werden geladen...")).toBeInTheDocument();
    });
  });

  it("renders modal actions when not loading", async () => {
    render(
      <DeviceDetailModal
        isOpen={true}
        onClose={mockOnClose}
        device={mockDevice}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
        loading={false}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("detail-modal-actions")).toBeInTheDocument();
    });
  });

  it("displays offline status correctly", async () => {
    const offlineDevice: Device = {
      ...mockDevice,
      is_online: false,
    };

    render(
      <DeviceDetailModal
        isOpen={true}
        onClose={mockOnClose}
        device={offlineDevice}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Offline")).toBeInTheDocument();
    });
  });

  it("displays API key section when api_key is present", async () => {
    render(
      <DeviceDetailModal
        isOpen={true}
        onClose={mockOnClose}
        device={mockDevice}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByText("API-Schlüssel (nur einmal sichtbar)"),
      ).toBeInTheDocument();
      expect(screen.getByText("Anzeigen")).toBeInTheDocument();
      expect(screen.getByText("Kopieren")).toBeInTheDocument();
    });
  });

  it("does not display API key section when api_key is null", async () => {
    const deviceWithoutApiKey: Device = {
      ...mockDevice,
      api_key: undefined,
    };

    render(
      <DeviceDetailModal
        isOpen={true}
        onClose={mockOnClose}
        device={deviceWithoutApiKey}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(
        screen.queryByText("API-Schlüssel (nur einmal sichtbar)"),
      ).not.toBeInTheDocument();
    });
  });

  it("toggles API key visibility when show/hide button is clicked", async () => {
    render(
      <DeviceDetailModal
        isOpen={true}
        onClose={mockOnClose}
        device={mockDevice}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Anzeigen")).toBeInTheDocument();
    });

    const showButton = screen.getByText("Anzeigen");
    fireEvent.click(showButton);

    await waitFor(() => {
      expect(screen.getByText("Verbergen")).toBeInTheDocument();
    });
  });

  it("copies API key to clipboard when copy button is clicked", async () => {
    render(
      <DeviceDetailModal
        isOpen={true}
        onClose={mockOnClose}
        device={mockDevice}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Kopieren")).toBeInTheDocument();
    });

    const copyButton = screen.getByText("Kopieren");
    fireEvent.click(copyButton);

    await waitFor(() => {
      expect(mockWriteText).toHaveBeenCalledWith("test-api-key-12345");
    });
  });

  it("uses device_id as fallback name when name is not provided", async () => {
    const deviceWithoutName: Device = {
      ...mockDevice,
      name: undefined,
    };

    render(
      <DeviceDetailModal
        isOpen={true}
        onClose={mockOnClose}
        device={deviceWithoutName}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      const headings = screen.getAllByText("DEV001");
      expect(headings.length).toBeGreaterThan(0);
    });
  });
});
