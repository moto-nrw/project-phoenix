/**
 * Tests for ActivityCreateModal
 * Tests the rendering and functionality of the activity creation modal
 */
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ActivityCreateModal } from "./activity-create-modal";

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
      <button onClick={() => onSubmit({ name: "Test Activity" })}>
        {submitLabel}
      </button>
      <button onClick={onCancel}>Cancel</button>
    </div>
  ),
}));

// Mock activitiesConfig and configToFormSection
vi.mock("@/lib/database/configs/activities.config", () => ({
  activitiesConfig: {
    labels: {
      createModalTitle: "Neue Aktivität",
    },
    theme: {
      primary: "#FF3130",
    },
    form: {
      sections: [
        {
          title: "Aktivitätsinformationen",
          fields: [
            {
              name: "name",
              label: "Aktivitätsname",
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

vi.mock("@/lib/database/types", () => ({
  configToFormSection: (section: unknown) => section,
}));

describe("ActivityCreateModal", () => {
  const mockOnClose = vi.fn();
  const mockOnCreate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when modal is closed", () => {
    render(
      <ActivityCreateModal
        isOpen={false}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders modal when isOpen is true", async () => {
    render(
      <ActivityCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByText("Neue Aktivität")).toBeInTheDocument();
    });
  });

  it("shows loading state when loading prop is true", async () => {
    render(
      <ActivityCreateModal
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
      <ActivityCreateModal
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
