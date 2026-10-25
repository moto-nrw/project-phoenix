import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { _resetTraegerHostCache } from "~/lib/traeger-url";

const { mockSignOut, mockPush, mockPathname, sessionUser } = vi.hoisted(() => ({
  mockSignOut: vi.fn(),
  mockPush: vi.fn(),
  mockPathname: vi.fn(() => "/"),
  sessionUser: {
    email: "buero@traeger.example",
    permissions: [] as string[],
  },
}));

vi.mock("next/navigation", async (importOriginal) => ({
  ...(await importOriginal<typeof import("next/navigation")>()),
  usePathname: () => mockPathname(),
  useRouter: () => ({ push: mockPush }),
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
    expect(screen.getByRole("link", { name: "Hilfe" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Ferien" })).toBeNull();
    unmount();

    sessionUser.permissions = ["traeger_enrollment:manage"];
    render(
      <TraegerShell>
        <p>Inhalt</p>
      </TraegerShell>,
    );
    fireEvent.click(screen.getAllByRole("button", { name: "Ferien" })[0]!);
    expect(mockPush).toHaveBeenCalledWith("/ferien");
    sessionUser.permissions = [];
  });

  // #4650: the page "Personen" exists only for the right to manage accounts.
  it("offers the page Personen only with the right to manage accounts", () => {
    sessionUser.permissions = ["traeger_overview:read"];
    const { unmount } = render(
      <TraegerShell>
        <p>Inhalt</p>
      </TraegerShell>,
    );
    expect(screen.getByRole("link", { name: "Hilfe" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Personen" })).toBeNull();
    unmount();

    sessionUser.permissions = [
      "traeger_overview:read",
      "traeger_accounts:manage",
    ];
    render(
      <TraegerShell>
        <p>Inhalt</p>
      </TraegerShell>,
    );
    fireEvent.click(screen.getAllByRole("button", { name: "Personen" })[0]!);
    expect(mockPush).toHaveBeenLastCalledWith("/accounts");
    expect(screen.queryByRole("button", { name: "Ferien" })).toBeNull();
    sessionUser.permissions = [];
  });

  // #4655: the access log is a tab only for those who may read it.
  it("offers the access log only with the right to read it", () => {
    sessionUser.permissions = [
      "traeger_overview:read",
      "traeger_enrollment:manage",
    ];
    const { unmount } = render(
      <TraegerShell>
        <p>Inhalt</p>
      </TraegerShell>,
    );
    expect(
      screen.getAllByRole("button", { name: "Ferien" }).length,
    ).toBeGreaterThan(0);
    expect(
      screen.queryByRole("button", { name: "Zugriffsprotokoll" }),
    ).toBeNull();
    unmount();

    sessionUser.permissions = [
      "traeger_overview:read",
      "traeger_access_log:read",
    ];
    render(
      <TraegerShell>
        <p>Inhalt</p>
      </TraegerShell>,
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: "Zugriffsprotokoll" })[0]!,
    );
    expect(mockPush).toHaveBeenLastCalledWith("/access-log");
    sessionUser.permissions = [];
  });

  // #4652: the settings open with the right to read the overview, and the
  // help opens the article of the page the person is on.
  it("leads to the settings and opens their help article there", () => {
    sessionUser.permissions = ["traeger_overview:read"];
    const { unmount } = render(
      <TraegerShell>
        <p>Inhalt</p>
      </TraegerShell>,
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: "Einstellungen" })[0]!,
    );
    expect(mockPush).toHaveBeenLastCalledWith("/einstellungen");
    unmount();

    mockPathname.mockReturnValue("/einstellungen");
    render(
      <TraegerShell>
        <p>Inhalt</p>
      </TraegerShell>,
    );
    expect(screen.getByRole("link", { name: "Hilfe" })).toHaveAttribute(
      "href",
      "/help/traeger-einstellungen?role=traeger&return_to=%2Feinstellungen",
    );
    sessionUser.permissions = [];
    mockPathname.mockReturnValue("/");
  });
});
