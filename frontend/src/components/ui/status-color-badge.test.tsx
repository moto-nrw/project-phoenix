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

  it.each([null, undefined])(
    "fällt bei fehlender Farbe neutral zurück (%s)",
    (color) => {
      render(<StatusColorBadge label="Keine Farbe" color={color} />);

      const pill = screen.getByText("Keine Farbe");
      const tone = getLocationBadgeTone(color);
      expect(pill.style.backgroundColor).toBe(tone.backgroundColor);
      expect(pill.style.color).toBe(tone.textColor);
    },
  );

  it("passt in dichte Karten wie StatusBadge compact", () => {
    render(
      <>
        <StatusColorBadge
          label="Keine Daten"
          color={LOCATION_COLORS.OTHER_ROOM}
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

  // #5617: „Schulhof“ und „Achtung“ standen als fast gleiche Pillen in einer
  // Liste. Die Achtung-Pille unterscheidet sich jetzt in Tönung und Symbol.
  it("unterscheidet die Achtung-Pille von der Schulhof-Pille", () => {
    render(
      <>
        <StatusColorBadge label="Schulhof" color={LOCATION_COLORS.SCHOOLYARD} />
        <StatusColorBadge
          label="Anfrage offen"
          color={LOCATION_COLORS.WARNING}
        />
      </>,
    );
    const schoolyard = screen.getByText("Schulhof");
    const warning = screen.getByText("Anfrage offen");
    expect(warning.style.backgroundColor).not.toBe(
      schoolyard.style.backgroundColor,
    );
    expect(warning.style.color).not.toBe(schoolyard.style.color);
    expect(warning.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
    // Anker: getByText findet die Schulhof-Pille selbst.
    expect(schoolyard.querySelector("svg")).toBeNull();
  });

  it.each([
    ["compact", { compact: true }],
    ["xs", { size: "xs" as const }],
  ])("hält die Achtung-Pille %s so dicht wie StatusBadge", (_, props) => {
    render(
      <>
        <StatusColorBadge
          label="Keine Daten"
          color={LOCATION_COLORS.WARNING}
          {...props}
        />
        <StatusBadge label="Offen" tone="warning" {...props} />
      </>,
    );
    expect(screen.getByText("Keine Daten").className).toBe(
      screen.getByText("Offen").className,
    );
  });

  it("zeichnet die Achtung-Pille wie StatusBadge tone=warning", () => {
    render(
      <>
        <StatusColorBadge label="Fehlt" color={LOCATION_COLORS.WARNING} />
        <StatusBadge label="Offen" tone="warning" />
      </>,
    );
    const a = screen.getByText("Fehlt");
    const b = screen.getByText("Offen");
    expect(a.style.cssText).toBe(b.style.cssText);
    expect(a.className).toBe(b.className);
  });
});
