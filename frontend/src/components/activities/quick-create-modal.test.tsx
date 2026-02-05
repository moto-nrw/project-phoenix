/**
 * Tests for QuickCreateActivityModal Component
 * Tests the rendering and creation functionality
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { QuickCreateActivityModal } from "./quick-create-modal";

// Mock all dependencies
vi.mock("react-dom", async () => ({
  ...(await vi.importActual("react-dom")),
  createPortal: (children: React.ReactNode) => children,
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

    expect(screen.getByTestId("modal-wrapper")).toBeInTheDocument();
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

    await waitFor(() => {
      expect(screen.getByText("Gruppenraum")).toBeInTheDocument();
      expect(screen.getByText("Hausaufgaben")).toBeInTheDocument();
      expect(screen.getByText("Kreatives/Musik")).toBeInTheDocument();
    });
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
        // eslint-disable-next-line @typescript-eslint/no-misused-promises
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
        // eslint-disable-next-line @typescript-eslint/no-misused-promises
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
});
