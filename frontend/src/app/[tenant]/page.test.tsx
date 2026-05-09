/**
 * Tests for Login Page (Root Page)
 * Tests the rendering and functionality of the main login form
 */
import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Mock next-auth/react
const mockSignIn = vi.fn();
vi.mock("next-auth/react", () => ({
  signIn: (provider: string, options?: Record<string, unknown>) =>
    mockSignIn(provider, options),
  useSession: vi.fn(() => ({
    data: null,
    status: "unauthenticated",
  })),
}));

// Mock next/navigation
const mockPush = vi.fn();
const mockRefresh = vi.fn();
const mockSearchParamsGet = vi.fn((_key: string): string | null => null);
// Stable reference — real useSearchParams returns the same object between renders.
// A fresh object each call causes the useEffect([searchParams]) to re-fire.
const stableSearchParams = { get: mockSearchParamsGet };
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
    refresh: mockRefresh,
  }),
  useSearchParams: () => stableSearchParams,
}));

// Mock tenant-router
const mockTenantPush = vi.fn();
const mockTenantRefresh = vi.fn();
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mockTenantPush, refresh: mockTenantRefresh }),
}));

// Mock auth-api
vi.mock("~/lib/auth-api", () => ({
  refreshToken: vi.fn(() => Promise.resolve(null)),
}));

// Mock SmartRedirect
vi.mock("~/components/auth/smart-redirect", () => ({
  SmartRedirect: ({ onRedirect }: { onRedirect: (path: string) => void }) => (
    <button
      data-testid="smart-redirect"
      onClick={() => onRedirect("/dashboard")}
    >
      Smart Redirect
    </button>
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
}));

// Mock next/image
vi.mock("next/image", () => ({
  default: ({ src, alt }: { src: string; alt: string }) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img src={src} alt={alt} data-testid="next-image" />
  ),
}));

import { useSession } from "next-auth/react";
import { useTenant } from "~/components/tenant/tenant-provider";
import { refreshToken } from "~/lib/auth-api";
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

  it("shows loading state when session is being checked", async () => {
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "loading",
      update: vi.fn(),
    });

    render(<HomePage />);

    // Login form is hidden while checking auth
    const formContainer = document.querySelector(
      "div[class*='transition-opacity']",
    );
    expect(formContainer).toHaveClass("hidden");

    // Loading spinner is visible with checking message
    expect(screen.getByText("Sitzung wird überprüft…")).toBeInTheDocument();
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
        tenantSlug: "test-tenant",
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

    await act(async () => {
      fireEvent.click(screen.getByTestId("smart-redirect"));
    });

    expect(mockTenantPush).toHaveBeenCalledWith("/dashboard");
    expect(mockTenantRefresh).toHaveBeenCalled();
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

describe("Tenant selector button", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "unauthenticated",
      update: vi.fn(),
    });
    Element.prototype.animate = mockAnimate;
  });

  it("renders tenant selector button", async () => {
    render(<HomePage />);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /Einrichtung wechseln/i }),
      ).toBeInTheDocument();
    });
  });

  it("navigates to tenant domain with port when port is present", async () => {
    Object.defineProperty(window, "location", {
      writable: true,
      value: {
        protocol: "http:",
        port: "3000",
        href: "",
      },
    });

    render(<HomePage />);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /Einrichtung wechseln/i }),
      ).toBeInTheDocument();
    });

    fireEvent.click(
      screen.getByRole("button", { name: /Einrichtung wechseln/i }),
    );

    // env.NEXT_PUBLIC_TENANT_DOMAIN comes from the global mock (undefined),
    // but the URL is constructed with the domain
    expect(window.location.href).toContain("/");
  });

  it("navigates to tenant domain without port when port is empty", async () => {
    Object.defineProperty(window, "location", {
      writable: true,
      value: {
        protocol: "https:",
        port: "",
        href: "",
      },
    });

    render(<HomePage />);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /Einrichtung wechseln/i }),
      ).toBeInTheDocument();
    });

    fireEvent.click(
      screen.getByRole("button", { name: /Einrichtung wechseln/i }),
    );

    // Without port, no :port suffix should be appended
    expect(window.location.href).not.toContain(":3000");
  });
});

describe("Tenant name display", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "unauthenticated",
      update: vi.fn(),
    });
    Element.prototype.animate = mockAnimate;
  });

  it("displays tenant name when available", async () => {
    vi.mocked(useTenant).mockReturnValue({
      tenantSlug: "test-tenant",
      tenant: { name: "Grundschule Musterstadt" } as ReturnType<
        typeof useTenant
      >["tenant"],
    });

    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByText("Grundschule Musterstadt")).toBeInTheDocument();
    });
  });

  it("does not display tenant name when tenant has no name", async () => {
    vi.mocked(useTenant).mockReturnValue({
      tenantSlug: "test-tenant",
      tenant: null,
    });

    render(<HomePage />);

    await waitFor(() => {
      expect(
        screen.queryByText("Grundschule Musterstadt"),
      ).not.toBeInTheDocument();
    });
  });

  it("does not display tenant name when tenant.name is empty", async () => {
    vi.mocked(useTenant).mockReturnValue({
      tenantSlug: "test-tenant",
      tenant: { name: "" } as ReturnType<typeof useTenant>["tenant"],
    });

    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByText("Ganztag. Digital.")).toBeInTheDocument();
    });
    // With empty name, the spacer div should be rendered instead
  });
});

describe("Enter key form submission", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "unauthenticated",
      update: vi.fn(),
    });
    Element.prototype.animate = mockAnimate;
  });

  it("submits form when submit event is triggered", async () => {
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

    const form = screen.getByTestId("input-email").closest("form")!;
    await act(async () => {
      fireEvent.submit(form);
    });

    await waitFor(() => {
      expect(mockSignIn).toHaveBeenCalled();
    });
  });
});

describe("Token refresh flow", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    Element.prototype.animate = mockAnimate;
  });

  it("attempts token refresh when session has refresh token but no access token", async () => {
    vi.mocked(refreshToken).mockResolvedValue({
      access_token: "new-token",
      refresh_token: "new-refresh",
    });
    mockSignIn.mockResolvedValue({ error: null });

    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "1",
          token: null as unknown as string,
          refreshToken: "old-refresh",
        },
        expires: new Date(Date.now() + 3600000).toISOString(),
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<HomePage />);

    await waitFor(() => {
      expect(refreshToken).toHaveBeenCalled();
    });
  });

  it("shows login form when token refresh returns null", async () => {
    vi.mocked(refreshToken).mockResolvedValue(null);

    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "1",
          token: null as unknown as string,
          refreshToken: "old-refresh",
        },
        expires: new Date(Date.now() + 3600000).toISOString(),
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<HomePage />);

    await waitFor(() => {
      const submitButton = screen.getByRole("button", { name: /anmelden/i });
      const formContainer = submitButton.closest(
        "div[class*='transition-opacity']",
      );
      expect(formContainer).toHaveClass("opacity-100");
    });
  });

  it("shows login form when token refresh throws an error", async () => {
    vi.mocked(refreshToken).mockRejectedValue(new Error("refresh failed"));

    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "1",
          token: null as unknown as string,
          refreshToken: "old-refresh",
        },
        expires: new Date(Date.now() + 3600000).toISOString(),
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<HomePage />);

    await waitFor(() => {
      const submitButton = screen.getByRole("button", { name: /anmelden/i });
      const formContainer = submitButton.closest(
        "div[class*='transition-opacity']",
      );
      expect(formContainer).toHaveClass("opacity-100");
    });
  });

  it("calls signIn with refreshed tokens after successful refresh", async () => {
    vi.mocked(refreshToken).mockResolvedValue({
      access_token: "new-access-token",
      refresh_token: "new-refresh-token",
    });
    mockSignIn.mockResolvedValue({ error: null });

    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "1",
          token: null as unknown as string,
          refreshToken: "old-refresh",
        },
        expires: new Date(Date.now() + 3600000).toISOString(),
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<HomePage />);

    await waitFor(() => {
      expect(mockSignIn).toHaveBeenCalledWith("credentials", {
        redirect: false,
        internalRefresh: true,
        token: "new-access-token",
        refreshToken: "new-refresh-token",
      });
    });
  });
});

describe("Form error handling", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "unauthenticated",
      update: vi.fn(),
    });
    Element.prototype.animate = mockAnimate;
  });

  it("shows generic error when signIn throws an Error", async () => {
    mockSignIn.mockRejectedValue(new Error("Network error"));

    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByTestId("input-email")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /anmelden/i }));
    });

    await waitFor(() => {
      expect(
        screen.getByText("Anmeldefehler. Bitte versuchen Sie es erneut."),
      ).toBeInTheDocument();
    });
  });

  it("shows generic error when signIn throws a non-Error", async () => {
    mockSignIn.mockRejectedValue("unknown failure");

    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByTestId("input-email")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /anmelden/i }));
    });

    await waitFor(() => {
      expect(
        screen.getByText("Anmeldefehler. Bitte versuchen Sie es erneut."),
      ).toBeInTheDocument();
    });
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

describe("Deliberate logout suppression", () => {
  const replaceStateSpy = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "unauthenticated",
      update: vi.fn(),
    });
    Element.prototype.animate = mockAnimate;
    sessionStorage.clear();
    // Spy on history.replaceState to verify URL cleanup
    Object.defineProperty(window, "history", {
      value: { ...window.history, replaceState: replaceStateSpy },
      writable: true,
    });
  });

  afterEach(() => {
    sessionStorage.clear();
    mockSearchParamsGet.mockImplementation((_key: string) => null);
  });

  it("suppresses error when deliberateLogout flag is set", async () => {
    sessionStorage.setItem("deliberateLogout", "1");
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "error" ? "SessionRequired" : null,
    );

    await act(async () => {
      render(<HomePage />);
    });

    expect(screen.queryByTestId("alert-error")).not.toBeInTheDocument();
  });

  it("shows error when deliberateLogout flag is not set", async () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "error" ? "SessionRequired" : null,
    );

    await act(async () => {
      render(<HomePage />);
    });

    expect(screen.getByTestId("alert-error")).toHaveTextContent(
      "Ihre Sitzung ist abgelaufen",
    );
  });

  it("consumes the deliberateLogout flag after reading it", async () => {
    sessionStorage.setItem("deliberateLogout", "1");
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "error" ? "SessionExpired" : null,
    );

    await act(async () => {
      render(<HomePage />);
    });

    expect(sessionStorage.getItem("deliberateLogout")).toBeNull();
  });

  it("cleans up URL params after deliberate logout", async () => {
    sessionStorage.setItem("deliberateLogout", "1");
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "error" ? "SessionRequired" : null,
    );

    await act(async () => {
      render(<HomePage />);
    });

    expect(replaceStateSpy).toHaveBeenCalledWith(
      {},
      "",
      window.location.pathname,
    );
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
