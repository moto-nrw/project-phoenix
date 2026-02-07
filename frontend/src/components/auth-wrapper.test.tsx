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

// Mock hooks
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    status: "authenticated",
    data: { user: { token: "test-token" } },
  })),
}));

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
  let consoleDebugSpy: MockInstance<typeof console.debug>;

  beforeEach(() => {
    vi.clearAllMocks();
    consoleDebugSpy = vi
      .spyOn(console, "debug")
      .mockImplementation(() => undefined);
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    consoleDebugSpy.mockRestore();
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
      expect(consoleDebugSpy).toHaveBeenCalled();
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
    const sseLogCalls = consoleDebugSpy.mock.calls.filter((call: unknown[]) =>
      String(call[0]).includes("auth wrapper state"),
    );
    expect(sseLogCalls.length).toBe(0);
  });
});
