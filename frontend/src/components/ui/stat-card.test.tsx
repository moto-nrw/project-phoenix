import { render, screen } from "@testing-library/react";
import type { AnchorHTMLAttributes } from "react";
import { describe, expect, it, vi } from "vitest";

import { LOCATION_COLORS, getAccessibleTextColor } from "~/lib/location-helper";

import { StatCard } from "./stat-card";
import { TileCard } from "./tile-card";

vi.mock("~/components/ui/navigation-link", () => ({
  default: ({
    children,
    ...props
  }: AnchorHTMLAttributes<HTMLAnchorElement>) => <a {...props}>{children}</a>,
}));

/** Klassen eines Elements, die mit einem der Präfixe beginnen. */
function classesWith(element: Element, prefixes: readonly string[]) {
  return [...element.classList]
    .filter((name) => prefixes.some((prefix) => name.startsWith(prefix)))
    .sort();
}

describe("StatCard", () => {
  it("zeigt Beschriftung, Wert und Hinweis", () => {
    render(<StatCard label="Stunden" value="12h 30min" hint="Diese Woche" />);

    expect(screen.getByText("Stunden")).toBeInTheDocument();
    expect(screen.getByText("12h 30min")).toBeInTheDocument();
    expect(screen.getByText("Diese Woche")).toBeInTheDocument();
  });

  describe("Fortschrittsbalken", () => {
    it("meldet den Fortschritt als progressbar mit dem Namen der Kachel", () => {
      render(<StatCard label="Diese Woche" value="12h" progressPct={40} />);

      const bar = screen.getByRole("progressbar", { name: "Diese Woche" });
      expect(bar).toHaveAttribute("aria-valuenow", "40");
      expect(bar).toHaveAttribute("aria-valuemax", "100");
    });

    it("begrenzt Werte über 100 und rundet für die Vorlese-Ausgabe", () => {
      const { rerender } = render(
        <StatCard label="Woche" value="50h" progressPct={120} />,
      );
      expect(screen.getByRole("progressbar")).toHaveAttribute(
        "aria-valuenow",
        "100",
      );

      rerender(<StatCard label="Woche" value="1h" progressPct={33.333} />);
      expect(screen.getByRole("progressbar")).toHaveAttribute(
        "aria-valuenow",
        "33",
      );
    });

    it("zeigt ohne progressPct keinen Balken", () => {
      render(<StatCard label="Woche" value="1h" />);

      expect(screen.getByText("Woche")).toBeInTheDocument();
      expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    });
  });

  describe('variant="inline"', () => {
    it("zeigt Wert, Zusatz und Beschriftung", () => {
      render(
        <StatCard
          variant="inline"
          label="Kinder"
          value="12 von 20"
          detail="8 frei"
        />,
      );

      expect(screen.getByText("Kinder")).toBeInTheDocument();
      expect(screen.getByText("12 von 20", { exact: false })).toHaveTextContent(
        "12 von 20 · 8 frei",
      );
    });

    it("lässt den Trenner ohne Zusatz weg", () => {
      render(<StatCard variant="inline" label="Offen" value={3} />);

      expect(screen.getByText("3")).toHaveTextContent(/^3$/);
    });

    it("zeigt das Symbol, wenn eines übergeben wird", () => {
      render(
        <StatCard
          variant="inline"
          label="Offen"
          value={3}
          icon={<svg data-testid="symbol" />}
        />,
      );

      expect(screen.getByTestId("symbol")).toBeInTheDocument();
    });
  });

  describe('variant="tile"', () => {
    it("zeigt Wert und Beschriftung", () => {
      render(<StatCard variant="tile" label="Erwartet" value={8} />);

      expect(screen.getByText("8")).toBeInTheDocument();
      expect(screen.getByText("Erwartet")).toBeInTheDocument();
    });

    it("nimmt auch kurze Texte als Wert", () => {
      render(
        <StatCard variant="tile" label="Geht so nach Hause" value="Abholung" />,
      );

      expect(screen.getByText("Abholung")).toBeInTheDocument();
    });

    it("bleibt ohne tone neutral", () => {
      render(<StatCard variant="tile" label="Krank" value={0} />);

      expect(screen.getByText("0")).not.toHaveStyle({ color: "#B91C1C" });
    });

    it("färbt den Wert mit tone", () => {
      render(
        <StatCard variant="tile" label="Ohne Meldung" value={3} tone="red" />,
      );

      expect(screen.getByText("3")).toHaveStyle({
        color: getAccessibleTextColor(LOCATION_COLORS.DANGER),
      });
    });
  });

  describe("mit Link wie TileCard (#6565)", () => {
    function renderBoth() {
      render(
        <>
          <StatCard label="Kinder anwesend" value={12} href="/students" />
          <TileCard href="/rooms" ariaLabel="Räume">
            Räume
          </TileCard>
        </>,
      );
      const statLink = screen.getByRole("link", { name: /Kinder anwesend/ });
      const tileLink = screen.getByRole("link", { name: "Räume" });
      return { statLink, tileLink };
    }

    it("zeigt beim Tastaturfokus denselben Fokusring", () => {
      const { statLink, tileLink } = renderBoth();

      const focus = classesWith(tileLink, ["focus-visible:"]);
      expect(focus).not.toHaveLength(0);
      expect(classesWith(statLink, ["focus-visible:", "focus:"])).toEqual(
        focus,
      );
    });

    it("hebt sich beim Überfahren wie TileCard, ohne eigenes Verschieben", () => {
      const { statLink } = renderBoth();
      const surface = statLink.firstElementChild as Element;

      // Anker: die Fläche trägt die Hover-Hervorhebung von TileCard.
      expect(surface).toHaveClass("moto-hover-elevated");
      expect(classesWith(surface, ["group-hover:", "transition-"])).toEqual([]);
    });
  });

  it("hat auf dem Telefon den Innenabstand von TileCard, mit und ohne Link (#6565)", () => {
    render(
      <>
        <StatCard label="Kinder anwesend" value={12} href="/students" />
        <StatCard label="Auslastung" value="80 %" />
        <TileCard href="/rooms" ariaLabel="Räume">
          Räume
        </TileCard>
      </>,
    );
    const padding = ["p-", "max-sm:p-", "compact:p-"];
    const tile = classesWith(
      screen.getByRole("link", { name: "Räume" }),
      padding,
    );
    const linked = screen.getByRole("link", { name: /Kinder anwesend/ })
      .firstElementChild as Element;
    const unlinked = screen
      .getByText("Auslastung")
      .closest(".moto-content-surface") as Element;

    expect(tile).not.toHaveLength(0);
    expect(classesWith(linked, padding)).toEqual(tile);
    expect(classesWith(unlinked, padding)).toEqual(tile);
  });
});
