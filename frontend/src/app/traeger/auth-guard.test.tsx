import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { _resetTraegerHostCache } from "~/lib/traeger-url";

const { mockUseSession, mockRedirect } = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockRedirect: vi.fn(),
}));

vi.mock("next-auth/react", () => ({ useSession: () => mockUseSession() }));
vi.mock("next/navigation", () => ({
  redirect: (path: string) => mockRedirect(path),
  usePathname: () => "/traeger",
}));
vi.mock("~/components/traeger/traeger-shell", () => ({
  TraegerShell: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="shell">{children}</div>
  ),
}));

import { TraegerAuthGuard } from "./auth-guard";

function session(user: Record<string, unknown>, error?: string) {
  return { status: "authenticated", data: { user, error } };
}

describe("TraegerAuthGuard", () => {
  beforeEach(() => {
    mockUseSession.mockReset();
    mockRedirect.mockReset();
    // Not the Träger host: paths keep their /traeger prefix.
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", "traeger.localhost:3000");
    _resetTraegerHostCache();
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    _resetTraegerHostCache();
  });

  it("renders the portal for a live Träger session", () => {
    mockUseSession.mockReturnValue(session({ scope: "org" }));
    render(
      <TraegerAuthGuard>
        <p>Schulen</p>
      </TraegerAuthGuard>,
    );

    expect(screen.getByTestId("shell")).toHaveTextContent("Schulen");
    expect(mockRedirect).not.toHaveBeenCalled();
  });

  // The layout's server snapshot carries no token (#4991).
  it("renders the portal for the layout's server snapshot", () => {
    mockUseSession.mockReturnValue(session({ scope: "org", orgId: 7 }));
    render(
      <TraegerAuthGuard>
        <p>Schulen</p>
      </TraegerAuthGuard>,
    );

    expect(screen.getByTestId("shell")).toHaveTextContent("Schulen");
    expect(mockRedirect).not.toHaveBeenCalled();
  });

  // The session callback keeps scope org after the refresh ended for good;
  // the page must not stay open with stale content.
  it.each([
    ["an expired refresh", session({ scope: "org" }, "RefreshTokenExpired")],
    // No browser session carries a token (#5005): the callback marks a
    // session without one with an error.
    ["a missing access token", session({ scope: "org" }, "RefreshTokenError")],
    ["another scope", session({ scope: "school" })],
  ])("sends %s back to the Träger login", (_case, value) => {
    mockUseSession.mockReturnValue(value);
    render(
      <TraegerAuthGuard>
        <p>Schulen</p>
      </TraegerAuthGuard>,
    );

    expect(mockRedirect).toHaveBeenCalledWith("/traeger/login");
  });
});
