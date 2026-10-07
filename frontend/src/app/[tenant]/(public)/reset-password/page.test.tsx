import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import ResetPasswordPage from "./page";

const ERROR_OBJECT = "das Zurücksetzen des Passworts";

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

vi.mock("next/image", () => ({
  default: ({ src, alt }: { src: string; alt: string }) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img src={src} alt={alt} data-testid="next-image" />
  ),
}));

const mockUseTenant = vi.fn();
vi.mock("~/lib/tenant-context", () => ({
  useTenantSafe: () => mockUseTenant(),
  useTenantSlugSafe: () => "demo-school",
  useTenantRoutingModeSafe: vi.fn(() => "path"),
  useNFCEnabled: vi.fn(() => true),
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
    mockUseTenant.mockReturnValue({
      tenantSlug: "demo-school",
      tenant: null,
    });
  });

  it("renders the reset password form", () => {
    render(<ResetPasswordPage />);

    expect(screen.getByText("Neues Passwort festlegen")).toBeInTheDocument();
    expect(
      screen.getByText("Wählen Sie ein starkes Passwort für Ihr Konto."),
    ).toBeInTheDocument();
  });

  it("displays password requirements", () => {
    render(<ResetPasswordPage />);

    expect(screen.getByText("Passwort-Anforderungen:")).toBeInTheDocument();
    expect(screen.getByText("Mindestens 8 Zeichen lang")).toBeInTheDocument();
    expect(screen.getByText("Groß- und Kleinbuchstaben")).toBeInTheDocument();
    expect(screen.getByText("Mindestens eine Zahl")).toBeInTheDocument();
    expect(
      screen.getByText("Mindestens ein Sonderzeichen"),
    ).toBeInTheDocument();
  });

  it("displays error when token is missing", async () => {
    mockToken = null;

    render(<ResetPasswordPage />);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Der Link ist unvollständig. Bitte fordern Sie einen neuen Link an.",
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
        screen.getByText("Passwort erfolgreich geändert"),
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
        screen.getByText(
          "Sie werden automatisch zur Anmeldeseite weitergeleitet.",
        ),
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

  // The backend answers an expired or used link with its own code (#2517).
  it("handles an expired or used link", async () => {
    vi.mocked(confirmPasswordReset).mockRejectedValue(
      new ApiError("invalid or expired reset token", 400, {
        code: "identity.password_reset_link_invalid",
      }),
    );

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
          catalogText("identity.password_reset_link_invalid", ERROR_OBJECT),
        ),
      ).toBeInTheDocument();
    });
  });

  it("handles a password the backend finds too weak", async () => {
    vi.mocked(confirmPasswordReset).mockRejectedValue(
      new ApiError("password too weak", 400, {
        code: "identity.password_too_weak",
        errors: [{ field: "new_password", reason: "too weak" }],
      }),
    );

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
          catalogText("identity.password_too_weak", ERROR_OBJECT),
        ),
      ).toBeInTheDocument();
    });
  });

  it("handles generic server error", async () => {
    vi.mocked(confirmPasswordReset).mockRejectedValue(
      new ApiError("boom", 500),
    );

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
        screen.getByText(catalogText("general.server", ERROR_OBJECT)),
      ).toBeInTheDocument();
    });
  });

  it("does not render a MOTO logo fallback", () => {
    render(<ResetPasswordPage />);

    expect(screen.queryByAltText("MOTO Logo")).not.toBeInTheDocument();
  });

  it("renders tenant logo when configured", () => {
    mockUseTenant.mockReturnValue({
      tenantSlug: "demo-school",
      tenant: {
        name: "Demo School",
        settings: {
          loginImageUrl: "/uploads/login-images/demo-logo.png",
        },
      },
    });

    render(<ResetPasswordPage />);

    expect(screen.getByAltText("Demo School Logo")).toBeInTheDocument();
  });
});
