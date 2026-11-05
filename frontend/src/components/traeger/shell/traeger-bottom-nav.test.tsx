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

import { TraegerBottomNav } from "./traeger-bottom-nav";

const ALL_RIGHTS = [
  "traeger_overview:read",
  "traeger_enrollment:manage",
  "traeger_billing:manage",
  "traeger_accounts:manage",
  "traeger_access_log:read",
];

function bar() {
  return within(screen.getByRole("navigation", { name: "Hauptnavigation" }));
}

describe("TraegerBottomNav", () => {
  beforeEach(() => {
    sessionStorage.clear();
    mockPathname.mockReturnValue("/");
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", window.location.host);
    _resetTraegerHostCache();
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    _resetTraegerHostCache();
    sessionUser.permissions = [];
  });

  it("follows the sidebar: main navigation, then Hilfe and Einstellungen (#5685)", () => {
    // Ohne Dashboard (#4674) passen alle Ziele in die Leiste.
    sessionUser.permissions = [
      "traeger_enrollment:manage",
      "traeger_billing:manage",
      "traeger_accounts:manage",
    ];
    render(<TraegerBottomNav />);

    expect(
      bar()
        .getAllByRole("link")
        .map((link) => link.getAttribute("aria-label")),
    ).toEqual([
      "Schulen",
      "Ferienbetreuung",
      "Abrechnung",
      "Hilfe",
      "Einstellungen",
    ]);
    expect(bar().getByRole("link", { name: "Schulen" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(bar().queryByRole("button", { name: "Mehr" })).toBeNull();
  });

  it("puts Hilfe and Einstellungen behind Mehr once the Dashboard fills the bar (#4674, #5685)", async () => {
    sessionUser.permissions = ALL_RIGHTS;
    render(<TraegerBottomNav />);

    expect(
      bar()
        .getAllByRole("link")
        .map((link) => link.getAttribute("aria-label")),
    ).toEqual(["Dashboard", "Schulen", "Ferienbetreuung", "Abrechnung"]);
    expect(bar().getByRole("link", { name: "Dashboard" })).toHaveAttribute(
      "href",
      "/uebersicht",
    );

    fireEvent.click(bar().getByRole("button", { name: "Mehr" }));
    const drawer = within(await screen.findByRole("dialog"));
    expect(drawer.getByRole("link", { name: "Hilfe" })).toHaveAttribute(
      "href",
      "/help?role=traeger",
    );
    expect(drawer.getByRole("link", { name: "Einstellungen" })).toHaveAttribute(
      "href",
      "/einstellungen",
    );
  });

  it("shows the settings with the way back, the rest behind Mehr (#5685)", async () => {
    sessionUser.permissions = ALL_RIGHTS;
    sessionStorage.setItem("traeger-settings-return-path", "/abrechnung");
    mockPathname.mockReturnValue("/einstellungen/abrechnung");
    render(<TraegerBottomNav />);

    expect(
      bar()
        .getAllByRole("link")
        .map((link) => link.getAttribute("aria-label")),
    ).toEqual([
      "Zurück zur App",
      "Allgemein",
      "Mitglieder und Rollen",
      "Briefkopf und Vorlagen",
    ]);
    expect(bar().getByRole("link", { name: "Zurück zur App" })).toHaveAttribute(
      "href",
      "/abrechnung",
    );
    const more = bar().getByRole("button", { name: "Mehr" });
    // A page behind Mehr is open: Mehr carries the mark.
    expect(more).toHaveAttribute("data-active", "true");

    fireEvent.click(more);
    const drawer = within(await screen.findByRole("dialog"));
    expect(
      drawer.getByRole("link", { name: "Abrechnungsvorgaben" }),
    ).toHaveAttribute("aria-current", "page");
    expect(
      drawer.getByRole("link", { name: "Zugriffsprotokoll" }),
    ).toHaveAttribute("href", "/einstellungen/zugriffsprotokoll");
  });

  it("shows only the settings the rights open, all in the bar while they fit", () => {
    sessionUser.permissions = ["traeger_overview:read"];
    mockPathname.mockReturnValue("/einstellungen");
    render(<TraegerBottomNav />);

    expect(bar().getAllByRole("link")).toHaveLength(4);
    expect(bar().queryByRole("button", { name: "Mehr" })).toBeNull();
    expect(bar().getByRole("link", { name: "Allgemein" })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });
});
