/**
 * Tests for SignupForm component
 *
 * This file tests:
 * - Form rendering and initial state
 * - Password visibility toggles
 * - Password requirement indicators
 * - Slug generation and validation
 * - Form field validation
 * - Form submission (success and error cases)
 * - Error handling for various API responses
 */

import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import {
  describe,
  it,
  expect,
  vi,
  beforeEach,
  afterEach,
  type Mock,
} from "vitest";
import { SignupForm } from "./signup-form";
import {
  signupWithOrganization,
  SignupWithOrgException,
} from "~/lib/auth-client";

// Mock next/navigation
const mockPush = vi.fn();
const mockRefresh = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
    refresh: mockRefresh,
  }),
}));

// Mock toast context
const mockToastSuccess = vi.fn();
const mockToastError = vi.fn();

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
  }),
}));

// Mock signupWithOrganization from auth-client
vi.mock("~/lib/auth-client", async (importOriginal) => {
  const original = await importOriginal<typeof import("~/lib/auth-client")>();
  return {
    ...original,
    signupWithOrganization: vi.fn(),
    SignupWithOrgException: original.SignupWithOrgException,
  };
});

// Mock slug-validation to use actual implementation
vi.mock("~/lib/slug-validation", async (importOriginal) => {
  const original =
    await importOriginal<typeof import("~/lib/slug-validation")>();
  return original;
});

// Mock email-validation to use actual implementation
vi.mock("~/lib/email-validation", async (importOriginal) => {
  const original =
    await importOriginal<typeof import("~/lib/email-validation")>();
  return original;
});

describe("SignupForm", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Mock navigator.onLine
    Object.defineProperty(navigator, "onLine", {
      value: true,
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  // =============================================================================
  // Rendering Tests
  // =============================================================================

  describe("initial rendering", () => {
    it("renders personal information section", () => {
      render(<SignupForm />);

      expect(screen.getByText("Deine Daten")).toBeInTheDocument();
      // Use getByLabelText with exact label text to avoid ambiguity
      expect(screen.getByLabelText("Name")).toBeInTheDocument();
      expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
    });

    it("renders password fields", () => {
      render(<SignupForm />);

      expect(screen.getByLabelText("Passwort")).toBeInTheDocument();
      expect(screen.getByLabelText("Passwort bestätigen")).toBeInTheDocument();
    });

    it("renders password requirements section", () => {
      render(<SignupForm />);

      expect(screen.getByText("Passwortanforderungen")).toBeInTheDocument();
      expect(screen.getByText("Mindestens 8 Zeichen")).toBeInTheDocument();
      expect(screen.getByText("Ein Großbuchstabe")).toBeInTheDocument();
      expect(screen.getByText("Ein Kleinbuchstabe")).toBeInTheDocument();
      expect(screen.getByText("Eine Zahl")).toBeInTheDocument();
      expect(screen.getByText("Ein Sonderzeichen")).toBeInTheDocument();
    });

    it("renders organization section", () => {
      render(<SignupForm />);

      expect(screen.getByText("Deine Organisation")).toBeInTheDocument();
      expect(
        screen.getByLabelText("Name der Organisation"),
      ).toBeInTheDocument();
      expect(screen.getByLabelText("Subdomain")).toBeInTheDocument();
      expect(screen.getByText(".moto-app.de")).toBeInTheDocument();
    });

    it("renders submit button", () => {
      render(<SignupForm />);

      expect(
        screen.getByRole("button", { name: /Organisation registrieren/ }),
      ).toBeInTheDocument();
    });

    it("renders info about approval", () => {
      render(<SignupForm />);

      expect(
        screen.getByText(/Nach der Registrierung wird deine Organisation/),
      ).toBeInTheDocument();
    });

    it("renders link to login page", () => {
      render(<SignupForm />);

      expect(screen.getByText("Bereits ein Konto?")).toBeInTheDocument();
      expect(screen.getByText("Zur Anmeldung")).toHaveAttribute("href", "/");
    });
  });

  // =============================================================================
  // Password Visibility Toggle Tests
  // =============================================================================

  describe("password visibility toggles", () => {
    it("toggles password visibility when button is clicked", () => {
      render(<SignupForm />);

      const passwordInput = screen.getByLabelText("Passwort");
      expect(passwordInput).toHaveAttribute("type", "password");

      // Find the toggle button for password (first one)
      const toggleButtons = screen.getAllByRole("button", {
        name: /Passwort/,
      });
      const passwordToggle = toggleButtons[0]!;

      fireEvent.click(passwordToggle);
      expect(passwordInput).toHaveAttribute("type", "text");

      fireEvent.click(passwordToggle);
      expect(passwordInput).toHaveAttribute("type", "password");
    });

    it("toggles confirm password visibility when button is clicked", () => {
      render(<SignupForm />);

      const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");
      expect(confirmPasswordInput).toHaveAttribute("type", "password");

      // Find the toggle button for confirm password (second one)
      const toggleButtons = screen.getAllByRole("button", {
        name: /Passwort/,
      });
      const confirmToggle = toggleButtons[1]!;

      fireEvent.click(confirmToggle);
      expect(confirmPasswordInput).toHaveAttribute("type", "text");

      fireEvent.click(confirmToggle);
      expect(confirmPasswordInput).toHaveAttribute("type", "password");
    });

    it("shows correct aria-label for password toggle states", () => {
      render(<SignupForm />);

      const toggleButtons = screen.getAllByRole("button", {
        name: /Passwort/,
      });
      const passwordToggle = toggleButtons[0]!;

      expect(passwordToggle).toHaveAttribute("aria-label", "Passwort anzeigen");

      fireEvent.click(passwordToggle);
      expect(passwordToggle).toHaveAttribute(
        "aria-label",
        "Passwort verbergen",
      );
    });
  });

  // =============================================================================
  // Password Requirements Indicator Tests
  // =============================================================================

  describe("password requirements indicators", () => {
    it("shows all requirements as not met initially", () => {
      render(<SignupForm />);

      // All requirements should show empty checkmarks (not met)
      const requirements = screen.getAllByText("", { selector: "span" });
      const checkmarks = requirements.filter(
        (el) => el.className.includes("rounded-full") && el.textContent === "",
      );
      expect(checkmarks.length).toBeGreaterThan(0);
    });

    it("updates requirements as password is typed", () => {
      render(<SignupForm />);

      const passwordInput = screen.getByLabelText("Passwort");

      // Type a password that meets length requirement only
      fireEvent.change(passwordInput, { target: { value: "12345678" } });

      // The "Mindestens 8 Zeichen" requirement should now show checkmark
      const requirementElements = screen.getAllByText("✓");
      expect(requirementElements.length).toBeGreaterThan(0);
    });

    it("shows all checkmarks when password meets all requirements", () => {
      render(<SignupForm />);

      const passwordInput = screen.getByLabelText("Passwort");

      // Type a password that meets all requirements
      fireEvent.change(passwordInput, { target: { value: "StrongP@ss1" } });

      // All 5 requirements should show checkmarks
      const checkmarks = screen.getAllByText("✓");
      expect(checkmarks.length).toBe(5);
    });
  });

  // =============================================================================
  // Slug Auto-Generation Tests
  // =============================================================================

  describe("slug auto-generation", () => {
    it("auto-generates slug from organization name", () => {
      render(<SignupForm />);

      const orgNameInput = screen.getByLabelText("Name der Organisation");
      const slugInput = screen.getByLabelText("Subdomain");

      fireEvent.change(orgNameInput, { target: { value: "OGS Musterstadt" } });

      expect(slugInput).toHaveValue("ogs-musterstadt");
    });

    it("handles German umlauts in slug generation", () => {
      render(<SignupForm />);

      const orgNameInput = screen.getByLabelText("Name der Organisation");
      const slugInput = screen.getByLabelText("Subdomain");

      fireEvent.change(orgNameInput, { target: { value: "OGS Müller" } });

      expect(slugInput).toHaveValue("ogs-mueller");
    });

    it("stops auto-generating slug after manual edit", () => {
      render(<SignupForm />);

      const orgNameInput = screen.getByLabelText("Name der Organisation");
      const slugInput = screen.getByLabelText("Subdomain");

      // First, auto-generate
      fireEvent.change(orgNameInput, { target: { value: "OGS Test" } });
      expect(slugInput).toHaveValue("ogs-test");

      // Manually edit slug
      fireEvent.change(slugInput, { target: { value: "custom-slug" } });
      expect(slugInput).toHaveValue("custom-slug");

      // Change org name again - slug should not change
      fireEvent.change(orgNameInput, { target: { value: "OGS Different" } });
      expect(slugInput).toHaveValue("custom-slug");
    });

    it("normalizes manually entered slug to lowercase", () => {
      render(<SignupForm />);

      const slugInput = screen.getByLabelText("Subdomain");

      fireEvent.change(slugInput, { target: { value: "MY-SLUG" } });

      expect(slugInput).toHaveValue("my-slug");
    });
  });

  // =============================================================================
  // Slug Validation Display Tests
  // =============================================================================

  describe("slug validation display", () => {
    it("shows error message for slug too short", () => {
      render(<SignupForm />);

      const slugInput = screen.getByLabelText("Subdomain");

      fireEvent.change(slugInput, { target: { value: "ab" } });

      expect(
        screen.getByText("Subdomain muss mindestens 3 Zeichen haben"),
      ).toBeInTheDocument();
    });

    it("shows success message for valid slug", () => {
      render(<SignupForm />);

      const slugInput = screen.getByLabelText("Subdomain");

      fireEvent.change(slugInput, { target: { value: "ogs-musterstadt" } });

      expect(
        screen.getByText(/Deine Organisation wird unter/),
      ).toBeInTheDocument();
      expect(
        screen.getByText("ogs-musterstadt.moto-app.de"),
      ).toBeInTheDocument();
    });

    it("shows error for slug starting with hyphen", () => {
      render(<SignupForm />);

      const slugInput = screen.getByLabelText("Subdomain");

      fireEvent.change(slugInput, { target: { value: "-invalid" } });

      expect(
        screen.getByText(/Subdomain darf nicht mit einem Bindestrich beginnen/),
      ).toBeInTheDocument();
    });

    it("shows error for reserved slugs", () => {
      render(<SignupForm />);

      const slugInput = screen.getByLabelText("Subdomain");

      fireEvent.change(slugInput, { target: { value: "admin" } });

      expect(
        screen.getByText(
          "Diese Subdomain ist reserviert und kann nicht verwendet werden",
        ),
      ).toBeInTheDocument();
    });
  });

  // =============================================================================
  // Form Validation Tests
  // =============================================================================

  describe("form validation", () => {
    it("shows error when name is empty", async () => {
      render(<SignupForm />);

      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });
      fireEvent.click(submitButton);

      expect(
        screen.getByText("Bitte gib deinen Namen an."),
      ).toBeInTheDocument();
    });

    it("shows error for invalid email", async () => {
      render(<SignupForm />);

      const nameInput = screen.getByLabelText("Name");
      fireEvent.change(nameInput, { target: { value: "Test User" } });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      fireEvent.change(emailInput, { target: { value: "invalid-email" } });

      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });
      fireEvent.click(submitButton);

      expect(
        screen.getByText("Bitte gib eine gültige E-Mail-Adresse an."),
      ).toBeInTheDocument();
    });

    it("shows error when password does not meet requirements", async () => {
      render(<SignupForm />);

      const nameInput = screen.getByLabelText("Name");
      fireEvent.change(nameInput, { target: { value: "Test User" } });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      const passwordInput = screen.getByLabelText("Passwort");
      fireEvent.change(passwordInput, { target: { value: "weak" } });

      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });
      fireEvent.click(submitButton);

      expect(
        screen.getByText(
          "Das Passwort erfüllt noch nicht alle Sicherheitsanforderungen.",
        ),
      ).toBeInTheDocument();
    });

    it("shows error when passwords do not match", async () => {
      render(<SignupForm />);

      const nameInput = screen.getByLabelText("Name");
      fireEvent.change(nameInput, { target: { value: "Test User" } });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      const passwordInput = screen.getByLabelText("Passwort");
      fireEvent.change(passwordInput, { target: { value: "StrongP@ss1" } });

      const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");
      fireEvent.change(confirmPasswordInput, {
        target: { value: "DifferentP@ss1" },
      });

      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });
      fireEvent.click(submitButton);

      expect(
        screen.getByText("Die Passwörter stimmen nicht überein."),
      ).toBeInTheDocument();
    });

    it("shows error when organization name is empty", async () => {
      render(<SignupForm />);

      const nameInput = screen.getByLabelText("Name");
      fireEvent.change(nameInput, { target: { value: "Test User" } });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      const passwordInput = screen.getByLabelText("Passwort");
      fireEvent.change(passwordInput, { target: { value: "StrongP@ss1" } });

      const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");
      fireEvent.change(confirmPasswordInput, {
        target: { value: "StrongP@ss1" },
      });

      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });
      fireEvent.click(submitButton);

      expect(
        screen.getByText("Bitte gib den Namen deiner Organisation an."),
      ).toBeInTheDocument();
    });

    it("shows error for invalid slug", async () => {
      render(<SignupForm />);

      // Fill in all valid fields
      const nameInput = screen.getByLabelText("Name");
      fireEvent.change(nameInput, { target: { value: "Test User" } });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      const passwordInput = screen.getByLabelText("Passwort");
      fireEvent.change(passwordInput, { target: { value: "StrongP@ss1" } });

      const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");
      fireEvent.change(confirmPasswordInput, {
        target: { value: "StrongP@ss1" },
      });

      const orgNameInput = screen.getByLabelText("Name der Organisation");
      fireEvent.change(orgNameInput, { target: { value: "Test Org" } });

      // Set invalid slug manually
      const slugInput = screen.getByLabelText("Subdomain");
      fireEvent.change(slugInput, { target: { value: "ab" } });

      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });
      fireEvent.click(submitButton);

      // Both the inline validation and form-level error show the same message
      const errorMessages = screen.getAllByText(
        "Subdomain muss mindestens 3 Zeichen haben",
      );
      expect(errorMessages.length).toBeGreaterThanOrEqual(1);
    });
  });

  // =============================================================================
  // Form Submission Tests
  // =============================================================================

  describe("form submission", () => {
    const fillValidForm = () => {
      const nameInput = screen.getByLabelText("Name");
      fireEvent.change(nameInput, { target: { value: "Test User" } });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      const passwordInput = screen.getByLabelText("Passwort");
      fireEvent.change(passwordInput, { target: { value: "StrongP@ss1" } });

      const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");
      fireEvent.change(confirmPasswordInput, {
        target: { value: "StrongP@ss1" },
      });

      const orgNameInput = screen.getByLabelText("Name der Organisation");
      fireEvent.change(orgNameInput, { target: { value: "OGS Test" } });

      // Slug is auto-generated to "ogs-test"
    };

    it("submits form successfully and shows success toast", async () => {
      (signupWithOrganization as Mock).mockResolvedValue({
        success: true,
        user: { id: "user-123", email: "test@example.com", name: "Test User" },
        organization: { id: "org-123", name: "OGS Test", slug: "ogs-test" },
      });

      render(<SignupForm />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(signupWithOrganization).toHaveBeenCalledWith({
          name: "Test User",
          email: "test@example.com",
          password: "StrongP@ss1",
          orgName: "OGS Test",
          orgSlug: "ogs-test",
        });
      });

      await waitFor(() => {
        expect(mockToastSuccess).toHaveBeenCalledWith(
          "Registrierung erfolgreich! Deine Organisation wird geprüft.",
        );
      });
    });

    it("shows loading state during submission", async () => {
      let resolvePromise: ((value: unknown) => void) | undefined;
      (signupWithOrganization as Mock).mockImplementation(
        () =>
          new Promise((resolve) => {
            resolvePromise = resolve;
          }),
      );

      render(<SignupForm />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: /Wird registriert.../ }),
        ).toBeInTheDocument();
      });

      // Inputs should be disabled during submission
      expect(screen.getByLabelText("Name")).toBeDisabled();
      expect(screen.getByLabelText("E-Mail-Adresse")).toBeDisabled();

      // Cleanup: resolve the promise
      resolvePromise?.({ success: true });
    });

    it("redirects to pending page after successful submission", async () => {
      vi.useFakeTimers();
      (signupWithOrganization as Mock).mockResolvedValue({ success: true });

      render(<SignupForm />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });
      fireEvent.click(submitButton);

      // Wait for the promise to resolve
      await vi.runAllTimersAsync();

      expect(mockToastSuccess).toHaveBeenCalled();

      // Fast-forward timers to trigger redirect
      await vi.advanceTimersByTimeAsync(1500);

      expect(mockPush).toHaveBeenCalledWith("/signup/pending");
      expect(mockRefresh).toHaveBeenCalled();
    });
  });

  // =============================================================================
  // Error Handling Tests
  // =============================================================================

  describe("error handling", () => {
    const fillValidForm = () => {
      const nameInput = screen.getByLabelText("Name");
      fireEvent.change(nameInput, { target: { value: "Test User" } });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      const passwordInput = screen.getByLabelText("Passwort");
      fireEvent.change(passwordInput, { target: { value: "StrongP@ss1" } });

      const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");
      fireEvent.change(confirmPasswordInput, {
        target: { value: "StrongP@ss1" },
      });

      const orgNameInput = screen.getByLabelText("Name der Organisation");
      fireEvent.change(orgNameInput, { target: { value: "OGS Test" } });
    };

    it("shows error for USER_ALREADY_EXISTS", async () => {
      (signupWithOrganization as Mock).mockRejectedValue(
        new SignupWithOrgException(
          "Email already registered",
          "USER_ALREADY_EXISTS",
        ),
      );

      render(<SignupForm />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(
            "Diese E-Mail-Adresse ist bereits registriert. Bitte melde dich an oder verwende eine andere E-Mail.",
          ),
        ).toBeInTheDocument();
      });
    });

    it("shows error for SLUG_ALREADY_EXISTS", async () => {
      (signupWithOrganization as Mock).mockRejectedValue(
        new SignupWithOrgException("Slug taken", "SLUG_ALREADY_EXISTS"),
      );

      render(<SignupForm />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(
            "Diese Subdomain ist bereits vergeben. Bitte wähle eine andere.",
          ),
        ).toBeInTheDocument();
      });
    });

    it("shows generic SignupWithOrgException message", async () => {
      (signupWithOrganization as Mock).mockRejectedValue(
        new SignupWithOrgException("Custom error message", "UNKNOWN_ERROR"),
      );

      render(<SignupForm />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText("Custom error message")).toBeInTheDocument();
      });
    });

    it("shows generic Error message", async () => {
      (signupWithOrganization as Mock).mockRejectedValue(
        new Error("Network error"),
      );

      render(<SignupForm />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText("Network error")).toBeInTheDocument();
      });
    });

    it("shows fallback error for non-Error throws", async () => {
      (signupWithOrganization as Mock).mockRejectedValue("String error");

      render(<SignupForm />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText("Bei der Registrierung ist ein Fehler aufgetreten."),
        ).toBeInTheDocument();
      });
    });

    it("shows offline error when navigator is offline", async () => {
      Object.defineProperty(navigator, "onLine", {
        value: false,
        writable: true,
        configurable: true,
      });

      (signupWithOrganization as Mock).mockRejectedValue(
        new Error("Network error"),
      );

      render(<SignupForm />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(
            "Keine Netzwerkverbindung. Bitte überprüfe deine Internetverbindung und versuche es erneut.",
          ),
        ).toBeInTheDocument();
      });
    });

    it("clears error when form is resubmitted", async () => {
      (signupWithOrganization as Mock)
        .mockRejectedValueOnce(new Error("First error"))
        .mockResolvedValueOnce({ success: true });

      render(<SignupForm />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });

      // First submission - error
      fireEvent.click(submitButton);
      await waitFor(() => {
        expect(screen.getByText("First error")).toBeInTheDocument();
      });

      // Wait for isSubmitting to become false
      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: /Organisation registrieren/ }),
        ).not.toBeDisabled();
      });

      // Second submission - should clear error
      fireEvent.click(submitButton);
      await waitFor(() => {
        expect(screen.queryByText("First error")).not.toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Edge Cases Tests
  // =============================================================================

  describe("edge cases", () => {
    it("trims whitespace from input fields before submission", async () => {
      (signupWithOrganization as Mock).mockResolvedValue({ success: true });

      render(<SignupForm />);

      const nameInput = screen.getByLabelText("Name");
      fireEvent.change(nameInput, { target: { value: "  Test User  " } });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      fireEvent.change(emailInput, {
        target: { value: "  test@example.com  " },
      });

      const passwordInput = screen.getByLabelText("Passwort");
      fireEvent.change(passwordInput, { target: { value: "StrongP@ss1" } });

      const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");
      fireEvent.change(confirmPasswordInput, {
        target: { value: "StrongP@ss1" },
      });

      const orgNameInput = screen.getByLabelText("Name der Organisation");
      fireEvent.change(orgNameInput, { target: { value: "  OGS Test  " } });

      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(signupWithOrganization).toHaveBeenCalledWith({
          name: "Test User",
          email: "test@example.com",
          password: "StrongP@ss1",
          orgName: "OGS Test",
          orgSlug: "ogs-test",
        });
      });
    });

    it("handles name with only whitespace as invalid", async () => {
      render(<SignupForm />);

      const nameInput = screen.getByLabelText("Name");
      fireEvent.change(nameInput, { target: { value: "   " } });

      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });
      fireEvent.click(submitButton);

      expect(
        screen.getByText("Bitte gib deinen Namen an."),
      ).toBeInTheDocument();
    });

    it("handles org name with only whitespace as invalid", async () => {
      render(<SignupForm />);

      const nameInput = screen.getByLabelText("Name");
      fireEvent.change(nameInput, { target: { value: "Test User" } });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      const passwordInput = screen.getByLabelText("Passwort");
      fireEvent.change(passwordInput, { target: { value: "StrongP@ss1" } });

      const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");
      fireEvent.change(confirmPasswordInput, {
        target: { value: "StrongP@ss1" },
      });

      const orgNameInput = screen.getByLabelText("Name der Organisation");
      fireEvent.change(orgNameInput, { target: { value: "   " } });

      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });
      fireEvent.click(submitButton);

      expect(
        screen.getByText("Bitte gib den Namen deiner Organisation an."),
      ).toBeInTheDocument();
    });

    it("shows error for slug ending with hyphen", () => {
      render(<SignupForm />);

      const slugInput = screen.getByLabelText("Subdomain");

      fireEvent.change(slugInput, { target: { value: "invalid-" } });

      expect(
        screen.getByText(/Subdomain darf nicht mit einem Bindestrich enden/),
      ).toBeInTheDocument();
    });

    it("shows error for consecutive hyphens in slug", () => {
      render(<SignupForm />);

      const slugInput = screen.getByLabelText("Subdomain");

      fireEvent.change(slugInput, { target: { value: "test--slug" } });

      expect(
        screen.getByText(
          /Subdomain darf keine aufeinanderfolgenden Bindestriche enthalten/,
        ),
      ).toBeInTheDocument();
    });

    it("shows error for slug too long", () => {
      render(<SignupForm />);

      const slugInput = screen.getByLabelText("Subdomain");

      const longSlug =
        "this-is-a-very-long-slug-that-exceeds-the-maximum-length-allowed";
      fireEvent.change(slugInput, { target: { value: longSlug } });

      expect(
        screen.getByText(/Subdomain darf maximal 30 Zeichen haben/),
      ).toBeInTheDocument();
    });

    it("normalizes slug when submitted", async () => {
      (signupWithOrganization as Mock).mockResolvedValue({ success: true });

      render(<SignupForm />);

      const nameInput = screen.getByLabelText("Name");
      fireEvent.change(nameInput, { target: { value: "Test User" } });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      const passwordInput = screen.getByLabelText("Passwort");
      fireEvent.change(passwordInput, { target: { value: "StrongP@ss1" } });

      const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");
      fireEvent.change(confirmPasswordInput, {
        target: { value: "StrongP@ss1" },
      });

      const orgNameInput = screen.getByLabelText("Name der Organisation");
      fireEvent.change(orgNameInput, { target: { value: "OGS Test" } });

      // Slug is auto-generated as lowercase
      const submitButton = screen.getByRole("button", {
        name: /Organisation registrieren/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(signupWithOrganization).toHaveBeenCalledWith(
          expect.objectContaining({
            orgSlug: "ogs-test",
          }),
        );
      });
    });
  });
});
