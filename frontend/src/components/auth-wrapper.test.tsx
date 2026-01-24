/**
 * Tests for AuthWrapper component
 *
 * Tests that AuthWrapper:
 * 1. Renders children correctly
 * 2. Calls useUserContext hook
 * 3. Calls useGlobalSSE hook
 * 4. Logs debug info in development mode
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { MockInstance } from "vitest";
import { render, screen } from "@testing-library/react";
import { AuthWrapper } from "./auth-wrapper";
import { useSession } from "~/lib/auth-client";

// Note: useSession from ~/lib/auth-client is mocked globally in test/setup.ts
// The global mock provides: { data: { user: {...} }, isPending: false, error: null }
const mockUseSession = vi.mocked(useSession);

vi.mock("~/lib/hooks/use-user-context", () => ({
  useUserContext: vi.fn(() => ({
    isReady: true,
    userContext: undefined,
    isLoading: false,
    error: undefined,
  })),
}));

vi.mock("~/lib/hooks/use-global-sse", () => ({
  useGlobalSSE: vi.fn(() => ({
    status: "connected",
    isConnected: true,
    error: null,
    reconnectAttempts: 0,
  })),
}));

describe("AuthWrapper", () => {
  let consoleLogSpy: MockInstance<typeof console.log>;

  beforeEach(() => {
    vi.clearAllMocks();
    consoleLogSpy = vi
      .spyOn(console, "log")
      .mockImplementation(() => undefined);
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    consoleLogSpy.mockRestore();
  });

  it("renders children correctly", () => {
    render(
      <AuthWrapper>
        <div data-testid="child">Test Child</div>
      </AuthWrapper>,
    );

    expect(screen.getByTestId("child")).toBeInTheDocument();
    expect(screen.getByText("Test Child")).toBeInTheDocument();
  });

  it("renders multiple children", () => {
    render(
      <AuthWrapper>
        <div data-testid="child1">First Child</div>
        <div data-testid="child2">Second Child</div>
      </AuthWrapper>,
    );

    expect(screen.getByTestId("child1")).toBeInTheDocument();
    expect(screen.getByTestId("child2")).toBeInTheDocument();
  });

  it("calls useUserContext hook", async () => {
    const { useUserContext } = await import("~/lib/hooks/use-user-context");

    render(
      <AuthWrapper>
        <div>Test</div>
      </AuthWrapper>,
    );

    expect(useUserContext).toHaveBeenCalled();
  });

  it("calls useGlobalSSE hook", async () => {
    const { useGlobalSSE } = await import("~/lib/hooks/use-global-sse");

    render(
      <AuthWrapper>
        <div>Test</div>
      </AuthWrapper>,
    );

    expect(useGlobalSSE).toHaveBeenCalled();
  });

  it("logs debug info in development mode when authenticated", async () => {
    // Set development mode
    vi.stubEnv("NODE_ENV", "development");

    render(
      <AuthWrapper>
        <div>Test</div>
      </AuthWrapper>,
    );

    // Wait for useEffect
    await vi.waitFor(() => {
      expect(consoleLogSpy).toHaveBeenCalled();
    });
  });

  it("does not log debug info in production mode", async () => {
    // Set production mode
    vi.stubEnv("NODE_ENV", "production");

    render(
      <AuthWrapper>
        <div>Test</div>
      </AuthWrapper>,
    );

    // Give time for any potential logs
    await new Promise((r) => setTimeout(r, 50));

    // Should not log SSE status in production
    const sseLogCalls = consoleLogSpy.mock.calls.filter((call: unknown[]) =>
      String(call[0]).includes("[AuthWrapper]"),
    );
    expect(sseLogCalls.length).toBe(0);
  });

  describe("BetterAuth session handling", () => {
    it("calls useSession hook from auth-client", async () => {
      render(
        <AuthWrapper>
          <div>Test</div>
        </AuthWrapper>,
      );

      expect(mockUseSession).toHaveBeenCalled();
    });

    it("renders children when session is pending", () => {
      mockUseSession.mockReturnValue({
        data: null,
        isPending: true,
        error: null,
      });

      render(
        <AuthWrapper>
          <div data-testid="content">Content</div>
        </AuthWrapper>,
      );

      expect(screen.getByTestId("content")).toBeInTheDocument();
    });

    it("renders children when session is null (unauthenticated)", () => {
      mockUseSession.mockReturnValue({
        data: null,
        isPending: false,
        error: null,
      });

      render(
        <AuthWrapper>
          <div data-testid="content">Content</div>
        </AuthWrapper>,
      );

      expect(screen.getByTestId("content")).toBeInTheDocument();
    });

    it("does not log when isPending is true", async () => {
      vi.stubEnv("NODE_ENV", "development");

      mockUseSession.mockReturnValue({
        data: {
          user: {
            id: "1",
            email: "test@example.com",
            name: "Test",
            emailVerified: true,
            image: null,
            createdAt: new Date(),
            updatedAt: new Date(),
          },
          session: {
            id: "s-1",
            userId: "1",
            expiresAt: new Date(),
            ipAddress: null,
            userAgent: null,
          },
          activeOrganizationId: "org-1",
        },
        isPending: true, // Pending should prevent logging
        error: null,
      });

      render(
        <AuthWrapper>
          <div>Test</div>
        </AuthWrapper>,
      );

      // Give time for any potential logs
      await new Promise((r) => setTimeout(r, 50));

      // Should not log when isPending is true (condition: session?.user && !isPending)
      const authWrapperLogs = consoleLogSpy.mock.calls.filter(
        (call: unknown[]) => String(call[0]).includes("[AuthWrapper]"),
      );
      expect(authWrapperLogs.length).toBe(0);
    });

    it("does not log when session user is null", async () => {
      vi.stubEnv("NODE_ENV", "development");

      mockUseSession.mockReturnValue({
        data: null,
        isPending: false,
        error: null,
      });

      render(
        <AuthWrapper>
          <div>Test</div>
        </AuthWrapper>,
      );

      // Give time for any potential logs
      await new Promise((r) => setTimeout(r, 50));

      // Should not log when no user (condition: session?.user && !isPending)
      const authWrapperLogs = consoleLogSpy.mock.calls.filter(
        (call: unknown[]) => String(call[0]).includes("[AuthWrapper]"),
      );
      expect(authWrapperLogs.length).toBe(0);
    });
  });
});
