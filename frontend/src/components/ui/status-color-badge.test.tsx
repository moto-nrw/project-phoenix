import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { StatusColorBadge } from "./status-color-badge";
import { LOCATION_COLORS, getLocationBadgeTone } from "~/lib/location-helper";

describe("StatusColorBadge", () => {
  it("rendert die Beschriftung", () => {
    render(
      <StatusColorBadge
        label="Stammdaten"
        color={LOCATION_COLORS.OTHER_ROOM}
      />,
    );
    expect(screen.getByText("Stammdaten")).toBeInTheDocument();
  });

  it("färbt Fläche und Text über den Ton der Farbe", () => {
    render(
      <StatusColorBadge
        label="Entschuldigung"
        color={LOCATION_COLORS.EXCUSED}
      />,
    );
    const pill = screen.getByText("Entschuldigung");
    const tone = getLocationBadgeTone(LOCATION_COLORS.EXCUSED);
    expect(pill.style.backgroundColor).toBe(tone.backgroundColor);
    expect(pill.style.color).toBe(tone.textColor);
  });

  it("zeigt die Beschriftung ohne dekorativen Punkt davor (#2476)", () => {
    render(
      <StatusColorBadge
        label="Entschuldigung"
        color={LOCATION_COLORS.EXCUSED}
      />,
    );
    const pill = screen.getByText("Entschuldigung");
    expect(pill.children).toHaveLength(0);
    expect(pill.textContent).toBe("Entschuldigung");
    // Kein Abstand mehr, wo der Punkt saß.
    expect(pill.className).not.toMatch(/\bgap-/);
  });
});
