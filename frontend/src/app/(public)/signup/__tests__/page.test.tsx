/**
 * Tests for app/(public)/signup/page.tsx
 *
 * Tests the public signup page including:
 * - Loading state during session check
 * - Redirect when already authenticated
 * - Display of signup form when not authenticated
 * - Page structure and branding
 */

import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

// Use vi.hoisted to define mocks before they're used in vi.mock
const { mockSessionRef, mockRouterRef } = vi.hoisted(() => ({
  mockSessionRef: {
    current: {
      data: null as { user: { id: string; email: string } } | null,
      isPending: false,
    },
  },
  mockRouterRef: {
    current: {
      push: vi.fn(),
    },
  },
}));

// Mock next/navigation
vi.mock("next/navigation", () => ({
  useRouter: () => mockRouterRef.current,
}));

// Mock auth-client
vi.mock("~/lib/auth-client", () => ({
  useSession: () => mockSessionRef.current,
}));

// Mock SignupForm component to simplify testing
vi.mock("~/components/auth/signup-form", () => ({
  SignupForm: () => <div data-testid="signup-form">Signup Form</div>,
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
    priority?: boolean;
  }) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img src={src} alt={alt} {...props} />
  ),
}));

// Import after mocks
import SignupPage from "../page";

beforeEach(() => {
  vi.clearAllMocks();
  mockSessionRef.current = { data: null, isPending: false };
  mockRouterRef.current = { push: vi.fn() };
});

describe("SignupPage", () => {
  // =============================================================================
  // Loading State Tests
  // =============================================================================

  describe("loading state", () => {
    it("shows loading when session is pending", () => {
      mockSessionRef.current = { data: null, isPending: true };

      render(<SignupPage />);

      // Should show loading indicator
      expect(screen.getByText("Laden...")).toBeInTheDocument();
    });

    it("shows form after auth check completes", async () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<SignupPage />);

      // After auth check completes, the form should be visible
      await waitFor(() => {
        expect(screen.getByTestId("signup-form")).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Authentication Redirect Tests
  // =============================================================================

  describe("authentication redirect", () => {
    it("redirects to dashboard when user is already authenticated", async () => {
      mockSessionRef.current = {
        data: { user: { id: "user-1", email: "test@example.com" } },
        isPending: false,
      };

      render(<SignupPage />);

      await waitFor(() => {
        expect(mockRouterRef.current.push).toHaveBeenCalledWith("/dashboard");
      });
    });

    it("does not redirect when not authenticated", async () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<SignupPage />);

      await waitFor(() => {
        expect(screen.getByTestId("signup-form")).toBeInTheDocument();
      });

      expect(mockRouterRef.current.push).not.toHaveBeenCalled();
    });
  });

  // =============================================================================
  // Page Structure Tests
  // =============================================================================

  describe("page structure", () => {
    it("renders page header with title", async () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<SignupPage />);

      await waitFor(() => {
        expect(screen.getByText("Konto erstellen")).toBeInTheDocument();
      });
    });

    it("renders subtitle text", async () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<SignupPage />);

      await waitFor(() => {
        expect(
          screen.getByText("Registriere dich, um moto zu nutzen."),
        ).toBeInTheDocument();
      });
    });

    it("renders the moto logo", async () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<SignupPage />);

      await waitFor(() => {
        const logo = screen.getByAltText("moto Logo");
        expect(logo).toBeInTheDocument();
        expect(logo).toHaveAttribute("src", "/images/moto_transparent.png");
      });
    });

    it("renders signup form component", async () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<SignupPage />);

      await waitFor(() => {
        expect(screen.getByTestId("signup-form")).toBeInTheDocument();
      });
    });

    it("renders logo as link to home", async () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<SignupPage />);

      await waitFor(() => {
        const logo = screen.getByAltText("moto Logo");
        const link = logo.closest("a");
        expect(link).toHaveAttribute("href", "/");
      });
    });
  });

  // =============================================================================
  // Suspense Fallback Tests
  // =============================================================================

  describe("suspense", () => {
    it("renders without crashing", () => {
      mockSessionRef.current = { data: null, isPending: false };

      const { container } = render(<SignupPage />);

      expect(container).toBeTruthy();
    });
  });
});
