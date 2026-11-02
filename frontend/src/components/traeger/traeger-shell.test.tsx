import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ShellIntlProvider } from "~/components/dashboard/shell-intl-provider";
import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { TraegerShellProvider } from "~/lib/shell-auth-context";
import { _resetTraegerHostCache } from "~/lib/traeger-url";

const { mockSignOut, mockPathname, sessionUser } = vi.hoisted(() => ({
  mockSignOut: vi.fn(),
  mockPathname: vi.fn(() => "/"),
  sessionUser: {
    name: "Petra Büro",
    email: "buero@traeger.example",
    permissions: [] as string[],
  },
}));

vi.mock("next/navigation", async (importOriginal) => ({
  ...(await importOriginal<typeof import("next/navigation")>()),
  usePathname: () => mockPathname(),
  useSearchParams: () => new URLSearchParams(),
  useRouter: () => ({ push: vi.fn(), prefetch: vi.fn() }),
}));

vi.mock("next-auth/react", () => ({
  signOut: mockSignOut,
  useSession: () => ({
    data: { user: sessionUser },
    status: "authenticated",
  }),
}));

import { TraegerShell } from "./traeger-shell";
import { ToastProvider } from "~/contexts/ToastContext";

function renderPortal({
  organizationName,
  testSystem = false,
}: { organizationName?: string; testSystem?: boolean } = {}) {
  return render(
    <ToastProvider>
      <TraegerShellProvider organizationName={organizationName}>
        <BreadcrumbProvider>
          <ShellIntlProvider>
            <TraegerShell testSystem={testSystem}>
              <p>Inhalt</p>
            </TraegerShell>
          </ShellIntlProvider>
        </BreadcrumbProvider>
      </TraegerShellProvider>
    </ToastProvider>,
  );
}

function sidebar() {
  return within(screen.getByRole("complementary"));
}

async function signOutThroughProfileMenu() {
  fireEvent.click(
    screen.getByRole("button", { name: /Profilmenü von Petra Büro/ }),
  );
  fireEvent.click(await screen.findByRole("button", { name: /Abmelden/ }));
  // The shared header asks before it signs out (#5145).
  const dialog = await screen.findByRole("dialog");
  expect(mockSignOut).not.toHaveBeenCalled();
  fireEvent.click(within(dialog).getByRole("button", { name: "Abmelden" }));
}

describe("TraegerShell", () => {
  beforeEach(() => {
    mockSignOut.mockReset();
    mockSignOut.mockResolvedValue(undefined);
    mockPathname.mockReturnValue("/");
    sessionUser.permissions = [];
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
    renderPortal();

    await signOutThroughProfileMenu();

    await waitFor(() =>
      expect(window.location.assign).toHaveBeenCalledWith("/login"),
    );
    expect(fetch).toHaveBeenCalledWith(
      "/api/traeger/auth/logout",
      expect.objectContaining({ method: "POST" }),
    );
    expect(mockSignOut).toHaveBeenCalledWith({ redirect: false });
  });

  it("leaves no personal entries of the account on the device (#5000)", async () => {
    localStorage.setItem("moto:personal-storage-owner", "3:9");
    sessionStorage.setItem("timetable-reopenable-instance", '{"title":"AG"}');
    localStorage.setItem("sidebar-collapsed", "true");
    let leftAtSignOut: (string | null)[] = [];
    mockSignOut.mockImplementation(() => {
      leftAtSignOut = [
        localStorage.getItem("moto:personal-storage-owner"),
        sessionStorage.getItem("timetable-reopenable-instance"),
      ];
      return Promise.resolve(undefined);
    });
    renderPortal();

    await signOutThroughProfileMenu();

    await waitFor(() =>
      expect(window.location.assign).toHaveBeenCalledWith("/login"),
    );
    expect(leftAtSignOut).toEqual([null, null]);
    expect(localStorage.getItem("sidebar-collapsed")).toBe("true");
    localStorage.clear();
  });

  // #5145: the shared header with the Träger's name, the page title and the
  // role, no head bar of its own.
  it("shows the shared header with the Träger's name and the page title", () => {
    mockPathname.mockReturnValue("/einstellungen");
    sessionUser.permissions = ["traeger_overview:read"];
    renderPortal({ organizationName: "Ganztag gGmbH" });

    const header = within(screen.getByRole("banner"));
    expect(header.getByText("Ganztag gGmbH")).toBeInTheDocument();
    expect(header.getAllByText("Einstellungen").length).toBeGreaterThan(0);
    expect(header.getByText("Träger-Büro")).toBeInTheDocument();
  });

  it("offers the holiday periods only to a role that manages enrollment", () => {
    sessionUser.permissions = ["traeger_overview:read"];
    const { unmount } = renderPortal();
    expect(sidebar().getByRole("link", { name: "Hilfe" })).toBeInTheDocument();
    expect(sidebar().queryByRole("link", { name: "Ferien" })).toBeNull();
    unmount();

    sessionUser.permissions = ["traeger_enrollment:manage"];
    renderPortal();
    expect(sidebar().getByRole("link", { name: "Ferien" })).toHaveAttribute(
      "href",
      "/ferien",
    );
  });

  // #4650: the page "Personen" exists only for the right to manage accounts.
  it("offers the page Personen only with the right to manage accounts", () => {
    sessionUser.permissions = ["traeger_overview:read"];
    const { unmount } = renderPortal();
    expect(sidebar().getByRole("link", { name: "Hilfe" })).toBeInTheDocument();
    expect(sidebar().queryByRole("link", { name: "Personen" })).toBeNull();
    unmount();

    sessionUser.permissions = [
      "traeger_overview:read",
      "traeger_accounts:manage",
    ];
    renderPortal();
    expect(sidebar().getByRole("link", { name: "Personen" })).toHaveAttribute(
      "href",
      "/accounts",
    );
    expect(sidebar().queryByRole("link", { name: "Ferien" })).toBeNull();
  });

  // #4655: the access log is a page only for those who may read it.
  it("offers the access log only with the right to read it", () => {
    sessionUser.permissions = [
      "traeger_overview:read",
      "traeger_enrollment:manage",
    ];
    const { unmount } = renderPortal();
    expect(sidebar().getByRole("link", { name: "Ferien" })).toBeInTheDocument();
    expect(
      sidebar().queryByRole("link", { name: "Zugriffsprotokoll" }),
    ).toBeNull();
    unmount();

    sessionUser.permissions = [
      "traeger_overview:read",
      "traeger_access_log:read",
    ];
    renderPortal();
    expect(
      sidebar().getByRole("link", { name: "Zugriffsprotokoll" }),
    ).toHaveAttribute("href", "/access-log");
  });

  // #4652: the settings open with the right to read the overview, and the
  // help opens the article of the page the person is on.
  it("leads to the settings and opens their help article there", () => {
    sessionUser.permissions = ["traeger_overview:read"];
    const { unmount } = renderPortal();
    expect(
      sidebar().getByRole("link", { name: "Einstellungen" }),
    ).toHaveAttribute("href", "/einstellungen");
    unmount();

    mockPathname.mockReturnValue("/einstellungen");
    renderPortal();
    const help = new URL(
      screen
        .getByRole("link", { name: "Hilfe zu dieser Seite" })
        .getAttribute("href")!,
      "http://localhost",
    );
    expect(help.pathname).toBe("/help/traeger-einstellungen");
    expect(help.searchParams.get("role")).toBe("traeger");
    expect(help.searchParams.get("return_to")).toBe("/einstellungen");
  });

  // #5145: real links, and the current page is marked as such.
  it("marks the current page in the sidebar, also on its sub-pages", () => {
    mockPathname.mockReturnValue("/ferien/5/anmeldungen");
    sessionUser.permissions = [
      "traeger_overview:read",
      "traeger_enrollment:manage",
    ];
    renderPortal();

    expect(sidebar().getByRole("link", { name: "Ferien" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(
      sidebar().getByRole("link", { name: "Schulen" }),
    ).not.toHaveAttribute("aria-current");
  });

  it("puts the test-system strip above the header only for a test Träger", () => {
    const { unmount } = renderPortal({ testSystem: true });
    expect(screen.getByText("Testsystem")).toBeInTheDocument();
    expect(screen.getByText(/Alle Kinder sind erfunden/)).toBeInTheDocument();
    unmount();

    renderPortal();
    expect(screen.getByText("Inhalt")).toBeInTheDocument();
    expect(screen.queryByText("Testsystem")).toBeNull();
  });
});
