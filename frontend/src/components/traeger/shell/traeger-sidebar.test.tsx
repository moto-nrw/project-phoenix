import { render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { _resetTraegerHostCache } from "~/lib/traeger-url";
import {
  expectIdleRenderBudget,
  RENDER_BUDGET_MAX_COMMITS,
} from "~/test/render-budget";

const { mockPathname, mockSearch, sessionUser } = vi.hoisted(() => ({
  mockPathname: vi.fn(() => "/"),
  mockSearch: vi.fn(() => new URLSearchParams()),
  sessionUser: { permissions: [] as string[] },
}));

vi.mock("next/navigation", async (importOriginal) => ({
  ...(await importOriginal<typeof import("next/navigation")>()),
  usePathname: () => mockPathname(),
  useSearchParams: () => mockSearch(),
  useRouter: () => ({ push: vi.fn(), prefetch: vi.fn() }),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: { user: sessionUser }, status: "authenticated" }),
}));

import { TraegerReturnPathMemory } from "./traeger-return-path";
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

function settingsNav() {
  return within(screen.getByRole("navigation", { name: "Einstellungen" }));
}

function linkNames(nav: ReturnType<typeof mainNav>) {
  return nav.getAllByRole("link").map((link) => link.textContent);
}

describe("TraegerSidebar", () => {
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

  it("groups the main navigation by task and keeps Personen and Ermäßigungen out (#5685)", () => {
    sessionUser.permissions = ALL_RIGHTS;
    render(<TraegerSidebar />);

    for (const [group, name] of [
      ["Übersicht", "Dashboard"],
      ["Übersicht", "Schulen"],
      ["Übersicht", "Stichtagszahlen"],
      ["Betreuung", "Ferienbetreuung"],
      ["Finanzen", "Abrechnung"],
    ] as const) {
      expect(
        within(mainNav().getByRole("list", { name: group })).getByRole("link", {
          name,
        }),
      ).toBeInTheDocument();
    }
    expect(linkNames(mainNav())).toEqual([
      "Dashboard",
      "Schulen",
      "Stichtagszahlen",
      "Belegung",
      "Ferienbetreuung",
      "Dokumente",
      "Abrechnung",
    ]);
    expect(
      mainNav().getByRole("link", { name: "Stichtagszahlen" }),
    ).toHaveAttribute("href", "/stichtagszahlen");
    expect(mainNav().getByRole("link", { name: "Dashboard" })).toHaveAttribute(
      "href",
      "/uebersicht",
    );
    expect(linkNames(bottomNav())).toEqual(["Hilfe", "Einstellungen"]);
    expect(bottomNav().getByRole("link", { name: "Hilfe" })).toHaveAttribute(
      "href",
      "/help?role=traeger",
    );
    expect(
      bottomNav().getByRole("link", { name: "Einstellungen" }),
    ).toHaveAttribute("href", "/einstellungen");
  });

  it("leaves out a group without an open entry", () => {
    sessionUser.permissions = ["traeger_overview:read"];
    render(<TraegerSidebar />);

    // Die Dokumente des Trägers liest jede Rolle (#4692); deshalb bleibt
    // „Betreuung“ mit ihnen stehen, ohne die Ferienbetreuung.
    expect(linkNames(mainNav())).toEqual([
      "Dashboard",
      "Schulen",
      "Stichtagszahlen",
      "Belegung",
      "Dokumente",
    ]);
    expect(mainNav().queryByText("Ferienbetreuung")).toBeNull();
    expect(mainNav().queryByText("Finanzen")).toBeNull();
  });

  it("shows the Dashboard and the Stichtagszahlen only with their right (#4674, #4673)", () => {
    sessionUser.permissions = [];
    render(<TraegerSidebar />);

    expect(linkNames(mainNav())).toEqual(["Schulen"]);
  });

  it("replaces the content of the same sidebar in the settings (#5685)", () => {
    sessionUser.permissions = ALL_RIGHTS;
    mockPathname.mockReturnValue("/einstellungen/zugriffsprotokoll");
    render(<TraegerSidebar />);

    expect(screen.getAllByRole("complementary")).toHaveLength(1);
    expect(linkNames(settingsNav())).toEqual([
      "Zurück zur App",
      "Allgemein",
      "Mitglieder und Rollen",
      "Briefkopf und Vorlagen",
      "Abrechnungsvorgaben",
      // Dokumentart je Leistung, für die Rolle „Abrechnung“ (#4688).
      "Dokumentarten",
      "Zugriffsprotokoll",
    ]);
    expect(
      settingsNav().getByRole("link", { name: "Zugriffsprotokoll" }),
    ).toHaveAttribute("aria-current", "page");
    expect(
      settingsNav().getByRole("link", { name: "Allgemein" }),
    ).not.toHaveAttribute("aria-current");
    expect(screen.queryByRole("button")).toBeNull();
    expect(
      screen.queryByRole("navigation", { name: "Hauptnavigation" }),
    ).toBeNull();
    // Der Name des Bereichs steht als Überschrift, nie als zweiter Eintrag.
    expect(
      settingsNav().queryByRole("link", { name: "Einstellungen" }),
    ).toBeNull();
    expect(settingsNav().getAllByText("Einstellungen")).toHaveLength(1);
  });

  it("marks Allgemein only on its own page", () => {
    sessionUser.permissions = ALL_RIGHTS;
    mockPathname.mockReturnValue("/einstellungen");
    render(<TraegerSidebar />);

    expect(
      settingsNav().getByRole("link", { name: "Allgemein" }),
    ).toHaveAttribute("aria-current", "page");
  });

  it("shows only the settings entries the rights open (#5685)", () => {
    mockPathname.mockReturnValue("/einstellungen");
    sessionUser.permissions = ["traeger_overview:read"];
    const { unmount } = render(<TraegerSidebar />);
    expect(linkNames(settingsNav())).toEqual([
      "Zurück zur App",
      "Allgemein",
      "Briefkopf und Vorlagen",
      "Abrechnungsvorgaben",
    ]);
    unmount();

    sessionUser.permissions = ["traeger_access_log:read"];
    render(<TraegerSidebar />);
    expect(linkNames(settingsNav())).toEqual([
      "Zurück zur App",
      "Zugriffsprotokoll",
    ]);
  });

  it("opens the first settings page the rights allow", () => {
    sessionUser.permissions = ["traeger_access_log:read"];
    render(<TraegerSidebar />);

    expect(
      bottomNav().getByRole("link", { name: "Einstellungen" }),
    ).toHaveAttribute("href", "/einstellungen/zugriffsprotokoll");
  });

  it("hides Einstellungen when no entry is open to the person", () => {
    sessionUser.permissions = ["traeger_enrollment:manage"];
    render(<TraegerSidebar />);

    expect(
      mainNav().getByRole("link", { name: "Ferienbetreuung" }),
    ).toBeInTheDocument();
    expect(
      bottomNav().getByRole("link", { name: "Hilfe" }),
    ).toBeInTheDocument();
    expect(
      bottomNav().queryByRole("link", { name: "Einstellungen" }),
    ).toBeNull();
  });

  describe("Zurück zur App (#5685)", () => {
    it("leads to the start page without a remembered page", () => {
      sessionUser.permissions = ALL_RIGHTS;
      mockPathname.mockReturnValue("/einstellungen");
      render(<TraegerSidebar />);

      expect(
        settingsNav().getByRole("link", { name: "Zurück zur App" }),
      ).toHaveAttribute("href", "/");
    });

    it("leads back to the page opened last, with its filter", () => {
      sessionUser.permissions = ALL_RIGHTS;
      mockPathname.mockReturnValue("/ferien/5/anmeldungen");
      mockSearch.mockReturnValue(new URLSearchParams("status=offen"));
      const memory = render(<TraegerReturnPathMemory />);
      memory.unmount();

      mockPathname.mockReturnValue("/einstellungen/briefkopf");
      mockSearch.mockReturnValue(new URLSearchParams());
      render(
        <>
          <TraegerReturnPathMemory />
          <TraegerSidebar />
        </>,
      );

      expect(
        settingsNav().getByRole("link", { name: "Zurück zur App" }),
      ).toHaveAttribute("href", "/ferien/5/anmeldungen?status=offen");
    });

    it("ignores a stored address that leaves the portal", () => {
      sessionUser.permissions = ALL_RIGHTS;
      sessionStorage.setItem(
        "traeger-settings-return-path",
        "//fremde-seite.example",
      );
      mockPathname.mockReturnValue("/einstellungen");
      render(<TraegerSidebar />);

      expect(
        settingsNav().getByRole("link", { name: "Zurück zur App" }),
      ).toHaveAttribute("href", "/");
    });
  });

  // Render-Budget (#2939): die Leiste ist auf jeder Seite des Portals
  // gemountet; der Wechsel des Inhalts darf keine Schleife sein.
  describe("render budget", () => {
    afterEach(() => {
      vi.useRealTimers();
    });

    it.each(["/", "/einstellungen/zugriffsprotokoll"])(
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
