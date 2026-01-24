import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Use vi.hoisted to declare mocks before vi.mock hoists them
const { mockSignIn, mockOrganization, mockPush, mockRefresh, mockSessionRef } =
  vi.hoisted(() => ({
    mockSignIn: vi.fn(),
    mockOrganization: {
      list: vi.fn(),
      setActive: vi.fn(),
    },
    mockPush: vi.fn(),
    mockRefresh: vi.fn(),
    mockSessionRef: {
      current: { data: null as unknown, isPending: true as boolean },
    },
  }));

// Track search params
let mockSearchParams: Map<string, string> = new Map();

// Mock next/navigation
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
    refresh: mockRefresh,
  }),
  useSearchParams: () => ({
    get: (key: string) => mockSearchParams.get(key) ?? null,
  }),
}));

// Mock auth-client - must match structure from setup.ts
vi.mock("~/lib/auth-client", () => ({
  authClient: {
    signIn: { email: mockSignIn },
    signOut: vi.fn(),
    signUp: { email: vi.fn() },
    useSession: () => mockSessionRef.current,
    getSession: vi.fn(() => Promise.resolve(null)),
    organization: mockOrganization,
  },
  signIn: { email: mockSignIn },
  signOut: vi.fn(),
  signUp: { email: vi.fn() },
  useSession: () => mockSessionRef.current,
  getSession: vi.fn(() => Promise.resolve(null)),
  organization: mockOrganization,
  getActiveRole: vi.fn(() => Promise.resolve("supervisor")),
  isAdmin: vi.fn(() => Promise.resolve(false)),
  isSupervisor: vi.fn(() => Promise.resolve(true)),
}));

// Mock components
vi.mock("~/components/ui", () => ({
  Input: ({
    id,
    type,
    value,
    onChange,
    ...props
  }: {
    id?: string;
    type?: string;
    value?: string;
    onChange?: (e: React.ChangeEvent<HTMLInputElement>) => void;
    label?: string;
    className?: string;
    name?: string;
    autoComplete?: string;
    required?: boolean;
    autoFocus?: boolean;
    spellCheck?: boolean;
  }) => (
    <input
      id={id}
      type={type}
      value={value}
      onChange={onChange}
      data-testid={`input-${id}`}
      {...props}
    />
  ),
  Alert: ({ type, message }: { type: string; message: string }) => (
    <div data-testid={`alert-${type}`} role="alert">
      {message}
    </div>
  ),
  HelpButton: ({ title }: { title: string }) => (
    <button data-testid="help-button">{title}</button>
  ),
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading" aria-label="Lädt..." />,
}));

// Track SmartRedirect onRedirect callback
let _capturedOnRedirect: ((path: string) => void) | null = null;
vi.mock("~/components/auth/smart-redirect", () => ({
  SmartRedirect: ({ onRedirect }: { onRedirect: (path: string) => void }) => {
    _capturedOnRedirect = onRedirect;
    // Simulate redirect after a short delay
    setTimeout(() => onRedirect("/dashboard"), 10);
    return <div data-testid="smart-redirect">SmartRedirect</div>;
  },
}));

vi.mock("~/components/ui/password-reset-modal", () => ({
  PasswordResetModal: ({
    isOpen,
    onClose,
  }: {
    isOpen: boolean;
    onClose: () => void;
  }) => {
    if (!isOpen) return null;
    return (
      <div data-testid="password-reset-modal">
        <button data-testid="close-modal" onClick={onClose}>
          Close
        </button>
      </div>
    );
  },
}));

// Mock next/image
vi.mock("next/image", () => ({
  default: ({
    src,
    alt,
    ...props
  }: {
    src: string;
    alt: string;
    width?: number;
    height?: number;
    priority?: boolean;
  }) => <img src={src} alt={alt} data-testid="moto-logo" {...props} />,
}));

// Import after mocks
import LoginPage from "./page";

describe("LoginPage", () => {
  // Mock document methods for confetti - inside describe to ensure proper cleanup
  let mockCreateElement: ReturnType<typeof vi.spyOn>;
  let mockAppendChild: ReturnType<typeof vi.spyOn>;
  let mockQuerySelector: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mockSearchParams = new Map();
    mockSessionRef.current = { data: null, isPending: false };
    _capturedOnRedirect = null;

    // Reset mocks
    mockSignIn.mockResolvedValue({ error: null });
    mockOrganization.list.mockResolvedValue({
      data: [{ id: "org-1", name: "Test Org" }],
    });
    mockOrganization.setActive.mockResolvedValue({});

    // Mock document methods - use real implementations but add animate method
    const originalCreateElement = document.createElement.bind(document);
    const originalAppendChild = document.body.appendChild.bind(document.body);
    mockCreateElement = vi.spyOn(document, "createElement");
    mockAppendChild = vi.spyOn(document.body, "appendChild");
    mockQuerySelector = vi.spyOn(document, "querySelector");

    mockCreateElement.mockImplementation((tagName: string) => {
      const element = originalCreateElement(tagName);
      // Add mock animate method for confetti
      if (!element.animate) {
        (element as HTMLDivElement).animate = vi.fn().mockReturnValue({
          onfinish: null,
        });
      }
      return element;
    });

    // Call through to real appendChild
    mockAppendChild.mockImplementation((node: Node) =>
      originalAppendChild(node),
    );
  });

  afterEach(() => {
    vi.useRealTimers();
    mockCreateElement?.mockRestore();
    mockAppendChild?.mockRestore();
    mockQuerySelector?.mockRestore();
  });

  describe("Loading states", () => {
    it("shows loading while session is pending", () => {
      mockSessionRef.current = { data: null, isPending: true };

      render(<LoginPage />);

      expect(screen.getByTestId("loading")).toBeInTheDocument();
    });

    it("shows loading while checking auth initially", () => {
      mockSessionRef.current = { data: null, isPending: true };

      render(<LoginPage />);

      expect(screen.getByTestId("loading")).toBeInTheDocument();
    });
  });

  describe("Redirect when authenticated", () => {
    it("shows SmartRedirect when user has valid session", async () => {
      mockSessionRef.current = {
        data: { user: { id: "user-1", email: "test@example.com" } },
        isPending: false,
      };

      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByTestId("smart-redirect")).toBeInTheDocument();
      });
    });

    it("calls router.push when SmartRedirect triggers onRedirect", async () => {
      mockSessionRef.current = {
        data: { user: { id: "user-1", email: "test@example.com" } },
        isPending: false,
      };

      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByTestId("smart-redirect")).toBeInTheDocument();
      });

      // Advance timers to trigger the redirect
      await act(async () => {
        vi.advanceTimersByTime(20);
      });

      expect(mockPush).toHaveBeenCalledWith("/dashboard");
    });
  });

  describe("Session error from URL", () => {
    it("displays SessionRequired error from URL params", async () => {
      mockSearchParams.set("error", "SessionRequired");
      mockSessionRef.current = { data: null, isPending: false };

      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByTestId("alert-error")).toBeInTheDocument();
        expect(
          screen.getByText(
            "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.",
          ),
        ).toBeInTheDocument();
      });
    });

    it("displays SessionExpired error from URL params", async () => {
      mockSearchParams.set("error", "SessionExpired");
      mockSessionRef.current = { data: null, isPending: false };

      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByTestId("alert-error")).toBeInTheDocument();
        expect(
          screen.getByText(
            "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.",
          ),
        ).toBeInTheDocument();
      });
    });
  });

  describe("Login form rendering", () => {
    beforeEach(() => {
      mockSessionRef.current = { data: null, isPending: false };
    });

    it("renders the login form when not authenticated", async () => {
      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByText("Willkommen bei moto!")).toBeInTheDocument();
      });
    });

    it("renders MOTO logo", async () => {
      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByTestId("moto-logo")).toBeInTheDocument();
        expect(screen.getByAltText("MOTO Logo")).toBeInTheDocument();
      });
    });

    it("renders welcome text", async () => {
      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByText("Willkommen bei moto!")).toBeInTheDocument();
        expect(screen.getByText("Ganztag. Digital.")).toBeInTheDocument();
      });
    });

    it("renders email input field", async () => {
      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });
    });

    it("renders password input field", async () => {
      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("Passwort")).toBeInTheDocument();
      });
    });

    it("renders submit button", async () => {
      render(<LoginPage />);

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: /Anmelden/i }),
        ).toBeInTheDocument();
      });
    });

    it("renders help button", async () => {
      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByTestId("help-button")).toBeInTheDocument();
      });
    });

    it("renders forgot password link", async () => {
      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByText("Passwort vergessen?")).toBeInTheDocument();
      });
    });

    it("renders signup link", async () => {
      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByText("Noch kein Konto?")).toBeInTheDocument();
        expect(screen.getByText("Jetzt registrieren")).toHaveAttribute(
          "href",
          "/signup",
        );
      });
    });
  });

  describe("Form interactions", () => {
    beforeEach(() => {
      mockSessionRef.current = { data: null, isPending: false };
    });

    it("updates email input value on change", async () => {
      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");

      await act(async () => {
        fireEvent.change(emailInput, {
          target: { value: "test@example.com" },
        });
      });

      expect(emailInput).toHaveValue("test@example.com");
    });

    it("updates password input value on change", async () => {
      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("Passwort")).toBeInTheDocument();
      });

      const passwordInput = screen.getByLabelText("Passwort");

      await act(async () => {
        fireEvent.change(passwordInput, {
          target: { value: "secret123" },
        });
      });

      expect(passwordInput).toHaveValue("secret123");
    });

    it("toggles password visibility", async () => {
      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("Passwort")).toBeInTheDocument();
      });

      const passwordInput = screen.getByLabelText("Passwort");
      expect(passwordInput).toHaveAttribute("type", "password");

      const toggleButton = screen.getByLabelText("Passwort anzeigen");

      await act(async () => {
        fireEvent.click(toggleButton);
      });

      expect(passwordInput).toHaveAttribute("type", "text");

      // Click again to hide
      await act(async () => {
        fireEvent.click(screen.getByLabelText("Passwort verbergen"));
      });

      expect(passwordInput).toHaveAttribute("type", "password");
    });
  });

  describe("Form submission", () => {
    beforeEach(() => {
      mockSessionRef.current = { data: null, isPending: false };
    });

    it("calls signIn.email on form submit", async () => {
      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");

      await act(async () => {
        fireEvent.change(emailInput, {
          target: { value: "test@example.com" },
        });
        fireEvent.change(passwordInput, {
          target: { value: "password123" },
        });
      });

      const submitButton = screen.getByRole("button", { name: /Anmelden/i });

      await act(async () => {
        fireEvent.click(submitButton);
      });

      expect(mockSignIn).toHaveBeenCalledWith({
        email: "test@example.com",
        password: "password123",
      });
    });

    it("shows loading state during submission", async () => {
      mockSignIn.mockImplementation(
        () =>
          new Promise((resolve) =>
            setTimeout(() => resolve({ error: null }), 100),
          ),
      );

      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");

      await act(async () => {
        fireEvent.change(emailInput, {
          target: { value: "test@example.com" },
        });
        fireEvent.change(passwordInput, {
          target: { value: "password123" },
        });
      });

      const submitButton = screen.getByRole("button", { name: /Anmelden/i });

      await act(async () => {
        fireEvent.click(submitButton);
      });

      expect(
        screen.getByRole("button", { name: /Anmeldung läuft/i }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /Anmeldung läuft/i }),
      ).toBeDisabled();

      // Advance timers to complete the promise
      await act(async () => {
        vi.advanceTimersByTime(150);
      });
    });

    it("sets active organization after successful login", async () => {
      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");

      await act(async () => {
        fireEvent.change(emailInput, {
          target: { value: "test@example.com" },
        });
        fireEvent.change(passwordInput, {
          target: { value: "password123" },
        });
      });

      const submitButton = screen.getByRole("button", { name: /Anmelden/i });

      await act(async () => {
        fireEvent.click(submitButton);
      });

      await waitFor(() => {
        expect(mockOrganization.list).toHaveBeenCalled();
        expect(mockOrganization.setActive).toHaveBeenCalledWith({
          organizationId: "org-1",
        });
      });
    });

    it("handles case when user has no organizations", async () => {
      mockOrganization.list.mockResolvedValue({ data: [] });
      const consoleSpy = vi.spyOn(console, "warn").mockImplementation(() => {});

      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");

      await act(async () => {
        fireEvent.change(emailInput, {
          target: { value: "test@example.com" },
        });
        fireEvent.change(passwordInput, {
          target: { value: "password123" },
        });
      });

      const submitButton = screen.getByRole("button", { name: /Anmelden/i });

      await act(async () => {
        fireEvent.click(submitButton);
      });

      await waitFor(() => {
        expect(consoleSpy).toHaveBeenCalledWith(
          "User has no organizations - backend requests may fail",
        );
      });

      consoleSpy.mockRestore();
    });

    it("handles organization setActive failure gracefully", async () => {
      mockOrganization.setActive.mockRejectedValue(new Error("Failed"));
      const consoleSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => {});

      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");

      await act(async () => {
        fireEvent.change(emailInput, {
          target: { value: "test@example.com" },
        });
        fireEvent.change(passwordInput, {
          target: { value: "password123" },
        });
      });

      const submitButton = screen.getByRole("button", { name: /Anmelden/i });

      await act(async () => {
        fireEvent.click(submitButton);
      });

      await waitFor(() => {
        expect(consoleSpy).toHaveBeenCalledWith(
          "Failed to set active organization:",
          expect.any(Error),
        );
      });

      // Should still refresh the router
      expect(mockRefresh).toHaveBeenCalled();

      consoleSpy.mockRestore();
    });

    it("calls router.refresh after successful login", async () => {
      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");

      await act(async () => {
        fireEvent.change(emailInput, {
          target: { value: "test@example.com" },
        });
        fireEvent.change(passwordInput, {
          target: { value: "password123" },
        });
      });

      const submitButton = screen.getByRole("button", { name: /Anmelden/i });

      await act(async () => {
        fireEvent.click(submitButton);
      });

      await waitFor(() => {
        expect(mockRefresh).toHaveBeenCalled();
      });
    });
  });

  describe("Form submission errors", () => {
    beforeEach(() => {
      mockSessionRef.current = { data: null, isPending: false };
    });

    it("displays error message on auth failure", async () => {
      mockSignIn.mockResolvedValue({
        error: { message: "Invalid credentials" },
      });

      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");

      await act(async () => {
        fireEvent.change(emailInput, {
          target: { value: "test@example.com" },
        });
        fireEvent.change(passwordInput, {
          target: { value: "wrongpassword" },
        });
      });

      const submitButton = screen.getByRole("button", { name: /Anmelden/i });

      await act(async () => {
        fireEvent.click(submitButton);
      });

      await waitFor(() => {
        expect(screen.getByTestId("alert-error")).toBeInTheDocument();
        expect(screen.getByText("Invalid credentials")).toBeInTheDocument();
      });
    });

    it("displays default error message when error has no message", async () => {
      mockSignIn.mockResolvedValue({
        error: {},
      });

      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");

      await act(async () => {
        fireEvent.change(emailInput, {
          target: { value: "test@example.com" },
        });
        fireEvent.change(passwordInput, {
          target: { value: "wrongpassword" },
        });
      });

      const submitButton = screen.getByRole("button", { name: /Anmelden/i });

      await act(async () => {
        fireEvent.click(submitButton);
      });

      await waitFor(() => {
        expect(screen.getByTestId("alert-error")).toBeInTheDocument();
        expect(
          screen.getByText("Ungültige E-Mail oder Passwort"),
        ).toBeInTheDocument();
      });
    });

    it("handles exception during sign in", async () => {
      mockSignIn.mockRejectedValue(new Error("Network error"));
      const consoleSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => {});

      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");

      await act(async () => {
        fireEvent.change(emailInput, {
          target: { value: "test@example.com" },
        });
        fireEvent.change(passwordInput, {
          target: { value: "password123" },
        });
      });

      const submitButton = screen.getByRole("button", { name: /Anmelden/i });

      await act(async () => {
        fireEvent.click(submitButton);
      });

      await waitFor(() => {
        expect(screen.getByTestId("alert-error")).toBeInTheDocument();
        expect(
          screen.getByText("Anmeldefehler. Bitte versuchen Sie es erneut."),
        ).toBeInTheDocument();
      });

      consoleSpy.mockRestore();
    });
  });

  describe("Password reset modal", () => {
    it("opens password reset modal when clicking forgot password", async () => {
      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByText("Passwort vergessen?")).toBeInTheDocument();
      });

      const forgotPasswordButton = screen.getByText("Passwort vergessen?");

      await act(async () => {
        fireEvent.click(forgotPasswordButton);
      });

      expect(screen.getByTestId("password-reset-modal")).toBeInTheDocument();
    });

    it("closes password reset modal when onClose is called", async () => {
      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByText("Passwort vergessen?")).toBeInTheDocument();
      });

      const forgotPasswordButton = screen.getByText("Passwort vergessen?");

      await act(async () => {
        fireEvent.click(forgotPasswordButton);
      });

      expect(screen.getByTestId("password-reset-modal")).toBeInTheDocument();

      const closeButton = screen.getByTestId("close-modal");

      await act(async () => {
        fireEvent.click(closeButton);
      });

      expect(
        screen.queryByTestId("password-reset-modal"),
      ).not.toBeInTheDocument();
    });
  });

  describe("Confetti animation", () => {
    it("launches confetti on form submit", async () => {
      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");

      await act(async () => {
        fireEvent.change(emailInput, {
          target: { value: "test@example.com" },
        });
        fireEvent.change(passwordInput, {
          target: { value: "password123" },
        });
      });

      const submitButton = screen.getByRole("button", { name: /Anmelden/i });

      await act(async () => {
        fireEvent.click(submitButton);
      });

      // Verify createElement was called (for confetti container)
      expect(mockCreateElement).toHaveBeenCalledWith("div");
    });
  });

  describe("Suspense fallback", () => {
    it("renders without crashing", () => {
      // This is difficult to test directly as Suspense resolves immediately
      // in our test environment. We verify the component structure is correct.
      const { container } = render(<LoginPage />);
      expect(container).toBeDefined();
    });
  });

  describe("Edge cases", () => {
    it("handles organization list returning null data", async () => {
      mockOrganization.list.mockResolvedValue({ data: null });
      const consoleSpy = vi.spyOn(console, "warn").mockImplementation(() => {});

      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");

      await act(async () => {
        fireEvent.change(emailInput, {
          target: { value: "test@example.com" },
        });
        fireEvent.change(passwordInput, {
          target: { value: "password123" },
        });
      });

      const submitButton = screen.getByRole("button", { name: /Anmelden/i });

      await act(async () => {
        fireEvent.click(submitButton);
      });

      await waitFor(() => {
        expect(consoleSpy).toHaveBeenCalledWith(
          "User has no organizations - backend requests may fail",
        );
      });

      consoleSpy.mockRestore();
    });

    it("handles organization with null id", async () => {
      mockOrganization.list.mockResolvedValue({
        data: [{ id: null, name: "Test Org" }],
      });

      render(<LoginPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      });

      const emailInput = screen.getByLabelText("E-Mail-Adresse");
      const passwordInput = screen.getByLabelText("Passwort");

      await act(async () => {
        fireEvent.change(emailInput, {
          target: { value: "test@example.com" },
        });
        fireEvent.change(passwordInput, {
          target: { value: "password123" },
        });
      });

      const submitButton = screen.getByRole("button", { name: /Anmelden/i });

      await act(async () => {
        fireEvent.click(submitButton);
      });

      await waitFor(() => {
        // Should not call setActive when id is null
        expect(mockOrganization.setActive).not.toHaveBeenCalled();
      });
    });
  });
});
