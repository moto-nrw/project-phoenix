import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { _resetTraegerHostCache } from "~/lib/traeger-url";
import {
  expectIdleRenderBudget,
  RENDER_BUDGET_MAX_COMMITS,
} from "~/test/render-budget";

const { mockPathname, sessionUser } = vi.hoisted(() => ({
  mockPathname: vi.fn(() => "/"),
  sessionUser: { permissions: [] as string[] },
}));

vi.mock("next/navigation", async (importOriginal) => ({
  ...(await importOriginal<typeof import("next/navigation")>()),
  usePathname: () => mockPathname(),
  useRouter: () => ({ push: vi.fn(), prefetch: vi.fn() }),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: { user: sessionUser }, status: "authenticated" }),
}));

import { TraegerSidebar } from "./traeger-sidebar";

const ALL_RIGHTS = [
  "traeger_overview:read",
  "traeger_enrollment:manage",
  "traeger_billing:manage",
  "traeger_accounts:manage",
  "traeger_access_log:read",
];

function mainNav() {
  return within(screen.getByRole("navigation", { name: "Hauptnavigation" }));
}

function bottomNav() {
  return within(
    screen.getByRole("navigation", { name: "Hilfe und Einstellungen" }),
  );
}

function settingsLinks() {
  return Array.from(
    document.querySelectorAll<HTMLAnchorElement>("[data-portal-nav-item]"),
  ).map((link) => link.textContent);
}

describe("TraegerSidebar", () => {
  beforeEach(() => {
    localStorage.clear();
    mockPathname.mockReturnValue("/");
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", window.location.host);
    _resetTraegerHostCache();
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    _resetTraegerHostCache();
    sessionUser.permissions = [];
  });

  it("puts Briefkopf, Einstellungen and Zugriffsprotokoll into the collapsed section at the bottom (#5241, #5242)", () => {
    sessionUser.permissions = ALL_RIGHTS;
    render(<TraegerSidebar />);

    // Anker: die Arbeitsbereiche stehen oben.
    expect(mainNav().getByRole("link", { name: "Personen" })).toHaveAttribute(
      "href",
      "/accounts",
    );
    for (const name of ["Briefkopf", "Einstellungen", "Zugriffsprotokoll"]) {
      expect(mainNav().queryByRole("link", { name })).toBeNull();
    }

    const section = bottomNav().getByRole("button", { name: "Einstellungen" });
    expect(section).toHaveAttribute("aria-expanded", "false");
    const body = document.getElementById(
      section.getAttribute("aria-controls")!,
    );
    expect(body).toHaveAttribute("inert");
    expect(bottomNav().getByRole("link", { name: "Hilfe" })).toHaveAttribute(
      "href",
      "/help?role=traeger",
    );

    fireEvent.click(section);
    expect(section).toHaveAttribute("aria-expanded", "true");
    expect(body).toHaveProperty("inert", false);
    expect(
      bottomNav().getByRole("link", { name: "Zugriffsprotokoll" }),
    ).toHaveAttribute("href", "/access-log");
    expect(settingsLinks()).toEqual([
      "Briefkopf",
      "Einstellungen",
      "Zugriffsprotokoll",
    ]);

    fireEvent.click(section);
    expect(section).toHaveAttribute("aria-expanded", "false");
    expect(body).toHaveAttribute("inert");
  });

  it("shows only the entries the rights open", () => {
    sessionUser.permissions = ["traeger_overview:read"];
    const { unmount } = render(<TraegerSidebar />);
    expect(settingsLinks()).toEqual(["Briefkopf", "Einstellungen"]);
    unmount();

    sessionUser.permissions = ["traeger_access_log:read"];
    render(<TraegerSidebar />);
    expect(settingsLinks()).toEqual(["Zugriffsprotokoll"]);
  });

  it("hides the section when no entry is open to the person", () => {
    sessionUser.permissions = ["traeger_accounts:manage"];
    render(<TraegerSidebar />);

    // Anker: die Sitzung ist geladen, ihr Arbeitsbereich steht da.
    expect(
      mainNav().getByRole("link", { name: "Personen" }),
    ).toBeInTheDocument();
    expect(
      bottomNav().getByRole("link", { name: "Hilfe" }),
    ).toBeInTheDocument();
    expect(
      bottomNav().queryByRole("button", { name: "Einstellungen" }),
    ).toBeNull();
  });

  it("opens the section on one of its pages and marks that page", () => {
    sessionUser.permissions = ALL_RIGHTS;
    mockPathname.mockReturnValue("/access-log");
    render(<TraegerSidebar />);

    expect(
      bottomNav().getByRole("button", { name: "Einstellungen" }),
    ).toHaveAttribute("aria-expanded", "true");
    expect(
      bottomNav().getByRole("link", { name: "Zugriffsprotokoll" }),
    ).toHaveAttribute("aria-current", "page");
  });

  it("opens the section when the person navigates into it", () => {
    sessionUser.permissions = ALL_RIGHTS;
    const { rerender } = render(<TraegerSidebar />);
    const section = bottomNav().getByRole("button", { name: "Einstellungen" });
    expect(section).toHaveAttribute("aria-expanded", "false");

    mockPathname.mockReturnValue("/briefkopf");
    rerender(<TraegerSidebar />);
    expect(section).toHaveAttribute("aria-expanded", "true");

    // Zugeklappt auf einer seiner Seiten trägt die Kopfzeile die Markierung.
    fireEvent.click(section);
    expect(section).toHaveAttribute("aria-expanded", "false");
    expect(section).toHaveAttribute("data-active", "true");
  });

  it("keeps the section collapsed after a reload on one of its pages (#5331)", async () => {
    sessionUser.permissions = ALL_RIGHTS;
    mockPathname.mockReturnValue("/access-log");
    const { unmount } = render(<TraegerSidebar />);
    fireEvent.click(bottomNav().getByRole("button", { name: "Einstellungen" }));
    unmount();

    render(<TraegerSidebar />);
    const section = bottomNav().getByRole("button", { name: "Einstellungen" });
    await waitFor(() =>
      expect(section).toHaveAttribute("aria-expanded", "false"),
    );
    // Zugeklappt bleibt die aktive Seite an der Kopfzeile erkennbar.
    expect(section).toHaveAttribute("data-active", "true");
  });

  it("keeps the section open after a reload outside its pages (#5331)", async () => {
    sessionUser.permissions = ALL_RIGHTS;
    const { unmount } = render(<TraegerSidebar />);
    fireEvent.click(bottomNav().getByRole("button", { name: "Einstellungen" }));
    unmount();

    render(<TraegerSidebar />);
    await waitFor(() =>
      expect(
        bottomNav().getByRole("button", { name: "Einstellungen" }),
      ).toHaveAttribute("aria-expanded", "true"),
    );
  });

  // Render-Budget (#2939): die Leiste ist auf jeder Seite des Portals
  // gemountet; das Aufklappen beim Seitenwechsel darf keine Schleife sein.
  describe("render budget", () => {
    afterEach(() => {
      vi.useRealTimers();
    });

    it.each(["/", "/access-log"])(
      `commits at most ${RENDER_BUDGET_MAX_COMMITS} times in idle on %s`,
      async (pathname) => {
        vi.useFakeTimers();
        sessionUser.permissions = ALL_RIGHTS;
        mockPathname.mockReturnValue(pathname);

        await expectIdleRenderBudget(<TraegerSidebar />);
      },
    );
  });
});
