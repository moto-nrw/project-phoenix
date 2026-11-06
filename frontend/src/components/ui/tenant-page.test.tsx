import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";
import Link from "next/link";
import { TenantPage, TenantPageStats } from "./tenant-page";
import { Button, ButtonLink } from "./button";
import { ModuleNotBookedSlots } from "~/components/tenant/module-not-booked-state";

// Der Zustand für nicht gebuchte Module liest die Sitzung (#4894).
vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: null, status: "unauthenticated" }),
}));

vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: ComponentProps<"a">) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

describe("TenantPage", () => {
  // Der Rückweg einer Unterseite steht über dem Titel, auf jeder Breite, mit
  // Pfeil und dem Ziel als Text (#4364).
  it("zeigt den Rückweg als Link mit Ziel über dem Titel", () => {
    render(
      <TenantPage
        title="Raum 1"
        back
        backHref="/rooms?tab=belegt"
        backLabel="Zurück zu den Räumen"
      />,
    );

    const link = screen.getByRole("link", { name: "Zurück zu den Räumen" });
    expect(link).toHaveAttribute("href", "/test-tenant/rooms?tab=belegt");
    expect(link.closest("header")).toBe(
      screen.getByRole("heading", { level: 1 }).closest("header"),
    );
    expect(link.className).not.toMatch(/(^|\s)(sm|md|lg|xl):hidden/);
    expect(link.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
  });

  it("führt ohne Ziel zur Datenverwaltung und sagt das", () => {
    render(<TenantPage title="Klassenliste" back />);

    expect(
      screen.getByRole("link", { name: "Zurück zur Datenverwaltung" }),
    ).toHaveAttribute("href", "/test-tenant/database");
  });

  it("nennt kein falsches Ziel, wenn nur das Ziel gesetzt ist", () => {
    render(<TenantPage title="Raum 1" back backHref="/rooms" />);

    expect(screen.getByRole("link", { name: "Zurück" })).toHaveAttribute(
      "href",
      "/test-tenant/rooms",
    );
  });

  it("zeigt ohne `back` keinen Rückweg", () => {
    render(<TenantPage title="Kinder" />);

    // Anker: der Seitenkopf mit Titel ist gerendert.
    expect(
      screen.getByRole("heading", { level: 1, name: "Kinder" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Zurück/ })).toBeNull();
  });

  it("rendert Titel als einzige h1, Statuszeile und Aktionen in der Kopfkarte", () => {
    render(
      <TenantPage
        title="Kinder"
        stats="116 Kinder · 107 zuhause"
        actions={<button type="button">Kind anlegen</button>}
      >
        <p>Inhalt</p>
      </TenantPage>,
    );

    const headings = screen.getAllByRole("heading", { level: 1 });
    expect(headings).toHaveLength(1);
    expect(headings[0]).toHaveTextContent("Kinder");
    expect(screen.getByText("116 Kinder · 107 zuhause")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Kind anlegen" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Inhalt")).toBeInTheDocument();
  });

  // #5531: Die Kopfkarte setzt ihre Bedienhöhe über Nachfahren-Selektoren.
  // Ein Link in Button-Optik muss davon genauso erfasst sein wie ein Knopf,
  // sonst steht er mit 32 oder 36 px neben 40-px-Knöpfen.
  it("bringt einen ButtonLink in der Kopfkarte auf die Höhe des Knopfs daneben", () => {
    render(
      <TenantPage
        title="Kinder"
        actions={
          <>
            <ButtonLink href="/students/import" variant="outline" size="md">
              Importieren
            </ButtonLink>
            <Button type="button" size="md">
              Kind anlegen
            </Button>
          </>
        }
      />,
    );

    const link = screen.getByRole("link", { name: "Importieren" });
    const button = screen.getByRole("button", { name: "Kind anlegen" });
    const container = button.parentElement!;
    expect(link.parentElement).toBe(container);

    const selectorsFor = (prefix: string, height: string) =>
      [...container.classList]
        .map((c) => new RegExp(`^${prefix}\\[&_(.+)\\]:${height}$`).exec(c))
        .filter((m): m is RegExpExecArray => m !== null)
        .map((m) => m[1]!.replaceAll("_", " "));

    for (const [prefix, height] of [
      ["", "h-10"],
      ["max-sm:", "h-11"],
    ] as const) {
      const selectors = selectorsFor(prefix, height);
      expect(selectors.some((sel) => button.matches(sel))).toBe(true);
      expect(selectors.some((sel) => link.matches(sel))).toBe(true);
    }
  });

  it("bleibt auf dem Telefon eine Karte mit engerem Innenrand", () => {
    const { container } = render(
      <TenantPage title="Kinder">
        <p>Inhalt</p>
      </TenantPage>,
    );

    const header = container.querySelector("header");
    expect(header).toHaveClass(
      "moto-content-surface",
      "rounded-2xl",
      "border",
      "max-sm:p-4",
    );
    expect(header).not.toHaveClass(
      "max-sm:bg-transparent",
      "max-sm:border-0",
      "max-sm:p-0",
    );
  });

  it("zeigt beim Laden ein Skelett statt der Statuszeile", () => {
    const { container } = render(
      <TenantPage title="Kinder" stats="116 Kinder" statsLoading>
        <p>Inhalt</p>
      </TenantPage>,
    );

    expect(screen.queryByText("116 Kinder")).not.toBeInTheDocument();
    expect(container.querySelector(".animate-pulse")).toBeInTheDocument();
  });

  it("reicht den deaktivierten Zustand an das Suchfeld weiter", () => {
    render(
      <TenantPage
        title="Räume"
        search={{
          value: "",
          onChange: vi.fn(),
          inputProps: { disabled: true },
        }}
      />,
    );

    for (const searchField of screen.getAllByRole("textbox")) {
      expect(searchField).toBeDisabled();
    }
  });

  it("rendert das Status-Skelett nicht innerhalb eines Absatzes", () => {
    const { container } = render(
      <TenantPage title="Kinder" statsLoading>
        <p>Inhalt</p>
      </TenantPage>,
    );

    // Anker: das Status-Skelett ist gerendert.
    expect(container.querySelector(".animate-pulse")).toBeInTheDocument();
    expect(container.querySelector("p .animate-pulse")).toBeNull();
  });

  it("meldet beim Laden einen neutralen Status", () => {
    render(
      <TenantPage title="Kinder" loading>
        <p>Inhalt</p>
      </TenantPage>,
    );

    expect(screen.getByRole("status")).toHaveAttribute(
      "aria-label",
      "Der Bereich „Kinder“ wird geladen…",
    );
  });

  it("ersetzt den Inhalt durch den Fehler, behält aber die Kopfkarte", () => {
    render(
      <TenantPage title="Kinder" error="Kinder konnten nicht geladen werden.">
        <p>Inhalt</p>
      </TenantPage>,
    );

    expect(
      screen.getByRole("heading", { level: 1, name: "Kinder" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Kinder konnten nicht geladen werden."),
    ).toBeInTheDocument();
    expect(screen.queryByText("Inhalt")).not.toBeInTheDocument();
  });

  it("zeigt einen Ladefehler vom Fehlerweg mit Wiederholen und Vorgangskennung", () => {
    const retry = vi.fn();
    render(
      <TenantPage
        title="Kind"
        error={{
          message: "Das Kind ist gerade nicht erreichbar.",
          retry: { label: "Wiederholen", onClick: retry },
          requestId: {
            value: "req-1",
            label: "Vorgangskennung: req-1",
            copyLabel: "Vorgangskennung kopieren",
            copiedLabel: "Kopiert.",
            copyFailedLabel: "Kopieren nicht möglich.",
          },
        }}
      >
        <p>Inhalt</p>
      </TenantPage>,
    );

    expect(
      screen.getByText("Das Kind ist gerade nicht erreichbar."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Vorgangskennung kopieren" }),
    ).toHaveTextContent("Vorgangskennung: req-1");
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(retry).toHaveBeenCalledOnce();
    expect(screen.queryByText("Inhalt")).not.toBeInTheDocument();
  });

  it("ersetzt den Inhalt durch den Leerzustand", () => {
    render(
      <TenantPage title="Kinder" empty={{ title: "Noch kein Kind angelegt" }}>
        <p>Inhalt</p>
      </TenantPage>,
    );

    // Ohne Aktion ist der Leerzustand EIN Satz in der Karte; der Titel
    // bekommt dabei einen Schlusspunkt.
    expect(screen.getByText("Noch kein Kind angelegt.")).toBeInTheDocument();
    expect(screen.queryByText("Inhalt")).not.toBeInTheDocument();
  });

  it("inszeniert den Leerzustand nur, wenn es einen nächsten Schritt gibt", () => {
    render(
      <TenantPage
        title="Kinder"
        empty={{
          title: "Noch kein Kind angelegt",
          description: "Legen Sie das erste Kind an.",
          action: <button type="button">Kind anlegen</button>,
        }}
      >
        <p>Inhalt</p>
      </TenantPage>,
    );

    // Mit Aktion bleibt die EmptyState-Form: Überschrift plus Knopf.
    expect(
      screen.getByRole("button", { name: "Kind anlegen" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Noch kein Kind angelegt")).toBeInTheDocument();
  });

  it("lässt den Rumpf die Höhe füllen: Wurzel als Flex-Spalte, Leerzustand als letzte Fläche darin", () => {
    const { container } = render(
      <TenantPage
        title="Kinder"
        empty={{ title: "Noch kein Kind angelegt" }}
        testId="seite"
      >
        <p>Inhalt</p>
      </TenantPage>,
    );

    expect(screen.getByTestId("seite")).toHaveClass(
      "flex",
      "flex-1",
      "flex-col",
    );
    const body = container.querySelector(".moto-tenant-body");
    expect(body).not.toBeNull();
    // `.moto-tenant-body > .moto-content-surface:last-child` ist die Fläche,
    // die bis zur Unterkante wächst (globals.css). Der Leersatz muss also
    // direkt und als letztes Kind im Rumpf stehen -- eine weitere Hülle
    // darum würde die Regel still ins Leere laufen lassen.
    const last = body?.lastElementChild;
    expect(last?.tagName).toBe("SECTION");
    expect(last).toHaveClass("moto-content-surface");
    expect(last).toHaveTextContent("Noch kein Kind angelegt.");
  });

  it("schaltet Seitenreiter per Klick, nicht erst per mousedown", () => {
    const onChange = vi.fn();
    render(
      <TenantPage
        title="Personalakte"
        tabs={{
          value: "stammdaten",
          onChange,
          items: [
            { value: "stammdaten", label: "Stammdaten" },
            { value: "dokumente", label: "Dokumente", badge: 3 },
          ],
        }}
      >
        <p>Inhalt</p>
      </TenantPage>,
    );

    const aktiv = screen.getByRole("tab", { name: "Stammdaten" });
    const inaktiv = screen.getByRole("tab", { name: /Dokumente/ });
    expect(aktiv).toHaveAttribute("aria-selected", "true");
    expect(inaktiv).toHaveAttribute("aria-selected", "false");
    expect(aktiv).toHaveAttribute("tabindex", "0");
    expect(inaktiv).toHaveAttribute("tabindex", "-1");
    expect(inaktiv).toHaveTextContent("3");

    fireEvent.click(inaktiv);
    expect(onChange).toHaveBeenCalledWith("dokumente");
  });

  it("wechselt Reiter mit den Pfeiltasten in einer Roving-Tabulator-Reihenfolge", () => {
    const onChange = vi.fn();
    render(
      <TenantPage
        title="Personalakte"
        tabs={{
          value: "stammdaten",
          onChange,
          items: [
            { value: "stammdaten", label: "Stammdaten" },
            { value: "dokumente", label: "Dokumente" },
            { value: "verlauf", label: "Verlauf" },
          ],
        }}
      />,
    );

    const first = screen.getByRole("tab", { name: "Stammdaten" });
    const second = screen.getByRole("tab", { name: "Dokumente" });
    first.focus();
    fireEvent.keyDown(first, { key: "ArrowRight" });

    expect(second).toHaveFocus();
    expect(onChange).toHaveBeenCalledWith("dokumente");

    fireEvent.keyDown(second, { key: "End" });
    expect(screen.getByRole("tab", { name: "Verlauf" })).toHaveFocus();
    expect(onChange).toHaveBeenLastCalledWith("verlauf");
  });

  it("keeps a tab reachable when the current value matches no tab", () => {
    const onChange = vi.fn();
    render(
      <TenantPage
        title="Personalakte"
        tabs={{
          value: "eigener-filter",
          onChange,
          items: [
            { value: "stammdaten", label: "Stammdaten" },
            { value: "dokumente", label: "Dokumente" },
          ],
        }}
      />,
    );

    const first = screen.getByRole("tab", { name: "Stammdaten" });
    const second = screen.getByRole("tab", { name: "Dokumente" });
    expect(first).toHaveAttribute("aria-selected", "false");
    expect(second).toHaveAttribute("aria-selected", "false");
    expect(first).toHaveAttribute("tabindex", "0");
    expect(second).toHaveAttribute("tabindex", "-1");

    first.focus();
    fireEvent.keyDown(first, { key: "ArrowRight" });

    expect(second).toHaveFocus();
    expect(onChange).toHaveBeenCalledWith("dokumente");
  });

  it("lässt einen Reiter mit Zielpfad wie einen Link navigieren", () => {
    const onChange = vi.fn();
    render(
      <TenantPage
        title="Datenverwaltung"
        tabs={{
          value: "kinder",
          onChange,
          items: [
            { value: "kinder", label: "Kinder" },
            { value: "personal", label: "Personal", href: "/personal" },
          ],
        }}
      />,
    );

    const tab = screen.getByRole("tab", { name: "Personal" });
    const event = new MouseEvent("click", { bubbles: true, cancelable: true });
    tab.dispatchEvent(event);

    expect(onChange).toHaveBeenCalledWith("personal");
    expect(event.defaultPrevented).toBe(false);
  });

  it("erhält den Zielpfad eines Reiters im Überlaufmenü", () => {
    const onChange = vi.fn();
    const clientWidth = vi
      .spyOn(HTMLElement.prototype, "clientWidth", "get")
      .mockReturnValue(120);
    const offsetWidth = vi
      .spyOn(HTMLElement.prototype, "offsetWidth", "get")
      .mockReturnValue(100);

    try {
      render(
        <TenantPage
          title="Datenverwaltung"
          tabs={{
            value: "personal",
            onChange,
            items: [
              { value: "kinder", label: "Kinder" },
              {
                value: "personal",
                label: "Personal",
                href: "/personal",
              },
            ],
          }}
        />,
      );

      const tablist = screen.getByRole("tablist", {
        name: "Seitenbereiche",
      });
      const moreButton = screen.getByRole("tab", { name: "Mehr" });

      expect(moreButton).toHaveAttribute("role", "tab");
      expect(moreButton).toHaveAttribute("aria-selected", "true");
      expect(moreButton).toHaveAttribute("tabindex", "0");

      expect(within(tablist).getByRole("tab", { name: "Mehr" })).toBe(
        moreButton,
      );
      moreButton.focus();
      fireEvent.keyDown(moreButton, { key: "ArrowLeft" });
      expect(screen.getByRole("tab", { name: "Kinder" })).toHaveFocus();
      expect(onChange).toHaveBeenCalledWith("kinder");

      fireEvent.click(moreButton);

      expect(
        screen.getByRole("menuitem", { name: "Personal" }),
      ).toHaveAttribute("href", "/personal");
      expect(
        screen.getByRole("menuitem", { name: "Personal" }),
      ).toHaveAttribute("aria-current", "true");
    } finally {
      clientWidth.mockRestore();
      offsetWidth.mockRestore();
    }
  });

  it("setzt zwischen die Wert-Label-Paare der Statuszeile ein Trennzeichen", () => {
    render(
      <TenantPageStats
        items={[
          { value: 116, label: "Kinder" },
          { value: 9, label: "krank" },
        ]}
      />,
    );

    expect(screen.getByText("116")).toBeInTheDocument();
    expect(screen.getByText("krank")).toBeInTheDocument();
    expect(screen.getByText("·")).toBeInTheDocument();
  });

  it("zeigt mehrere Werte ohne Label ohne doppelten Schlüssel", () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);
    try {
      render(
        <TenantPageStats
          items={[
            { value: "Bären", label: "" },
            { value: "07.09. bis 13.09.2026", label: "" },
          ]}
        />,
      );

      expect(screen.getByText("Bären")).toBeInTheDocument();
      expect(screen.getByText("07.09. bis 13.09.2026")).toBeInTheDocument();
      expect(consoleError).not.toHaveBeenCalled();
    } finally {
      consoleError.mockRestore();
    }
  });
  it("zeigt die Reiter auf jeder Breite als ein Band, nie als Auswahlliste", () => {
    render(
      <TenantPage
        title="Einstellungen"
        tabs={{
          value: "operations",
          onChange: vi.fn(),
          items: [
            { value: "operations", label: "Betrieb" },
            { value: "gdpr", label: "Datenschutz" },
          ],
          label: "Einstellungsbereiche",
        }}
      >
        <p>Inhalt</p>
      </TenantPage>,
    );

    // Eine Auswahlliste zeigte auf dem Telefon nur den aktiven Wert
    // („Betrieb") und las sich als Filter. Das Band nennt alle Bereiche.
    expect(screen.queryByRole("combobox")).toBeNull();
    const band = screen.getByRole("tablist", { name: "Einstellungsbereiche" });
    expect(band.className).toContain("max-sm:overflow-x-auto");
    expect(within(band).getAllByRole("tab")).toHaveLength(2);
  });

  it("legt die Erklärung hinter einen Info-Knopf neben dem Titel", () => {
    render(
      <TenantPage
        title="Datenverwaltung"
        info={
          <p>
            Hier legen Sie Daten an.{" "}
            <Link href="/students/search">Alle Kinder</Link>
          </p>
        }
      >
        <p>Inhalt</p>
      </TenantPage>,
    );

    // Im Alltag kostet die Erklärung keine Zeile: sie ist zu, bis jemand
    // sie öffnet.
    expect(screen.queryByText(/Hier legen Sie Daten an/)).toBeNull();
    const heading = screen.getByRole("heading", { level: 1 });
    expect(heading).toHaveTextContent(/^Datenverwaltung$/);

    const trigger = screen.getByRole("button", {
      name: "Erklärung zu Datenverwaltung",
    });
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(heading.contains(trigger)).toBe(false);

    fireEvent.click(trigger);

    const panel = screen.getByRole("dialog", {
      name: "Erklärung zu Datenverwaltung",
    });
    expect(trigger).toHaveAttribute("aria-expanded", "true");
    expect(trigger).toHaveAttribute("aria-controls", panel.id);
    // Der Rahmen hängt am Ende des Dokuments; ohne Fokus darin erreichte
    // die Tastatur den Link erst nach der ganzen Seite.
    expect(panel.contains(document.activeElement)).toBe(true);
    expect(panel).toHaveTextContent(/Hier legen Sie Daten an/);
    expect(
      within(panel).getByRole("link", { name: "Alle Kinder" }),
    ).toHaveAttribute("href", "/students/search");

    fireEvent.keyDown(panel, { key: "Escape" });
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(trigger).toHaveFocus();
  });

  describe("Info-Knopf mit der Maus (#5682)", () => {
    const renderInfoPage = () =>
      render(
        <>
          <button type="button">Daneben</button>
          <TenantPage
            title="Schulen"
            info={
              <p>
                Alle Schulen. <Link href="/traeger/hilfe">Mehr dazu</Link>
              </p>
            }
          >
            <p>Inhalt</p>
          </TenantPage>
        </>,
      );
    const infoButton = () =>
      screen.getByRole("button", { name: "Erklärung zu Schulen" });
    const findPanel = () =>
      screen.findByRole("dialog", { name: "Erklärung zu Schulen" });

    it("öffnet den Hinweis beim Darüberfahren, ohne den Fokus zu nehmen", async () => {
      const user = userEvent.setup();
      renderInfoPage();
      const elsewhere = screen.getByRole("button", { name: "Daneben" });
      elsewhere.focus();

      await user.hover(infoButton());

      const panel = await findPanel();
      expect(panel).toHaveTextContent("Alle Schulen.");
      expect(infoButton()).toHaveAttribute("aria-expanded", "true");
      expect(infoButton()).toHaveAttribute("aria-controls", panel.id);
      expect(elsewhere).toHaveFocus();
    });

    it("schließt nach dem Verlassen erst mit Verzögerung", async () => {
      const user = userEvent.setup();
      renderInfoPage();
      await user.hover(infoButton());
      await findPanel();

      await user.unhover(infoButton());

      // Noch offen: der Weg vom Knopf in den Hinweis darf ihn nicht schließen.
      expect(screen.getByRole("dialog")).toBeInTheDocument();
      await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
      expect(infoButton()).toHaveAttribute("aria-expanded", "false");
    });

    it("hält den Link im Hinweis per Maus erreichbar", async () => {
      const user = userEvent.setup();
      renderInfoPage();
      await user.hover(infoButton());
      const panel = await findPanel();

      await user.unhover(infoButton());
      await user.hover(panel);
      // Länger warten, als das Schließen dauern würde.
      await new Promise((resolve) => setTimeout(resolve, 400));

      expect(
        within(screen.getByRole("dialog")).getByRole("link", {
          name: "Mehr dazu",
        }),
      ).toBeInTheDocument();
      await user.unhover(panel);
      await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    });

    it("öffnet nichts, wenn die Maus nur kurz vorbeifährt", async () => {
      const user = userEvent.setup();
      renderInfoPage();

      await user.hover(infoButton());
      await user.unhover(infoButton());
      await new Promise((resolve) => setTimeout(resolve, 300));

      // Anker: die Seite steht, nur der Hinweis ist zu; dass er per Darüberfahren
      // öffnen kann, zeigt der erste Test dieses Blocks mit demselben Query.
      expect(infoButton()).toHaveAttribute("aria-expanded", "false");
      expect(screen.queryByRole("dialog")).toBeNull();
    });

    it("hält einen per Darüberfahren geöffneten Hinweis beim Klick fest", async () => {
      const user = userEvent.setup();
      renderInfoPage();
      await user.hover(infoButton());
      const panel = await findPanel();

      await user.click(infoButton());

      expect(screen.getByRole("dialog")).toBe(panel);
      expect(panel.contains(document.activeElement)).toBe(true);
      await user.unhover(infoButton());
      await new Promise((resolve) => setTimeout(resolve, 400));
      expect(screen.getByRole("dialog")).toBe(panel);

      // Der zweite Klick schließt wie bisher.
      await user.click(infoButton());
      expect(screen.queryByRole("dialog")).toBeNull();
    });

    it("lässt einen per Klick geöffneten Hinweis beim Verlassen offen", async () => {
      const user = userEvent.setup();
      renderInfoPage();

      await user.click(infoButton());
      const panel = screen.getByRole("dialog");
      expect(panel.contains(document.activeElement)).toBe(true);
      await user.unhover(infoButton());
      await new Promise((resolve) => setTimeout(resolve, 400));

      expect(screen.getByRole("dialog")).toBe(panel);
    });

    it("gibt den Fokus bei Escape nur zurück, wenn der Hinweis ihn hatte", async () => {
      const user = userEvent.setup();
      renderInfoPage();
      const elsewhere = screen.getByRole("button", { name: "Daneben" });
      elsewhere.focus();
      await user.hover(infoButton());
      await findPanel();

      await user.keyboard("{Escape}");

      expect(screen.queryByRole("dialog")).toBeNull();
      expect(elsewhere).toHaveFocus();
    });

    it("hält den Hinweis fest, sobald der Fokus hineinkommt", async () => {
      const user = userEvent.setup();
      renderInfoPage();
      await user.hover(infoButton());
      const panel = await findPanel();

      within(panel).getByRole("link", { name: "Mehr dazu" }).focus();
      await user.unhover(infoButton());
      await new Promise((resolve) => setTimeout(resolve, 400));
      expect(screen.getByRole("dialog")).toBe(panel);

      await user.keyboard("{Escape}");
      expect(screen.queryByRole("dialog")).toBeNull();
      expect(infoButton()).toHaveFocus();
    });

    it("öffnet per Tastatur wie bisher und nicht schon beim Fokus", async () => {
      const user = userEvent.setup();
      renderInfoPage();

      infoButton().focus();
      await new Promise((resolve) => setTimeout(resolve, 300));
      // Anker: der Knopf hat den Fokus, der Hinweis öffnet erst bei Enter.
      expect(infoButton()).toHaveFocus();
      expect(screen.queryByRole("dialog")).toBeNull();

      await user.keyboard("{Enter}");
      const panel = screen.getByRole("dialog");
      expect(panel.contains(document.activeElement)).toBe(true);

      await user.keyboard("{Escape}");
      expect(screen.queryByRole("dialog")).toBeNull();
      expect(infoButton()).toHaveFocus();

      await user.keyboard(" ");
      expect(screen.getByRole("dialog")).toBeInTheDocument();
    });

    it("öffnet bei Touch erst durch das Tippen", async () => {
      renderInfoPage();

      fireEvent.pointerEnter(infoButton(), { pointerType: "touch" });
      await new Promise((resolve) => setTimeout(resolve, 300));
      // Anker: der Knopf steht, der Hinweis öffnet erst beim Tippen darunter.
      expect(infoButton()).toHaveAttribute("aria-expanded", "false");
      expect(screen.queryByRole("dialog")).toBeNull();

      fireEvent.click(infoButton());
      expect(screen.getByRole("dialog")).toBeInTheDocument();
    });
  });

  it("zeigt ohne Erklärung keinen Info-Knopf", () => {
    render(
      <TenantPage title="Kinder">
        <p>Inhalt</p>
      </TenantPage>,
    );

    // Anker: der Seitenkopf mit Titel ist gerendert.
    expect(
      screen.getByRole("heading", { level: 1, name: "Kinder" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Erklärung/ })).toBeNull();
  });

  it("zeigt im Fehlerzustand die mitgegebene Aktion", () => {
    render(
      <TenantPage
        title="Speiseplan"
        error={{
          message: "Der Speiseplan konnte nicht geladen werden.",
          action: <button type="button">Erneut versuchen</button>,
        }}
      >
        <p>Inhalt</p>
      </TenantPage>,
    );

    expect(
      screen.getByRole("button", { name: "Erneut versuchen" }),
    ).toBeInTheDocument();
  });
});

// Eine mit module.not_booked abgelehnte Anfrage (#4894) ersetzt den Inhalt
// durch den erklärenden Zustand statt durch eine rote Fehlermeldung.
describe("TenantPage mit nicht gebuchtem Modul", () => {
  it("erklärt das Modul statt einen Fehler zu zeigen", () => {
    render(
      <ModuleNotBookedSlots>
        <TenantPage
          title="Mein Dienstplan"
          error={{
            message:
              "Ihre Schule hat das Modul Dienstplan nicht gebucht. Für eine Buchung wenden Sie sich an das moto-Team.",
            notBookedModule: "dienstplan",
          }}
        >
          <p>Inhalt</p>
        </TenantPage>
      </ModuleNotBookedSlots>,
    );

    expect(screen.getByText("Dienstplan ist nicht gebucht")).toBeVisible();
    expect(screen.queryByText("Inhalt")).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  // #5034: Der Leerzustand wiederholt keine Kopfaktion. Das Gerüst meldet
  // eine Wiederholung außerhalb der Produktion.
  describe("Leerzustand neben einer Kopfaktion", () => {
    it("meldet einen Leerzustand-Knopf, der die Kopfaktion wiederholt", () => {
      const consoleError = vi
        .spyOn(console, "error")
        .mockImplementation(() => undefined);
      const { rerender } = render(
        <TenantPage
          title="Räume"
          actions={<button type="button">+ Raum</button>}
          empty={{
            title: "Keine Räume vorhanden",
            action: <button type="button">Raum</button>,
          }}
        />,
      );
      rerender(
        <TenantPage
          title="Räume"
          actions={<button type="button">+ Raum</button>}
          empty={{
            title: "Keine Räume vorhanden",
            action: <button type="button">Raum</button>,
          }}
        />,
      );
      expect(consoleError).toHaveBeenCalledWith(
        "tenant_page_empty_action_repeats_head_action",
        expect.objectContaining({ page: "Räume", action: "raum" }),
      );
      expect(consoleError).toHaveBeenCalledTimes(1);
      consoleError.mockRestore();
    });

    it("lässt eine Leerzustand-Aktion für eine andere Handlung zu", () => {
      const consoleError = vi
        .spyOn(console, "error")
        .mockImplementation(() => undefined);
      render(
        <TenantPage
          title="Räume"
          actions={<button type="button">+ Raum</button>}
          empty={{
            title: "Keine passenden Räume",
            action: <button type="button">Filter zurücksetzen</button>,
          }}
        />,
      );
      expect(
        screen.getByRole("button", { name: "Filter zurücksetzen" }),
      ).toBeInTheDocument();
      expect(consoleError).not.toHaveBeenCalledWith(
        "tenant_page_empty_action_repeats_head_action",
        expect.anything(),
      );
      consoleError.mockRestore();
    });
  });
});
