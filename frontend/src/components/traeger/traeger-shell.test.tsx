import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { _resetTraegerHostCache } from "~/lib/traeger-url";

const { mockSignOut, mockPermissions } = vi.hoisted(() => ({
  mockSignOut: vi.fn(),
  mockPermissions: { current: [] as string[] },
}));

vi.mock("next-auth/react", () => ({
  signOut: mockSignOut,
  useSession: () => ({
    data: {
      user: {
        email: "buero@traeger.example",
        permissions: mockPermissions.current,
      },
    },
    status: "authenticated",
  }),
}));
vi.mock("next/navigation", async (importOriginal) => ({
  ...(await importOriginal<typeof import("next/navigation")>()),
  usePathname: () => "/access-log",
}));

import { TraegerShell } from "./traeger-shell";

describe("TraegerShell", () => {
  beforeEach(() => {
    mockPermissions.current = [];
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

  // #4655: the access log is a link only for those who may read it.
  it("links the access log only with the right to read it", () => {
    const { unmount } = render(
      <TraegerShell>
        <p>Inhalt</p>
      </TraegerShell>,
    );
    const nav = screen.getByRole("navigation", { name: "Bereiche" });
    expect(screen.getByRole("link", { name: "Schulen" })).toHaveAttribute(
      "href",
      "/",
    );
    expect(nav).not.toHaveTextContent("Zugriffsprotokoll");
    unmount();

    mockPermissions.current = [
      "traeger_overview:read",
      "traeger_access_log:read",
    ];
    render(
      <TraegerShell>
        <p>Inhalt</p>
      </TraegerShell>,
    );
    const link = screen.getByRole("link", { name: "Zugriffsprotokoll" });
    expect(link).toHaveAttribute("href", "/access-log");
    expect(link).toHaveAttribute("aria-current", "page");
  });
});
