/**
 * Tests for PasswordResetModal Component
 * Tests the rendering and basic functionality of password reset modal
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { PasswordResetModal } from "./password-reset-modal";

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
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <button onClick={onClose}>Close Modal</button>
        {children}
      </div>
    ) : null,
}));

vi.mock("./index", () => ({
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
    mockRequestPasswordReset.mockResolvedValue({});
  });

  it("renders the modal when open", () => {
    render(<PasswordResetModal isOpen={true} onClose={mockOnClose} />);

    expect(screen.getByTestId("modal")).toBeInTheDocument();
  });

  it("does not render when closed", () => {
    render(<PasswordResetModal isOpen={false} onClose={mockOnClose} />);

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders password reset form", () => {
    render(<PasswordResetModal isOpen={true} onClose={mockOnClose} />);

    expect(screen.getByText(/Passwort zurücksetzen/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/E-Mail-Adresse/i)).toBeInTheDocument();
  });

  it("renders email input field", () => {
    render(<PasswordResetModal isOpen={true} onClose={mockOnClose} />);

    expect(screen.getByTestId("input-reset-email")).toBeInTheDocument();
  });

  it("renders submit and cancel buttons", () => {
    render(<PasswordResetModal isOpen={true} onClose={mockOnClose} />);

    expect(
      screen.getByRole("button", { name: /Abbrechen/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Link senden/i }),
    ).toBeInTheDocument();
  });

  it("handles email input changes", () => {
    render(<PasswordResetModal isOpen={true} onClose={mockOnClose} />);

    const emailInput = screen.getByTestId("input-reset-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    expect(emailInput).toHaveValue("test@example.com");
  });

  it("displays success state after successful submission", async () => {
    render(<PasswordResetModal isOpen={true} onClose={mockOnClose} />);

    const emailInput = screen.getByTestId("input-reset-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const submitButton = screen.getByRole("button", { name: /Link senden/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText(/E-Mail versendet!/i)).toBeInTheDocument();
    });
  });

  it("displays instructions in success state", async () => {
    render(<PasswordResetModal isOpen={true} onClose={mockOnClose} />);

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
    render(<PasswordResetModal isOpen={true} onClose={mockOnClose} />);

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
    render(<PasswordResetModal isOpen={true} onClose={mockOnClose} />);

    const cancelButton = screen.getByRole("button", { name: /Abbrechen/i });
    fireEvent.click(cancelButton);

    expect(mockOnClose).toHaveBeenCalled();
  });

  it("disables submit button during loading", async () => {
    mockRequestPasswordReset.mockImplementation(
      () => new Promise((resolve) => setTimeout(resolve, 1000)),
    );

    render(<PasswordResetModal isOpen={true} onClose={mockOnClose} />);

    const emailInput = screen.getByTestId("input-reset-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const submitButton = screen.getByRole("button", { name: /Link senden/i });
    fireEvent.click(submitButton);

    expect(submitButton).toBeDisabled();
  });

  it("handles API errors", async () => {
    mockRequestPasswordReset.mockRejectedValue({
      message: "Email not found",
    });

    render(<PasswordResetModal isOpen={true} onClose={mockOnClose} />);

    const emailInput = screen.getByTestId("input-reset-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const submitButton = screen.getByRole("button", { name: /Link senden/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByTestId("alert-error")).toBeInTheDocument();
    });
  });

  it("handles rate limit errors with countdown", async () => {
    mockRequestPasswordReset.mockRejectedValue({
      status: 429,
      retryAfterSeconds: 60,
    });

    render(<PasswordResetModal isOpen={true} onClose={mockOnClose} />);

    const emailInput = screen.getByTestId("input-reset-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const submitButton = screen.getByRole("button", { name: /Link senden/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText(/Zu viele Versuche/i)).toBeInTheDocument();
    });
  });

  it("persists rate limit to localStorage", async () => {
    mockRequestPasswordReset.mockRejectedValue({
      status: 429,
      retryAfterSeconds: 60,
    });

    render(<PasswordResetModal isOpen={true} onClose={mockOnClose} />);

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

    render(<PasswordResetModal isOpen={true} onClose={mockOnClose} />);

    expect(screen.getByText(/Zu viele Versuche/i)).toBeInTheDocument();
  });

  it("clears expired rate limit from localStorage", () => {
    const pastTimestamp = Date.now() - 1000;
    localStorageMock.setItem(
      "passwordResetRateLimitUntil",
      pastTimestamp.toString(),
    );

    render(<PasswordResetModal isOpen={true} onClose={mockOnClose} />);

    // Should not show rate limit error
    expect(screen.queryByText(/Zu viele Versuche/i)).not.toBeInTheDocument();
  });
});
