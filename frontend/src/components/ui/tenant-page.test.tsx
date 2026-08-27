import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { TenantPage, TenantPageStats } from "./tenant-page";

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

  it("zeigt beim Laden ein Skelett statt der Statuszeile", () => {
    const { container } = render(
      <TenantPage title="Kinder" stats="116 Kinder" statsLoading>
        <p>Inhalt</p>
      </TenantPage>,
    );

    expect(screen.queryByText("116 Kinder")).not.toBeInTheDocument();
    expect(container.querySelector(".animate-pulse")).toBeInTheDocument();
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

    expect(screen.getByText("Noch kein Kind angelegt")).toBeInTheDocument();
    expect(screen.queryByText("Inhalt")).not.toBeInTheDocument();
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
  it("bietet die Reiter auf schmalen Geraeten zusaetzlich als Auswahlliste an", () => {
    const onChange = vi.fn();
    render(
      <TenantPage
        title="Einstellungen"
        tabs={{
          value: "operations",
          onChange,
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

    const auswahl = screen.getByRole("combobox", {
      name: "Einstellungsbereiche",
    });
    fireEvent.change(auswahl, { target: { value: "gdpr" } });
    expect(onChange).toHaveBeenCalledWith("gdpr");
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
