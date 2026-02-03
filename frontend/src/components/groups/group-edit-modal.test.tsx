/**
 * Tests for GroupEditModal
 * Tests the rendering and functionality of the group edit modal
 */
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { GroupEditModal } from "./group-edit-modal";
import type { Group } from "@/lib/group-helpers";

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
      <button onClick={() => onSubmit({ name: "Updated Group" })}>
        {submitLabel}
      </button>
      <button onClick={onCancel}>Cancel</button>
    </div>
  ),
}));

// Mock groupsConfig
vi.mock("@/lib/database/configs/groups.config", () => ({
  groupsConfig: {
    labels: {
      editModalTitle: "Gruppe bearbeiten",
    },
    theme: {
      primary: "#83CD2D",
    },
    form: {
      sections: [
        {
          title: "Grundinformationen",
          fields: [
            {
              name: "name",
              label: "Gruppenname",
              type: "text",
              required: true,
            },
          ],
        },
      ],
    },
  },
}));

describe("GroupEditModal", () => {
  const mockOnClose = vi.fn();
  const mockOnSave = vi.fn();

  const mockGroup: Group = {
    id: "1",
    name: "Test Group",
    room_name: "Room A",
    student_count: 15,
    supervisors: [],
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when modal is closed", () => {
    render(
      <GroupEditModal
        isOpen={false}
        onClose={mockOnClose}
        group={mockGroup}
        onSave={mockOnSave}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders nothing when group is null", () => {
    render(
      <GroupEditModal
        isOpen={true}
        onClose={mockOnClose}
        group={null}
        onSave={mockOnSave}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders modal when isOpen is true and group exists", async () => {
    render(
      <GroupEditModal
        isOpen={true}
        onClose={mockOnClose}
        group={mockGroup}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByText("Gruppe bearbeiten")).toBeInTheDocument();
    });
  });

  it("shows loading state when loading prop is true", async () => {
    render(
      <GroupEditModal
        isOpen={true}
        onClose={mockOnClose}
        group={mockGroup}
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
      <GroupEditModal
        isOpen={true}
        onClose={mockOnClose}
        group={mockGroup}
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
