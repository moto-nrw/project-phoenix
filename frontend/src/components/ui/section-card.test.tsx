import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createPortal } from "react-dom";
import { describe, expect, it, vi } from "vitest";

import { SectionCard } from "./section-card";

describe("SectionCard Kopfzeilen-Aktionen", () => {
  it("bricht Aktionen auf dem Telefon standardmäßig als eigene Zeile um", () => {
    render(
      <SectionCard
        title="Ablauf"
        actions={<button type="button">Öffnen</button>}
      >
        Inhalt
      </SectionCard>,
    );

    const wrapper = screen.getByRole("button", {
      name: "Öffnen",
    }).parentElement;
    expect(wrapper?.className).toContain("order-last");
    expect(wrapper?.className).toContain("w-full");
  });

  it("hält Aktionen mit inlineActions auf jeder Breite neben dem Titel", () => {
    render(
      <SectionCard
        title="Ablauf"
        inlineActions
        actions={<button type="button">Öffnen</button>}
      >
        Inhalt
      </SectionCard>,
    );

    const wrapper = screen.getByRole("button", {
      name: "Öffnen",
    }).parentElement;
    expect(wrapper?.className).toContain("shrink-0");
    expect(wrapper?.className).toContain("h-10");
    expect(wrapper?.className).not.toContain("order-last");
    expect(wrapper?.className).not.toContain("w-full");
  });
});

describe("SectionCard einklappbar: ganzer Kopf schaltet (#5021)", () => {
  function renderCollapsible(
    props: Partial<Parameters<typeof SectionCard>[0]> = {},
  ) {
    return render(
      <SectionCard
        collapsible
        defaultCollapsed
        title="Ankunfts- und Abholzeiten"
        titleBadge={<span>1 geändert</span>}
        description="Wann Kinder frühestens kommen"
        {...props}
      >
        Inhalt der Karte
      </SectionCard>,
    );
  }

  it("hat genau einen Knopf mit aria-expanded und behält die Überschrift", () => {
    renderCollapsible({ headingLevel: 3 });

    const toggles = screen
      .getAllByRole("button")
      .filter((button) => button.hasAttribute("aria-expanded"));
    expect(toggles).toHaveLength(1);
    expect(toggles[0]).toHaveAccessibleName(
      "Ankunfts- und Abholzeiten ausklappen",
    );
    expect(
      screen.getByRole("heading", {
        level: 3,
        name: "Ankunfts- und Abholzeiten",
      }),
    ).toBeInTheDocument();
  });

  it("klappt per Klick auf Titel, Abzeichen und Beschreibung auf und zu", async () => {
    const user = userEvent.setup();
    renderCollapsible();

    await user.click(screen.getByText("Ankunfts- und Abholzeiten"));
    const toggle = screen.getByRole("button", {
      name: "Ankunfts- und Abholzeiten einklappen",
    });
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    const body = screen.getByText("Inhalt der Karte");
    expect(toggle).toHaveAttribute("aria-controls", body.id);

    await user.click(screen.getByText("1 geändert"));
    expect(
      screen.getByRole("button", {
        name: "Ankunfts- und Abholzeiten ausklappen",
      }),
    ).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("Inhalt der Karte")).not.toBeInTheDocument();

    await user.click(screen.getByText("Wann Kinder frühestens kommen"));
    expect(screen.getByText("Inhalt der Karte")).toBeInTheDocument();
  });

  it("meldet im kontrollierten Modus jeden Klick genau einmal", async () => {
    const user = userEvent.setup();
    const onCollapsedChange = vi.fn();
    renderCollapsible({ collapsed: true, onCollapsedChange });

    await user.click(screen.getByText("Ankunfts- und Abholzeiten"));
    expect(onCollapsedChange).toHaveBeenCalledTimes(1);
    expect(onCollapsedChange).toHaveBeenLastCalledWith(false);

    await user.click(
      screen.getByRole("button", {
        name: "Ankunfts- und Abholzeiten ausklappen",
      }),
    );
    expect(onCollapsedChange).toHaveBeenCalledTimes(2);
  });

  it("schaltet per Enter und Leertaste auf dem fokussierten Kopf", async () => {
    const user = userEvent.setup();
    renderCollapsible();

    await user.tab();
    const toggle = screen.getByRole("button", {
      name: "Ankunfts- und Abholzeiten ausklappen",
    });
    expect(toggle).toHaveFocus();

    await user.keyboard("{Enter}");
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    await user.keyboard(" ");
    expect(toggle).toHaveAttribute("aria-expanded", "false");
  });

  it("lässt Kopf-Aktionen klicken, ohne umzuschalten", async () => {
    const user = userEvent.setup();
    const onAction = vi.fn();
    renderCollapsible({
      actions: (
        <button type="button" onClick={onAction}>
          Abrechnung
        </button>
      ),
    });

    await user.click(screen.getByRole("button", { name: "Abrechnung" }));
    expect(onAction).toHaveBeenCalledTimes(1);
    expect(
      screen.getByRole("button", {
        name: "Ankunfts- und Abholzeiten ausklappen",
      }),
    ).toHaveAttribute("aria-expanded", "false");
  });

  it("schaltet nicht bei Klicks in einem Portal einer Kopf-Aktion", async () => {
    const user = userEvent.setup();
    renderCollapsible({
      actions: createPortal(<p>Menütext</p>, document.body),
    });

    await user.click(screen.getByText("Menütext"));
    expect(
      screen.getByRole("button", {
        name: "Ankunfts- und Abholzeiten ausklappen",
      }),
    ).toHaveAttribute("aria-expanded", "false");
  });

  it("macht einen nicht einklappbaren Kopf nicht klickbar", async () => {
    const user = userEvent.setup();
    render(
      <SectionCard title="Ablauf" description="Nur zur Information">
        Inhalt
      </SectionCard>,
    );

    await user.click(screen.getByText("Ablauf"));
    expect(screen.getByText("Inhalt")).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});

describe("SectionCard Überschrift", () => {
  it("benennt die Karte mit namedRegion nach ihrem Titel auf der gewählten Ebene", () => {
    render(
      <SectionCard title="Rückmeldung" headingLevel={4} namedRegion>
        Inhalt
      </SectionCard>,
    );

    expect(
      screen.getByRole("heading", { level: 4, name: "Rückmeldung" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("region", { name: "Rückmeldung" }),
    ).toBeInTheDocument();
  });

  it("bleibt ohne namedRegion eine unbenannte Fläche", () => {
    render(<SectionCard title="Ablauf">Inhalt</SectionCard>);

    // Anker: die Karte ist gerendert.
    expect(
      screen.getByRole("heading", { level: 2, name: "Ablauf" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("region")).not.toBeInTheDocument();
  });
});
