/**
 * Tests for GroupCreateModal
 * Tests the rendering and functionality of the group creation modal
 */
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { GroupCreateModal } from "./group-create-modal";

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
      <button onClick={() => onSubmit({ name: "Test Group" })}>
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
      createModalTitle: "Neue Gruppe",
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
      defaultValues: {},
    },
  },
}));

describe("GroupCreateModal", () => {
  const mockOnClose = vi.fn();
  const mockOnCreate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when modal is closed", () => {
    render(
      <GroupCreateModal
        isOpen={false}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders modal when isOpen is true", async () => {
    render(
      <GroupCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByText("Neue Gruppe")).toBeInTheDocument();
    });
  });

  it("shows loading state when loading prop is true", async () => {
    render(
      <GroupCreateModal
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
      <GroupCreateModal
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
