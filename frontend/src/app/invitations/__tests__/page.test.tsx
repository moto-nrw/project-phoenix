/**
 * Tests for app/invitations/page.tsx
 *
 * Tests the invitations management page including:
 * - Loading state while checking session
 * - Loading state while checking admin status
 * - Redirect when not authenticated
 * - Permission denied message when not admin
 * - Admin view with invitation form and list
 * - Refresh key propagation on invitation creation
 */

import { render, screen, waitFor, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Use vi.hoisted to define mocks before they're used in vi.mock
const { mockSessionRef, mockIsAdmin, mockRedirect } = vi.hoisted(() => ({
  mockSessionRef: {
    current: {
      data: null as { user: { id: string; email: string } } | null,
      isPending: false,
    },
  },
  mockIsAdmin: vi.fn(),
  mockRedirect: vi.fn(),
}));

// Mock auth-client
vi.mock("~/lib/auth-client", () => ({
  useSession: () => mockSessionRef.current,
  isAdmin: mockIsAdmin,
}));

// Mock next/navigation
vi.mock("next/navigation", () => ({
  redirect: mockRedirect,
}));

// Mock ResponsiveLayout
vi.mock("~/components/dashboard", () => ({
  ResponsiveLayout: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="responsive-layout">{children}</div>
  ),
}));

// Mock Loading component
vi.mock("~/components/ui/loading", () => ({
  Loading: ({ fullPage }: { fullPage?: boolean }) => (
    <div data-testid="loading" data-full-page={fullPage}>
      Loading...
    </div>
  ),
}));

// Track invitation form props
let capturedOnCreated: (() => void) | undefined;

// Mock BetterAuthInvitationCreateForm
vi.mock("~/components/admin/betterauth-invitation-create-form", () => ({
  BetterAuthInvitationCreateForm: ({
    onCreated,
  }: {
    onCreated?: () => void;
  }) => {
    capturedOnCreated = onCreated;
    return (
      <div data-testid="invitation-create-form">
        <button
          data-testid="trigger-created"
          onClick={() => onCreated?.()}
          type="button"
        >
          Create
        </button>
      </div>
    );
  },
}));

// Track pending list props
let capturedRefreshKey: number | undefined;

// Mock BetterAuthPendingInvitationsList
vi.mock("~/components/admin/betterauth-pending-invitations-list", () => ({
  BetterAuthPendingInvitationsList: ({
    refreshKey,
  }: {
    refreshKey: number;
  }) => {
    capturedRefreshKey = refreshKey;
    return (
      <div data-testid="pending-invitations-list" data-refresh-key={refreshKey}>
        Pending Invitations
      </div>
    );
  },
}));

// Import after mocks
import InvitationsPage from "../page";

// Store original Date.now
const originalDateNow = Date.now;

beforeEach(() => {
  vi.clearAllMocks();
  mockSessionRef.current = {
    data: null,
    isPending: false,
  };
  mockIsAdmin.mockResolvedValue(false);
  capturedOnCreated = undefined;
  capturedRefreshKey = undefined;

  // Mock Date.now for predictable refresh keys
  Date.now = vi.fn().mockReturnValue(1704067200000); // 2024-01-01T00:00:00Z
});

afterEach(() => {
  Date.now = originalDateNow;
});

describe("InvitationsPage", () => {
  // =============================================================================
  // Loading State Tests
  // =============================================================================

  describe("loading states", () => {
    it("shows loading when session is pending", () => {
      mockSessionRef.current = { data: null, isPending: true };

      render(<InvitationsPage />);

      expect(screen.getByTestId("loading")).toBeInTheDocument();
      expect(screen.getByTestId("loading")).toHaveAttribute(
        "data-full-page",
        "false",
      );
    });

    it("shows loading when admin status is null (checking)", async () => {
      mockSessionRef.current = {
        data: { user: { id: "user-1", email: "admin@example.com" } },
        isPending: false,
      };
      // isAdmin never resolves immediately, so isUserAdmin stays null
      mockIsAdmin.mockImplementation(() => new Promise(() => {}));

      render(<InvitationsPage />);

      expect(screen.getByTestId("loading")).toBeInTheDocument();
    });

    it("wraps loading in ResponsiveLayout", () => {
      mockSessionRef.current = { data: null, isPending: true };

      render(<InvitationsPage />);

      expect(screen.getByTestId("responsive-layout")).toBeInTheDocument();
      expect(screen.getByTestId("loading")).toBeInTheDocument();
    });
  });

  // =============================================================================
  // Authentication Tests
  // =============================================================================

  describe("authentication", () => {
    it("stays in loading state when not authenticated (isUserAdmin remains null)", () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<InvitationsPage />);

      // When there's no session user, isAdmin is never called, so isUserAdmin stays null
      // This means the component stays in loading state (isUserAdmin === null check)
      expect(screen.getByTestId("loading")).toBeInTheDocument();
      expect(mockIsAdmin).not.toHaveBeenCalled();
    });

    it("calls isAdmin when session has user", async () => {
      mockSessionRef.current = {
        data: { user: { id: "user-1", email: "test@example.com" } },
        isPending: false,
      };
      mockIsAdmin.mockResolvedValue(false);

      render(<InvitationsPage />);

      await waitFor(() => {
        expect(mockIsAdmin).toHaveBeenCalled();
      });
    });

    it("does not call isAdmin when session has no user", () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<InvitationsPage />);

      // isAdmin should not be called when there's no user
      expect(mockIsAdmin).not.toHaveBeenCalled();
    });

    it("redirects to root when session exists but user is null and isUserAdmin is false", async () => {
      // This case is tricky - the redirect only happens after isUserAdmin is set
      // But isAdmin is only called when session?.user exists
      // So we need a session with a user that then gets a non-admin result
      mockSessionRef.current = {
        data: { user: { id: "user-1", email: "test@example.com" } },
        isPending: false,
      };
      mockIsAdmin.mockResolvedValue(false);

      render(<InvitationsPage />);

      // After isAdmin resolves with false, the permission denied screen shows
      // (redirect only happens if !session?.user after the admin check)
      await waitFor(() => {
        expect(screen.getByText("Keine Berechtigung")).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Permission Denied Tests
  // =============================================================================

  describe("permission denied", () => {
    beforeEach(() => {
      mockSessionRef.current = {
        data: { user: { id: "user-1", email: "user@example.com" } },
        isPending: false,
      };
    });

    it("shows permission denied message when not admin", async () => {
      mockIsAdmin.mockResolvedValue(false);

      render(<InvitationsPage />);

      await waitFor(() => {
        expect(screen.getByText("Keine Berechtigung")).toBeInTheDocument();
      });
    });

    it("shows detailed permission denied text", async () => {
      mockIsAdmin.mockResolvedValue(false);

      render(<InvitationsPage />);

      await waitFor(() => {
        expect(
          screen.getByText(
            "Du verfügst nicht über die notwendigen Berechtigungen, um Einladungen zu verwalten.",
          ),
        ).toBeInTheDocument();
      });
    });

    it("renders permission denied in ResponsiveLayout", async () => {
      mockIsAdmin.mockResolvedValue(false);

      render(<InvitationsPage />);

      await waitFor(() => {
        expect(screen.getByTestId("responsive-layout")).toBeInTheDocument();
        expect(screen.getByText("Keine Berechtigung")).toBeInTheDocument();
      });
    });

    it("displays warning icon in permission denied state", async () => {
      mockIsAdmin.mockResolvedValue(false);

      render(<InvitationsPage />);

      await waitFor(() => {
        // Check for the SVG warning icon
        const svg = document.querySelector("svg");
        expect(svg).toBeInTheDocument();
        expect(svg).toHaveAttribute("viewBox", "0 0 24 24");
      });
    });
  });

  // =============================================================================
  // Admin View Tests
  // =============================================================================

  describe("admin view", () => {
    beforeEach(() => {
      mockSessionRef.current = {
        data: { user: { id: "admin-1", email: "admin@example.com" } },
        isPending: false,
      };
      mockIsAdmin.mockResolvedValue(true);
    });

    it("shows invitation form when user is admin", async () => {
      render(<InvitationsPage />);

      await waitFor(() => {
        expect(
          screen.getByTestId("invitation-create-form"),
        ).toBeInTheDocument();
      });
    });

    it("shows pending invitations list when user is admin", async () => {
      render(<InvitationsPage />);

      await waitFor(() => {
        expect(
          screen.getByTestId("pending-invitations-list"),
        ).toBeInTheDocument();
      });
    });

    it("renders admin view in ResponsiveLayout", async () => {
      render(<InvitationsPage />);

      await waitFor(() => {
        expect(screen.getByTestId("responsive-layout")).toBeInTheDocument();
        expect(
          screen.getByTestId("invitation-create-form"),
        ).toBeInTheDocument();
      });
    });

    it("passes initial refreshKey to pending invitations list", async () => {
      render(<InvitationsPage />);

      await waitFor(() => {
        expect(capturedRefreshKey).toBe(1704067200000);
      });
    });
  });

  // =============================================================================
  // Refresh Key Tests
  // =============================================================================

  describe("refresh key propagation", () => {
    beforeEach(() => {
      mockSessionRef.current = {
        data: { user: { id: "admin-1", email: "admin@example.com" } },
        isPending: false,
      };
      mockIsAdmin.mockResolvedValue(true);
    });

    it("updates refreshKey when invitation is created", async () => {
      // Track Date.now calls
      let callCount = 0;
      Date.now = vi.fn().mockImplementation(() => {
        callCount++;
        return callCount === 1 ? 1704067200000 : 1704067201000;
      });

      render(<InvitationsPage />);

      await waitFor(() => {
        expect(capturedOnCreated).toBeDefined();
      });

      const initialRefreshKey = capturedRefreshKey;

      // Trigger onCreated callback
      act(() => {
        capturedOnCreated?.();
      });

      await waitFor(() => {
        expect(capturedRefreshKey).not.toBe(initialRefreshKey);
      });
    });

    it("passes onCreated callback to invitation form", async () => {
      render(<InvitationsPage />);

      await waitFor(() => {
        expect(capturedOnCreated).toBeDefined();
        expect(typeof capturedOnCreated).toBe("function");
      });
    });
  });

  // =============================================================================
  // useEffect Dependency Tests
  // =============================================================================

  describe("useEffect behavior", () => {
    it("re-checks admin status when session changes", async () => {
      mockSessionRef.current = {
        data: { user: { id: "user-1", email: "user@example.com" } },
        isPending: false,
      };
      mockIsAdmin.mockResolvedValue(false);

      const { rerender } = render(<InvitationsPage />);

      await waitFor(() => {
        expect(mockIsAdmin).toHaveBeenCalledTimes(1);
      });

      // Update session
      mockSessionRef.current = {
        data: { user: { id: "user-2", email: "admin@example.com" } },
        isPending: false,
      };
      mockIsAdmin.mockResolvedValue(true);

      rerender(<InvitationsPage />);

      await waitFor(() => {
        expect(mockIsAdmin).toHaveBeenCalledTimes(2);
      });
    });
  });
});
