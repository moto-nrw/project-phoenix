/**
 * Tests for QuickCreateActivityModal Component
 * Tests the rendering and creation functionality
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { ButtonHTMLAttributes, ReactNode } from "react";
import { QuickCreateActivityModal } from "./quick-create-modal";

// Mock all dependencies
vi.mock("~/lib/use-notification", () => ({
  getDbOperationMessage: vi.fn(
    (operation: string, entity: string, name: string) =>
      `${operation} ${entity} ${name}`,
  ),
}));

vi.mock("~/hooks/useActivityForm", () => ({
  useActivityForm: vi.fn(() => ({
    form: {
      name: "",
      category_id: "",
      max_participants: "15",
    },
    setForm: vi.fn(),
    categories: [
      { id: "1", name: "Gruppenraum" },
      { id: "2", name: "Hausaufgaben" },
      { id: "3", name: "Kreatives/Musik" },
    ],
    loading: false,
    error: null,
    setError: vi.fn(),
    handleInputChange: vi.fn(),
    validateForm: vi.fn(() => null),
  })),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: vi.fn(() => ({
    success: vi.fn(),
    error: vi.fn(),
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

// Mock fetch
global.fetch = vi.fn();

describe("QuickCreateActivityModal", () => {
  const mockOnClose = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: async () => ({ id: "1", name: "New Activity" }),
    });
  });

  it("renders modal when open", () => {
    render(<QuickCreateActivityModal isOpen={true} onClose={mockOnClose} />);

    expect(screen.getByTestId("form-modal")).toBeInTheDocument();
  });

  it("displays modal title", async () => {
    render(<QuickCreateActivityModal isOpen={true} onClose={mockOnClose} />);

    await waitFor(() => {
      // Use getByRole to be specific about the heading
      expect(
        screen.getByRole("heading", { name: /Aktivität erstellen/ }),
      ).toBeInTheDocument();
    });
  });

  it("renders form fields", async () => {
    render(<QuickCreateActivityModal isOpen={true} onClose={mockOnClose} />);

    await waitFor(() => {
      // Labels contain nested elements, use regex for partial matching
      expect(screen.getByLabelText(/Aktivitätsname/)).toBeInTheDocument();
      expect(screen.getByLabelText(/Kategorie/)).toBeInTheDocument();
      expect(
        screen.getByLabelText(/Maximale Teilnehmerzahl/),
      ).toBeInTheDocument();
    });
  });

  it("renders category options", async () => {
    render(<QuickCreateActivityModal isOpen={true} onClose={mockOnClose} />);

    const select = await screen.findByRole("combobox");
    fireEvent.click(select);

    await waitFor(() => {
      expect(
        screen.getByRole("option", { name: "Gruppenraum" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("option", { name: "Hausaufgaben" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("option", { name: "Kreatives/Musik" }),
      ).toBeInTheDocument();
    });
  });

  it("does not clip the open category menu with an overflow-hidden ancestor", async () => {
    render(<QuickCreateActivityModal isOpen={true} onClose={mockOnClose} />);

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

  it("renders action buttons", async () => {
    render(<QuickCreateActivityModal isOpen={true} onClose={mockOnClose} />);

    await waitFor(() => {
      // Use button role to avoid ambiguity with heading
      expect(
        screen.getByRole("button", { name: /Aktivität erstellen/ }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /Abbrechen/ }),
      ).toBeInTheDocument();
    });
  });

  it("renders increment and decrement buttons for max participants", async () => {
    render(<QuickCreateActivityModal isOpen={true} onClose={mockOnClose} />);

    await waitFor(() => {
      expect(
        screen.getByLabelText("Teilnehmer reduzieren"),
      ).toBeInTheDocument();
      expect(screen.getByLabelText("Teilnehmer erhöhen")).toBeInTheDocument();
    });
  });

  it("displays info message", async () => {
    render(<QuickCreateActivityModal isOpen={true} onClose={mockOnClose} />);

    await waitFor(() => {
      expect(
        screen.getByText(
          /Die Aktivität ist sofort für NFC-Terminals verfügbar/,
        ),
      ).toBeInTheDocument();
    });
  });

  it("calls onClose when cancel button clicked", async () => {
    render(<QuickCreateActivityModal isOpen={true} onClose={mockOnClose} />);

    const cancelButton = screen.getByText("Abbrechen");
    fireEvent.click(cancelButton);

    expect(mockOnClose).toHaveBeenCalled();
  });

  it("renders nothing when closed", () => {
    const { container } = render(
      <QuickCreateActivityModal isOpen={false} onClose={mockOnClose} />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("disables submit button when form is invalid", () => {
    render(<QuickCreateActivityModal isOpen={true} onClose={mockOnClose} />);

    // Use button role with submit type to be specific
    const submitButton = screen.getByRole("button", {
      name: /Aktivität erstellen/,
    });
    expect(submitButton).toBeDisabled();
  });

  describe("double-submit prevention", () => {
    it("disables submit button when loading prop is true", async () => {
      // Override mock to return loading: true with a valid form
      const { useActivityForm } = await import("~/hooks/useActivityForm");
      vi.mocked(useActivityForm).mockReturnValue({
        form: {
          name: "Test Activity",
          category_id: "1",
          max_participants: "15",
        },
        setForm: vi.fn(),
        categories: [
          {
            id: "1",
            name: "Gruppenraum",
            created_at: new Date("2024-01-01"),
            updated_at: new Date("2024-01-01"),
          },
        ],
        loading: true, // Loading is true - this should disable the button
        error: null,
        setError: vi.fn(),
        handleInputChange: vi.fn(),
        validateForm: vi.fn(() => null),
        loadCategories: vi.fn(),
      });

      render(<QuickCreateActivityModal isOpen={true} onClose={mockOnClose} />);

      // Submit button should be disabled because loading is true
      // (form itself is valid with name and category_id filled)
      const submitButton = screen.getByRole("button", {
        name: /Aktivität erstellen/,
      });
      expect(submitButton).toBeDisabled();
    });

    it("shows loading state in submit button text while submitting", async () => {
      // This tests that the isSubmitting state shows "Wird erstellt..." in the button
      // The mock returns a valid form, so we can test the button text changes
      const { useActivityForm } = await import("~/hooks/useActivityForm");
      vi.mocked(useActivityForm).mockReturnValue({
        form: {
          name: "Test Activity",
          category_id: "1",
          max_participants: "15",
        },
        setForm: vi.fn(),
        categories: [
          {
            id: "1",
            name: "Gruppenraum",
            created_at: new Date("2024-01-01"),
            updated_at: new Date("2024-01-01"),
          },
        ],
        loading: false,
        error: null,
        setError: vi.fn(),
        handleInputChange: vi.fn(),
        validateForm: vi.fn(() => null),
        loadCategories: vi.fn(),
      });

      // Make fetch hang to test loading state
      let resolveSubmit: (value: unknown) => void;
      const fetchPromise = new Promise((resolve) => {
        resolveSubmit = resolve;
      });
      (global.fetch as ReturnType<typeof vi.fn>).mockImplementationOnce(
        () => fetchPromise,
      );

      render(<QuickCreateActivityModal isOpen={true} onClose={mockOnClose} />);

      const submitButton = screen.getByRole("button", {
        name: /Aktivität erstellen/,
      });

      // Click submit
      fireEvent.click(submitButton);

      // Button should show loading text
      await waitFor(() => {
        expect(screen.getByText(/Wird erstellt.../)).toBeInTheDocument();
      });

      // Resolve to clean up
      resolveSubmit!({
        ok: true,
        json: async () => ({ id: "1", name: "Test" }),
      });
    });

    it("resets isSubmitting after validation error", async () => {
      const mockSetError = vi.fn();
      const { useActivityForm } = await import("~/hooks/useActivityForm");
      vi.mocked(useActivityForm).mockReturnValue({
        form: {
          name: "Test Activity",
          category_id: "1",
          max_participants: "15",
        },
        setForm: vi.fn(),
        categories: [
          {
            id: "1",
            name: "Gruppenraum",
            created_at: new Date("2024-01-01"),
            updated_at: new Date("2024-01-01"),
          },
        ],
        loading: false,
        error: null,
        setError: mockSetError,
        handleInputChange: vi.fn(),
        validateForm: vi.fn(() => "Validation error"), // Return an error
        loadCategories: vi.fn(),
      });

      render(<QuickCreateActivityModal isOpen={true} onClose={mockOnClose} />);

      const submitButton = screen.getByRole("button", {
        name: /Aktivität erstellen/,
      });

      // Click submit - validation will fail
      fireEvent.click(submitButton);

      // setError should be called with validation error
      expect(mockSetError).toHaveBeenCalledWith("Validation error");

      // Button should be enabled again (isSubmitting reset)
      await waitFor(() => {
        expect(submitButton).not.toBeDisabled();
      });
    });

    it("handles API error and resets isSubmitting", async () => {
      const mockSetError = vi.fn();
      const { useActivityForm } = await import("~/hooks/useActivityForm");
      vi.mocked(useActivityForm).mockReturnValue({
        form: {
          name: "Test Activity",
          category_id: "1",
          max_participants: "15",
        },
        setForm: vi.fn(),
        categories: [
          {
            id: "1",
            name: "Gruppenraum",
            created_at: new Date("2024-01-01"),
            updated_at: new Date("2024-01-01"),
          },
        ],
        loading: false,
        error: null,
        setError: mockSetError,
        handleInputChange: vi.fn(),
        validateForm: vi.fn(() => null),
        loadCategories: vi.fn(),
      });

      // Mock fetch to return error
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
        ok: false,
        status: 500,
      });

      render(<QuickCreateActivityModal isOpen={true} onClose={mockOnClose} />);

      const submitButton = screen.getByRole("button", {
        name: /Aktivität erstellen/,
      });

      fireEvent.click(submitButton);

      // Wait for error handling
      await waitFor(() => {
        expect(mockSetError).toHaveBeenCalled();
      });

      // Button should be enabled again (isSubmitting reset in finally)
      await waitFor(() => {
        expect(submitButton).not.toBeDisabled();
      });
    });

    it("prevents double-click during submission", async () => {
      const { useActivityForm } = await import("~/hooks/useActivityForm");
      vi.mocked(useActivityForm).mockReturnValue({
        form: {
          name: "Test Activity",
          category_id: "1",
          max_participants: "15",
        },
        setForm: vi.fn(),
        categories: [
          {
            id: "1",
            name: "Gruppenraum",
            created_at: new Date("2024-01-01"),
            updated_at: new Date("2024-01-01"),
          },
        ],
        loading: false,
        error: null,
        setError: vi.fn(),
        handleInputChange: vi.fn(),
        validateForm: vi.fn(() => null),
        loadCategories: vi.fn(),
      });

      // Make fetch hang
      let resolveSubmit: (value: unknown) => void;
      const fetchPromise = new Promise((resolve) => {
        resolveSubmit = resolve;
      });
      (global.fetch as ReturnType<typeof vi.fn>).mockImplementation(
        () => fetchPromise,
      );

      render(<QuickCreateActivityModal isOpen={true} onClose={mockOnClose} />);

      const submitButton = screen.getByRole("button", {
        name: /Aktivität erstellen/,
      });

      // Click submit twice rapidly
      fireEvent.click(submitButton);
      fireEvent.click(submitButton);

      // Should only call fetch once (second click blocked by isSubmitting)
      expect(global.fetch).toHaveBeenCalledTimes(1);

      // Clean up
      resolveSubmit!({
        ok: true,
        json: async () => ({ id: "1", name: "Test" }),
      });
    });
  });

  describe("Scroll to error", () => {
    it("scrolls to error when validation fails", async () => {
      const scrollIntoViewMock = vi.fn();
      Element.prototype.scrollIntoView = scrollIntoViewMock;

      const mockSetError = vi.fn();
      const { useActivityForm } = await import("~/hooks/useActivityForm");
      vi.mocked(useActivityForm).mockReturnValue({
        form: {
          name: "Test Activity",
          category_id: "1",
          max_participants: "15",
        },
        setForm: vi.fn(),
        categories: [
          {
            id: "1",
            name: "Gruppenraum",
            created_at: new Date("2024-01-01"),
            updated_at: new Date("2024-01-01"),
          },
        ],
        loading: false,
        error: "Bitte gib einen name ein",
        setError: mockSetError,
        handleInputChange: vi.fn(),
        validateForm: vi.fn(() => "Bitte gib einen name ein"),
        loadCategories: vi.fn(),
      });

      render(<QuickCreateActivityModal isOpen={true} onClose={mockOnClose} />);

      // The error is already set via the mock, so the error div with ref should render
      // and trigger the scroll effect
      await waitFor(() => {
        expect(scrollIntoViewMock).toHaveBeenCalledWith({
          behavior: "smooth",
          block: "start",
        });
      });
    });
  });
});
