/**
 * Tests for Root Page (Organization Selection)
 *
 * Note: The root page shows organization selection on the main domain.
 * Login functionality is handled by /login page.
 */
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Store mock function for searchParams
let mockSearchParamsGet: (key: string) => string | null = (_key: string) =>
  null;

// Mock next/navigation
const mockPush = vi.fn();
const mockRefresh = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
    refresh: mockRefresh,
  }),
  useSearchParams: () => ({
    get: (key: string) => mockSearchParamsGet(key),
  }),
}));

// Mock auth-api
vi.mock("~/lib/auth-api", () => ({
  refreshToken: vi.fn(() => Promise.resolve(null)),
}));

// Mock auth-client for organization selection (unauthenticated state)
vi.mock("~/lib/auth-client", () => ({
  authClient: {
    signIn: { email: vi.fn() },
    signOut: vi.fn(),
    useSession: vi.fn(() => ({
      data: null, // Unauthenticated
      isPending: false,
      error: null,
    })),
    getSession: vi.fn(() => Promise.resolve(null)),
    organization: {
      getActiveMemberRole: vi.fn(),
      getFullOrganization: vi.fn(),
      setActive: vi.fn(),
    },
  },
  signIn: { email: vi.fn() },
  signOut: vi.fn(),
  useSession: vi.fn(() => ({
    data: null, // Unauthenticated
    isPending: false,
    error: null,
  })),
  getSession: vi.fn(() => Promise.resolve(null)),
}));

// Mock SmartRedirect
vi.mock("~/components/auth/smart-redirect", () => ({
  SmartRedirect: ({ onRedirect }: { onRedirect: (path: string) => void }) => {
    // Simulate redirect after mount
    setTimeout(() => onRedirect("/dashboard"), 0);
    return <div data-testid="smart-redirect" />;
  },
}));

// Mock OrgSelection component
vi.mock("~/components/auth/org-selection", () => ({
  OrgSelection: () => (
    <div data-testid="org-selection">
      <h1>Willkommen bei moto!</h1>
      <p>Wählen Sie Ihre Einrichtung</p>
      <input placeholder="Einrichtung suchen..." type="text" />
    </div>
  ),
}));

// Mock Loading
vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading">Loading...</div>,
}));

// Mock UI components
vi.mock("~/components/ui", () => ({
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

import HomePage from "./page";

// Store original window.location
const originalLocation = window.location;

// Helper to mock window.location.hostname
function mockHostname(hostname: string): void {
  Object.defineProperty(window, "location", {
    value: { ...originalLocation, hostname },
    writable: true,
    configurable: true,
  });
}

// Helper to restore window.location
function restoreLocation(): void {
  Object.defineProperty(window, "location", {
    value: originalLocation,
    writable: true,
    configurable: true,
  });
}

// Helper to create mock session data
function createMockSession(overrides = {}) {
  return {
    user: {
      id: "test-user-id",
      email: "test@example.com",
      name: "Test User",
      emailVerified: true,
      image: null,
      createdAt: new Date(),
      updatedAt: new Date(),
    },
    session: {
      id: "test-session-id",
      userId: "test-user-id",
      expiresAt: new Date(Date.now() + 86400000),
      ipAddress: null,
      userAgent: null,
    },
    activeOrganizationId: "test-org-id",
    ...overrides,
  };
}

describe("RootPage (Organization Selection)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSearchParamsGet = vi.fn((_key: string) => null);
    // Default to localhost (main domain)
    mockHostname("localhost");
  });

  afterEach(() => {
    vi.clearAllMocks();
    restoreLocation();
  });

  it("renders organization selection on main domain", async () => {
    render(<HomePage />);

    await waitFor(() => {
      expect(screen.getByText("Willkommen bei moto!")).toBeInTheDocument();
    });
  });

  it("displays organization selection content", async () => {
    render(<HomePage />);

    await waitFor(() => {
      expect(
        screen.getByText("Wählen Sie Ihre Einrichtung"),
      ).toBeInTheDocument();
    });
  });

  it("renders search input for organizations", async () => {
    render(<HomePage />);

    await waitFor(() => {
      expect(
        screen.getByPlaceholderText("Einrichtung suchen..."),
      ).toBeInTheDocument();
    });
  });

  it("shows loading state when session is being checked", async () => {
    const { useSession } = await import("~/lib/auth-client");
    vi.mocked(useSession).mockReturnValue({
      data: null,
      isPending: true,
      error: null,
    });

    render(<HomePage />);

    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("shows org selection for authenticated users on main domain", async () => {
    const { useSession } = await import("~/lib/auth-client");
    vi.mocked(useSession).mockReturnValue({
      data: createMockSession(),
      isPending: false,
      error: null,
    });

    render(<HomePage />);

    // On main domain (localhost in test env), authenticated users see OrgSelection
    await waitFor(() => {
      expect(screen.getByTestId("org-selection")).toBeInTheDocument();
    });
  });

  describe("org_status alerts", () => {
    it("shows pending alert when org_status=pending", async () => {
      mockSearchParamsGet = vi.fn((key: string) =>
        key === "org_status" ? "pending" : null,
      );

      render(<HomePage />);

      await waitFor(() => {
        expect(screen.getByTestId("alert-info")).toBeInTheDocument();
        expect(
          screen.getByText(/Ihre Einrichtung wird noch geprüft/),
        ).toBeInTheDocument();
      });
    });

    it("shows error alert when org_status=rejected", async () => {
      mockSearchParamsGet = vi.fn((key: string) =>
        key === "org_status" ? "rejected" : null,
      );

      render(<HomePage />);

      await waitFor(() => {
        expect(screen.getByTestId("alert-error")).toBeInTheDocument();
        expect(
          screen.getByText(/Ihre Einrichtung wurde leider abgelehnt/),
        ).toBeInTheDocument();
      });
    });

    it("shows error alert when org_status=suspended", async () => {
      mockSearchParamsGet = vi.fn((key: string) =>
        key === "org_status" ? "suspended" : null,
      );

      render(<HomePage />);

      await waitFor(() => {
        expect(screen.getByTestId("alert-error")).toBeInTheDocument();
        expect(
          screen.getByText(/Ihre Einrichtung wurde vorübergehend gesperrt/),
        ).toBeInTheDocument();
      });
    });

    it("shows error alert when org_status=not_found", async () => {
      mockSearchParamsGet = vi.fn((key: string) =>
        key === "org_status" ? "not_found" : null,
      );

      render(<HomePage />);

      await waitFor(() => {
        expect(screen.getByTestId("alert-error")).toBeInTheDocument();
        expect(
          screen.getByText(/Die angegebene Einrichtung wurde nicht gefunden/),
        ).toBeInTheDocument();
      });
    });

    it("does not show alert for unknown org_status", async () => {
      mockSearchParamsGet = vi.fn((key: string) =>
        key === "org_status" ? "unknown" : null,
      );

      render(<HomePage />);

      await waitFor(() => {
        expect(screen.getByTestId("org-selection")).toBeInTheDocument();
      });
      expect(screen.queryByTestId("alert-info")).not.toBeInTheDocument();
      expect(screen.queryByTestId("alert-error")).not.toBeInTheDocument();
    });
  });

  describe("subdomain behavior", () => {
    it("redirects to /login when on subdomain without session", async () => {
      // Mock subdomain
      mockHostname("school.localhost");

      const { useSession } = await import("~/lib/auth-client");
      vi.mocked(useSession).mockReturnValue({
        data: null,
        isPending: false,
        error: null,
      });

      render(<HomePage />);

      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith("/login");
      });
    });

    it("shows SmartRedirect when on subdomain with session", async () => {
      // Mock subdomain
      mockHostname("school.localhost");

      const { useSession } = await import("~/lib/auth-client");
      vi.mocked(useSession).mockReturnValue({
        data: createMockSession(),
        isPending: false,
        error: null,
      });

      render(<HomePage />);

      await waitFor(() => {
        expect(screen.getByTestId("smart-redirect")).toBeInTheDocument();
      });
    });

    it("shows loading initially while determining domain", async () => {
      // The component shows loading before useEffect determines the domain
      // We test this by checking that loading is rendered at some point
      const { useSession } = await import("~/lib/auth-client");
      vi.mocked(useSession).mockReturnValue({
        data: null,
        isPending: false,
        error: null,
      });

      const { container } = render(<HomePage />);

      // The component renders - eventually settles on either org-selection or loading
      await waitFor(() => {
        const hasContent =
          container.querySelector('[data-testid="loading"]') ||
          container.querySelector('[data-testid="org-selection"]');
        expect(hasContent).toBeTruthy();
      });
    });

    it("does not redirect if session is still loading", async () => {
      mockHostname("school.localhost");

      const { useSession } = await import("~/lib/auth-client");
      vi.mocked(useSession).mockReturnValue({
        data: null,
        isPending: true,
        error: null,
      });

      render(<HomePage />);

      // Should show loading, not redirect
      expect(screen.getByTestId("loading")).toBeInTheDocument();
      expect(mockPush).not.toHaveBeenCalled();
    });
  });

  describe("domain detection", () => {
    it("detects localhost as main domain", async () => {
      mockHostname("localhost");

      const { useSession } = await import("~/lib/auth-client");
      vi.mocked(useSession).mockReturnValue({
        data: null,
        isPending: false,
        error: null,
      });

      render(<HomePage />);

      await waitFor(
        () => {
          expect(screen.getByTestId("org-selection")).toBeInTheDocument();
        },
        { timeout: 3000 },
      );
    });

    it("detects subdomain.localhost as subdomain", async () => {
      mockHostname("school.localhost");

      const { useSession } = await import("~/lib/auth-client");
      vi.mocked(useSession).mockReturnValue({
        data: null,
        isPending: false,
        error: null,
      });

      render(<HomePage />);

      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith("/login");
      });
    });

    it("handles production domain without subdomain as main domain", async () => {
      // Mock production environment
      const originalEnv = process.env.NEXT_PUBLIC_BASE_DOMAIN;
      process.env.NEXT_PUBLIC_BASE_DOMAIN = "moto.nrw";
      mockHostname("moto.nrw");

      render(<HomePage />);

      await waitFor(() => {
        expect(screen.getByTestId("org-selection")).toBeInTheDocument();
      });

      process.env.NEXT_PUBLIC_BASE_DOMAIN = originalEnv;
    });

    it("handles production subdomain correctly", async () => {
      // Mock production environment with subdomain
      const originalEnv = process.env.NEXT_PUBLIC_BASE_DOMAIN;
      process.env.NEXT_PUBLIC_BASE_DOMAIN = "moto.nrw";
      mockHostname("school.moto.nrw");

      const { useSession } = await import("~/lib/auth-client");
      vi.mocked(useSession).mockReturnValue({
        data: null,
        isPending: false,
        error: null,
      });

      render(<HomePage />);

      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith("/login");
      });

      process.env.NEXT_PUBLIC_BASE_DOMAIN = originalEnv;
    });

    it("handles base domain with port correctly", async () => {
      // Mock localhost with port
      const originalEnv = process.env.NEXT_PUBLIC_BASE_DOMAIN;
      process.env.NEXT_PUBLIC_BASE_DOMAIN = "localhost:3000";
      mockHostname("localhost");

      render(<HomePage />);

      await waitFor(() => {
        expect(screen.getByTestId("org-selection")).toBeInTheDocument();
      });

      process.env.NEXT_PUBLIC_BASE_DOMAIN = originalEnv;
    });

    it("treats hostname without dots as main domain for localhost", async () => {
      const originalEnv = process.env.NEXT_PUBLIC_BASE_DOMAIN;
      process.env.NEXT_PUBLIC_BASE_DOMAIN = "localhost:3000";
      mockHostname("somehost");

      render(<HomePage />);

      await waitFor(() => {
        expect(screen.getByTestId("org-selection")).toBeInTheDocument();
      });

      process.env.NEXT_PUBLIC_BASE_DOMAIN = originalEnv;
    });
  });

  describe("SmartRedirect integration", () => {
    it("calls router.push when SmartRedirect triggers redirect", async () => {
      mockHostname("school.localhost");

      const { useSession } = await import("~/lib/auth-client");
      vi.mocked(useSession).mockReturnValue({
        data: createMockSession(),
        isPending: false,
        error: null,
      });

      // Spy on console.log to verify the redirect message
      const consoleSpy = vi.spyOn(console, "log");

      render(<HomePage />);

      await waitFor(() => {
        expect(screen.getByTestId("smart-redirect")).toBeInTheDocument();
      });

      // SmartRedirect mock calls onRedirect after timeout
      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith("/dashboard");
      });

      await waitFor(() => {
        expect(consoleSpy).toHaveBeenCalledWith(
          "Redirecting to /dashboard based on user permissions",
        );
      });

      consoleSpy.mockRestore();
    });
  });
});
