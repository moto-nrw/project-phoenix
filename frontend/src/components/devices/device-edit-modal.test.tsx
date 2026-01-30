/**
 * Tests for DeviceEditModal
 * Tests the rendering and functionality of the device edit modal
 */
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { DeviceEditModal } from "./device-edit-modal";
import type { Device } from "@/lib/iot-helpers";

// Mock Modal component
vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    onClose,
    title,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <h1>{title}</h1>
        <button onClick={onClose}>Close</button>
        {children}
      </div>
    ) : null,
}));

// Mock DatabaseForm component
vi.mock("~/components/ui/database/database-form", () => ({
  DatabaseForm: ({
    onSubmit,
    onCancel,
    submitLabel,
  }: {
    onSubmit: (data: unknown) => Promise<void>;
    onCancel: () => void;
    submitLabel: string;
  }) => (
    <div data-testid="database-form">
      <button onClick={() => onSubmit({ name: "Updated Device" })}>
        {submitLabel}
      </button>
      <button onClick={onCancel}>Cancel</button>
    </div>
  ),
}));

// Mock devicesConfig
vi.mock("@/lib/database/configs/devices.config", () => ({
  devicesConfig: {
    labels: {
      editModalTitle: "Gerät bearbeiten",
    },
    theme: {
      primary: "#eab308",
    },
    form: {
      sections: [
        {
          title: "Geräteinformationen",
          fields: [
            {
              name: "device_id",
              label: "Geräte-ID",
              type: "text",
              required: true,
            },
          ],
        },
      ],
    },
  },
}));

describe("DeviceEditModal", () => {
  const mockOnClose = vi.fn();
  const mockOnSave = vi.fn();

  const mockDevice: Device = {
    id: "1",
    device_id: "DEV001",
    name: "Test Device",
    device_type: "rfid_reader",
    status: "active",
    is_online: true,
    last_seen: "2024-01-01T12:00:00Z",
    created_at: "2024-01-01T10:00:00Z",
    updated_at: "2024-01-01T12:00:00Z",
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when modal is closed", () => {
    render(
      <DeviceEditModal
        isOpen={false}
        onClose={mockOnClose}
        device={mockDevice}
        onSave={mockOnSave}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders nothing when device is null", () => {
    render(
      <DeviceEditModal
        isOpen={true}
        onClose={mockOnClose}
        device={null}
        onSave={mockOnSave}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders modal when isOpen is true and device exists", async () => {
    render(
      <DeviceEditModal
        isOpen={true}
        onClose={mockOnClose}
        device={mockDevice}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByText("Gerät bearbeiten")).toBeInTheDocument();
    });
  });

  it("shows loading state when loading prop is true", async () => {
    render(
      <DeviceEditModal
        isOpen={true}
        onClose={mockOnClose}
        device={mockDevice}
        onSave={mockOnSave}
        loading={true}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Daten werden geladen...")).toBeInTheDocument();
    });
  });

  it("renders DatabaseForm when not loading", async () => {
    render(
      <DeviceEditModal
        isOpen={true}
        onClose={mockOnClose}
        device={mockDevice}
        onSave={mockOnSave}
        loading={false}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("database-form")).toBeInTheDocument();
      expect(screen.getByText("Speichern")).toBeInTheDocument();
    });
  });
});
