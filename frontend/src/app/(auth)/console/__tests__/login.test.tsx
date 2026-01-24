/**
 * Tests for app/(auth)/console/login/page.tsx
 *
 * Tests the console login form including:
 * - Initial loading state
 * - Session checking
 * - Login form rendering
 * - Form submission
 * - SaaS admin verification
 * - Error handling
 * - Password visibility toggle
 */

import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Use vi.hoisted to define mocks before they're used in vi.mock (avoids hoisting issues)
const { mockPush, mockSearchParamsGet, mockSignIn, mockSessionRef, mockFetch } =
  vi.hoisted(() => ({
    mockPush: vi.fn(),
    mockSearchParamsGet: vi.fn(),
    mockSignIn: vi.fn(),
    mockSessionRef: {
      current: {
        data: null as { user: { id: string; email: string } | null } | null,
        isPending: false,
      },
    },
    mockFetch: vi.fn(),
  }));

// Mock next/navigation
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
  useSearchParams: () => ({ get: mockSearchParamsGet }),
}));

// Mock auth-client
vi.mock("~/lib/auth-client", () => ({
  useSession: () => mockSessionRef.current,
  signIn: { email: mockSignIn },
}));

// Mock fetch
globalThis.fetch = mockFetch;

// Import after mocks
import ConsoleLoginPage from "../login/page";

// Store original console.error
const originalConsoleError = console.error;

beforeEach(() => {
  vi.clearAllMocks();
  mockFetch.mockReset();
  mockSessionRef.current = { data: null, isPending: false };
  mockSearchParamsGet.mockReturnValue(null);

  // Suppress console.error during tests
  console.error = vi.fn();
});

afterEach(() => {
  console.error = originalConsoleError;
});

describe("ConsoleLoginPage", () => {
  // =============================================================================
  // Loading State Tests
  // =============================================================================

  describe("loading state", () => {
    it("shows loading spinner when checking session", () => {
      mockSessionRef.current = { data: null, isPending: true };

      render(<ConsoleLoginPage />);

      // Should show loading component
      const loadingContainer = document.querySelector(".flex.min-h-dvh");
      expect(loadingContainer).toBeInTheDocument();
    });

    it("shows loading when checkingAuth is true", () => {
      mockSessionRef.current = { data: null, isPending: true };

      const { container } = render(<ConsoleLoginPage />);

      // Should show loading due to session pending
      expect(container.querySelector(".flex.min-h-dvh")).toBeInTheDocument();
    });
  });

  // =============================================================================
  // Form Rendering Tests
  // =============================================================================

  describe("form rendering", () => {
    it("renders the login form when not authenticated", async () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(screen.getByText("Plattform-Konsole")).toBeInTheDocument();
      });
    });

    it("renders email input field", async () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(screen.getByText("E-Mail-Adresse")).toBeInTheDocument();
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });
    });

    it("renders password input field", async () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(screen.getByText("Passwort")).toBeInTheDocument();
        expect(screen.getByLabelText("Passwort")).toBeInTheDocument();
      });
    });

    it("renders submit button", async () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: "Anmelden" }),
        ).toBeInTheDocument();
      });
    });

    it("renders MOTO logo", async () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        const logo = screen.getByAltText("MOTO Logo");
        expect(logo).toBeInTheDocument();
      });
    });

    it("renders administrator info text", async () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(
          screen.getByText("Anmeldung für Administratoren"),
        ).toBeInTheDocument();
        expect(
          screen.getByText("Nur für autorisierte Plattform-Administratoren"),
        ).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Session Check Tests
  // =============================================================================

  describe("session check", () => {
    it("redirects to console when already logged in as SaaS admin", async () => {
      mockSessionRef.current = {
        data: { user: { id: "user-1", email: "admin@example.com" } },
        isPending: false,
      };
      mockFetch.mockResolvedValueOnce({
        json: () => Promise.resolve({ isSaasAdmin: true }),
      });

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith("/console");
      });
    });

    it("shows error when logged in but not SaaS admin", async () => {
      mockSessionRef.current = {
        data: { user: { id: "user-1", email: "user@example.com" } },
        isPending: false,
      };
      mockFetch.mockResolvedValueOnce({
        json: () => Promise.resolve({ isSaasAdmin: false }),
      });

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(
          screen.getByText(
            "Zugriff verweigert. Nur Plattform-Administratoren können sich hier anmelden.",
          ),
        ).toBeInTheDocument();
      });
    });

    it("handles session check error gracefully", async () => {
      mockSessionRef.current = {
        data: { user: { id: "user-1", email: "admin@example.com" } },
        isPending: false,
      };
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      render(<ConsoleLoginPage />);

      // Should show login form after error
      await waitFor(() => {
        expect(screen.getByText("Plattform-Konsole")).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // URL Error Parameter Tests
  // =============================================================================

  describe("URL error parameter", () => {
    it("shows error message when URL has error=Unauthorized", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSearchParamsGet.mockImplementation((key: string) => {
        if (key === "error") return "Unauthorized";
        return null;
      });

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(
          screen.getByText(
            "Zugriff verweigert. Nur Plattform-Administratoren können auf die Konsole zugreifen.",
          ),
        ).toBeInTheDocument();
      });
    });

    it("does not show error message when URL has no error parameter", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSearchParamsGet.mockReturnValue(null);

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(screen.getByText("Plattform-Konsole")).toBeInTheDocument();
      });

      // Should not have error message initially
      expect(screen.queryByText(/Zugriff verweigert/)).not.toBeInTheDocument();
    });
  });

  // =============================================================================
  // Form Submission Tests
  // =============================================================================

  describe("form submission", () => {
    it("submits login form with email and password", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSignIn.mockResolvedValueOnce({});
      mockFetch.mockResolvedValueOnce({
        json: () => Promise.resolve({ isSaasAdmin: true }),
      });

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");
      const submitButton = screen.getByRole("button", { name: "Anmelden" });

      fireEvent.change(emailInput, { target: { value: "admin@example.com" } });
      fireEvent.change(passwordInput, { target: { value: "password123" } });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(mockSignIn).toHaveBeenCalledWith({
          email: "admin@example.com",
          password: "password123",
        });
      });
    });

    it("shows loading state during submission", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      let resolveSignIn: ((value: unknown) => void) | undefined;
      mockSignIn.mockImplementation(
        () =>
          new Promise((resolve) => {
            resolveSignIn = resolve;
          }),
      );

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");
      const submitButton = screen.getByRole("button", { name: "Anmelden" });

      fireEvent.change(emailInput, { target: { value: "admin@example.com" } });
      fireEvent.change(passwordInput, { target: { value: "password123" } });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: "Anmeldung läuft..." }),
        ).toBeInTheDocument();
      });

      // Cleanup
      resolveSignIn?.({});
    });

    it("redirects to console after successful login as SaaS admin", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSignIn.mockResolvedValueOnce({});
      mockFetch.mockResolvedValueOnce({
        json: () => Promise.resolve({ isSaasAdmin: true }),
      });

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");
      const submitButton = screen.getByRole("button", { name: "Anmelden" });

      fireEvent.change(emailInput, { target: { value: "admin@example.com" } });
      fireEvent.change(passwordInput, { target: { value: "password123" } });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith("/console");
      });
    });

    it("shows error when login succeeds but user is not SaaS admin", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSignIn.mockResolvedValueOnce({});
      mockFetch.mockResolvedValueOnce({
        json: () => Promise.resolve({ isSaasAdmin: false }),
      });

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");
      const submitButton = screen.getByRole("button", { name: "Anmelden" });

      fireEvent.change(emailInput, { target: { value: "user@example.com" } });
      fireEvent.change(passwordInput, { target: { value: "password123" } });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(
            "Zugriff verweigert. Nur Plattform-Administratoren können sich hier anmelden.",
          ),
        ).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Error Handling Tests
  // =============================================================================

  describe("error handling", () => {
    it("shows error when signIn returns error", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSignIn.mockResolvedValueOnce({
        error: { message: "Invalid credentials" },
      });

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");
      const submitButton = screen.getByRole("button", { name: "Anmelden" });

      fireEvent.change(emailInput, { target: { value: "admin@example.com" } });
      fireEvent.change(passwordInput, { target: { value: "wrongpassword" } });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText("Invalid credentials")).toBeInTheDocument();
      });
    });

    it("shows default error when signIn error has no message", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSignIn.mockResolvedValueOnce({
        error: {},
      });

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");
      const submitButton = screen.getByRole("button", { name: "Anmelden" });

      fireEvent.change(emailInput, { target: { value: "admin@example.com" } });
      fireEvent.change(passwordInput, { target: { value: "password" } });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText("Ungültige E-Mail oder Passwort"),
        ).toBeInTheDocument();
      });
    });

    it("shows generic error on network failure", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSignIn.mockRejectedValueOnce(new Error("Network error"));

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");
      const submitButton = screen.getByRole("button", { name: "Anmelden" });

      fireEvent.change(emailInput, { target: { value: "admin@example.com" } });
      fireEvent.change(passwordInput, { target: { value: "password" } });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText("Anmeldefehler. Bitte versuchen Sie es erneut."),
        ).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Password Visibility Toggle Tests
  // =============================================================================

  describe("password visibility toggle", () => {
    it("shows password as dots by default", async () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        const passwordInput = screen.getByLabelText("Passwort");
        expect(passwordInput).toHaveAttribute("type", "password");
      });
    });

    it("shows password as text when toggle clicked", async () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("Passwort")).toBeInTheDocument();
      });

      const toggleButton = screen.getByRole("button", {
        name: "Passwort anzeigen",
      });
      fireEvent.click(toggleButton);

      const passwordInput = screen.getByLabelText("Passwort");
      expect(passwordInput).toHaveAttribute("type", "text");
    });

    it("hides password when toggle clicked again", async () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("Passwort")).toBeInTheDocument();
      });

      const toggleButton = screen.getByRole("button", {
        name: "Passwort anzeigen",
      });
      fireEvent.click(toggleButton); // Show
      fireEvent.click(
        screen.getByRole("button", { name: "Passwort verbergen" }),
      ); // Hide

      const passwordInput = screen.getByLabelText("Passwort");
      expect(passwordInput).toHaveAttribute("type", "password");
    });

    it("has correct aria-label based on visibility state", async () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: "Passwort anzeigen" }),
        ).toBeInTheDocument();
      });

      const toggleButton = screen.getByRole("button", {
        name: "Passwort anzeigen",
      });
      fireEvent.click(toggleButton);

      expect(
        screen.getByRole("button", { name: "Passwort verbergen" }),
      ).toBeInTheDocument();
    });
  });

  // =============================================================================
  // Form Input Tests
  // =============================================================================

  describe("form inputs", () => {
    it("updates email state on input change", async () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      expect(emailInput).toHaveValue("test@example.com");
    });

    it("updates password state on input change", async () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("Passwort")).toBeInTheDocument();
      });

      const passwordInput = screen.getByLabelText("Passwort");
      fireEvent.change(passwordInput, { target: { value: "mypassword" } });

      expect(passwordInput).toHaveValue("mypassword");
    });

    it("prevents default form submission", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSignIn.mockResolvedValueOnce({});
      mockFetch.mockResolvedValueOnce({
        json: () => Promise.resolve({ isSaasAdmin: true }),
      });

      render(<ConsoleLoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const form = document.querySelector("form")!;
      const submitEvent = new Event("submit", {
        bubbles: true,
        cancelable: true,
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");

      fireEvent.change(emailInput, { target: { value: "admin@example.com" } });
      fireEvent.change(passwordInput, { target: { value: "password" } });

      form.dispatchEvent(submitEvent);

      // Should not reload the page (form submission is handled)
      await waitFor(() => {
        expect(mockSignIn).toHaveBeenCalled();
      });
    });
  });
});
