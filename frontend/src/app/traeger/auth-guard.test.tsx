import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { _resetTraegerHostCache } from "~/lib/traeger-url";

const { mockUseSession, mockRedirect, mockFetchOrganization, mockPathname } =
  vi.hoisted(() => ({
    mockUseSession: vi.fn(),
    mockRedirect: vi.fn(),
    mockFetchOrganization: vi.fn(),
    mockPathname: vi.fn(() => "/traeger"),
  }));

vi.mock("next-auth/react", () => ({ useSession: () => mockUseSession() }));
vi.mock("next/navigation", () => ({
  redirect: (path: string) => mockRedirect(path),
  usePathname: () => mockPathname(),
}));
vi.mock("~/components/traeger/traeger-shell", () => ({
  TraegerShell: ({
    children,
    testSystem,
  }: {
    children: React.ReactNode;
    testSystem?: boolean;
  }) => (
    <div data-testid="shell" data-test-system={String(testSystem === true)}>
      {children}
    </div>
  ),
}));
vi.mock("~/lib/traeger-api", () => ({
  fetchTraegerOrganization: mockFetchOrganization,
}));

import { TraegerAuthGuard } from "./auth-guard";

function session(user: Record<string, unknown>, error?: string) {
  return { status: "authenticated", data: { user, error } };
}

describe("TraegerAuthGuard", () => {
  beforeEach(() => {
    mockUseSession.mockReset();
    mockRedirect.mockReset();
    mockFetchOrganization.mockReset();
    mockPathname.mockReturnValue("/traeger");
    mockFetchOrganization.mockResolvedValue({
      id: "5",
      name: "Echter Träger",
      testSystem: false,
    });
    // Not the Träger host: paths keep their /traeger prefix.
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", "traeger.localhost:3000");
    _resetTraegerHostCache();
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    _resetTraegerHostCache();
  });

  // The reset link opens without a session (#5149), on the Träger host as
  // /reset-password and elsewhere with its /traeger prefix.
  it.each(["/traeger/reset-password", "/reset-password"])(
    "shows %s without a session",
    (path) => {
      mockPathname.mockReturnValue(path);
      mockUseSession.mockReturnValue({ status: "unauthenticated", data: null });
      render(
        <TraegerAuthGuard>
          <p>Neues Passwort</p>
        </TraegerAuthGuard>,
      );

      expect(screen.getByText("Neues Passwort")).toBeInTheDocument();
      expect(mockRedirect).not.toHaveBeenCalled();
    },
  );

  it("renders the portal for a live Träger session", () => {
    mockUseSession.mockReturnValue(session({ scope: "org" }));
    render(
      <TraegerAuthGuard>
        <p>Schulen</p>
      </TraegerAuthGuard>,
      { wrapper: ToastProvider },
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
      { wrapper: ToastProvider },
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
      { wrapper: ToastProvider },
    );

    expect(mockRedirect).toHaveBeenCalledWith("/traeger/login");
  });

  // #4658: a test Träger is marked on every page; a real one is not (#5145
  // moved the strip from the page into the shell).
  it("marks the portal of a test Träger as a test system", async () => {
    mockFetchOrganization.mockResolvedValue({
      id: "4",
      name: "Test-Träger",
      testSystem: true,
    });
    mockUseSession.mockReturnValue(session({ scope: "org" }));
    render(
      <TraegerAuthGuard>
        <p>Schulen</p>
      </TraegerAuthGuard>,
      { wrapper: ToastProvider },
    );

    await waitFor(() =>
      expect(screen.getByTestId("shell")).toHaveAttribute(
        "data-test-system",
        "true",
      ),
    );
  });

  it("shows no test-system mark for a real Träger", async () => {
    mockUseSession.mockReturnValue(session({ scope: "org" }));
    render(
      <TraegerAuthGuard>
        <p>Schulen</p>
      </TraegerAuthGuard>,
      { wrapper: ToastProvider },
    );

    await waitFor(() => expect(mockFetchOrganization).toHaveBeenCalled());
    expect(screen.getByTestId("shell")).toHaveAttribute(
      "data-test-system",
      "false",
    );
  });
});
