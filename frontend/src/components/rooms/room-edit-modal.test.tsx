/**
 * Tests for RoomEditModal
 * Tests the rendering and functionality of the room edit modal
 */
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { RoomEditModal } from "./room-edit-modal";
import type { Room } from "@/lib/room-helpers";

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
      <button onClick={() => onSubmit({ name: "Updated Room" })}>
        {submitLabel}
      </button>
      <button onClick={onCancel}>Cancel</button>
    </div>
  ),
}));

// Mock roomsConfig and configToFormSection
vi.mock("@/lib/database/configs/rooms.config", () => ({
  roomsConfig: {
    labels: {
      editModalTitle: "Raum bearbeiten",
    },
    theme: {
      primary: "#6366f1",
    },
    form: {
      sections: [
        {
          title: "Grundinformationen",
          fields: [
            {
              name: "name",
              label: "Raumname",
              type: "text",
              required: true,
            },
            {
              name: "category",
              label: "Kategorie",
              type: "select",
              options: [
                { value: "Normaler Raum", label: "Normaler Raum" },
                { value: "Gruppenraum", label: "Gruppenraum" },
                { value: "Themenraum", label: "Themenraum" },
                { value: "Sport", label: "Sport" },
              ],
            },
          ],
        },
      ],
    },
  },
}));

vi.mock("@/lib/database/types", () => ({
  configToFormSection: (section: unknown) => section,
}));

describe("RoomEditModal", () => {
  const mockOnClose = vi.fn();
  const mockOnSave = vi.fn();

  const mockRoom: Room = {
    id: "1",
    name: "Test Room",
    category: "Gruppenraum",
    capacity: 20,
    isOccupied: false,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when modal is closed", () => {
    render(
      <RoomEditModal
        isOpen={false}
        onClose={mockOnClose}
        room={mockRoom}
        onSave={mockOnSave}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders nothing when room is null", () => {
    render(
      <RoomEditModal
        isOpen={true}
        onClose={mockOnClose}
        room={null}
        onSave={mockOnSave}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders modal when isOpen is true and room exists", async () => {
    render(
      <RoomEditModal
        isOpen={true}
        onClose={mockOnClose}
        room={mockRoom}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByText("Raum bearbeiten")).toBeInTheDocument();
    });
  });

  it("shows loading state when loading prop is true", async () => {
    render(
      <RoomEditModal
        isOpen={true}
        onClose={mockOnClose}
        room={mockRoom}
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
      <RoomEditModal
        isOpen={true}
        onClose={mockOnClose}
        room={mockRoom}
        onSave={mockOnSave}
        loading={false}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("database-form")).toBeInTheDocument();
      expect(screen.getByText("Speichern")).toBeInTheDocument();
    });
  });

  it("handles room with standard category", async () => {
    render(
      <RoomEditModal
        isOpen={true}
        onClose={mockOnClose}
        room={mockRoom}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("database-form")).toBeInTheDocument();
    });
  });

  it("handles room with legacy category", async () => {
    const roomWithLegacyCategory: Room = {
      ...mockRoom,
      category: "Old Category",
    };

    render(
      <RoomEditModal
        isOpen={true}
        onClose={mockOnClose}
        room={roomWithLegacyCategory}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("database-form")).toBeInTheDocument();
    });
  });
});
