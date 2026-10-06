/**
 * Tests for ActivityManagementModal Component
 * Tests the rendering, update functionality, and the shared error path
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { ButtonHTMLAttributes, ReactNode } from "react";
import { ActivityManagementModal } from "./activity-management-modal";
import { deleteActivity, updateActivity } from "~/lib/activity-api";
import type { Activity } from "~/lib/activity-api";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

// =============================================================================
// Component Tests for ActivityManagementModal
// =============================================================================

// Mock all dependencies
vi.mock("~/lib/activity-api", () => ({
  updateActivity: vi.fn(),
  deleteActivity: vi.fn(),
}));

// The real hook returns a stable setForm (useState); the modal's reset effect
// depends on it.
const { mockSetForm } = vi.hoisted(() => ({ mockSetForm: vi.fn() }));

vi.mock("~/hooks/useActivityForm", () => ({
  parseParticipantLimit: (value: string) =>
    value ? Number.parseInt(value, 10) : null,
  useActivityForm: vi.fn(() => ({
    form: {
      name: "Test Activity",
      category_id: "1",
      max_participants: "15",
    },
    setForm: mockSetForm,
    categories: [
      { id: "1", name: "Category 1" },
      { id: "2", name: "Category 2" },
    ],
    loading: false,
    loadError: null,
    handleInputChange: vi.fn(),
    validateForm: vi.fn(() => null),
  })),
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

  it("shows the catalog text of a failed save in the form", async () => {
    vi.mocked(updateActivity).mockRejectedValueOnce(
      new ApiError("boom", 500, { code: "general.server" }),
    );
    render(
      <ActivityManagementModal
        isOpen={true}
        onClose={mockOnClose}
        activity={mockActivity}
      />,
    );

    fireEvent.submit(screen.getByLabelText(/Aktivitätsname/).closest("form")!);

    expect(
      await screen.findByText(catalogText("general.server", "die Aktivität")),
    ).toBeInTheDocument();
    expect(mockOnClose).not.toHaveBeenCalled();
  });

  it("keeps a failed delete in the open delete dialog", async () => {
    vi.mocked(deleteActivity).mockRejectedValueOnce(
      new ApiError("nope", 403, { code: "general.permission" }),
    );
    render(
      <ActivityManagementModal
        isOpen={true}
        onClose={mockOnClose}
        activity={mockActivity}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Aktivität löschen" }));
    fireEvent.click(await screen.findByRole("button", { name: "Ja, löschen" }));
    fireEvent.click(screen.getByRole("button", { name: "Endgültig löschen" }));

    expect(
      await screen.findByText(
        catalogText("general.permission", "die Aktivität"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Endgültig löschen" }),
    ).toBeInTheDocument();
    expect(mockOnClose).not.toHaveBeenCalled();
  });
});
