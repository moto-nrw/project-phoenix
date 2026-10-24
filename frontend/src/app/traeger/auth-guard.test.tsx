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
    mockUseSession.mockReturnValue(session({ scope: "org", token: "access" }));
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
    [
      "an expired refresh",
      session({ scope: "org", token: "access" }, "RefreshTokenExpired"),
    ],
    ["a missing access token", session({ scope: "org", token: "" })],
    ["another scope", session({ scope: "school", token: "access" })],
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
