/**
 * Tests for ActivityManagementModal Component
 * Tests the rendering, update functionality, and error message handling
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { ButtonHTMLAttributes, ReactNode } from "react";
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
      "Diese Aktivität kann nicht gelöscht werden, da noch Kinder eingeschrieben sind. Bitte entfernen Sie zuerst alle Kinder aus der Aktivität.",
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

vi.mock("~/hooks/useActivityForm", () => ({
  parseParticipantLimit: (value: string) =>
    value ? Number.parseInt(value, 10) : null,
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

vi.mock("~/lib/api-error-message", () => ({
  getApiErrorMessage: vi.fn((_err: unknown) => "Error message"),
}));

vi.mock("~/components/ui/form-modal", () => ({
  FormModal: ({
    isOpen,
    title,
    children,
    footer,
  }: {
    isOpen: boolean;
    title: string;
    children: ReactNode;
    footer?: ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="form-modal">
        <h2>{title}</h2>
        {children}
        {footer && <div>{footer}</div>}
      </div>
    ) : null,
}));

vi.mock("~/components/ui/alert", () => ({
  Alert: ({ type, message }: { type: string; message: string }) => (
    <div role="alert" data-type={type}>
      {message}
    </div>
  ),
}));

type MockButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  readonly variant?: string;
  readonly size?: string;
};

vi.mock("~/components/ui/button", () => ({
  Button: ({
    children,
    variant: _variant,
    size: _size,
    ...props
  }: MockButtonProps) => (
    <button type="button" {...props}>
      {children}
    </button>
  ),
}));

vi.mock("~/components/ui/icons", () => ({
  SpinnerIcon: () => (
    <span data-testid="button-spinner" aria-label="Wird geladen" />
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

    expect(screen.getByTestId("form-modal")).toBeInTheDocument();
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

  it("does not cap the participant limit at 50", () => {
    render(
      <ActivityManagementModal
        isOpen={true}
        onClose={mockOnClose}
        activity={mockActivity}
      />,
    );

    expect(
      screen.getByLabelText(/Maximale Teilnehmerzahl/),
    ).not.toHaveAttribute("max");
  });

  it("offers an explicit unlimited option", () => {
    render(
      <ActivityManagementModal
        isOpen={true}
        onClose={mockOnClose}
        activity={mockActivity}
      />,
    );

    expect(
      screen.getByRole("checkbox", { name: "Keine Begrenzung" }),
    ).toBeInTheDocument();
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

    const select = await screen.findByRole("combobox");
    fireEvent.click(select);

    await waitFor(() => {
      expect(
        screen.getByRole("option", { name: "Category 1" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("option", { name: "Category 2" }),
      ).toBeInTheDocument();
    });
  });

  it("does not clip the open category menu with an overflow-hidden ancestor", async () => {
    render(
      <ActivityManagementModal
        isOpen={true}
        onClose={mockOnClose}
        activity={mockActivity}
      />,
    );

    const select = await screen.findByRole("combobox");
    fireEvent.click(select);

    const listbox = await screen.findByRole("listbox");
    for (
      let node = listbox.parentElement;
      node && node !== document.body;
      node = node.parentElement
    ) {
      expect(node.className).not.toMatch(/(?:^|\s)overflow-hidden(?:\s|$)/);
    }
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
