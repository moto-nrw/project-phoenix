import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { StatusDotBadge } from "./status-dot-badge";
import { LOCATION_COLORS } from "~/lib/location-helper";

describe("StatusDotBadge", () => {
  it("rendert die Beschriftung", () => {
    render(
      <StatusDotBadge label="Stammdaten" color={LOCATION_COLORS.OTHER_ROOM} />,
    );
    expect(screen.getByText("Stammdaten")).toBeInTheDocument();
  });

  it("rendert einen dekorativen Punkt, den Hilfstechnik überspringt", () => {
    const { container } = render(
      <StatusDotBadge label="Entschuldigung" color={LOCATION_COLORS.EXCUSED} />,
    );
    expect(container.querySelector("span[aria-hidden='true']")).not.toBeNull();
  });

  it("lässt den Punkt auf Wunsch weg", () => {
    const { container } = render(
      <StatusDotBadge
        label="Entschuldigung"
        color={LOCATION_COLORS.EXCUSED}
        showDot={false}
      />,
    );
    expect(container.querySelector("span[aria-hidden='true']")).toBeNull();
    // Ohne Punkt trägt die Beschriftung die Farbe allein.
    expect(screen.getByText("Entschuldigung")).toBeInTheDocument();
  });
});
