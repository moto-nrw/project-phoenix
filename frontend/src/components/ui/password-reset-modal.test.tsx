/**
 * Tests for PasswordResetModal Component
 * Tests the rendering and basic functionality of password reset modal
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { useRef, type ComponentProps } from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { useApiFormError } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { PasswordResetModal } from "./password-reset-modal";

/** The modal on the shared error path, as every owner mounts it. */
function ResetModal(
  props: Omit<
    ComponentProps<typeof PasswordResetModal>,
    "errorPath" | "formRef"
  >,
) {
  const formRef = useRef<HTMLFormElement>(null);
  const errorPath = useApiFormError(formRef);
  return (
    <PasswordResetModal {...props} errorPath={errorPath} formRef={formRef} />
  );
}

function rateLimited(retryAfterSeconds: number): ApiError {
  const error = new ApiError("too many reset requests", 429, {
    code: "identity.password_reset_rate_limited",
  });
  error.retryAfterSeconds = retryAfterSeconds;
  return error;
}

const modalOnCloseHistory: Array<() => void> = [];

// Mock UI components
vi.mock("./modal", () => ({
  Modal: ({
    isOpen,
    children,
    onClose,
  }: {
    isOpen: boolean;
    children: React.ReactNode;
    onClose: () => void;
  }) => {
    modalOnCloseHistory.push(onClose);

    return isOpen ? (
      <div data-testid="modal">
        <button type="button" onClick={onClose}>
          Close Modal
        </button>
        {children}
      </div>
    ) : null;
  },
}));

vi.mock("./input", () => ({
  Input: ({
    id,
    value,
    onChange,
    disabled,
    ...props
  }: {
    id: string;
    value: string;
    onChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
    disabled?: boolean;
    type?: string;
    name?: string;
    autoComplete?: string;
    required?: boolean;
    className?: string;
    label?: string;
  }) => (
    <input
      id={id}
      data-testid={`input-${id}`}
      value={value}
      onChange={onChange}
      disabled={disabled}
      {...props}
    />
  ),
}));

vi.mock("./alert", () => ({
  Alert: ({ type, message }: { type: string; message: string }) => (
    <div data-testid={`alert-${type}`}>{message}</div>
  ),
}));

// Mock auth-api
const mockRequestPasswordReset = vi.fn();
vi.mock("~/lib/auth-api", () => ({
  requestPasswordReset: (email: string): unknown =>
    mockRequestPasswordReset(email),
}));

// Mock localStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {};

  return {
    getItem: (key: string) => store[key] ?? null,
    setItem: (key: string, value: string) => {
      store[key] = value;
    },
    removeItem: (key: string) => {
      delete store[key];
    },
    clear: () => {
      store = {};
    },
  };
})();

Object.defineProperty(globalThis, "localStorage", {
  value: localStorageMock,
});

describe("PasswordResetModal", () => {
  const mockOnClose = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    localStorageMock.clear();
    modalOnCloseHistory.length = 0;
    mockRequestPasswordReset.mockResolvedValue({});
  });

  it("renders the modal when open", () => {
    render(<ResetModal isOpen={true} onClose={mockOnClose} />);

    expect(screen.getByTestId("modal")).toBeInTheDocument();
  });

  it("renders password reset form", () => {
    render(<ResetModal isOpen={true} onClose={mockOnClose} />);

    expect(screen.getByText(/Passwort zurücksetzen/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/E-Mail-Adresse/i)).toBeInTheDocument();
  });

  it("renders email input field", () => {
    render(<ResetModal isOpen={true} onClose={mockOnClose} />);

    expect(screen.getByTestId("input-reset-email")).toBeInTheDocument();
  });

  it("renders submit and cancel buttons", () => {
    render(<ResetModal isOpen={true} onClose={mockOnClose} />);

    expect(
      screen.getByRole("button", { name: /Abbrechen/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Link senden/i }),
    ).toBeInTheDocument();
  });

  it("handles email input changes", () => {
    render(<ResetModal isOpen={true} onClose={mockOnClose} />);

    const emailInput = screen.getByTestId("input-reset-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    expect(emailInput).toHaveValue("test@example.com");
  });

  it("keeps a stable modal close handler while typing", () => {
    render(<ResetModal isOpen={true} onClose={mockOnClose} />);

    const initialOnClose = modalOnCloseHistory.at(-1);
    const emailInput = screen.getByTestId("input-reset-email");

    fireEvent.change(emailInput, { target: { value: "y.we" } });

    expect(modalOnCloseHistory.at(-1)).toBe(initialOnClose);
  });

  it("displays success state after successful submission", async () => {
    render(<ResetModal isOpen={true} onClose={mockOnClose} />);

    const emailInput = screen.getByTestId("input-reset-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const submitButton = screen.getByRole("button", { name: /Link senden/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText(/E-Mail versendet!/i)).toBeInTheDocument();
    });
  });

  it("displays instructions in success state", async () => {
    render(<ResetModal isOpen={true} onClose={mockOnClose} />);

    const emailInput = screen.getByTestId("input-reset-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const submitButton = screen.getByRole("button", { name: /Link senden/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText(/Wir haben Ihnen eine E-Mail/i),
      ).toBeInTheDocument();
      expect(
        screen.getByText(/Bitte überprüfen Sie Ihren Posteingang/i),
      ).toBeInTheDocument();
    });
  });

  it("displays close button in success state", async () => {
    render(<ResetModal isOpen={true} onClose={mockOnClose} />);

    const emailInput = screen.getByTestId("input-reset-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const submitButton = screen.getByRole("button", { name: /Link senden/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /Schließen/i }),
      ).toBeInTheDocument();
    });
  });

  it("calls onClose when cancel button is clicked", () => {
    render(<ResetModal isOpen={true} onClose={mockOnClose} />);

    const cancelButton = screen.getByRole("button", { name: /Abbrechen/i });
    fireEvent.click(cancelButton);

    expect(mockOnClose).toHaveBeenCalled();
  });

  it("disables submit button during loading", async () => {
    mockRequestPasswordReset.mockImplementation(
      () => new Promise((resolve) => setTimeout(resolve, 1000)),
    );

    render(<ResetModal isOpen={true} onClose={mockOnClose} />);

    const emailInput = screen.getByTestId("input-reset-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const submitButton = screen.getByRole("button", { name: /Link senden/i });
    fireEvent.click(submitButton);

    expect(submitButton).toBeDisabled();
  });

  it("handles API errors", async () => {
    mockRequestPasswordReset.mockRejectedValue(
      new ApiError("smtp unavailable", 500),
    );

    render(<ResetModal isOpen={true} onClose={mockOnClose} />);

    const emailInput = screen.getByTestId("input-reset-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const submitButton = screen.getByRole("button", { name: /Link senden/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByTestId("alert-error")).toHaveTextContent(
        /Das Senden des Links konnte nicht bearbeitet werden/i,
      );
    });
  });

  it("handles rate limit errors with countdown", async () => {
    mockRequestPasswordReset.mockRejectedValue(rateLimited(60));

    render(<ResetModal isOpen={true} onClose={mockOnClose} />);

    const emailInput = screen.getByTestId("input-reset-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const submitButton = screen.getByRole("button", { name: /Link senden/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText(/Sie haben zu oft einen Link angefordert/i),
      ).toBeInTheDocument();
    });
    expect(
      screen.getByText(/Einen neuen Link können Sie in 01:00 anfordern/i),
    ).toBeInTheDocument();
    expect(submitButton).toBeDisabled();
  });

  it("persists rate limit to localStorage", async () => {
    mockRequestPasswordReset.mockRejectedValue(rateLimited(60));

    render(<ResetModal isOpen={true} onClose={mockOnClose} />);

    const emailInput = screen.getByTestId("input-reset-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const submitButton = screen.getByRole("button", { name: /Link senden/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      const stored = localStorageMock.getItem("passwordResetRateLimitUntil");
      expect(stored).toBeTruthy();
    });
  });

  it("restores rate limit from localStorage", () => {
    const futureTimestamp = Date.now() + 60000;
    localStorageMock.setItem(
      "passwordResetRateLimitUntil",
      futureTimestamp.toString(),
    );

    render(<ResetModal isOpen={true} onClose={mockOnClose} />);

    expect(
      screen.getByText(/Einen neuen Link können Sie in/i),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Link senden/i })).toBeDisabled();
  });

  it("clears expired rate limit from localStorage", () => {
    const pastTimestamp = Date.now() - 1000;
    localStorageMock.setItem(
      "passwordResetRateLimitUntil",
      pastTimestamp.toString(),
    );

    render(<ResetModal isOpen={true} onClose={mockOnClose} />);

    // Anker: das Formular steht da und der Knopf ist wieder frei.
    expect(screen.getByRole("button", { name: /Link senden/i })).toBeEnabled();
    // Should not show the wait
    expect(
      screen.queryByText(/Einen neuen Link können Sie in/i),
    ).not.toBeInTheDocument();
  });

  describe("Scroll to error", () => {
    it("scrolls to error when API request fails", async () => {
      const scrollIntoViewMock = vi.fn();
      Element.prototype.scrollIntoView = scrollIntoViewMock;

      mockRequestPasswordReset.mockRejectedValue(
        new ApiError("smtp unavailable", 500),
      );

      render(<ResetModal isOpen={true} onClose={mockOnClose} />);

      const emailInput = screen.getByTestId("input-reset-email");
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      const submitButton = screen.getByRole("button", {
        name: /Link senden/i,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(screen.getByTestId("alert-error")).toBeInTheDocument();
      });

      await waitFor(() => {
        expect(scrollIntoViewMock).toHaveBeenCalledWith({
          behavior: "smooth",
          block: "nearest",
        });
      });
    });
  });
});
