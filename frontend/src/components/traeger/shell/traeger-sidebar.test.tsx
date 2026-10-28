import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { _resetTraegerHostCache } from "~/lib/traeger-url";

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
    expect(bottomNav().getByRole("link", { name: "Hilfe" })).toHaveAttribute(
      "href",
      "/help?role=traeger",
    );

    fireEvent.click(section);
    expect(section).toHaveAttribute("aria-expanded", "true");
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
});
