/**
 * Tests for BetterAuthInvitationForm component
 *
 * This file tests:
 * - Form rendering with invitation data
 * - Password visibility toggles
 * - Password requirement indicators
 * - Form field validation
 * - Form submission (success and error cases)
 * - Error handling for various API responses
 */

import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { BetterAuthInvitationForm } from "./betterauth-invitation-form";

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

// Mock authClient - use hoisted mocks
const mockSignUpEmail = vi.hoisted(() => vi.fn());
const mockAcceptInvitation = vi.hoisted(() => vi.fn());
const mockSetActive = vi.hoisted(() => vi.fn());

vi.mock("~/lib/auth-client", () => ({
  authClient: {
    signUp: {
      email: mockSignUpEmail,
    },
    organization: {
      acceptInvitation: mockAcceptInvitation,
      setActive: mockSetActive,
    },
  },
}));

const defaultProps = {
  invitationId: "invitation-123",
  email: "invited@example.com",
  organizationName: "Test Organization",
  role: "Admin",
};

describe("BetterAuthInvitationForm", () => {
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
    it("renders invitation info with email", () => {
      render(<BetterAuthInvitationForm {...defaultProps} />);

      expect(screen.getByText("Einladung für")).toBeInTheDocument();
      expect(screen.getByText("invited@example.com")).toBeInTheDocument();
    });

    it("renders organization name when provided", () => {
      render(<BetterAuthInvitationForm {...defaultProps} />);

      expect(screen.getByText("Organisation")).toBeInTheDocument();
      expect(screen.getByText("Test Organization")).toBeInTheDocument();
    });

    it("renders role when provided", () => {
      render(<BetterAuthInvitationForm {...defaultProps} />);

      expect(screen.getByText("Rolle")).toBeInTheDocument();
      expect(screen.getByText("Admin")).toBeInTheDocument();
    });

    it("does not render organization name when not provided", () => {
      render(
        <BetterAuthInvitationForm
          {...defaultProps}
          organizationName={undefined}
        />,
      );

      expect(screen.queryByText("Organisation")).not.toBeInTheDocument();
    });

    it("does not render role when not provided", () => {
      render(<BetterAuthInvitationForm {...defaultProps} role={undefined} />);

      expect(screen.queryByText("Rolle")).not.toBeInTheDocument();
    });

    it("renders name input", () => {
      render(<BetterAuthInvitationForm {...defaultProps} />);

      expect(screen.getByLabelText("Name")).toBeInTheDocument();
    });

    it("renders email input as locked/disabled", () => {
      render(<BetterAuthInvitationForm {...defaultProps} />);

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      expect(emailInput).toBeDisabled();
      expect(emailInput).toHaveValue("invited@example.com");
    });

    it("renders password fields", () => {
      render(<BetterAuthInvitationForm {...defaultProps} />);

      expect(screen.getByLabelText("Passwort")).toBeInTheDocument();
      expect(screen.getByLabelText("Passwort bestätigen")).toBeInTheDocument();
    });

    it("renders password requirements", () => {
      render(<BetterAuthInvitationForm {...defaultProps} />);

      expect(screen.getByText("Passwortanforderungen")).toBeInTheDocument();
      expect(screen.getByText("Mindestens 8 Zeichen")).toBeInTheDocument();
      expect(screen.getByText("Ein Großbuchstabe")).toBeInTheDocument();
      expect(screen.getByText("Ein Kleinbuchstabe")).toBeInTheDocument();
      expect(screen.getByText("Eine Zahl")).toBeInTheDocument();
      expect(screen.getByText("Ein Sonderzeichen")).toBeInTheDocument();
    });

    it("renders submit button", () => {
      render(<BetterAuthInvitationForm {...defaultProps} />);

      expect(
        screen.getByRole("button", { name: /Konto erstellen & beitreten/ }),
      ).toBeInTheDocument();
    });

    it("renders link to login page", () => {
      render(<BetterAuthInvitationForm {...defaultProps} />);

      // Text is part of a larger paragraph, use regex
      expect(screen.getByText(/Bereits ein Konto\?/)).toBeInTheDocument();
      expect(screen.getByText("Anmelden")).toHaveAttribute("href", "/");
    });
  });

  // =============================================================================
  // Password Visibility Toggle Tests
  // =============================================================================

  describe("password visibility toggles", () => {
    it("toggles password visibility when button is clicked", () => {
      render(<BetterAuthInvitationForm {...defaultProps} />);

      const passwordInput = screen.getByLabelText("Passwort");
      expect(passwordInput).toHaveAttribute("type", "password");

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
      render(<BetterAuthInvitationForm {...defaultProps} />);

      const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");
      expect(confirmPasswordInput).toHaveAttribute("type", "password");

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
      render(<BetterAuthInvitationForm {...defaultProps} />);

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
    it("updates requirements as password is typed", () => {
      render(<BetterAuthInvitationForm {...defaultProps} />);

      const passwordInput = screen.getByLabelText("Passwort");

      // Type a password that meets length requirement only
      fireEvent.change(passwordInput, { target: { value: "12345678" } });

      const requirementElements = screen.getAllByText("✓");
      expect(requirementElements.length).toBeGreaterThan(0);
    });

    it("shows all checkmarks when password meets all requirements", () => {
      render(<BetterAuthInvitationForm {...defaultProps} />);

      const passwordInput = screen.getByLabelText("Passwort");

      fireEvent.change(passwordInput, { target: { value: "StrongP@ss1" } });

      const checkmarks = screen.getAllByText("✓");
      expect(checkmarks.length).toBe(5);
    });
  });

  // =============================================================================
  // Form Validation Tests
  // =============================================================================

  describe("form validation", () => {
    it("shows error when name is empty", async () => {
      render(<BetterAuthInvitationForm {...defaultProps} />);

      const submitButton = screen.getByRole("button", {
        name: /Konto erstellen & beitreten/,
      });
      fireEvent.click(submitButton);

      expect(
        screen.getByText("Bitte gib deinen Namen an."),
      ).toBeInTheDocument();
    });

    it("shows error when name is only whitespace", async () => {
      render(<BetterAuthInvitationForm {...defaultProps} />);

      const nameInput = screen.getByLabelText("Name");
      fireEvent.change(nameInput, { target: { value: "   " } });

      const submitButton = screen.getByRole("button", {
        name: /Konto erstellen & beitreten/,
      });
      fireEvent.click(submitButton);

      expect(
        screen.getByText("Bitte gib deinen Namen an."),
      ).toBeInTheDocument();
    });

    it("shows error when password does not meet requirements", async () => {
      render(<BetterAuthInvitationForm {...defaultProps} />);

      const nameInput = screen.getByLabelText("Name");
      fireEvent.change(nameInput, { target: { value: "Test User" } });

      const passwordInput = screen.getByLabelText("Passwort");
      fireEvent.change(passwordInput, { target: { value: "weak" } });

      const submitButton = screen.getByRole("button", {
        name: /Konto erstellen & beitreten/,
      });
      fireEvent.click(submitButton);

      expect(
        screen.getByText(
          "Das Passwort erfüllt noch nicht alle Sicherheitsanforderungen.",
        ),
      ).toBeInTheDocument();
    });

    it("shows error when passwords do not match", async () => {
      render(<BetterAuthInvitationForm {...defaultProps} />);

      const nameInput = screen.getByLabelText("Name");
      fireEvent.change(nameInput, { target: { value: "Test User" } });

      const passwordInput = screen.getByLabelText("Passwort");
      fireEvent.change(passwordInput, { target: { value: "StrongP@ss1" } });

      const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");
      fireEvent.change(confirmPasswordInput, {
        target: { value: "DifferentP@ss1" },
      });

      const submitButton = screen.getByRole("button", {
        name: /Konto erstellen & beitreten/,
      });
      fireEvent.click(submitButton);

      expect(
        screen.getByText("Die Passwörter stimmen nicht überein."),
      ).toBeInTheDocument();
    });
  });

  // =============================================================================
  // Form Submission Tests
  // =============================================================================

  describe("form submission", () => {
    const fillValidForm = () => {
      const nameInput = screen.getByLabelText("Name");
      fireEvent.change(nameInput, { target: { value: "Test User" } });

      const passwordInput = screen.getByLabelText("Passwort");
      fireEvent.change(passwordInput, { target: { value: "StrongP@ss1" } });

      const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");
      fireEvent.change(confirmPasswordInput, {
        target: { value: "StrongP@ss1" },
      });
    };

    it("submits form successfully", async () => {
      mockSignUpEmail.mockResolvedValue({
        data: { user: { id: "user-123" } },
        error: null,
      });
      mockAcceptInvitation.mockResolvedValue({
        data: { invitation: { organizationId: "org-123" } },
        error: null,
      });
      mockSetActive.mockResolvedValue({});

      render(<BetterAuthInvitationForm {...defaultProps} />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Konto erstellen & beitreten/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(mockSignUpEmail).toHaveBeenCalledWith({
          name: "Test User",
          email: "invited@example.com",
          password: "StrongP@ss1",
        });
      });

      await waitFor(() => {
        expect(mockAcceptInvitation).toHaveBeenCalledWith({
          invitationId: "invitation-123",
        });
      });

      await waitFor(() => {
        expect(mockSetActive).toHaveBeenCalledWith({
          organizationId: "org-123",
        });
      });

      await waitFor(() => {
        expect(mockToastSuccess).toHaveBeenCalledWith(
          "Willkommen bei Test Organization!",
        );
      });
    });

    it("shows welcome message without org name when not provided", async () => {
      mockSignUpEmail.mockResolvedValue({
        data: { user: { id: "user-123" } },
        error: null,
      });
      mockAcceptInvitation.mockResolvedValue({
        data: { invitation: { organizationId: "org-123" } },
        error: null,
      });
      mockSetActive.mockResolvedValue({});

      render(
        <BetterAuthInvitationForm
          {...defaultProps}
          organizationName={undefined}
        />,
      );
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Konto erstellen & beitreten/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(mockToastSuccess).toHaveBeenCalledWith(
          "Willkommen in der Organisation!",
        );
      });
    });

    it("shows loading state during submission", async () => {
      let resolvePromise: ((value: unknown) => void) | undefined;
      mockSignUpEmail.mockImplementation(
        () =>
          new Promise((resolve) => {
            resolvePromise = resolve;
          }),
      );

      render(<BetterAuthInvitationForm {...defaultProps} />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Konto erstellen & beitreten/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: /Wird verarbeitet.../ }),
        ).toBeInTheDocument();
      });

      // Inputs should be disabled during submission
      expect(screen.getByLabelText("Name")).toBeDisabled();

      // Cleanup
      resolvePromise?.({ data: null, error: null });
    });

    it("redirects to dashboard after successful submission", async () => {
      vi.useFakeTimers();
      mockSignUpEmail.mockResolvedValue({
        data: { user: { id: "user-123" } },
        error: null,
      });
      mockAcceptInvitation.mockResolvedValue({
        data: { invitation: { organizationId: "org-123" } },
        error: null,
      });
      mockSetActive.mockResolvedValue({});

      render(<BetterAuthInvitationForm {...defaultProps} />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Konto erstellen & beitreten/,
      });
      fireEvent.click(submitButton);

      await vi.runAllTimersAsync();

      // Fast-forward timers to trigger redirect
      await vi.advanceTimersByTimeAsync(1000);

      expect(mockPush).toHaveBeenCalledWith("/dashboard");
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

      const passwordInput = screen.getByLabelText("Passwort");
      fireEvent.change(passwordInput, { target: { value: "StrongP@ss1" } });

      const confirmPasswordInput = screen.getByLabelText("Passwort bestätigen");
      fireEvent.change(confirmPasswordInput, {
        target: { value: "StrongP@ss1" },
      });
    };

    it("shows error for USER_ALREADY_EXISTS", async () => {
      mockSignUpEmail.mockResolvedValue({
        data: null,
        error: { code: "USER_ALREADY_EXISTS" },
      });

      render(<BetterAuthInvitationForm {...defaultProps} />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Konto erstellen & beitreten/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(
            "Diese E-Mail-Adresse ist bereits registriert. Bitte melde dich an, um die Einladung anzunehmen.",
          ),
        ).toBeInTheDocument();
      });
    });

    it("shows generic signup error message", async () => {
      mockSignUpEmail.mockResolvedValue({
        data: null,
        error: { code: "OTHER_ERROR", message: "Something went wrong" },
      });

      render(<BetterAuthInvitationForm {...defaultProps} />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Konto erstellen & beitreten/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText("Something went wrong")).toBeInTheDocument();
      });
    });

    it("shows fallback signup error message when no message provided", async () => {
      mockSignUpEmail.mockResolvedValue({
        data: null,
        error: { code: "OTHER_ERROR" },
      });

      render(<BetterAuthInvitationForm {...defaultProps} />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Konto erstellen & beitreten/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText("Bei der Registrierung ist ein Fehler aufgetreten."),
        ).toBeInTheDocument();
      });
    });

    it("handles accept invitation error and redirects to dashboard", async () => {
      mockSignUpEmail.mockResolvedValue({
        data: { user: { id: "user-123" } },
        error: null,
      });
      mockAcceptInvitation.mockResolvedValue({
        data: null,
        error: { message: "Invitation expired" },
      });

      render(<BetterAuthInvitationForm {...defaultProps} />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Konto erstellen & beitreten/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(mockToastError).toHaveBeenCalledWith(
          "Fehler beim Beitreten der Organisation.",
        );
      });

      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith("/dashboard");
      });
    });

    it("handles network/offline error", async () => {
      Object.defineProperty(navigator, "onLine", {
        value: false,
        writable: true,
        configurable: true,
      });

      mockSignUpEmail.mockRejectedValue(new Error("Network error"));

      render(<BetterAuthInvitationForm {...defaultProps} />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Konto erstellen & beitreten/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(
            "Keine Netzwerkverbindung. Bitte überprüfe deine Internetverbindung.",
          ),
        ).toBeInTheDocument();
      });
    });

    it("handles generic thrown error", async () => {
      mockSignUpEmail.mockRejectedValue(new Error("Server error"));

      render(<BetterAuthInvitationForm {...defaultProps} />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Konto erstellen & beitreten/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText("Server error")).toBeInTheDocument();
      });
    });

    it("handles non-Error thrown value", async () => {
      mockSignUpEmail.mockRejectedValue("String error");

      render(<BetterAuthInvitationForm {...defaultProps} />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Konto erstellen & beitreten/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText("Bei der Registrierung ist ein Fehler aufgetreten."),
        ).toBeInTheDocument();
      });
    });

    it("skips setActive when orgId is not returned", async () => {
      mockSignUpEmail.mockResolvedValue({
        data: { user: { id: "user-123" } },
        error: null,
      });
      mockAcceptInvitation.mockResolvedValue({
        data: { invitation: {} }, // No organizationId
        error: null,
      });

      render(<BetterAuthInvitationForm {...defaultProps} />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Konto erstellen & beitreten/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(mockSetActive).not.toHaveBeenCalled();
      });
    });

    it("clears error on resubmission", async () => {
      // First call fails
      mockSignUpEmail.mockResolvedValueOnce({
        data: null,
        error: { code: "OTHER_ERROR", message: "First error" },
      });

      render(<BetterAuthInvitationForm {...defaultProps} />);
      fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Konto erstellen & beitreten/,
      });

      // First submission - error
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText("First error")).toBeInTheDocument();
      });

      // Wait for isSubmitting to become false
      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: /Konto erstellen & beitreten/ }),
        ).not.toBeDisabled();
      });

      // Setup second call to succeed
      mockSignUpEmail.mockResolvedValueOnce({
        data: { user: { id: "user-123" } },
        error: null,
      });
      mockAcceptInvitation.mockResolvedValue({
        data: { invitation: { organizationId: "org-123" } },
        error: null,
      });
      mockSetActive.mockResolvedValue({});

      // Second submission - error should be cleared
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(screen.queryByText("First error")).not.toBeInTheDocument();
      });
    });
  });
});
