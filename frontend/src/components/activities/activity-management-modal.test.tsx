/**
 * Tests for ActivityManagementModal Component
 * Tests the rendering and update functionality
 */
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ActivityManagementModal } from "./activity-management-modal";
import type { Activity } from "~/lib/activity-api";

// Mock all dependencies
vi.mock("react-dom", async () => ({
  ...(await vi.importActual("react-dom")),
  createPortal: (children: React.ReactNode) => children,
}));

vi.mock("~/lib/activity-api", () => ({
  updateActivity: vi.fn(),
  deleteActivity: vi.fn(),
}));

vi.mock("~/lib/use-notification", () => ({
  getDbOperationMessage: vi.fn(
    (operation: string, entity: string, name: string) =>
      `${operation} ${entity} ${name}`,
  ),
}));

vi.mock("~/hooks/useScrollLock", () => ({
  useScrollLock: vi.fn(),
}));

vi.mock("~/hooks/useModalAnimation", () => ({
  useModalAnimation: vi.fn((isOpen: boolean, onClose: () => void) => ({
    isAnimating: true,
    isExiting: false,
    handleClose: onClose,
  })),
}));

vi.mock("~/hooks/useModalBlurEffect", () => ({
  useModalBlurEffect: vi.fn(),
}));

vi.mock("~/hooks/useActivityForm", () => ({
  useActivityForm: vi.fn(() => ({
    form: {
      name: "Test Activity",
      category_id: "1",
      max_participants: "15",
    },
    setForm: vi.fn(),
    categories: [
      { id: "1", name: "Category 1" },
      { id: "2", name: "Category 2" },
    ],
    loading: false,
    error: null,
    setError: vi.fn(),
    handleInputChange: vi.fn(),
    validateForm: vi.fn(() => null),
  })),
}));

vi.mock("~/components/ui/modal-utils", () => ({
  scrollableContentClassName: "scrollable-content",
  getContentAnimationClassName: vi.fn(() => "animated-content"),
  renderModalCloseButton: vi.fn(({ onClose }: { onClose: () => void }) => (
    <button onClick={onClose} data-testid="close-button">
      Close
    </button>
  )),
  renderModalLoadingSpinner: vi.fn(() => (
    <div data-testid="loading-spinner">Loading...</div>
  )),
  renderModalErrorAlert: vi.fn(({ message }: { message: string }) => (
    <div data-testid="error-alert">{message}</div>
  )),
  renderButtonSpinner: vi.fn(() => <span data-testid="button-spinner" />),
  getApiErrorMessage: vi.fn((_err: unknown) => "Error message"),
  ModalWrapper: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="modal-wrapper">{children}</div>
  ),
}));

const mockActivity: Activity = {
  id: "1",
  name: "Test Activity",
  ag_category_id: "1",
  max_participant: 15,
  is_open_ags: true,
  supervisor_id: "1",
  created_at: new Date(),
  updated_at: new Date(),
  supervisors: [
    {
      id: "1",
      staff_id: "1",
      is_primary: true,
      full_name: "John Doe",
    },
  ],
};

describe("ActivityManagementModal", () => {
  const mockOnClose = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders modal when open", () => {
    render(
      <ActivityManagementModal
        isOpen={true}
        onClose={mockOnClose}
        activity={mockActivity}
      />,
    );

    expect(screen.getByTestId("modal-wrapper")).toBeInTheDocument();
  });

  it("displays activity name in header", async () => {
    render(
      <ActivityManagementModal
        isOpen={true}
        onClose={mockOnClose}
        activity={mockActivity}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText(/Aktivität: Test Activity/)).toBeInTheDocument();
    });
  });

  it("displays creator information", async () => {
    render(
      <ActivityManagementModal
        isOpen={true}
        onClose={mockOnClose}
        activity={mockActivity}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText(/Erstellt von:/)).toBeInTheDocument();
      // Text content includes whitespace, use a more flexible matcher
      expect(screen.getByText(/John Doe/)).toBeInTheDocument();
    });
  });

  it("renders form fields", async () => {
    render(
      <ActivityManagementModal
        isOpen={true}
        onClose={mockOnClose}
        activity={mockActivity}
      />,
    );

    await waitFor(() => {
      // Labels contain additional nested elements, so use regex for partial matching
      expect(screen.getByLabelText(/Aktivitätsname/)).toBeInTheDocument();
      expect(screen.getByLabelText(/Kategorie/)).toBeInTheDocument();
      expect(
        screen.getByLabelText(/Maximale Teilnehmerzahl/),
      ).toBeInTheDocument();
    });
  });

  it("renders action buttons when not read-only", async () => {
    render(
      <ActivityManagementModal
        isOpen={true}
        onClose={mockOnClose}
        activity={mockActivity}
        readOnly={false}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Speichern")).toBeInTheDocument();
      expect(screen.getByText("Abbrechen")).toBeInTheDocument();
    });
  });

  it("hides save button when read-only", async () => {
    render(
      <ActivityManagementModal
        isOpen={true}
        onClose={mockOnClose}
        activity={mockActivity}
        readOnly={true}
      />,
    );

    await waitFor(() => {
      expect(screen.queryByText("Speichern")).not.toBeInTheDocument();
    });
  });

  it("renders delete button when not read-only", async () => {
    render(
      <ActivityManagementModal
        isOpen={true}
        onClose={mockOnClose}
        activity={mockActivity}
        readOnly={false}
      />,
    );

    await waitFor(() => {
      expect(screen.getByLabelText("Aktivität löschen")).toBeInTheDocument();
    });
  });

  it("renders categories in dropdown", async () => {
    render(
      <ActivityManagementModal
        isOpen={true}
        onClose={mockOnClose}
        activity={mockActivity}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Category 1")).toBeInTheDocument();
      expect(screen.getByText("Category 2")).toBeInTheDocument();
    });
  });

  it("disables inputs when read-only", () => {
    render(
      <ActivityManagementModal
        isOpen={true}
        onClose={mockOnClose}
        activity={mockActivity}
        readOnly={true}
      />,
    );

    const nameInput = screen.getByLabelText(/Aktivitätsname/);
    expect(nameInput).toBeDisabled();
  });

  it("renders info message for read-only mode", async () => {
    render(
      <ActivityManagementModal
        isOpen={true}
        onClose={mockOnClose}
        activity={mockActivity}
        readOnly={true}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByText(/Sie können nur Aktivitäten bearbeiten/),
      ).toBeInTheDocument();
    });
  });
});
