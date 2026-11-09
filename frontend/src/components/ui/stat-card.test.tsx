import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LOCATION_COLORS, getAccessibleTextColor } from "~/lib/location-helper";

import { StatCard } from "./stat-card";

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
});
