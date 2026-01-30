/**
 * Tests for DeviceCreateModal
 * Tests the rendering and functionality of the device creation modal
 */
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { DeviceCreateModal } from "./device-create-modal";

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
      <button onClick={() => onSubmit({ device_id: "DEV001" })}>
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
      createModalTitle: "Neues Gerät",
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
      defaultValues: {},
    },
  },
}));

describe("DeviceCreateModal", () => {
  const mockOnClose = vi.fn();
  const mockOnCreate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when modal is closed", () => {
    render(
      <DeviceCreateModal
        isOpen={false}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders modal when isOpen is true", async () => {
    render(
      <DeviceCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByText("Neues Gerät")).toBeInTheDocument();
    });
  });

  it("shows loading state when loading prop is true", async () => {
    render(
      <DeviceCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
        loading={true}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Daten werden geladen...")).toBeInTheDocument();
    });
  });

  it("renders DatabaseForm when not loading", async () => {
    render(
      <DeviceCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
        loading={false}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("database-form")).toBeInTheDocument();
      expect(screen.getByText("Erstellen")).toBeInTheDocument();
    });
  });
});
