import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { _resetTraegerHostCache } from "~/lib/traeger-url";

const { mockPathname } = vi.hoisted(() => ({
  mockPathname: vi.fn(() => "/"),
}));

vi.mock("next/navigation", async (importOriginal) => ({
  ...(await importOriginal<typeof import("next/navigation")>()),
  usePathname: () => mockPathname(),
}));

const { mockSignOut } = vi.hoisted(() => ({ mockSignOut: vi.fn() }));

vi.mock("next-auth/react", () => ({
  signOut: mockSignOut,
  useSession: () => ({
    data: { user: { email: "buero@traeger.example" } },
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

  it("leads to both pages and marks the settings page (#4652)", () => {
    mockPathname.mockReturnValue("/einstellungen");
    render(
      <TraegerShell>
        <p>Inhalt</p>
      </TraegerShell>,
    );

    const settings = screen.getByRole("link", { name: "Einstellungen" });
    expect(settings).toHaveAttribute("href", "/einstellungen");
    expect(settings).toHaveAttribute("aria-current", "page");
    const schools = screen.getByRole("link", { name: "Schulen" });
    expect(schools).toHaveAttribute("href", "/");
    expect(schools).not.toHaveAttribute("aria-current");
    // The help opens the article of the page the person is on.
    expect(screen.getByRole("link", { name: "Hilfe" })).toHaveAttribute(
      "href",
      "/help/traeger-einstellungen?role=traeger&return_to=%2Feinstellungen",
    );
  });
});
