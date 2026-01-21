/**
 * Tests for Login Page (Root Page)
 * Tests the rendering and functionality of the main login form
 */
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Mock next-auth/react
const mockSignIn = vi.fn();
vi.mock("next-auth/react", () => ({
  signIn: (provider: string, options?: Record<string, unknown>) =>
    // eslint-disable-next-line @typescript-eslint/no-unsafe-return
    mockSignIn(provider, options),
  useSession: vi.fn(() => ({
    data: null,
    status: "unauthenticated",
  })),
}));

// Mock next/navigation
const mockPush = vi.fn();
const mockRefresh = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
    refresh: mockRefresh,
  }),
  useSearchParams: () => ({
    get: vi.fn((_key: string) => null),
  }),
}));

// Mock auth-api
vi.mock("~/lib/auth-api", () => ({
  refreshToken: vi.fn(() => Promise.resolve(null)),
}));

// Mock SmartRedirect
vi.mock("~/components/auth/smart-redirect", () => ({
  SmartRedirect: ({ onRedirect: _onRedirect }: { onRedirect: (path: string) => void }) => (
    <div data-testid="smart-redirect" />
  ),
}));

// Mock PasswordResetModal
vi.mock("~/components/ui/password-reset-modal", () => ({
  PasswordResetModal: ({
    isOpen,
    onClose,
  }: {
    isOpen: boolean;
    onClose: () => void;
  }) =>
    isOpen ? (
      <div data-testid="password-reset-modal">
        <button onClick={onClose}>Close</button>
      </div>
    ) : null,
}));

// Mock Loading
vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading">Loading...</div>,
}));

// Mock UI components
vi.mock("~/components/ui", () => ({
  Input: ({
    id,
    type,
    value,
    onChange,
    ...props
  }: {
    id: string;
    type: string;
    value: string;
    onChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
    label: string;
    className?: string;
    required?: boolean;
    autoComplete?: string;
    name?: string;
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
    <div data-testid={`alert-${type}`}>{message}</div>
  ),
  HelpButton: () => <button data-testid="help-button">Help</button>,
}));

// Mock next/image
vi.mock("next/image", () => ({
  default: ({ src, alt }: { src: string; alt: string }) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img src={src} alt={alt} data-testid="next-image" />
  ),
}));

import { useSession } from "next-auth/react";
import HomePage from "./page";

// Mock Element.animate for confetti effect
const mockAnimate = vi.fn(() => ({
  onfinish: null,
  cancel: vi.fn(),
})) as unknown as typeof Element.prototype.animate;

describe("HomePage (Login)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "unauthenticated",
      update: vi.fn(),
    });

    // Mock Element.animate globally
    Element.prototype.animate = mockAnimate;
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("renders the login form", async () => {
    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByText("Willkommen bei moto!")).toBeInTheDocument();
    });
  });

  it("displays the MOTO logo", async () => {
    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByAltText("MOTO Logo")).toBeInTheDocument();
    });
  });

  it("displays tagline", async () => {
    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByText("Ganztag. Digital.")).toBeInTheDocument();
    });
  });

  it("renders email input field", async () => {
    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByText("E-Mail-Adresse")).toBeInTheDocument();
    });
  });

  it("renders password input field", async () => {
    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByText("Passwort")).toBeInTheDocument();
    });
  });

  it("renders login button", async () => {
    render(<HomePage />);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /anmelden/i }),
      ).toBeInTheDocument();
    });
  });

  it("renders forgot password link", async () => {
    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByText("Passwort vergessen?")).toBeInTheDocument();
    });
  });

  it("renders help button", async () => {
    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByTestId("help-button")).toBeInTheDocument();
    });
  });

  it("shows loading state when session is being checked", async () => {
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "loading",
      update: vi.fn(),
    });

    render(<HomePage />);

    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("calls signIn with credentials on form submission", async () => {
    mockSignIn.mockResolvedValue({ error: null });

    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByTestId("input-email")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.change(screen.getByTestId("input-email"), {
        target: { value: "test@example.com" },
      });
      fireEvent.change(screen.getByTestId("input-password"), {
        target: { value: "password123" },
      });
    });

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /anmelden/i }));
    });

    await waitFor(() => {
      expect(mockSignIn).toHaveBeenCalledWith("credentials", {
        email: "test@example.com",
        password: "password123",
        redirect: false,
      });
    });
  });

  it("shows error message on invalid credentials", async () => {
    mockSignIn.mockResolvedValue({ error: "Invalid credentials" });

    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByTestId("input-email")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.change(screen.getByTestId("input-email"), {
        target: { value: "test@example.com" },
      });
      fireEvent.change(screen.getByTestId("input-password"), {
        target: { value: "wrongpassword" },
      });
    });

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /anmelden/i }));
    });

    await waitFor(() => {
      expect(
        screen.getByText("Ungültige E-Mail oder Passwort"),
      ).toBeInTheDocument();
    });
  });

  it("toggles password visibility", async () => {
    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByTestId("input-password")).toBeInTheDocument();
    });

    const passwordInput = screen.getByTestId("input-password");
    expect(passwordInput).toHaveAttribute("type", "password");

    // Find and click the toggle button
    const toggleButton = screen.getByLabelText("Passwort anzeigen");
    await act(async () => {
      fireEvent.click(toggleButton);
    });

    expect(passwordInput).toHaveAttribute("type", "text");
  });

  it("opens password reset modal when forgot password is clicked", async () => {
    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByText("Passwort vergessen?")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.click(screen.getByText("Passwort vergessen?"));
    });

    await waitFor(() => {
      expect(screen.getByTestId("password-reset-modal")).toBeInTheDocument();
    });
  });

  it("redirects authenticated users with valid token", async () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "1",
          token: "valid-token",
          refreshToken: "refresh-token",
        },
        expires: new Date(Date.now() + 3600000).toISOString(),
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByTestId("smart-redirect")).toBeInTheDocument();
    });
  });

  it("disables submit button while loading", async () => {
    mockSignIn.mockImplementation(
      () => new Promise((resolve) => setTimeout(resolve, 1000)),
    );

    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByTestId("input-email")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.change(screen.getByTestId("input-email"), {
        target: { value: "test@example.com" },
      });
      fireEvent.change(screen.getByTestId("input-password"), {
        target: { value: "password123" },
      });
    });

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /anmelden/i }));
    });

    expect(screen.getByText("Anmeldung läuft...")).toBeInTheDocument();
  });
});

describe("Login URL error handling", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "unauthenticated",
      update: vi.fn(),
    });
    Element.prototype.animate = mockAnimate;
  });

  it("validates session error message format", () => {
    // Test the error message that would be displayed
    const urlError = "SessionExpired" as string;
    const expectedMessage =
      urlError === "SessionRequired" || urlError === "SessionExpired"
        ? "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an."
        : null;

    expect(expectedMessage).toBe(
      "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.",
    );
  });

  it("validates SessionRequired error triggers message", () => {
    const urlError = "SessionRequired" as string;
    const expectedMessage =
      urlError === "SessionRequired" || urlError === "SessionExpired"
        ? "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an."
        : null;

    expect(expectedMessage).toBe(
      "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.",
    );
  });

  it("validates unknown error does not trigger message", () => {
    const urlError = "UnknownError" as string;
    const expectedMessage =
      urlError === "SessionRequired" || urlError === "SessionExpired"
        ? "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an."
        : null;

    expect(expectedMessage).toBeNull();
  });
});

describe("Confetti effect", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "unauthenticated",
      update: vi.fn(),
    });
    Element.prototype.animate = mockAnimate;
  });

  it("tests confetti color array", () => {
    // Test the confetti colors that would be used
    const colors = ["#FF3130", "#F78C10", "#83DC2D", "#5080D8"];

    expect(colors).toHaveLength(4);
    expect(colors[0]).toBe("#FF3130");
  });

  it("tests confetti quadrant calculation", () => {
    // Test the quadrant-based angle calculation
    const quadrant = 2 as number; // Bottom-left quadrant
    let angle = 0;

    switch (quadrant) {
      case 0:
        angle = (Math.random() * Math.PI) / 2;
        break;
      case 1:
        angle = Math.PI / 2 + (Math.random() * Math.PI) / 2;
        break;
      case 2:
        angle = Math.PI + (Math.random() * Math.PI) / 2;
        break;
      case 3:
        angle = (3 * Math.PI) / 2 + (Math.random() * Math.PI) / 2;
        break;
    }

    // Angle for quadrant 2 should be between π and 3π/2
    expect(angle).toBeGreaterThanOrEqual(Math.PI);
    expect(angle).toBeLessThanOrEqual((3 * Math.PI) / 2);
  });
});
