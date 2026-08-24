import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { StatTile } from "./stat-tile";

describe("StatTile", () => {
  it("zeigt Wert und Beschriftung", () => {
    render(<StatTile label="Erwartet" value={8} />);

    expect(screen.getByText("8")).toBeInTheDocument();
    expect(screen.getByText("Erwartet")).toBeInTheDocument();
  });

  it("nimmt auch kurze Texte als Wert", () => {
    render(<StatTile label="Geht so nach Hause" value="Abholung" />);

    expect(screen.getByText("Abholung")).toBeInTheDocument();
  });
});
