/**
 * Tests for Root Page (Organization Selection)
 *
 * Note: The root page shows organization selection on the main domain.
 * Login functionality is handled by /login page.
 */
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

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
  SmartRedirect: () => <div data-testid="smart-redirect" />,
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

describe("RootPage (Organization Selection)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.clearAllMocks();
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
      data: {
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
      },
      isPending: false,
      error: null,
    });

    render(<HomePage />);

    // On main domain (localhost in test env), authenticated users see OrgSelection
    await waitFor(() => {
      expect(screen.getByTestId("org-selection")).toBeInTheDocument();
    });
  });
});
