import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { StatusBadge } from "./status-badge";
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

  it("legt data-Attribute auf die Pille selbst", () => {
    render(
      <StatusColorBadge
        label="Schulhof"
        color={LOCATION_COLORS.SCHOOLYARD}
        data-location-status="Schulhof"
      />,
    );
    expect(screen.getByText("Schulhof")).toHaveAttribute(
      "data-location-status",
      "Schulhof",
    );
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

  it("passt in dichte Karten wie StatusBadge compact", () => {
    render(
      <>
        <StatusColorBadge
          label="Keine Daten"
          color={LOCATION_COLORS.WARNING}
          compact
        />
        <StatusBadge label="3 Std." tone="green" compact />
      </>,
    );
    expect(screen.getByText("Keine Daten").className).toBe(
      screen.getByText("3 Std.").className,
    );
  });

  it("lässt eine lange Beschriftung auf Wunsch umbrechen", () => {
    render(
      <StatusColorBadge
        label="Konto aktiv, kein Portalzugriff"
        color={LOCATION_COLORS.SCHOOLYARD}
        wrap
      />,
    );
    expect(screen.getByText("Konto aktiv, kein Portalzugriff")).toHaveClass(
      "whitespace-normal",
    );
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
