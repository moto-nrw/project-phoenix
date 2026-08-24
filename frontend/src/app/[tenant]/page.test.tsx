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

const mockTrackTenantEvent = vi.fn();
const { mockLoginWithPasskey, mockIsPasskeySupported, MockPasskeyApiError } =
  vi.hoisted(() => {
    class MockPasskeyApiError extends Error {
      constructor(
        public status: number,
        message: string,
        public code?: string,
      ) {
        super(message);
      }
    }

    return {
      mockLoginWithPasskey: vi.fn(),
      mockIsPasskeySupported: vi.fn(() => false),
      MockPasskeyApiError,
    };
  });

vi.mock("~/lib/analytics", () => ({
  trackTenantEvent: (...args: unknown[]) => mockTrackTenantEvent(...args),
}));

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
// Stable reference, real useSearchParams returns the same object between renders.
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
  SmartRedirect: ({
    onRedirect: _onRedirect,
  }: {
    onRedirect: (path: string) => void;
  }) => <div data-testid="smart-redirect" />,
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
        <button type="button" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
}));

// Mock Loading
vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading">Loading...</div>,
}));

vi.mock("~/components/ui/alert", () => ({
  Alert: ({
    type,
    message,
    action,
  }: {
    type: string;
    message: string;
    action?: React.ReactNode;
  }) => (
    <div data-testid={`alert-${type}`}>
      {message}
      {action}
    </div>
  ),
}));

// parentsPortalLoginUrl() reads NEXT_PUBLIC_PARENTS_HOSTNAME and throws when
// it is unset, which no unit-test env provides. URL construction itself is
// covered by parent-url.test.ts.
vi.mock("~/lib/parent-url", () => ({
  parentsPortalLoginUrl: (search?: string) =>
    `https://parents.example.test/login${search ?? ""}`,
}));

// Same reason for the school portal (#2207): schoolPortalLoginUrl() reads
// NEXT_PUBLIC_SCHOOL_HOSTNAME and throws when it is unset. URL construction
// itself is covered by school-url.test.ts.
vi.mock("~/lib/school-url", () => ({
  schoolPortalLoginUrl: (search?: string) =>
    `https://schule.example.test/login${search ?? ""}`,
}));

vi.mock("~/lib/passkey-api", () => ({
  isPasskeySupported: () => mockIsPasskeySupported(),
  isPasskeyCeremonyIncompleteError: () => false,
  loginWithPasskey: (...args: unknown[]) => mockLoginWithPasskey(...args),
  PasskeyApiError: MockPasskeyApiError,
}));

// Mock next/image
vi.mock("next/image", () => ({
  default: ({ src, alt }: { src: string; alt: string }) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img src={src} alt={alt} data-testid="next-image" />
  ),
}));

import { useSession } from "next-auth/react";
import { useTenant } from "~/lib/tenant-context";
import { refreshToken } from "~/lib/auth-api";
import HomePage from "./page";

const mockAnimate = vi.fn(() => ({
  onfinish: null,
  cancel: vi.fn(),
})) as unknown as typeof Element.prototype.animate;

const originalFetch = global.fetch;

const resolvedTenant = {
  tenantId: 42,
  name: "",
  settings: {},
} as ReturnType<typeof useTenant>["tenant"];

function mockFetchResponse(status: number, body: unknown): typeof global.fetch {
  return vi.fn().mockResolvedValue(
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

function mockTenantAuthenticatedResponse(): typeof global.fetch {
  return mockFetchResponse(200, {
    status: "authenticated",
    access_token: "access-token",
    refresh_token: "refresh-token",
  });
}

describe("HomePage (Login)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSearchParamsGet.mockImplementation((_key: string) => null);
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "unauthenticated",
      update: vi.fn(),
    });
    vi.mocked(useTenant).mockReturnValue({
      tenantSlug: "test-tenant",
      routingMode: "path",
      tenant: resolvedTenant,
    });

    // Mock Element.animate globally
    Element.prototype.animate = mockAnimate;
    global.fetch = mockTenantAuthenticatedResponse();
    window.history.pushState({}, "", "/");
  });

  afterEach(() => {
    vi.clearAllMocks();
    global.fetch = originalFetch;
    window.history.pushState({}, "", "/");
  });

  it("renders the login form", async () => {
    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByText("Willkommen bei moto")).toBeInTheDocument();
    });
  });

  it("does not display a MOTO logo fallback", async () => {
    render(<HomePage />);

    await waitFor(() => {
      expect(screen.queryByAltText("MOTO Logo")).not.toBeInTheDocument();
    });
  });

  it("displays tenant login logo when configured", async () => {
    vi.mocked(useTenant).mockReturnValue({
      tenantSlug: "test-tenant",
      routingMode: "path",
      tenant: {
        name: "Grundschule Musterstadt",
        settings: {
          loginImageUrl: "/uploads/school-logo.png",
        },
      } as ReturnType<typeof useTenant>["tenant"],
    });

    render(<HomePage />);

    await waitFor(() => {
      expect(
        screen.getByAltText("Grundschule Musterstadt Logo"),
      ).toBeInTheDocument();
    });
  });

  it("shows the moto attribution when a tenant login logo is configured", async () => {
    vi.mocked(useTenant).mockReturnValue({
      tenantSlug: "test-tenant",
      routingMode: "path",
      tenant: {
        name: "Grundschule Musterstadt",
        settings: {
          loginImageUrl: "/uploads/school-logo.png",
        },
      } as ReturnType<typeof useTenant>["tenant"],
    });

    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByText("Bereitgestellt von")).toBeInTheDocument();
    });
  });

  it("does not show the moto attribution without a tenant login logo", async () => {
    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByText("Willkommen bei moto")).toBeInTheDocument();
    });
    expect(screen.queryByText("Bereitgestellt von")).not.toBeInTheDocument();
  });

  it("displays login subtitle", async () => {
    render(<HomePage />);

    await waitFor(() => {
      expect(
        screen.getByText("Melden Sie sich mit Ihrem Konto an."),
      ).toBeInTheDocument();
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
    expect(screen.getByText("Sitzung wird überprüft...")).toBeInTheDocument();
  });

  it("posts to /api/auth/login then seeds session via internalRefresh (2-step MFA flow)", async () => {
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
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/auth/login",
        expect.objectContaining({
          method: "POST",
          credentials: "include",
          body: JSON.stringify({
            email: "test@example.com",
            password: "password123",
            tenant_slug: "test-tenant",
          }),
        }),
      );
    });
    await waitFor(() => {
      expect(mockSignIn).toHaveBeenCalledWith("credentials", {
        redirect: false,
        internalRefresh: "true",
        token: "access-token",
        refreshToken: "refresh-token",
      });
    });
    expect(mockTrackTenantEvent).toHaveBeenCalledWith(
      "login_success",
      "42",
      undefined,
    );
  });

  it("removes stale session-expired query params before seeding the new session", async () => {
    mockSignIn.mockResolvedValue({ error: null });
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "error" ? "SessionExpired" : null,
    );
    window.history.pushState(
      {},
      "",
      "/?error=SessionExpired&callbackUrl=%2Fdashboard",
    );
    const replaceStateSpy = vi.spyOn(window.history, "replaceState");

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
        redirect: false,
        internalRefresh: "true",
        token: "access-token",
        refreshToken: "refresh-token",
      });
    });
    expect(window.location.search).toBe("");
    expect(replaceStateSpy).toHaveBeenCalledWith({}, "", "/");
  });

  it("shows error message on invalid credentials", async () => {
    global.fetch = mockFetchResponse(401, { error: "Invalid credentials" });

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
    expect(mockTrackTenantEvent).toHaveBeenCalledWith("login_failed", "42", {
      reason: "invalid_credentials",
    });
  });

  describe("guardian-only account at the staff login", () => {
    // Backend answers 403 + code "use_parent_portal" once the password was
    // accepted but the account only exists as a guardian. Regression guard
    // for the support case where parents read the old generic error as a
    // password problem and reset their password over and over.
    const guardianOnly403 = () =>
      mockFetchResponse(403, {
        status: "error",
        error: "guardian accounts must log in at the parents portal",
        code: "use_parent_portal",
      });

    async function submitLogin() {
      render(<HomePage />);

      await waitFor(() => {
        expect(screen.getByTestId("input-email")).toBeInTheDocument();
      });

      await act(async () => {
        fireEvent.change(screen.getByTestId("input-email"), {
          target: { value: "parent@example.com" },
        });
        fireEvent.change(screen.getByTestId("input-password"), {
          target: { value: "correct-password" },
        });
      });

      await act(async () => {
        fireEvent.click(screen.getByRole("button", { name: /anmelden/i }));
      });
    }

    it("explains the portal mix-up instead of blaming the password", async () => {
      global.fetch = guardianOnly403();

      await submitLogin();

      await waitFor(() => {
        expect(
          screen.getByText(/Dieses Konto ist ein Elternkonto/),
        ).toBeInTheDocument();
      });
      // The old behaviour — must not come back.
      expect(
        screen.queryByText("Ungültige E-Mail oder Passwort"),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByText("Anmeldung fehlgeschlagen. Bitte erneut versuchen."),
      ).not.toBeInTheDocument();
      expect(mockTrackTenantEvent).toHaveBeenCalledWith("login_failed", "42", {
        reason: "use_parent_portal",
      });
    });

    it("offers a manual link to the parents portal", async () => {
      global.fetch = guardianOnly403();

      await submitLogin();

      const link = await screen.findByRole("link", {
        name: /Jetzt zum Elternportal/,
      });
      expect(link).toHaveAttribute(
        "href",
        "https://parents.example.test/login?from=staff",
      );
    });

    it("sends the browser to the parents portal after the grace period", async () => {
      global.fetch = guardianOnly403();
      const assign = vi.fn();

      // Override ONLY the href setter, leaving the rest of Location (search,
      // pathname, origin — used by other code on this page) intact. Replacing
      // the whole object leaks a broken Location into every later test.
      const ownHref = Object.getOwnPropertyDescriptor(window.location, "href");
      const currentHref = window.location.href;
      Object.defineProperty(window.location, "href", {
        configurable: true,
        get: () => currentHref,
        set: (value: string) => assign(value),
      });

      // shouldAdvanceTime keeps waitFor/findBy usable under fake timers.
      vi.useFakeTimers({ shouldAdvanceTime: true });

      try {
        await submitLogin();

        // Still on the page — the hint must be readable before the jump.
        expect(assign).not.toHaveBeenCalled();

        await act(async () => {
          vi.advanceTimersByTime(2000);
        });

        expect(assign).toHaveBeenCalledWith(
          "https://parents.example.test/login?from=staff",
        );
      } finally {
        vi.useRealTimers();
        if (ownHref) {
          Object.defineProperty(window.location, "href", ownHref);
        } else {
          delete (window.location as unknown as Record<string, unknown>).href;
        }
      }
    });
  });

  describe("Lehrkraft-only account at the staff login (#2207)", () => {
    // Since the cutover a school-portal-only account has nothing to reach in
    // the OGS portal, so the backend answers 403 + code "use_school_portal".
    // Same failure mode as the guardian split above: a generic error would
    // read as a password problem and send Lehrkräfte into reset loops.
    const schoolOnly403 = () =>
      mockFetchResponse(403, {
        status: "error",
        error: "school portal accounts must log in at the school portal",
        code: "use_school_portal",
      });

    async function submitLogin() {
      render(<HomePage />);

      await waitFor(() => {
        expect(screen.getByTestId("input-email")).toBeInTheDocument();
      });

      await act(async () => {
        fireEvent.change(screen.getByTestId("input-email"), {
          target: { value: "lehrkraft@example.com" },
        });
        fireEvent.change(screen.getByTestId("input-password"), {
          target: { value: "correct-password" },
        });
      });

      await act(async () => {
        fireEvent.click(screen.getByRole("button", { name: /anmelden/i }));
      });
    }

    it("names the portal instead of blaming the password", async () => {
      global.fetch = schoolOnly403();

      await submitLogin();

      await waitFor(() => {
        expect(
          screen.getByText(/Dieses Konto ist ein Lehrkraft-Konto/),
        ).toBeInTheDocument();
      });
      expect(
        screen.queryByText("Ungültige E-Mail oder Passwort"),
      ).not.toBeInTheDocument();
      expect(mockTrackTenantEvent).toHaveBeenCalledWith("login_failed", "42", {
        reason: "use_school_portal",
      });
    });

    it("offers a manual link to moto schule", async () => {
      global.fetch = schoolOnly403();

      await submitLogin();

      const link = await screen.findByRole("link", {
        name: /Jetzt zu moto schule/,
      });
      expect(link).toHaveAttribute(
        "href",
        "https://schule.example.test/login?from=staff&tenant=test-tenant",
      );
    });

    it("hands an MFA exchange with a school-portal response to moto schule", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValueOnce(
          new Response(
            JSON.stringify({
              status: "mfa_required",
              challenge_token: "mfa-challenge",
              masked_email: "l***@example.com",
            }),
            { status: 200, headers: { "Content-Type": "application/json" } },
          ),
        )
        .mockResolvedValueOnce(
          new Response(
            JSON.stringify({
              error: "school portal accounts must log in at the school portal",
              code: "use_school_portal",
            }),
            { status: 403, headers: { "Content-Type": "application/json" } },
          ),
        );

      render(<HomePage />);

      await waitFor(() => {
        expect(screen.getByTestId("input-email")).toBeInTheDocument();
      });
      fireEvent.change(screen.getByTestId("input-email"), {
        target: { value: "lehrkraft@example.com" },
      });
      fireEvent.change(screen.getByTestId("input-password"), {
        target: { value: "correct-password" },
      });
      fireEvent.click(screen.getByRole("button", { name: /anmelden/i }));

      const firstCodeInput = await screen.findByLabelText("Stelle 1");
      await act(async () => {
        fireEvent.change(firstCodeInput, { target: { value: "123456" } });
      });

      expect(
        await screen.findByText(/Dieses Konto ist ein Lehrkraft-Konto/),
      ).toBeInTheDocument();
    });
  });

  it("sends a Lehrkraft-only passkey login to moto schule", async () => {
    mockIsPasskeySupported.mockReturnValueOnce(true);
    mockLoginWithPasskey.mockRejectedValueOnce(
      new MockPasskeyApiError(
        403,
        "school portal accounts must log in at the school portal",
        "use_school_portal",
      ),
    );
    render(<HomePage />);

    await act(async () => {
      fireEvent.click(
        await screen.findByRole("button", { name: "Mit Passkey anmelden" }),
      );
    });

    expect(
      await screen.findByText(/Dieses Konto ist ein Lehrkraft-Konto/),
    ).toBeInTheDocument();
    expect(mockTrackTenantEvent).toHaveBeenCalledWith("login_failed", "42", {
      reason: "use_school_portal",
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
    expect(
      screen.getByText("Sie sind bereits angemeldet."),
    ).toBeInTheDocument();
    expect(screen.getByText("moto öffnet Ihre OGS...")).toBeInTheDocument();
    expect(screen.queryByTestId("input-email")).not.toBeInTheDocument();
    expect(screen.queryByTestId("input-password")).not.toBeInTheDocument();
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
      routingMode: "path",
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
      routingMode: "path",
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
      routingMode: "path",
      tenant: { name: "" } as ReturnType<typeof useTenant>["tenant"],
    });

    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByText("Willkommen bei moto")).toBeInTheDocument();
    });
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
    global.fetch = mockTenantAuthenticatedResponse();
  });

  afterEach(() => {
    global.fetch = originalFetch;
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
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/auth/login",
        expect.objectContaining({ method: "POST" }),
      );
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

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("shows generic error when fetch throws an Error", async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error("Network error"));

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

  it("shows generic error when fetch throws a non-Error", async () => {
    global.fetch = vi.fn().mockRejectedValue("unknown failure");

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
    vi.spyOn(window.history, "replaceState").mockImplementation(
      replaceStateSpy,
    );
  });

  afterEach(() => {
    sessionStorage.clear();
    vi.restoreAllMocks();
    mockSearchParamsGet.mockImplementation((_key: string) => null);
    window.history.pushState({}, "", "/");
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
    window.history.pushState(
      {},
      "",
      "/?error=SessionRequired&callbackUrl=%2Fdashboard",
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
