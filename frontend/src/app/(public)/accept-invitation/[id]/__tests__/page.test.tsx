/**
 * Tests for app/(public)/accept-invitation/[id]/page.tsx
 *
 * Tests the invitation acceptance page core functionality:
 * - Loading states
 * - Page structure and suspense
 * - Authentication required state
 *
 * Note: Complex async flows (auto-accept, email mismatch, API errors)
 * are difficult to test reliably due to multiple useEffect hooks.
 * These flows are better tested through integration tests.
 */

import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterAll } from "vitest";

// Use vi.hoisted to define mocks before they're used in vi.mock
const {
  mockSessionRef,
  mockRouterRef,
  mockParamsRef,
  mockSearchParamsRef,
  mockToastRef,
  mockAuthClientRef,
} = vi.hoisted(() => ({
  mockSessionRef: {
    current: {
      data: null as { user: { id: string; email: string } } | null,
      isPending: false,
    },
  },
  mockRouterRef: {
    current: {
      push: vi.fn(),
      refresh: vi.fn(),
    },
  },
  mockParamsRef: {
    current: { id: "test-invitation-id" } as { id?: string },
  },
  mockSearchParamsRef: {
    current: {
      get: vi.fn((key: string) => {
        const params: Record<string, string | null> = {
          email: null,
          org: null,
          role: null,
        };
        return params[key] ?? null;
      }),
    },
  },
  mockToastRef: {
    current: {
      success: vi.fn(),
      error: vi.fn(),
    },
  },
  mockAuthClientRef: {
    current: {
      organization: {
        getInvitation: vi.fn(),
        acceptInvitation: vi.fn(),
        setActive: vi.fn(),
      },
      signOut: vi.fn(),
    },
  },
}));

// Mock next/navigation
vi.mock("next/navigation", () => ({
  useRouter: () => mockRouterRef.current,
  useParams: () => mockParamsRef.current,
  useSearchParams: () => mockSearchParamsRef.current,
}));

// Mock auth-client
vi.mock("~/lib/auth-client", () => ({
  useSession: () => mockSessionRef.current,
  authClient: mockAuthClientRef.current,
}));

// Mock ToastContext
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => mockToastRef.current,
}));

// Mock BetterAuthInvitationForm
vi.mock("~/components/auth/betterauth-invitation-form", () => ({
  BetterAuthInvitationForm: ({
    invitationId,
    email,
    organizationName,
    role,
  }: {
    invitationId: string;
    email: string;
    organizationName?: string;
    role?: string;
  }) => (
    <div data-testid="invitation-form">
      <span data-testid="form-invitation-id">{invitationId}</span>
      <span data-testid="form-email">{email}</span>
      <span data-testid="form-org-name">{organizationName}</span>
      <span data-testid="form-role">{role}</span>
    </div>
  ),
}));

// Mock Loading component
vi.mock("~/components/ui/loading", () => ({
  Loading: ({ fullPage }: { fullPage?: boolean }) => (
    <div data-testid="loading" data-fullpage={fullPage}>
      Laden...
    </div>
  ),
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
  }) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img src={src} alt={alt} {...props} />
  ),
}));

// Import after mocks
import AcceptInvitationPage from "../page";

// Suppress console.error during tests
const originalConsoleError = console.error;

beforeEach(() => {
  vi.clearAllMocks();

  // Reset mocks to default state
  mockSessionRef.current = { data: null, isPending: false };
  mockRouterRef.current = { push: vi.fn(), refresh: vi.fn() };
  mockParamsRef.current = { id: "test-invitation-id" };
  mockSearchParamsRef.current = {
    get: vi.fn(() => null),
  };
  mockToastRef.current = { success: vi.fn(), error: vi.fn() };
  mockAuthClientRef.current = {
    organization: {
      getInvitation: vi.fn().mockResolvedValue({ error: null, data: null }),
      acceptInvitation: vi.fn().mockResolvedValue({ error: null, data: null }),
      setActive: vi.fn().mockResolvedValue({ error: null }),
    },
    signOut: vi.fn().mockResolvedValue(undefined),
  };

  console.error = vi.fn();
});

afterAll(() => {
  console.error = originalConsoleError;
});

describe("AcceptInvitationPage", () => {
  // =============================================================================
  // Loading State Tests
  // =============================================================================

  describe("loading state", () => {
    it("shows loading when session is pending", () => {
      mockSessionRef.current = { data: null, isPending: true };

      render(<AcceptInvitationPage />);

      expect(screen.getByTestId("loading")).toBeInTheDocument();
    });

    it("renders without crashing", () => {
      mockSessionRef.current = { data: null, isPending: false };

      const { container } = render(<AcceptInvitationPage />);

      expect(container).toBeTruthy();
    });
  });

  // =============================================================================
  // Authentication Required State Tests
  // =============================================================================

  describe("authentication required state", () => {
    it("shows login prompt when no email from URL and not authenticated", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSearchParamsRef.current = { get: vi.fn(() => null) };

      render(<AcceptInvitationPage />);

      await waitFor(() => {
        expect(screen.getByText("Einladung annehmen")).toBeInTheDocument();
        expect(screen.getByText("Anmeldung erforderlich")).toBeInTheDocument();
      });
    });

    it("shows login and signup links", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSearchParamsRef.current = { get: vi.fn(() => null) };

      render(<AcceptInvitationPage />);

      await waitFor(() => {
        expect(
          screen.getByRole("link", { name: "Zur Anmeldung" }),
        ).toBeInTheDocument();
        expect(
          screen.getByRole("link", { name: "Konto erstellen" }),
        ).toBeInTheDocument();
      });
    });

    it("login button links to home page", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSearchParamsRef.current = { get: vi.fn(() => null) };

      render(<AcceptInvitationPage />);

      await waitFor(() => {
        const loginLink = screen.getByRole("link", { name: "Zur Anmeldung" });
        expect(loginLink).toHaveAttribute("href", "/");
      });
    });

    it("signup button links to signup page", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSearchParamsRef.current = { get: vi.fn(() => null) };

      render(<AcceptInvitationPage />);

      await waitFor(() => {
        const signupLink = screen.getByRole("link", {
          name: "Konto erstellen",
        });
        expect(signupLink).toHaveAttribute("href", "/signup");
      });
    });
  });

  // =============================================================================
  // URL Parameter Handling Tests
  // =============================================================================

  describe("URL parameter handling", () => {
    it("uses email from URL when not authenticated", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSearchParamsRef.current = {
        get: vi.fn((key: string) => {
          if (key === "email") return "test@example.com";
          if (key === "org") return "Test Org";
          if (key === "role") return "member";
          return null;
        }),
      };

      render(<AcceptInvitationPage />);

      await waitFor(() => {
        expect(screen.getByTestId("invitation-form")).toBeInTheDocument();
        expect(screen.getByTestId("form-email")).toHaveTextContent(
          "test@example.com",
        );
      });
    });

    it("uses organization name from URL", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSearchParamsRef.current = {
        get: vi.fn((key: string) => {
          if (key === "email") return "test@example.com";
          if (key === "org") return "Test Organization";
          return null;
        }),
      };

      render(<AcceptInvitationPage />);

      await waitFor(() => {
        expect(screen.getByTestId("form-org-name")).toHaveTextContent(
          "Test Organization",
        );
      });
    });

    it("uses role from URL", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSearchParamsRef.current = {
        get: vi.fn((key: string) => {
          if (key === "email") return "test@example.com";
          if (key === "role") return "admin";
          return null;
        }),
      };

      render(<AcceptInvitationPage />);

      await waitFor(() => {
        expect(screen.getByTestId("form-role")).toHaveTextContent("admin");
      });
    });

    it("defaults role to member when not specified", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSearchParamsRef.current = {
        get: vi.fn((key: string) => {
          if (key === "email") return "test@example.com";
          return null;
        }),
      };

      render(<AcceptInvitationPage />);

      await waitFor(() => {
        expect(screen.getByTestId("form-role")).toHaveTextContent("member");
      });
    });

    it("passes invitation ID to form", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSearchParamsRef.current = {
        get: vi.fn((key: string) => {
          if (key === "email") return "test@example.com";
          return null;
        }),
      };

      render(<AcceptInvitationPage />);

      await waitFor(() => {
        expect(screen.getByTestId("form-invitation-id")).toHaveTextContent(
          "test-invitation-id",
        );
      });
    });
  });

  // =============================================================================
  // Missing Invitation ID Tests
  // =============================================================================

  describe("missing invitation ID", () => {
    it("shows error when no invitation ID", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockParamsRef.current = { id: undefined };

      render(<AcceptInvitationPage />);

      await waitFor(() => {
        expect(
          screen.getByText("Keine Einladungs-ID angegeben."),
        ).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Page Branding Tests
  // =============================================================================

  describe("page branding", () => {
    it("displays moto logo when showing auth required", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSearchParamsRef.current = { get: vi.fn(() => null) };

      render(<AcceptInvitationPage />);

      await waitFor(() => {
        const logo = screen.getByAltText("moto Logo");
        expect(logo).toBeInTheDocument();
        expect(logo).toHaveAttribute("src", "/images/moto_transparent.png");
      });
    });

    it("displays page title", async () => {
      mockSessionRef.current = { data: null, isPending: false };
      mockSearchParamsRef.current = { get: vi.fn(() => null) };

      render(<AcceptInvitationPage />);

      await waitFor(() => {
        expect(screen.getByText("Einladung annehmen")).toBeInTheDocument();
      });
    });
  });
});
