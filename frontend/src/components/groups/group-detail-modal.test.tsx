/**
 * Tests for GroupDetailModal
 * Tests the rendering and functionality of the group detail modal
 */
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { GroupDetailModal } from "./group-detail-modal";
import type { Group } from "@/lib/group-helpers";

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

describe("GroupDetailModal", () => {
  const mockOnClose = vi.fn();
  const mockOnEdit = vi.fn();
  const mockOnDelete = vi.fn();

  const mockGroup: Group = {
    id: "1",
    name: "Test Group",
    room_name: "Room A",
    student_count: 15,
    supervisors: [
      { id: "1", name: "John Doe" },
      { id: "2", name: "Jane Smith" },
    ],
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when modal is closed", () => {
    render(
      <GroupDetailModal
        isOpen={false}
        onClose={mockOnClose}
        group={mockGroup}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders nothing when group is null", () => {
    render(
      <GroupDetailModal
        isOpen={true}
        onClose={mockOnClose}
        group={null}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders modal with group details when open", async () => {
    render(
      <GroupDetailModal
        isOpen={true}
        onClose={mockOnClose}
        group={mockGroup}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByText("Test Group")).toBeInTheDocument();
      expect(screen.getAllByText("Room A").length).toBeGreaterThan(0);
      expect(screen.getByText("15")).toBeInTheDocument();
      expect(screen.getByText("John Doe, Jane Smith")).toBeInTheDocument();
    });
  });

  it("shows loading state when loading prop is true", async () => {
    render(
      <GroupDetailModal
        isOpen={true}
        onClose={mockOnClose}
        group={mockGroup}
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
      <GroupDetailModal
        isOpen={true}
        onClose={mockOnClose}
        group={mockGroup}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
        loading={false}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("detail-modal-actions")).toBeInTheDocument();
    });
  });

  it("displays default text when no room assigned", async () => {
    const groupWithoutRoom: Group = {
      ...mockGroup,
      room_name: undefined,
    };

    render(
      <GroupDetailModal
        isOpen={true}
        onClose={mockOnClose}
        group={groupWithoutRoom}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByText("Kein Gruppenraum zugewiesen"),
      ).toBeInTheDocument();
      expect(screen.getByText("Kein Gruppenraum")).toBeInTheDocument();
    });
  });

  it("displays default text when no supervisors assigned", async () => {
    const groupWithoutSupervisors: Group = {
      ...mockGroup,
      supervisors: [],
    };

    render(
      <GroupDetailModal
        isOpen={true}
        onClose={mockOnClose}
        group={groupWithoutSupervisors}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByText("Keine Gruppenleitung zugewiesen"),
      ).toBeInTheDocument();
    });
  });
});
