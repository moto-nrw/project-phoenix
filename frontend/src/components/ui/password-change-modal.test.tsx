/**
 * Tests for PasswordChangeModal Component
 * Tests the rendering and basic functionality of password change modal
 */
import {
  act,
  render,
  screen,
  waitFor,
  fireEvent,
} from "@testing-library/react";
import { useRef, type ComponentProps } from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { useApiFormError } from "~/contexts/ToastContext";
import { PasswordChangeModal } from "./password-change-modal";

/** The modal on the shared error path, as every owner mounts it. */
function ChangeModal(
  props: Omit<
    ComponentProps<typeof PasswordChangeModal>,
    "errorPath" | "formRef"
  >,
) {
  const formRef = useRef<HTMLFormElement>(null);
  const errorPath = useApiFormError(formRef);
  return (
    <PasswordChangeModal {...props} errorPath={errorPath} formRef={formRef} />
  );
}

function problemResponse(status: number, code: string, field?: string) {
  return new Response(
    JSON.stringify({
      status,
      detail: "diagnostic text that is never shown",
      code,
      ...(field && { errors: [{ field, reason: "invalid" }] }),
    }),
    { status, headers: { "Content-Type": "application/problem+json" } },
  );
}

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
      <ChangeModal
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

  it("renders all password input fields", () => {
    render(
      <ChangeModal
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
      <ChangeModal
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
      <ChangeModal
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
      <ChangeModal
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
      <ChangeModal
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
      <ChangeModal
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
      <ChangeModal
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
      <ChangeModal
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
      <ChangeModal
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
      <ChangeModal
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
      <ChangeModal
        isOpen={true}
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    const cancelButton = screen.getByRole("button", { name: /Abbrechen/i });
    fireEvent.click(cancelButton);

    expect(mockOnClose).toHaveBeenCalled();
  });

  // The text comes from the catalog entry of the code; the backend's
  // diagnostic text is never read (ADR 0006).
  it("shows the catalog text of a refused current password and marks the field", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      problemResponse(
        400,
        "identity.current_password_wrong",
        "current_password",
      ),
    );

    render(
      <ChangeModal
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
        screen.getByText(/Das aktuelle Passwort stimmt nicht/i),
      ).toBeInTheDocument();
    });
    expect(screen.getByText(/Aktuelles Passwort/i).className).toContain(
      "text-moto-red",
    );
    expect(
      screen.queryByText(/diagnostic text that is never shown/i),
    ).not.toBeInTheDocument();
  });

  it("shows the catalog text of a weak new password", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      problemResponse(400, "identity.password_too_weak", "new_password"),
    );

    render(
      <ChangeModal
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
        screen.getByText(/Das Passwort ist zu schwach/i),
      ).toBeInTheDocument();
    });
  });

  it("shows the class text for a code the catalog does not know", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      problemResponse(404, "identity.some_future_code"),
    );

    render(
      <ChangeModal
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
        screen.getByText(
          /Das Ändern des Passworts konnte nicht übernommen werden/i,
        ),
      ).toBeInTheDocument();
    });
  });

  it("offers a retry after a server error", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      problemResponse(500, "general.server"),
    );

    render(
      <ChangeModal
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
        screen.getByText(
          /Das Ändern des Passworts konnte nicht bearbeitet werden/i,
        ),
      ).toBeInTheDocument();
    });
    expect(
      screen.getByRole("button", { name: /erneut|Wiederholen/i }),
    ).toBeInTheDocument();
  });

  it("calls onSuccess after successful password change timeout", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    render(
      <ChangeModal
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

    // Skip the 2 s delay before onSuccess + handleClose.
    await act(() => vi.advanceTimersByTimeAsync(2100));
    await waitFor(
      () => {
        expect(mockOnSuccess).toHaveBeenCalled();
      },
      { timeout: 3000 },
    );
    expect(mockOnClose).toHaveBeenCalled();
  });

  describe("Scroll to error and field highlighting", () => {
    it("scrolls to the first empty field when submitting with all empty fields", async () => {
      const scrollIntoViewMock = vi.fn();
      Element.prototype.scrollIntoView = scrollIntoViewMock;

      render(
        <ChangeModal
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
          block: "center",
        });
        // FormErrorAlert brings the marked field into view, not itself.
        expect(scrollIntoViewMock.mock.contexts[0]).toHaveAttribute(
          "aria-invalid",
          "true",
        );
      });
    });

    it("highlights the current password label when it is empty", async () => {
      Element.prototype.scrollIntoView = vi.fn();

      render(
        <ChangeModal
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
