import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { _resetTraegerHostCache } from "~/lib/traeger-url";

const { mockSignOut, sessionUser } = vi.hoisted(() => ({
  mockSignOut: vi.fn(),
  sessionUser: {
    email: "buero@traeger.example",
    permissions: [] as string[],
  },
}));

vi.mock("next-auth/react", () => ({
  signOut: mockSignOut,
  useSession: () => ({
    data: { user: sessionUser },
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

  it("offers the holiday periods only to a role that manages enrollment", () => {
    sessionUser.permissions = ["traeger_overview:read"];
    const { unmount } = render(
      <TraegerShell>
        <p>Inhalt</p>
      </TraegerShell>,
    );
    expect(screen.getByRole("link", { name: "Schulen" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Ferien" })).toBeNull();
    unmount();

    sessionUser.permissions = ["traeger_enrollment:manage"];
    render(
      <TraegerShell>
        <p>Inhalt</p>
      </TraegerShell>,
    );
    expect(screen.getByRole("link", { name: "Ferien" })).toHaveAttribute(
      "href",
      "/ferien",
    );
  });
});
