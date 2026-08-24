import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { StatCard } from "./stat-card";

describe("StatCard", () => {
  it("zeigt Beschriftung, Wert und Hinweis", () => {
    render(<StatCard label="Stunden" value="12h 30min" hint="Diese Woche" />);

    expect(screen.getByText("Stunden")).toBeInTheDocument();
    expect(screen.getByText("12h 30min")).toBeInTheDocument();
    expect(screen.getByText("Diese Woche")).toBeInTheDocument();
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
  });
});
