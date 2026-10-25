import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { _resetTraegerHostCache } from "~/lib/traeger-url";

const { mockSignOut, session } = vi.hoisted(() => ({
  mockSignOut: vi.fn(),
  session: { permissions: [] as string[] },
}));

vi.mock("next-auth/react", () => ({
  signOut: mockSignOut,
  useSession: () => ({
    data: {
      user: {
        email: "buero@traeger.example",
        permissions: session.permissions,
      },
    },
    status: "authenticated",
  }),
}));

import { TraegerShell } from "./traeger-shell";

describe("TraegerShell", () => {
  beforeEach(() => {
    mockSignOut.mockReset();
    mockSignOut.mockResolvedValue(undefined);
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null)));
    vi.spyOn(window.location, "assign").mockImplementation(() => undefined);
    // The test page counts as the Träger host.
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", window.location.host);
    _resetTraegerHostCache();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    vi.unstubAllEnvs();
    _resetTraegerHostCache();
  });

  // NextAuth resolves a callbackUrl against NEXTAUTH_URL, the tenant origin,
  // which sent the Träger to the school chooser after logout.
  it("revokes the session and returns to the Träger login on the Träger host", async () => {
    render(
      <TraegerShell>
        <p>Inhalt</p>
      </TraegerShell>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Abmelden" }));

    await waitFor(() =>
      expect(window.location.assign).toHaveBeenCalledWith("/login"),
    );
    expect(fetch).toHaveBeenCalledWith(
      "/api/traeger/auth/logout",
      expect.objectContaining({ method: "POST" }),
    );
    expect(mockSignOut).toHaveBeenCalledWith({ redirect: false });
  });

  // #4650: the page "Personen" exists only for the right to manage accounts.
  it("offers the page Personen only with the right to manage accounts", () => {
    session.permissions = ["traeger_overview:read"];
    const { unmount } = render(
      <TraegerShell>
        <p>Inhalt</p>
      </TraegerShell>,
    );
    expect(screen.getByRole("link", { name: "Schulen" })).toHaveAttribute(
      "href",
      "/",
    );
    expect(screen.queryAllByRole("link", { name: "Personen" })).toEqual([]);
    unmount();

    session.permissions = ["traeger_overview:read", "traeger_accounts:manage"];
    render(
      <TraegerShell>
        <p>Inhalt</p>
      </TraegerShell>,
    );
    expect(screen.getByRole("link", { name: "Personen" })).toHaveAttribute(
      "href",
      "/accounts",
    );
    session.permissions = [];
  });
});
