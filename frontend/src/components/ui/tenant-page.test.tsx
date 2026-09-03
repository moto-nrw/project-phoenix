import { fireEvent, render, screen, within } from "@testing-library/react";
import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";
import { TenantPage, TenantPageStats } from "./tenant-page";

vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: ComponentProps<"a">) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

describe("TenantPage", () => {
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

  it("rendert die Kopfzeile auf Mobilgeräten ohne Kartenrahmen", () => {
    const { container } = render(
      <TenantPage title="Kinder">
        <p>Inhalt</p>
      </TenantPage>,
    );

    expect(container.querySelector("header")).toHaveClass(
      "max-sm:rounded-none",
      "max-sm:border-0",
      "max-sm:bg-transparent",
      "max-sm:p-0",
      "max-sm:shadow-none",
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

  it("rendert das Status-Skelett nicht innerhalb eines Absatzes", () => {
    const { container } = render(
      <TenantPage title="Kinder" statsLoading>
        <p>Inhalt</p>
      </TenantPage>,
    );

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
    expect(inaktiv).toHaveTextContent("3");

    fireEvent.click(inaktiv);
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
            onChange: vi.fn(),
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

      expect(within(tablist).getByRole("tab", { name: "Mehr" })).toBe(
        moreButton,
      );
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
