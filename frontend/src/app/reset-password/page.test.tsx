import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import ResetPasswordPage from "./page";

const mockPush = vi.fn();

let mockToken: string | null = "valid-reset-token";

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
  }),
  useSearchParams: () => ({
    get: (key: string) => (key === "token" ? mockToken : null),
  }),
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: ({ fullPage }: { fullPage?: boolean }) => (
    <div data-testid="loading" data-fullpage={fullPage} aria-label="Lädt..." />
  ),
}));

vi.mock("~/lib/auth-api", () => ({
  confirmPasswordReset: vi.fn(),
}));

import { confirmPasswordReset } from "~/lib/auth-api";

describe("ResetPasswordPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockToken = "valid-reset-token";
    vi.mocked(confirmPasswordReset).mockResolvedValue({ message: "success" });
  });

  it("renders the reset password form", () => {
    render(<ResetPasswordPage />);

    expect(screen.getByText("Neues Passwort festlegen")).toBeInTheDocument();
    expect(
      screen.getByText("Bitte geben Sie Ihr neues Passwort ein."),
    ).toBeInTheDocument();
  });

  it("displays password requirements", () => {
    render(<ResetPasswordPage />);

    expect(screen.getByText("Passwort-Anforderungen:")).toBeInTheDocument();
    expect(screen.getByText("• Mindestens 8 Zeichen lang")).toBeInTheDocument();
    expect(screen.getByText("• Groß- und Kleinbuchstaben")).toBeInTheDocument();
    expect(screen.getByText("• Mindestens eine Zahl")).toBeInTheDocument();
    expect(
      screen.getByText("• Mindestens ein Sonderzeichen"),
    ).toBeInTheDocument();
  });

  it("displays error when token is missing", async () => {
    mockToken = null;

    render(<ResetPasswordPage />);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Ungültiger oder fehlender Reset-Token. Bitte fordern Sie einen neuen Link an.",
        ),
      ).toBeInTheDocument();
    });
  });

  it("renders password input fields", () => {
    render(<ResetPasswordPage />);

    expect(screen.getByLabelText("Neues Passwort")).toBeInTheDocument();
    expect(screen.getByLabelText("Passwort bestätigen")).toBeInTheDocument();
  });

  it("renders submit button", () => {
    render(<ResetPasswordPage />);

    const submitButton = screen.getByRole("button", {
      name: /Passwort ändern/i,
    });
    expect(submitButton).toBeInTheDocument();
  });

  it("displays back to login link", () => {
    render(<ResetPasswordPage />);

    const backLink = screen.getByText("Zurück zur Anmeldung");
    expect(backLink).toBeInTheDocument();
    expect(backLink).toHaveAttribute("href", "/");
  });

  it("disables inputs when token is missing", async () => {
    mockToken = null;

    render(<ResetPasswordPage />);

    await waitFor(() => {
      expect(screen.getByLabelText("Neues Passwort")).toBeDisabled();
      expect(screen.getByLabelText("Passwort bestätigen")).toBeDisabled();
    });
  });

  it("disables submit button when token is missing", async () => {
    mockToken = null;

    render(<ResetPasswordPage />);

    await waitFor(() => {
      const submitButton = screen.getByRole("button", {
        name: /Passwort ändern/i,
      });
      expect(submitButton).toBeDisabled();
    });
  });

  it("has password visibility toggle buttons", () => {
    render(<ResetPasswordPage />);

    const toggleButtons = screen.getAllByLabelText("Passwort anzeigen");
    expect(toggleButtons).toHaveLength(2);
  });

  it("toggles password visibility", async () => {
    render(<ResetPasswordPage />);

    const passwordInput = screen.getByLabelText("Neues Passwort");
    expect(passwordInput).toHaveAttribute("type", "password");

    const toggleButtons = screen.getAllByLabelText("Passwort anzeigen");

    await act(async () => {
      fireEvent.click(toggleButtons[0]!);
    });

    expect(passwordInput).toHaveAttribute("type", "text");
  });

  it("toggles confirm password visibility", async () => {
    render(<ResetPasswordPage />);

    const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");
    expect(confirmPasswordInput).toHaveAttribute("type", "password");

    const toggleButtons = screen.getAllByLabelText("Passwort anzeigen");

    await act(async () => {
      fireEvent.click(toggleButtons[1]!);
    });

    expect(confirmPasswordInput).toHaveAttribute("type", "text");
  });

  it("calls confirmPasswordReset with form values on submit", async () => {
    render(<ResetPasswordPage />);

    const passwordInput = screen.getByLabelText("Neues Passwort");
    const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");

    await act(async () => {
      fireEvent.change(passwordInput, { target: { value: "ValidPass1!" } });
      fireEvent.change(confirmPasswordInput, {
        target: { value: "ValidPass1!" },
      });
    });

    const submitButton = screen.getByRole("button", {
      name: /Passwort ändern/i,
    });

    await act(async () => {
      fireEvent.click(submitButton);
    });

    await waitFor(() => {
      expect(confirmPasswordReset).toHaveBeenCalledWith(
        "valid-reset-token",
        "ValidPass1!",
        "ValidPass1!",
      );
    });
  });

  it("shows success state after successful submission", async () => {
    render(<ResetPasswordPage />);

    const passwordInput = screen.getByLabelText("Neues Passwort");
    const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");

    await act(async () => {
      fireEvent.change(passwordInput, { target: { value: "ValidPass1!" } });
      fireEvent.change(confirmPasswordInput, {
        target: { value: "ValidPass1!" },
      });
    });

    const submitButton = screen.getByRole("button", {
      name: /Passwort ändern/i,
    });

    await act(async () => {
      fireEvent.click(submitButton);
    });

    await waitFor(() => {
      expect(
        screen.getByText("Passwort erfolgreich geändert!"),
      ).toBeInTheDocument();
    });
  });

  it("shows redirect message after success", async () => {
    render(<ResetPasswordPage />);

    const passwordInput = screen.getByLabelText("Neues Passwort");
    const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");

    await act(async () => {
      fireEvent.change(passwordInput, { target: { value: "ValidPass1!" } });
      fireEvent.change(confirmPasswordInput, {
        target: { value: "ValidPass1!" },
      });
    });

    const submitButton = screen.getByRole("button", {
      name: /Passwort ändern/i,
    });

    await act(async () => {
      fireEvent.click(submitButton);
    });

    await waitFor(() => {
      expect(
        screen.getByText("Sie werden zur Anmeldeseite weitergeleitet..."),
      ).toBeInTheDocument();
    });
  });

  it("validates password length", async () => {
    render(<ResetPasswordPage />);

    const passwordInput = screen.getByLabelText("Neues Passwort");
    const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");

    await act(async () => {
      fireEvent.change(passwordInput, { target: { value: "Short1!" } });
      fireEvent.change(confirmPasswordInput, { target: { value: "Short1!" } });
    });

    const submitButton = screen.getByRole("button", {
      name: /Passwort ändern/i,
    });

    await act(async () => {
      fireEvent.click(submitButton);
    });

    await waitFor(() => {
      expect(
        screen.getByText("Das Passwort muss mindestens 8 Zeichen lang sein."),
      ).toBeInTheDocument();
    });

    expect(confirmPasswordReset).not.toHaveBeenCalled();
  });

  it("validates password requires uppercase", async () => {
    render(<ResetPasswordPage />);

    const passwordInput = screen.getByLabelText("Neues Passwort");
    const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");

    await act(async () => {
      fireEvent.change(passwordInput, { target: { value: "lowercase1!" } });
      fireEvent.change(confirmPasswordInput, {
        target: { value: "lowercase1!" },
      });
    });

    const submitButton = screen.getByRole("button", {
      name: /Passwort ändern/i,
    });

    await act(async () => {
      fireEvent.click(submitButton);
    });

    await waitFor(() => {
      expect(
        screen.getByText(
          "Das Passwort muss mindestens einen Großbuchstaben enthalten.",
        ),
      ).toBeInTheDocument();
    });
  });

  it("validates passwords must match", async () => {
    render(<ResetPasswordPage />);

    const passwordInput = screen.getByLabelText("Neues Passwort");
    const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");

    await act(async () => {
      fireEvent.change(passwordInput, { target: { value: "ValidPass1!" } });
      fireEvent.change(confirmPasswordInput, {
        target: { value: "Different1!" },
      });
    });

    const submitButton = screen.getByRole("button", {
      name: /Passwort ändern/i,
    });

    await act(async () => {
      fireEvent.click(submitButton);
    });

    await waitFor(() => {
      expect(
        screen.getByText("Die Passwörter stimmen nicht überein."),
      ).toBeInTheDocument();
    });
  });

  it("handles 410 expired token error", async () => {
    vi.mocked(confirmPasswordReset).mockRejectedValue({
      status: 410,
      message: "Token expired",
    });

    render(<ResetPasswordPage />);

    const passwordInput = screen.getByLabelText("Neues Passwort");
    const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");

    await act(async () => {
      fireEvent.change(passwordInput, { target: { value: "ValidPass1!" } });
      fireEvent.change(confirmPasswordInput, {
        target: { value: "ValidPass1!" },
      });
    });

    const submitButton = screen.getByRole("button", {
      name: /Passwort ändern/i,
    });

    await act(async () => {
      fireEvent.click(submitButton);
    });

    await waitFor(() => {
      expect(
        screen.getByText(
          "Dieser Passwort-Reset-Link ist abgelaufen. Bitte fordere einen neuen Link an.",
        ),
      ).toBeInTheDocument();
    });
  });

  it("handles 404 not found error", async () => {
    vi.mocked(confirmPasswordReset).mockRejectedValue({
      status: 404,
      message: "Not found",
    });

    render(<ResetPasswordPage />);

    const passwordInput = screen.getByLabelText("Neues Passwort");
    const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");

    await act(async () => {
      fireEvent.change(passwordInput, { target: { value: "ValidPass1!" } });
      fireEvent.change(confirmPasswordInput, {
        target: { value: "ValidPass1!" },
      });
    });

    const submitButton = screen.getByRole("button", {
      name: /Passwort ändern/i,
    });

    await act(async () => {
      fireEvent.click(submitButton);
    });

    await waitFor(() => {
      expect(
        screen.getByText(
          "Wir konnten diesen Passwort-Reset-Link nicht finden. Bitte fordere einen neuen Link an.",
        ),
      ).toBeInTheDocument();
    });
  });

  it("handles generic server error", async () => {
    vi.mocked(confirmPasswordReset).mockRejectedValue({
      status: 500,
    });

    render(<ResetPasswordPage />);

    const passwordInput = screen.getByLabelText("Neues Passwort");
    const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");

    await act(async () => {
      fireEvent.change(passwordInput, { target: { value: "ValidPass1!" } });
      fireEvent.change(confirmPasswordInput, {
        target: { value: "ValidPass1!" },
      });
    });

    const submitButton = screen.getByRole("button", {
      name: /Passwort ändern/i,
    });

    await act(async () => {
      fireEvent.click(submitButton);
    });

    await waitFor(() => {
      expect(
        screen.getByText(
          "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
        ),
      ).toBeInTheDocument();
    });
  });

  it("renders MOTO logo", () => {
    render(<ResetPasswordPage />);

    expect(screen.getByAltText("MOTO Logo")).toBeInTheDocument();
  });
});

describe("Password validation logic", () => {
  it("validates all password requirements", () => {
    const validatePassword = (pwd: string): string | null => {
      if (pwd.length < 8) {
        return "Das Passwort muss mindestens 8 Zeichen lang sein.";
      }
      if (!/[A-Z]/.test(pwd)) {
        return "Das Passwort muss mindestens einen Großbuchstaben enthalten.";
      }
      if (!/[a-z]/.test(pwd)) {
        return "Das Passwort muss mindestens einen Kleinbuchstaben enthalten.";
      }
      if (!/\d/.test(pwd)) {
        return "Das Passwort muss mindestens eine Zahl enthalten.";
      }
      if (!/[^A-Za-z0-9]/.test(pwd)) {
        return "Das Passwort muss mindestens ein Sonderzeichen enthalten.";
      }
      return null;
    };

    // Invalid passwords
    expect(validatePassword("short")).toBe(
      "Das Passwort muss mindestens 8 Zeichen lang sein.",
    );
    expect(validatePassword("nouppercase1!")).toBe(
      "Das Passwort muss mindestens einen Großbuchstaben enthalten.",
    );
    expect(validatePassword("NOLOWERCASE1!")).toBe(
      "Das Passwort muss mindestens einen Kleinbuchstaben enthalten.",
    );
    expect(validatePassword("NoNumber!!")).toBe(
      "Das Passwort muss mindestens eine Zahl enthalten.",
    );
    expect(validatePassword("NoSpecial1")).toBe(
      "Das Passwort muss mindestens ein Sonderzeichen enthalten.",
    );

    // Valid passwords
    expect(validatePassword("ValidPass1!")).toBeNull();
    expect(validatePassword("Test1234!")).toBeNull();
    expect(validatePassword("Abcdefg1@")).toBeNull();
  });
});
