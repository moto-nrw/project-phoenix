/**
 * Tests for PasswordChangeModal Component
 * Tests the rendering and basic functionality of password change modal
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { PasswordChangeModal } from "./password-change-modal";

// Mock UI components
vi.mock("./modal", () => ({
  Modal: ({
    isOpen,
    children,
    title,
    onClose,
  }: {
    isOpen: boolean;
    children: React.ReactNode;
    title: string;
    onClose: () => void;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <h1>{title}</h1>
        <button type="button" onClick={onClose}>
          Close Modal
        </button>
        {children}
      </div>
    ) : null,
}));

vi.mock("./alert", () => ({
  Alert: ({ type, message }: { type: string; message: string }) => (
    <div data-testid={`alert-${type}`}>{message}</div>
  ),
}));

vi.mock("./icons", () => ({
  EyeIcon: () => <div data-testid="eye-icon">Eye</div>,
  EyeOffIcon: () => <div data-testid="eye-off-icon">EyeOff</div>,
  CheckIcon: ({ className }: { className?: string }) => (
    <div data-testid="check-icon" className={className}>
      Check
    </div>
  ),
  SpinnerIcon: ({ className }: { className?: string }) => (
    <div data-testid="spinner-icon" className={className}>
      Spinner
    </div>
  ),
}));

// Mock fetch
global.fetch = vi.fn();

describe("PasswordChangeModal", () => {
  const mockOnClose = vi.fn();
  const mockOnSuccess = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: async () => ({}),
    });
  });

  it("renders the modal when open", () => {
    render(
      <PasswordChangeModal
        isOpen={true}
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    expect(screen.getByTestId("modal")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Passwort ändern" }),
    ).toBeInTheDocument();
  });

  it("does not render when closed", () => {
    render(
      <PasswordChangeModal
        isOpen={false}
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders all password input fields", () => {
    render(
      <PasswordChangeModal
        isOpen={true}
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    expect(screen.getByLabelText(/Aktuelles Passwort/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/^Neues Passwort$/i)).toBeInTheDocument();
    expect(
      screen.getByLabelText(/Neues Passwort bestätigen/i),
    ).toBeInTheDocument();
  });

  it("renders password requirements", () => {
    render(
      <PasswordChangeModal
        isOpen={true}
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    expect(screen.getByText(/Passwort-Anforderungen/i)).toBeInTheDocument();
    expect(screen.getByText(/Mindestens 8 Zeichen/i)).toBeInTheDocument();
    expect(
      screen.getByText(/Groß- und Kleinbuchstaben empfohlen/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Zahlen und Sonderzeichen empfohlen/i),
    ).toBeInTheDocument();
  });

  it("renders cancel and submit buttons", () => {
    render(
      <PasswordChangeModal
        isOpen={true}
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    expect(
      screen.getByRole("button", { name: /Abbrechen/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Passwort ändern/i }),
    ).toBeInTheDocument();
  });

  it("allows toggling password visibility", async () => {
    render(
      <PasswordChangeModal
        isOpen={true}
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    const currentPasswordInput = screen.getByLabelText(/Aktuelles Passwort/i);
    expect(currentPasswordInput).toHaveAttribute("type", "password");

    // Find the first toggle button (eye icon button)
    const eyeIcons = screen.getAllByTestId("eye-icon");
    const toggleButton = eyeIcons[0]!.closest("button");
    fireEvent.click(toggleButton!);

    expect(currentPasswordInput).toHaveAttribute("type", "text");
  });

  it("handles input changes", async () => {
    render(
      <PasswordChangeModal
        isOpen={true}
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    const currentPasswordInput = screen.getByLabelText(/Aktuelles Passwort/i);
    fireEvent.change(currentPasswordInput, {
      target: { value: "OldPass123!" },
    });

    expect(currentPasswordInput).toHaveValue("OldPass123!");
  });

  it("validates required fields", async () => {
    render(
      <PasswordChangeModal
        isOpen={true}
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    const submitButton = screen.getByRole("button", {
      name: /Passwort ändern/i,
    });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText(/Bitte füllen Sie alle Felder aus/i),
      ).toBeInTheDocument();
    });
  });

  it("validates password mismatch", async () => {
    render(
      <PasswordChangeModal
        isOpen={true}
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    const currentPasswordInput = screen.getByLabelText(/Aktuelles Passwort/i);
    const newPasswordInput = screen.getByLabelText(/^Neues Passwort$/i);
    const confirmPasswordInput = screen.getByLabelText(
      /Neues Passwort bestätigen/i,
    );

    fireEvent.change(currentPasswordInput, {
      target: { value: "OldPass123!" },
    });
    fireEvent.change(newPasswordInput, { target: { value: "NewPass123!" } });
    fireEvent.change(confirmPasswordInput, {
      target: { value: "DifferentPass123!" },
    });

    const submitButton = screen.getByRole("button", {
      name: /Passwort ändern/i,
    });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText(/Die neuen Passwörter stimmen nicht überein/i),
      ).toBeInTheDocument();
    });
  });

  it("validates minimum password length", async () => {
    render(
      <PasswordChangeModal
        isOpen={true}
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    const currentPasswordInput = screen.getByLabelText(/Aktuelles Passwort/i);
    const newPasswordInput = screen.getByLabelText(/^Neues Passwort$/i);
    const confirmPasswordInput = screen.getByLabelText(
      /Neues Passwort bestätigen/i,
    );

    fireEvent.change(currentPasswordInput, {
      target: { value: "OldPass123!" },
    });
    fireEvent.change(newPasswordInput, { target: { value: "Short1!" } });
    fireEvent.change(confirmPasswordInput, { target: { value: "Short1!" } });

    const submitButton = screen.getByRole("button", {
      name: /Passwort ändern/i,
    });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText(/mindestens 8 Zeichen lang sein/i),
      ).toBeInTheDocument();
    });
  });

  it("validates that new password is different from current", async () => {
    render(
      <PasswordChangeModal
        isOpen={true}
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    const currentPasswordInput = screen.getByLabelText(/Aktuelles Passwort/i);
    const newPasswordInput = screen.getByLabelText(/^Neues Passwort$/i);
    const confirmPasswordInput = screen.getByLabelText(
      /Neues Passwort bestätigen/i,
    );

    fireEvent.change(currentPasswordInput, {
      target: { value: "SamePass123!" },
    });
    fireEvent.change(newPasswordInput, { target: { value: "SamePass123!" } });
    fireEvent.change(confirmPasswordInput, {
      target: { value: "SamePass123!" },
    });

    const submitButton = screen.getByRole("button", {
      name: /Passwort ändern/i,
    });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText(/nicht mit dem aktuellen Passwort identisch sein/i),
      ).toBeInTheDocument();
    });
  });

  it("displays success state after successful submission", async () => {
    render(
      <PasswordChangeModal
        isOpen={true}
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    const currentPasswordInput = screen.getByLabelText(/Aktuelles Passwort/i);
    const newPasswordInput = screen.getByLabelText(/^Neues Passwort$/i);
    const confirmPasswordInput = screen.getByLabelText(
      /Neues Passwort bestätigen/i,
    );

    fireEvent.change(currentPasswordInput, {
      target: { value: "OldPass123!" },
    });
    fireEvent.change(newPasswordInput, { target: { value: "NewPass123!" } });
    fireEvent.change(confirmPasswordInput, {
      target: { value: "NewPass123!" },
    });

    const submitButton = screen.getByRole("button", {
      name: /Passwort ändern/i,
    });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText(/Passwort erfolgreich geändert!/i),
      ).toBeInTheDocument();
    });
  });

  it("calls onClose when cancel button is clicked", () => {
    render(
      <PasswordChangeModal
        isOpen={true}
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    const cancelButton = screen.getByRole("button", { name: /Abbrechen/i });
    fireEvent.click(cancelButton);

    expect(mockOnClose).toHaveBeenCalled();
  });

  it("shows German error for invalid password backend response", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      json: async () => ({ error: "invalid username or password" }),
    });

    render(
      <PasswordChangeModal
        isOpen={true}
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    fireEvent.change(screen.getByLabelText(/Aktuelles Passwort/i), {
      target: { value: "WrongPass1!" },
    });
    fireEvent.change(screen.getByLabelText(/^Neues Passwort$/i), {
      target: { value: "NewPass123!" },
    });
    fireEvent.change(screen.getByLabelText(/Neues Passwort bestätigen/i), {
      target: { value: "NewPass123!" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Passwort ändern/i }));

    await waitFor(() => {
      expect(
        screen.getByText(/Das aktuelle Passwort ist falsch/i),
      ).toBeInTheDocument();
    });
  });

  it("shows German error for weak password backend response", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      json: async () => ({
        error: "password doesn't meet complexity requirements",
      }),
    });

    render(
      <PasswordChangeModal
        isOpen={true}
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    fireEvent.change(screen.getByLabelText(/Aktuelles Passwort/i), {
      target: { value: "OldPass123!" },
    });
    fireEvent.change(screen.getByLabelText(/^Neues Passwort$/i), {
      target: { value: "NewPass123!" },
    });
    fireEvent.change(screen.getByLabelText(/Neues Passwort bestätigen/i), {
      target: { value: "NewPass123!" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Passwort ändern/i }));

    await waitFor(() => {
      expect(
        screen.getByText(/Das neue Passwort ist zu schwach/i),
      ).toBeInTheDocument();
    });
  });

  it("shows German error for account not found backend response", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      json: async () => ({ error: "account not found" }),
    });

    render(
      <PasswordChangeModal
        isOpen={true}
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    fireEvent.change(screen.getByLabelText(/Aktuelles Passwort/i), {
      target: { value: "OldPass123!" },
    });
    fireEvent.change(screen.getByLabelText(/^Neues Passwort$/i), {
      target: { value: "NewPass123!" },
    });
    fireEvent.change(screen.getByLabelText(/Neues Passwort bestätigen/i), {
      target: { value: "NewPass123!" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Passwort ändern/i }));

    await waitFor(() => {
      expect(
        screen.getByText(/Ihr Konto konnte nicht gefunden werden/i),
      ).toBeInTheDocument();
    });
  });

  it("shows generic German error for unknown backend error", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      json: async () => ({ error: "some unknown error" }),
    });

    render(
      <PasswordChangeModal
        isOpen={true}
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    fireEvent.change(screen.getByLabelText(/Aktuelles Passwort/i), {
      target: { value: "OldPass123!" },
    });
    fireEvent.change(screen.getByLabelText(/^Neues Passwort$/i), {
      target: { value: "NewPass123!" },
    });
    fireEvent.change(screen.getByLabelText(/Neues Passwort bestätigen/i), {
      target: { value: "NewPass123!" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Passwort ändern/i }));

    await waitFor(() => {
      expect(
        screen.getByText(/Passwortänderung fehlgeschlagen/i),
      ).toBeInTheDocument();
    });
  });

  it("calls onSuccess after successful password change timeout", async () => {
    render(
      <PasswordChangeModal
        isOpen={true}
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    fireEvent.change(screen.getByLabelText(/Aktuelles Passwort/i), {
      target: { value: "OldPass123!" },
    });
    fireEvent.change(screen.getByLabelText(/^Neues Passwort$/i), {
      target: { value: "NewPass123!" },
    });
    fireEvent.change(screen.getByLabelText(/Neues Passwort bestätigen/i), {
      target: { value: "NewPass123!" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Passwort ändern/i }));

    await waitFor(() => {
      expect(
        screen.getByText(/Passwort erfolgreich geändert!/i),
      ).toBeInTheDocument();
    });

    // Wait for the 2s setTimeout to fire onSuccess + handleClose
    await waitFor(
      () => {
        expect(mockOnSuccess).toHaveBeenCalled();
      },
      { timeout: 3000 },
    );
    expect(mockOnClose).toHaveBeenCalled();
  });

  describe("Scroll to error and field highlighting", () => {
    it("scrolls to error when submitting with all empty fields", async () => {
      const scrollIntoViewMock = vi.fn();
      Element.prototype.scrollIntoView = scrollIntoViewMock;

      render(
        <PasswordChangeModal
          isOpen={true}
          onClose={mockOnClose}
          onSuccess={mockOnSuccess}
        />,
      );

      const submitButton = screen.getByRole("button", {
        name: /Passwort ändern/i,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(/Bitte füllen Sie alle Felder aus/i),
        ).toBeInTheDocument();
        expect(scrollIntoViewMock).toHaveBeenCalledWith({
          behavior: "smooth",
          block: "start",
        });
      });
    });

    it("highlights the current password label when it is empty", async () => {
      Element.prototype.scrollIntoView = vi.fn();

      render(
        <PasswordChangeModal
          isOpen={true}
          onClose={mockOnClose}
          onSuccess={mockOnSuccess}
        />,
      );

      const submitButton = screen.getByRole("button", {
        name: /Passwort ändern/i,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(/Bitte füllen Sie alle Felder aus/i),
        ).toBeInTheDocument();
      });

      const currentPasswordLabel = screen.getByText(/Aktuelles Passwort/i);
      expect(currentPasswordLabel.className).toContain("text-moto-red");
    });
  });
});
