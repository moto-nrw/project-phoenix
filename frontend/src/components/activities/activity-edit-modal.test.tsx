/**
 * Tests for ActivityEditModal
 * Tests the rendering and functionality of the activity edit modal
 */
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ActivityEditModal } from "./activity-edit-modal";
import type { Activity } from "@/lib/activity-helpers";

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
      <button onClick={() => onSubmit({ name: "Updated Activity" })}>
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
      editModalTitle: "Aktivität bearbeiten",
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
    },
  },
}));

vi.mock("@/lib/database/types", () => ({
  configToFormSection: (section: unknown) => section,
}));

describe("ActivityEditModal", () => {
  const mockOnClose = vi.fn();
  const mockOnSave = vi.fn();

  const mockActivity: Activity = {
    id: "1",
    name: "Test Activity",
    category_name: "Sports",
    max_participant: 15,
    is_open_ags: false,
    supervisor_id: "1",
    ag_category_id: "1",
    created_at: new Date(),
    updated_at: new Date(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when modal is closed", () => {
    render(
      <ActivityEditModal
        isOpen={false}
        onClose={mockOnClose}
        activity={mockActivity}
        onSave={mockOnSave}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders nothing when activity is null", () => {
    render(
      <ActivityEditModal
        isOpen={true}
        onClose={mockOnClose}
        activity={null}
        onSave={mockOnSave}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders modal when isOpen is true and activity exists", async () => {
    render(
      <ActivityEditModal
        isOpen={true}
        onClose={mockOnClose}
        activity={mockActivity}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByText("Aktivität bearbeiten")).toBeInTheDocument();
    });
  });

  it("shows loading state when loading prop is true", async () => {
    render(
      <ActivityEditModal
        isOpen={true}
        onClose={mockOnClose}
        activity={mockActivity}
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
      <ActivityEditModal
        isOpen={true}
        onClose={mockOnClose}
        activity={mockActivity}
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
