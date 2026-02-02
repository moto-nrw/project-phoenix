/**
 * Tests for ActivityManagementModal Component
 * Tests the rendering, update functionality, and error message handling
 */
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  ActivityManagementModal,
  getDeleteErrorMessage,
} from "./activity-management-modal";
import type { Activity } from "~/lib/activity-api";

// =============================================================================
// Unit Tests for getDeleteErrorMessage
// =============================================================================

describe("getDeleteErrorMessage", () => {
  it("returns default message for non-Error objects", () => {
    expect(getDeleteErrorMessage(null)).toBe(
      "Fehler beim Löschen der Aktivität",
    );
    expect(getDeleteErrorMessage(undefined)).toBe(
      "Fehler beim Löschen der Aktivität",
    );
    expect(getDeleteErrorMessage("string error")).toBe(
      "Fehler beim Löschen der Aktivität",
    );
    expect(getDeleteErrorMessage(123)).toBe(
      "Fehler beim Löschen der Aktivität",
    );
  });

  it("returns students enrolled message when error mentions students", () => {
    const error = new Error("Cannot delete: students enrolled in activity");
    const result = getDeleteErrorMessage(error);
    expect(result).toBe(
      "Diese Aktivität kann nicht gelöscht werden, da noch Schüler eingeschrieben sind. Bitte entfernen Sie zuerst alle Schüler aus der Aktivität.",
    );
  });

  it("returns ownership error message for 403 with ownership context", () => {
    const error = new Error("403 you can only modify your own activities");
    const result = getDeleteErrorMessage(error);
    expect(result).toBe(
      "Sie können diese Aktivität nicht löschen, da Sie sie nicht erstellt haben und kein Betreuer sind.",
    );
  });

  it("returns ownership error message for 403 with supervise context", () => {
    const error = new Error("403 activities you created or supervise");
    const result = getDeleteErrorMessage(error);
    expect(result).toBe(
      "Sie können diese Aktivität nicht löschen, da Sie sie nicht erstellt haben und kein Betreuer sind.",
    );
  });

  it("returns session expired message for 401 error", () => {
    const error = new Error("Request failed with status 401");
    const result = getDeleteErrorMessage(error);
    expect(result).toBe(
      "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.",
    );
  });

  it("returns generic permission denied message for other 403 errors", () => {
    const error = new Error("Forbidden 403");
    const result = getDeleteErrorMessage(error);
    expect(result).toBe(
      "Sie haben keine Berechtigung, diese Aktivität zu löschen.",
    );
  });

  it("returns original error message for other errors", () => {
    const error = new Error("Network timeout");
    const result = getDeleteErrorMessage(error);
    expect(result).toBe("Network timeout");
  });
});

// =============================================================================
// Component Tests for ActivityManagementModal
// =============================================================================

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
